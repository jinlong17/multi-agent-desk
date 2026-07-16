package codex

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

type SessionConfig struct {
	SessionID            domain.ID
	AccountID            domain.ID
	CredentialInstanceID domain.ID
	RuntimeProfileID     domain.ID
	WorkspaceID          domain.ID
	ProviderVersion      string
	Capabilities         CapabilitySet
}

type ProviderSession struct {
	Client       *Client
	Config       SessionConfig
	StartedAt    time.Time
	ProviderTurn string
	stopped      bool
	mu           sync.Mutex
	pending      map[string]pendingApproval
}

type pendingApproval struct {
	RequestID json.RawMessage
	Method    string
}

func NewProviderSession(client *Client, config SessionConfig) (*ProviderSession, error) {
	if client == nil {
		return nil, domain.NewError(domain.CodeInvalidArgument, "codex session client is unavailable")
	}
	for _, id := range []domain.ID{config.SessionID, config.AccountID, config.CredentialInstanceID, config.RuntimeProfileID, config.WorkspaceID} {
		if err := domain.ValidateID(id); err != nil {
			return nil, err
		}
	}
	if config.ProviderVersion == "" || config.Capabilities.Provider != ProviderName || config.Capabilities.Status != CapabilitySupported {
		return nil, domain.NewError(domain.CodeProviderVersionUnsupported, "codex session compatibility is not verified")
	}
	return &ProviderSession{Client: client, Config: config, pending: make(map[string]pendingApproval)}, nil
}

func (s *ProviderSession) Start(ctx context.Context) error {
	if s == nil || s.Client == nil || ctx == nil {
		return domain.NewError(domain.CodeInvalidArgument, "codex session is incomplete")
	}
	if s.StartedAt.IsZero() {
		if err := s.Client.ConfigureMethods(s.Config.Capabilities.Methods); err != nil {
			return err
		}
		_, err := s.Client.Handshake(ctx, InitializeParams{ClientInfo: ClientInfo{Name: "multi-agent-desk", Version: "phase2"},
			Capabilities: &InitializeCapabilities{ExperimentalAPI: false}})
		if err != nil {
			return err
		}
		s.StartedAt = time.Now().UTC()
		return nil
	}
	return domain.NewError(domain.CodeConflict, "codex session is already started")
}

func (s *ProviderSession) ReadEvent(ctx context.Context) (ProviderEvent, error) {
	if s == nil || s.Client == nil || s.StartedAt.IsZero() || s.stopped || ctx == nil {
		return ProviderEvent{}, domain.NewError(domain.CodeProviderFailed, "codex session is not running")
	}
	frame, err := s.Client.ReadInbound(ctx)
	if err != nil {
		return ProviderEvent{}, err
	}
	var notification struct {
		JSONRPC string          `json:"jsonrpc,omitempty"`
		ID      json.RawMessage `json:"id,omitempty"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := DecodeObject(frame, &notification); err != nil {
		return ProviderEvent{}, err
	}
	if notification.Method == "" || len(notification.Params) == 0 {
		return ProviderEvent{}, domain.NewError(domain.CodeProviderProtocolError, "codex session event is incomplete")
	}
	event, err := MapEvent(notification.Method, notification.Params, s.Config.ProviderVersion, time.Now().UTC())
	if err != nil {
		return ProviderEvent{}, err
	}
	if event.ProviderApprovalID != "" {
		if len(notification.ID) == 0 {
			return ProviderEvent{}, domain.NewError(domain.CodeProviderProtocolError, "codex Approval is missing a server request id")
		}
		if !s.Config.Capabilities.Allows(notification.Method) {
			return ProviderEvent{}, domain.NewError(domain.CodeProviderVersionUnsupported, "codex Approval method is not enabled")
		}
		s.mu.Lock()
		if _, exists := s.pending[event.ProviderApprovalID]; exists {
			s.mu.Unlock()
			return ProviderEvent{}, domain.NewError(domain.CodeConflict, "codex Approval request is already pending")
		}
		s.pending[event.ProviderApprovalID] = pendingApproval{RequestID: append(json.RawMessage(nil), notification.ID...), Method: notification.Method}
		s.mu.Unlock()
		s.ProviderTurn = event.ProviderApprovalID
	} else if len(notification.ID) != 0 {
		return ProviderEvent{}, domain.NewError(domain.CodeProviderProtocolError, "codex server request method is unmapped")
	}
	return event, nil
}

func (s *ProviderSession) ReadAccount(ctx context.Context) (AccountSnapshot, error) {
	if err := s.requireRunning(MethodAccountRead); err != nil {
		return AccountSnapshot{}, err
	}
	var raw json.RawMessage
	if err := s.Client.Call(ctx, MethodAccountRead, map[string]any{}, &raw); err != nil {
		return AccountSnapshot{}, err
	}
	return DecodeAccountResponse(raw, time.Now().UTC())
}

func (s *ProviderSession) ReadUsage(ctx context.Context) (UsageProjection, error) {
	if err := s.requireRunning(MethodAccountUsage); err != nil {
		return UsageProjection{}, err
	}
	var raw json.RawMessage
	if err := s.Client.Call(ctx, MethodAccountUsage, map[string]any{}, &raw); err != nil {
		return UsageProjection{}, err
	}
	return DecodeUsageResponse(raw, s.Config.ProviderVersion, time.Now().UTC())
}

func (s *ProviderSession) RespondApproval(ctx context.Context, providerApprovalID, decision string) error {
	if s == nil || s.Client == nil || s.StartedAt.IsZero() || s.stopped {
		return domain.NewError(domain.CodeProviderFailed, "codex session is not running")
	}
	if strings.TrimSpace(providerApprovalID) == "" || len(providerApprovalID) > 256 {
		return domain.NewError(domain.CodeApprovalUnknown, "approval id is invalid")
	}
	decision = strings.TrimSpace(decision)
	if decision != "approved" && decision != "denied" {
		return domain.NewError(domain.CodeInvalidArgument, "approval decision is invalid")
	}
	s.mu.Lock()
	pending, ok := s.pending[providerApprovalID]
	if ok {
		delete(s.pending, providerApprovalID)
	}
	s.mu.Unlock()
	if !ok {
		return domain.NewError(domain.CodeApprovalUnknown, "codex Approval request is not pending")
	}
	if pending.Method == MethodApprovalPermissions {
		s.mu.Lock()
		s.pending[providerApprovalID] = pending
		s.mu.Unlock()
		return domain.NewError(domain.CodeProviderVersionUnsupported, "codex permissions Approval response is not mapped")
	}
	providerDecision := "accept"
	if decision == "denied" {
		providerDecision = "decline"
	}
	if err := s.Client.RespondServerRequest(ctx, pending.RequestID, map[string]any{"decision": providerDecision}); err != nil {
		s.mu.Lock()
		s.pending[providerApprovalID] = pending
		s.mu.Unlock()
		return err
	}
	return nil
}

func (s *ProviderSession) Stop(ctx context.Context) error {
	if s == nil || s.stopped {
		return nil
	}
	if !s.Config.Capabilities.Allows("session/stop") {
		s.stopped = true
		return domain.NewError(domain.CodeProviderUnsupported, "codex app-server stop method is not verified")
	}
	s.stopped = true
	return s.Client.Call(ctx, "session/stop", map[string]any{}, nil)
}

func (s *ProviderSession) Resume(_ context.Context) error {
	return domain.NewError(domain.CodeProviderResumeUnsupported, "codex Provider continuation is not verified")
}

func (s *ProviderSession) requireRunning(method string) error {
	if s == nil || s.Client == nil || s.StartedAt.IsZero() || s.stopped {
		return domain.NewError(domain.CodeProviderFailed, "codex session is not running")
	}
	if !s.Config.Capabilities.Allows(method) {
		return domain.NewError(domain.CodeProviderVersionUnsupported, "codex session method is not enabled")
	}
	return nil
}
