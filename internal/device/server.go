package device

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

type RequestHandler interface {
	Handle(context.Context, AuthContext, Request) (any, error)
}

type HandlerFunc func(context.Context, AuthContext, Request) (any, error)

func (f HandlerFunc) Handle(ctx context.Context, auth AuthContext, request Request) (any, error) {
	return f(ctx, auth, request)
}

type Server struct {
	Listener       Listener
	Authenticator  *ServerAuthenticator
	Authorizer     func(context.Context, AuthContext, string) error
	Handler        RequestHandler
	MaxConnections int

	closeOnce sync.Once
	activeMu  sync.Mutex
	active    map[io.ReadWriteCloser]struct{}
}

func (s *Server) Serve(ctx context.Context) error {
	if s.Listener == nil || s.Authenticator == nil || s.Handler == nil {
		return domain.NewError(domain.CodeInvalidArgument, "daemon server is incomplete")
	}
	s.activeMu.Lock()
	if s.active == nil {
		s.active = make(map[io.ReadWriteCloser]struct{})
	}
	s.activeMu.Unlock()
	maxConnections := s.MaxConnections
	if maxConnections <= 0 {
		maxConnections = 32
	}
	semaphore := make(chan struct{}, maxConnections)
	serveDone := make(chan struct{})
	defer close(serveDone)
	defer s.closeActive()
	go func() {
		select {
		case <-ctx.Done():
			_ = s.Close()
		case <-serveDone:
		}
	}()
	for {
		connection, err := s.Listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return domain.WrapError(domain.CodeDaemonUnavailable, "daemon endpoint accept failed", err)
		}
		select {
		case semaphore <- struct{}{}:
			s.track(connection)
			go func() { defer func() { s.untrack(connection); <-semaphore }(); _ = s.serveConnection(ctx, connection) }()
		default:
			_ = connection.Close()
		}
	}
}

func (s *Server) Close() error {
	var err error
	s.closeOnce.Do(func() {
		if s.Listener != nil {
			err = s.Listener.Close()
		}
		s.closeActive()
	})
	return err
}

func (s *Server) track(connection io.ReadWriteCloser) {
	s.activeMu.Lock()
	if s.active == nil {
		s.active = make(map[io.ReadWriteCloser]struct{})
	}
	s.active[connection] = struct{}{}
	s.activeMu.Unlock()
}
func (s *Server) untrack(connection io.ReadWriteCloser) {
	s.activeMu.Lock()
	delete(s.active, connection)
	s.activeMu.Unlock()
}
func (s *Server) closeActive() {
	s.activeMu.Lock()
	connections := make([]io.ReadWriteCloser, 0, len(s.active))
	for connection := range s.active {
		connections = append(connections, connection)
	}
	s.activeMu.Unlock()
	for _, connection := range connections {
		_ = connection.Close()
	}
}

func (s *Server) serveConnection(ctx context.Context, connection io.ReadWriteCloser) error {
	defer connection.Close()
	auth, err := s.Authenticator.Handshake(ctx, connection)
	if err != nil {
		return err
	}
	seenRequestIDs := make(map[string]struct{}, 128)
	for {
		if err := setContextDeadline(connection, ctx, 5*time.Minute); err != nil {
			return err
		}
		body, err := readFrame(connection)
		if err != nil {
			return err
		}
		var request Request
		if err := decodeStrict(body, &request); err != nil {
			return writeErrorResponse(connection, "", domain.NewError(domain.CodeInvalidArgument, "request is invalid"))
		}
		if err := validateRequest(request, seenRequestIDs); err != nil {
			_ = writeErrorResponse(connection, request.RequestID, err)
			return err
		}
		if s.Authorizer != nil {
			if err := s.Authorizer(ctx, auth, request.Method); err != nil {
				if writeErr := writeErrorResponse(connection, request.RequestID, err); writeErr != nil {
					return writeErr
				}
				continue
			}
		}
		result, err := s.Handler.Handle(ctx, auth, request)
		if err != nil {
			if writeErr := writeErrorResponse(connection, request.RequestID, err); writeErr != nil {
				return writeErr
			}
			continue
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return writeErrorResponse(connection, request.RequestID, domain.NewError(domain.CodeConflict, "response could not be encoded"))
		}
		if err := writeFrame(connection, Response{ProtocolMajor: ProtocolMajor, RequestID: request.RequestID, OK: true, Result: encoded}); err != nil {
			return err
		}
	}
}

func validateRequest(request Request, seen map[string]struct{}) error {
	if request.ProtocolMajor != ProtocolMajor {
		return domain.NewError(domain.CodeUnsupportedVersion, "protocol version is unsupported")
	}
	if len(request.RequestID) == 0 || len(request.RequestID) > MaxRequestIDBytes || len(request.Method) == 0 || len(request.Method) > MaxMethodBytes || request.Body != nil && len(request.Body) > MaxFrameBytes {
		return domain.NewError(domain.CodeInvalidArgument, "request is invalid")
	}
	if _, exists := seen[request.RequestID]; exists {
		return domain.NewError(domain.CodeConflict, "request ID was reused")
	}
	if len(seen) >= 128 {
		return domain.NewError(domain.CodeResourceExhausted, "connection request limit reached")
	}
	seen[request.RequestID] = struct{}{}
	return nil
}

func writeErrorResponse(connection io.Writer, requestID string, err error) error {
	code := domain.CodeConflict
	message := "request failed"
	if stable := domain.CodeOf(err); stable != "" {
		code = stable
	}
	if safe, ok := err.(*domain.Error); ok {
		message = safe.Message
	}
	return writeFrame(connection, Response{ProtocolMajor: ProtocolMajor, RequestID: requestID, OK: false, Error: &WireError{Code: code, Message: message}})
}
