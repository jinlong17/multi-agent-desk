package controlplane

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

var BuildVersion = "devel"
var BuildCommit = "unknown"

type Server struct {
	config          Config
	store           *Store
	http            *http.Server
	ready           atomic.Bool
	webauthn        *WebAuthnService
	bootstrap       *BootstrapService
	auth            *AuthService
	recoveryLimiter *RecoveryLimiter
	preAuthLimiter  *RequestLimiter
	ceremonyLimiter *RequestLimiter
}

func NewServer(config Config, store *Store) (*Server, error) {
	if store == nil {
		return nil, fmt.Errorf("control-plane store is required")
	}
	webauthnService, err := NewWebAuthnService(config, store)
	if err != nil {
		return nil, fmt.Errorf("configure WebAuthn: %w", err)
	}
	if err := webauthnService.Ceremonies.InvalidateAll(context.Background()); err != nil {
		return nil, err
	}
	server := &Server{
		config: config, store: store, recoveryLimiter: &RecoveryLimiter{},
		preAuthLimiter:  &RequestLimiter{PerSource: 30, Global: 300},
		ceremonyLimiter: &RequestLimiter{PerSource: 30, Global: 300},
		webauthn:        webauthnService,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/healthz", server.health)
	mux.HandleFunc("GET /v1/readyz", server.readiness)
	mux.HandleFunc("GET /v1/version", server.version)
	server.bootstrap = &BootstrapService{Config: config, Store: store, WebAuthn: webauthnService}
	server.auth = &AuthService{Config: config, Store: store, WebAuthn: webauthnService}
	server.mountP2(mux)
	mux.HandleFunc("/", func(writer http.ResponseWriter, _ *http.Request) {
		safeError(writer, http.StatusNotFound, "not_found", "endpoint not found", writer.Header().Get("X-Request-ID"))
	})
	server.http = &http.Server{Addr: config.Listen, Handler: server.middleware(mux), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	server.ready.Store(true)
	return server, nil
}

func (s *Server) Run(ctx context.Context) error {
	if ctx.Err() != nil {
		s.ready.Store(false)
		return nil
	}
	result := make(chan error, 1)
	go func() { result <- s.http.ListenAndServeTLS(s.config.TLSCertificateFile, s.config.TLSPrivateKeyFile) }()
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTPS: %w", err)
	case <-ctx.Done():
		s.ready.Store(false)
		shutdownContext, cancel := context.WithTimeout(context.Background(), s.config.shutdownTimeout)
		defer cancel()
		if err := s.http.Shutdown(shutdownContext); err != nil {
			_ = s.http.Close()
			return fmt.Errorf("shutdown HTTPS server: %w", err)
		}
		if err := <-result; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTPS: %w", err)
		}
		return nil
	}
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID, err := transport.NewUUIDv7()
		if err != nil {
			http.Error(writer, "internal error", http.StatusInternalServerError)
			return
		}
		writer.Header().Set("X-Request-ID", requestID)
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		if encoding := request.Header.Get("Content-Encoding"); encoding != "" && encoding != "identity" {
			safeError(writer, http.StatusUnsupportedMediaType, "invalid_argument", "content encoding is not supported", requestID)
			return
		}
		headerCount, headerBytes := 0, 0
		for name, values := range request.Header {
			headerCount += len(values)
			headerBytes += len(name)
			for _, value := range values {
				headerBytes += len(value)
			}
		}
		if headerCount > 64 || headerBytes > 32<<10 {
			safeError(writer, http.StatusRequestHeaderFieldsTooLarge, "request_too_large", "request headers exceed the limit", requestID)
			return
		}
		if len(request.URL.Query()) > 16 {
			safeError(writer, http.StatusBadRequest, "invalid_argument", "too many query parameters", requestID)
			return
		}
		for name, values := range request.URL.Query() {
			if len(values) != 1 || len(name) > 128 || len(values[0]) > 2048 {
				safeError(writer, http.StatusBadRequest, "invalid_argument", "query parameters are invalid", requestID)
				return
			}
		}
		if request.Method == http.MethodGet && (request.ContentLength > 0 || len(request.TransferEncoding) != 0) {
			safeError(writer, http.StatusBadRequest, "invalid_argument", "GET endpoints do not accept bodies", requestID)
			return
		}
		deadlineContext, cancel := context.WithTimeout(request.Context(), 30*time.Second)
		defer cancel()
		next.ServeHTTP(writer, request.WithContext(deadlineContext))
	})
}

func (s *Server) health(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{"apiVersion": "v1", "data": map[string]any{"status": "ok"}, "meta": map[string]any{"requestId": writer.Header().Get("X-Request-ID"), "nextCursor": nil}})
}

func (s *Server) readiness(writer http.ResponseWriter, request *http.Request) {
	if !s.ready.Load() {
		safeError(writer, http.StatusServiceUnavailable, "daemon_unavailable", "server is not ready", writer.Header().Get("X-Request-ID"))
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), time.Second)
	defer cancel()
	if err := s.store.Ready(ctx); err != nil {
		safeError(writer, http.StatusServiceUnavailable, "daemon_unavailable", "server is not ready", writer.Header().Get("X-Request-ID"))
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"apiVersion": "v1", "data": map[string]any{"status": "ready", "database": "ready"}, "meta": map[string]any{"requestId": writer.Header().Get("X-Request-ID"), "nextCursor": nil}})
}

func (s *Server) version(writer http.ResponseWriter, _ *http.Request) {
	features := []string{"foundation"}
	if s.auth != nil {
		features = append(features, "bootstrap", "passkey", "recovery", "browser-session")
	}
	writeJSON(writer, http.StatusOK, map[string]any{"apiVersion": "v1", "data": map[string]any{"version": boundedBuildValue(BuildVersion), "commit": boundedBuildValue(BuildCommit), "minimumClientProtocol": "v1", "enabledFeatures": features}, "meta": map[string]any{"requestId": writer.Header().Get("X-Request-ID"), "nextCursor": nil}})
}

func boundedBuildValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	if len(value) > 64 {
		return value[:64]
	}
	return value
}
func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		slog.Error("write bounded JSON response", "error", err)
	}
}
func safeError(writer http.ResponseWriter, status int, code, message, requestID string) {
	if len(message) > 256 {
		message = message[:256]
	}
	writeJSON(writer, status, map[string]any{"apiVersion": "v1", "error": map[string]any{"code": code, "message": message, "requestId": requestID, "details": []any{}}})
}
