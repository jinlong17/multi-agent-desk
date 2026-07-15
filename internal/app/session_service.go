package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/device"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/runtime"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
)

type SessionService struct {
	Store   *storage.Store
	Runtime *runtime.Manager
	Now     func() time.Time
	mu      sync.Mutex
}

func NewSessionService(store *storage.Store, manager *runtime.Manager) *SessionService {
	return &SessionService{Store: store, Runtime: manager, Now: func() time.Time { return time.Now().UTC() }}
}

func (s *SessionService) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *SessionService) Handle(ctx context.Context, auth device.AuthContext, request device.Request) (any, error) {
	if s == nil || s.Store == nil || s.Runtime == nil {
		return nil, domain.NewError(domain.CodeInvalidArgument, "session service is incomplete")
	}
	if requiresIdempotency(request.Method) {
		if request.IdempotencyKey == "" {
			return nil, domain.NewError(domain.CodeInvalidArgument, "idempotency key is required")
		}
		return s.withIdempotency(ctx, auth.ClientID, request, func() (any, error) {
			return s.dispatch(ctx, auth, request)
		})
	}
	return s.dispatch(ctx, auth, request)
}

func requiresIdempotency(method string) bool {
	switch method {
	case "sessions.start", "sessions.attach", "sessions.detach", "control.acquire", "terminal.input", "terminal.resize", "sessions.stop", "sessions.kill", "sessions.resume":
		return true
	default:
		return false
	}
}

func (s *SessionService) withIdempotency(ctx context.Context, clientID domain.ID, request device.Request, fn func() (any, error)) (any, error) {
	digest := sha256.Sum256(append([]byte(request.Method+"\x00"+request.IdempotencyKey+"\x00"), request.Body...))
	digestText := hex.EncodeToString(digest[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.Store.IdempotencyRecord(ctx, clientID, request.Method, request.IdempotencyKey)
	if err == nil {
		if record.RequestDigest != digestText {
			return nil, domain.NewError(domain.CodeConflict, "idempotency key was reused with a different request")
		}
		return json.RawMessage(record.ResponseMetadata), nil
	}
	if domain.CodeOf(err) != domain.CodeNotFound {
		return nil, err
	}
	result, err := fn()
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) > device.MaxFrameBytes {
		return nil, domain.NewError(domain.CodeFrameTooLarge, "idempotent response exceeds limit")
	}
	if err := s.Store.SaveIdempotencyRecord(ctx, storage.IdempotencyRecord{ClientID: clientID, Method: request.Method,
		IdempotencyKey: request.IdempotencyKey, RequestDigest: digestText, ResponseCode: "ok",
		ResponseMetadata: encoded, CreatedAt: s.now()}); err != nil {
		// A concurrent writer may have committed the same key. Re-read and
		// compare instead of ever returning an unverified duplicate result.
		if domain.CodeOf(err) == domain.CodeConflict {
			stored, readErr := s.Store.IdempotencyRecord(ctx, clientID, request.Method, request.IdempotencyKey)
			if readErr == nil && stored.RequestDigest == digestText {
				return json.RawMessage(stored.ResponseMetadata), nil
			}
		}
		return nil, err
	}
	return result, nil
}

func (s *SessionService) dispatch(ctx context.Context, auth device.AuthContext, request device.Request) (any, error) {
	switch request.Method {
	case "daemon.status":
		return map[string]any{"status": "ok", "schema_version": 1}, nil
	case "sessions.list":
		sessions, err := s.Store.ListSessions(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]any, 0, len(sessions))
		for _, session := range sessions {
			result = append(result, sessionView(session))
		}
		return result, nil
	case "sessions.show":
		var body sessionBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		session, err := s.Store.Session(ctx, body.SessionID)
		if err != nil {
			return nil, err
		}
		return sessionView(session), nil
	case "sessions.start":
		var body startBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		session, err := s.Runtime.StartFake(ctx, runtime.StartRequest{DeviceID: body.DeviceID,
			CredentialInstanceID: body.CredentialInstanceID, RuntimeProfileID: body.RuntimeProfileID,
			WorkspaceID: body.WorkspaceID, Capabilities: body.Capabilities})
		if err != nil {
			return nil, err
		}
		return sessionView(session), nil
	case "sessions.attach":
		var body attachBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		mode := domain.AttachmentObserver
		if body.Mode != "" {
			mode = domain.AttachmentMode(body.Mode)
		}
		if err := s.Runtime.Attach(ctx, body.SessionID, auth.ClientID, mode); err != nil {
			return nil, err
		}
		return map[string]any{"session_id": body.SessionID, "client_id": auth.ClientID, "mode": mode}, nil
	case "sessions.detach":
		var body sessionBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		if err := s.Runtime.Detach(ctx, body.SessionID, auth.ClientID); err != nil {
			return nil, err
		}
		return map[string]any{"session_id": body.SessionID, "client_id": auth.ClientID, "detached": true}, nil
	case "sessions.observe":
		var body observeBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		replay, replayErr := s.Runtime.Replay(ctx, body.SessionID, body.FromSequence)
		result := map[string]any{"session_id": body.SessionID, "next_sequence": replay.NextSequence, "truncated": replay.Truncated, "chunks": replay.Chunks}
		if replayErr != nil && domain.CodeOf(replayErr) != domain.CodeReplayUnavailable {
			return nil, replayErr
		}
		return result, nil
	case "control.acquire":
		var body sessionBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		lease, err := s.Runtime.Acquire(ctx, body.SessionID, auth.ClientID)
		if err != nil {
			return nil, err
		}
		return leaseView(lease), nil
	case "control.heartbeat":
		var body sessionBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		revision, err := requestRevision(request)
		if err != nil {
			return nil, err
		}
		lease, err := s.Runtime.Heartbeat(ctx, body.SessionID, auth.ClientID, revision)
		if err != nil {
			return nil, err
		}
		return leaseView(lease), nil
	case "control.release":
		var body sessionBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		revision, err := requestRevision(request)
		if err != nil {
			return nil, err
		}
		lease, err := s.Runtime.Release(ctx, body.SessionID, auth.ClientID, revision)
		if err != nil {
			return nil, err
		}
		return leaseView(lease), nil
	case "terminal.input":
		var body inputBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		revision, err := requestRevision(request)
		if err != nil {
			return nil, err
		}
		return s.Runtime.Input(ctx, runtime.InputRequest{SessionID: body.SessionID, ClientID: auth.ClientID,
			Revision: revision, Sequence: body.Sequence, Payload: body.Payload})
	case "terminal.resize":
		var body resizeBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		revision, err := requestRevision(request)
		if err != nil {
			return nil, err
		}
		if err := s.Runtime.Resize(ctx, runtime.ResizeRequest{SessionID: body.SessionID, ClientID: auth.ClientID,
			Revision: revision, Rows: body.Rows, Cols: body.Cols}); err != nil {
			return nil, err
		}
		return map[string]any{"session_id": body.SessionID, "rows": body.Rows, "cols": body.Cols}, nil
	case "sessions.stop", "sessions.kill":
		var body sessionBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		revision, err := requestRevision(request)
		if err != nil {
			return nil, err
		}
		var session domain.Session
		if request.Method == "sessions.stop" {
			session, err = s.Runtime.Stop(ctx, body.SessionID, auth.ClientID, revision)
		} else {
			session, err = s.Runtime.Kill(ctx, body.SessionID, auth.ClientID, revision)
		}
		if err != nil {
			return nil, err
		}
		return sessionView(session), nil
	case "sessions.resume":
		var body sessionBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		session, err := s.Runtime.Resume(ctx, body.SessionID)
		if err != nil {
			return nil, err
		}
		return sessionView(session), nil
	default:
		return nil, domain.NewError(domain.CodeMethodNotFound, "method is not available")
	}
}

type sessionBody struct {
	SessionID domain.ID `json:"session_id"`
}
type observeBody struct {
	SessionID    domain.ID `json:"session_id"`
	FromSequence int64     `json:"from_sequence,omitempty"`
}
type attachBody struct {
	SessionID domain.ID `json:"session_id"`
	Mode      string    `json:"mode,omitempty"`
}
type startBody struct {
	DeviceID             domain.ID           `json:"device_id"`
	CredentialInstanceID domain.ID           `json:"credential_instance_id"`
	RuntimeProfileID     domain.ID           `json:"runtime_profile_id"`
	WorkspaceID          domain.ID           `json:"workspace_id"`
	Capabilities         []domain.Capability `json:"capabilities"`
}
type inputBody struct {
	SessionID domain.ID `json:"session_id"`
	Sequence  int64     `json:"sequence"`
	Payload   string    `json:"payload"`
}
type resizeBody struct {
	SessionID domain.ID `json:"session_id"`
	Rows      int       `json:"rows"`
	Cols      int       `json:"cols"`
}

func decodeBody(body json.RawMessage, target any) error {
	if len(body) == 0 {
		return domain.NewError(domain.CodeInvalidArgument, "request body is required")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return domain.NewError(domain.CodeInvalidArgument, "request body is invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return domain.NewError(domain.CodeInvalidArgument, "request body is invalid")
	}
	return nil
}

func requestRevision(request device.Request) (int64, error) {
	if request.LeaseRevision == nil || *request.LeaseRevision < 1 {
		return 0, domain.NewError(domain.CodeInvalidArgument, "lease revision is required")
	}
	return *request.LeaseRevision, nil
}

func sessionView(session domain.Session) map[string]any {
	return map[string]any{"id": session.ID, "device_id": session.DeviceID, "provider": session.Provider,
		"credential_instance_id": session.CredentialInstanceID, "runtime_profile_id": session.RuntimeProfileID,
		"workspace_id": session.WorkspaceID, "provider_session_id": session.ProviderSessionID,
		"resumed_from_session_id": session.ResumedFromSessionID, "status": session.Status,
		"started_at": session.StartedAt, "ended_at": session.EndedAt, "exit_code": session.ExitCode,
		"capability_snapshot": session.CapabilitySnapshot, "failure_code": session.FailureCode}
}

func leaseView(lease domain.ControllerLease) map[string]any {
	return map[string]any{"session_id": lease.SessionID, "holder_device_id": lease.HolderDeviceID,
		"revision": lease.Revision, "expires_at": lease.ExpiresAt, "last_heartbeat_at": lease.LastHeartbeat,
		"released_at": lease.ReleasedAt}
}
