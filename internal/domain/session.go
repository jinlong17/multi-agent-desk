package domain

import "time"

type SessionStatus string

const (
	SessionStarting SessionStatus = "starting"
	SessionRunning  SessionStatus = "running"
	SessionStopping SessionStatus = "stopping"
	SessionExited   SessionStatus = "exited"
	SessionFailed   SessionStatus = "failed"
	SessionKilled   SessionStatus = "killed"
)

type Session struct {
	ID                   ID
	AccountID            ID
	DeviceID             ID
	AccountID            ID
	Provider             string
	CredentialInstanceID ID
	RuntimeProfileID     ID
	WorkspaceID          ID
	ProviderSessionID    string
	ResumedFromSessionID ID
	Status               SessionStatus
	StartedAt            time.Time
	EndedAt              *time.Time
	ExitCode             *int
	CapabilitySnapshot   []Capability
	FailureCode          string
}

var sessionTransitions = map[SessionStatus]map[SessionStatus]struct{}{
	SessionStarting: {
		SessionRunning: {},
		SessionFailed:  {},
		SessionKilled:  {},
	},
	SessionRunning: {
		SessionStopping: {},
		SessionFailed:   {},
		SessionKilled:   {},
	},
	SessionStopping: {
		SessionExited: {},
		SessionFailed: {},
		SessionKilled: {},
	},
}

func (s SessionStatus) Terminal() bool {
	return s == SessionExited || s == SessionFailed || s == SessionKilled
}

// NewSession validates and freezes the fields that cannot change after start.
func NewSession(session Session) (Session, error) {
	for _, id := range []ID{session.ID, session.DeviceID, session.CredentialInstanceID, session.RuntimeProfileID, session.WorkspaceID} {
		if err := ValidateID(id); err != nil {
			return Session{}, err
		}
	}
	if session.AccountID != "" {
		if err := ValidateID(session.AccountID); err != nil {
			return Session{}, err
		}
	}
	if session.Provider == "" || session.StartedAt.IsZero() {
		return Session{}, NewError(CodeInvalidArgument, "session requires provider and start time")
	}
	if len(session.ProviderSessionID) > 256 {
		return Session{}, NewError(CodeInvalidArgument, "provider session identity is too large")
	}
	if session.Provider == ProviderCodex && session.AccountID == "" {
		return Session{}, NewError(CodeInvalidArgument, "codex session requires an account")
	}
	if !ProviderKnown(session.Provider) {
		return Session{}, NewError(CodeInvalidArgument, "session provider is unsupported")
	}
	if session.ResumedFromSessionID != "" {
		if err := ValidateID(session.ResumedFromSessionID); err != nil {
			return Session{}, err
		}
	}
	if session.Status == "" {
		session.Status = SessionStarting
	}
	if session.Status != SessionStarting {
		return Session{}, NewError(CodeInvalidTransition, "new session must start in starting state")
	}
	capabilities, err := CanonicalCapabilities(session.CapabilitySnapshot)
	if err != nil {
		return Session{}, err
	}
	if session.ResumedFromSessionID != "" && !HasCapability(capabilities, CapabilitySessionResume) {
		return Session{}, NewError(CodePermissionDenied, "session resume capability is required")
	}
	session.CapabilitySnapshot = capabilities
	session.StartedAt = session.StartedAt.UTC()
	session.EndedAt = nil
	session.ExitCode = nil
	session.FailureCode = ""
	return session, nil
}

// Transition applies one legal state edge. Terminal Sessions are immutable.
func (s *Session) Transition(next SessionStatus, at time.Time, exitCode *int, failureCode string) error {
	if s == nil || at.IsZero() {
		return NewError(CodeInvalidArgument, "session transition requires a timestamp")
	}
	allowed, ok := sessionTransitions[s.Status]
	if !ok {
		return NewError(CodeInvalidTransition, "terminal session cannot transition")
	}
	if _, ok := allowed[next]; !ok {
		return NewError(CodeInvalidTransition, "illegal session transition")
	}
	if at.Before(s.StartedAt) {
		return NewError(CodeInvalidArgument, "session transition precedes start")
	}
	if next == SessionFailed && failureCode == "" {
		return NewError(CodeInvalidArgument, "failed session requires a safe failure code")
	}
	if !next.Terminal() && (exitCode != nil || failureCode != "") {
		return NewError(CodeInvalidArgument, "non-terminal session cannot contain exit metadata")
	}

	s.Status = next
	if next.Terminal() {
		endedAt := at.UTC()
		s.EndedAt = &endedAt
		s.ExitCode = exitCode
		s.FailureCode = failureCode
	}
	return nil
}

// Resume creates a new starting Session and never mutates the source record.
func (s Session) Resume(newID ID, at time.Time) (Session, error) {
	if !s.Status.Terminal() {
		return Session{}, NewError(CodeInvalidTransition, "only a terminal session can be resumed")
	}
	if !HasCapability(s.CapabilitySnapshot, CapabilitySessionResume) {
		return Session{}, NewError(CodePermissionDenied, "session resume capability is required")
	}
	if err := ValidateID(newID); err != nil {
		return Session{}, err
	}
	if newID == s.ID {
		return Session{}, NewError(CodeConflict, "resume requires a new session identifier")
	}
	if s.EndedAt == nil || at.Before(*s.EndedAt) {
		return Session{}, NewError(CodeInvalidArgument, "resumed session cannot precede source end")
	}
	resumed := Session{
		ID:                   newID,
		AccountID:            s.AccountID,
		DeviceID:             s.DeviceID,
		AccountID:            s.AccountID,
		Provider:             s.Provider,
		CredentialInstanceID: s.CredentialInstanceID,
		RuntimeProfileID:     s.RuntimeProfileID,
		WorkspaceID:          s.WorkspaceID,
		ProviderSessionID:    s.ProviderSessionID,
		ResumedFromSessionID: s.ID,
		Status:               SessionStarting,
		StartedAt:            at.UTC(),
		CapabilitySnapshot:   append([]Capability(nil), s.CapabilitySnapshot...),
	}
	return NewSession(resumed)
}
