package runtime

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
)

type StartRequest struct {
	DeviceID             domain.ID
	Provider             string
	AccountID            domain.ID
	CredentialInstanceID domain.ID
	RuntimeProfileID     domain.ID
	WorkspaceID          domain.ID
	Capabilities         []domain.Capability
	ResumedFromSessionID domain.ID
}

// Start is the P0 provider gate. Fake remains fully runnable; Codex records
// are representable in the Device store but cannot start until the versioned
// app-server adapter lands in a later approved phase.
func (m *Manager) Start(ctx context.Context, request StartRequest) (domain.Session, error) {
	if request.Provider == "" || request.Provider == domain.ProviderFake {
		return m.StartFake(ctx, request)
	}
	if request.Provider == domain.ProviderCodex {
		return domain.Session{}, domain.NewError(domain.CodeProviderUnsupported, "codex adapter is not implemented in P0")
	}
	return domain.Session{}, domain.NewError(domain.CodeInvalidArgument, "provider is unsupported")
}

type InputRequest struct {
	SessionID domain.ID
	ClientID  domain.ID
	Revision  int64
	Sequence  int64
	Payload   string
}

type InputResult struct {
	Sequence  int64 `json:"sequence"`
	Duplicate bool  `json:"duplicate"`
}

type ResizeRequest struct {
	SessionID domain.ID
	ClientID  domain.ID
	Revision  int64
	Rows      int
	Cols      int
}

type Manager struct {
	Store          *storage.Store
	Executable     string
	Now            func() time.Time
	Vault          interface{ RequireUnlocked() error }
	LeaseDuration  time.Duration
	LeaseHeartbeat time.Duration
	StopTimeout    time.Duration

	mu        sync.Mutex
	processes map[domain.ID]*Process
	rings     map[domain.ID]*RingBuffer
	eventSeq  map[domain.ID]int64
	inputSeq  map[string]int64
}

func NewManager(store *storage.Store, executable string) *Manager {
	return &Manager{
		Store: store, Executable: executable, Now: func() time.Time { return time.Now().UTC() },
		LeaseDuration: domain.DefaultLeaseDuration, LeaseHeartbeat: domain.DefaultLeaseHeartbeat,
		StopTimeout: 5 * time.Second, processes: make(map[domain.ID]*Process), rings: make(map[domain.ID]*RingBuffer),
		eventSeq: make(map[domain.ID]int64), inputSeq: make(map[string]int64),
	}
}

func (m *Manager) now() time.Time {
	if m != nil && m.Now != nil {
		return m.Now().UTC()
	}
	return time.Now().UTC()
}

func (m *Manager) StartFake(ctx context.Context, request StartRequest) (domain.Session, error) {
	if m == nil || m.Store == nil || m.Executable == "" || ctx == nil {
		return domain.Session{}, domain.NewError(domain.CodeInvalidArgument, "runtime manager is incomplete")
	}
	if m.Vault != nil {
		if err := m.Vault.RequireUnlocked(); err != nil {
			return domain.Session{}, err
		}
	}
	for _, id := range []domain.ID{request.DeviceID, request.CredentialInstanceID, request.RuntimeProfileID, request.WorkspaceID} {
		if err := domain.ValidateID(id); err != nil {
			return domain.Session{}, err
		}
	}
	caps, err := domain.CanonicalCapabilities(request.Capabilities)
	if err != nil {
		return domain.Session{}, err
	}
	sessionID, err := domain.NewID("session")
	if err != nil {
		return domain.Session{}, err
	}
	session := domain.Session{ID: sessionID, DeviceID: request.DeviceID, Provider: "fake",
		AccountID:            request.AccountID,
		CredentialInstanceID: request.CredentialInstanceID, RuntimeProfileID: request.RuntimeProfileID,
		WorkspaceID: request.WorkspaceID, ResumedFromSessionID: request.ResumedFromSessionID,
		Status: domain.SessionStarting, StartedAt: m.now(), CapabilitySnapshot: caps}
	if err := m.Store.CreateSession(ctx, session); err != nil {
		return domain.Session{}, err
	}
	process, err := StartProcess(m.Executable)
	if err != nil {
		_, _ = m.Store.TransitionSession(ctx, session.ID, domain.SessionStarting, domain.SessionFailed, m.now(), nil, string(domain.CodeProviderFailed))
		return domain.Session{}, err
	}
	if err := waitReady(process, 5*time.Second); err != nil {
		_ = process.Kill()
		_, _ = m.Store.TransitionSession(ctx, session.ID, domain.SessionStarting, domain.SessionFailed, m.now(), nil, string(domain.CodeProviderFailed))
		return domain.Session{}, err
	}
	if _, err := m.Store.TransitionSession(ctx, session.ID, domain.SessionStarting, domain.SessionRunning, m.now(), nil, ""); err != nil {
		_ = process.Kill()
		return domain.Session{}, err
	}
	ring := NewDefaultRingBuffer()
	m.mu.Lock()
	m.processes[session.ID] = process
	m.rings[session.ID] = ring
	m.mu.Unlock()
	go m.observeProcess(session.ID, process)
	return m.Store.Session(ctx, session.ID)
}

func (m *Manager) observeProcess(sessionID domain.ID, process *Process) {
	for event := range process.Events() {
		switch event.Kind {
		case "output":
			m.mu.Lock()
			ring := m.rings[sessionID]
			m.mu.Unlock()
			if ring != nil {
				chunks := ring.Append([]byte(event.Payload))
				for _, chunk := range chunks {
					m.appendEvent(sessionID, "output", map[string]any{"sequence": chunk.Sequence, "bytes": len(chunk.Data)})
				}
			}
		case "ready", "resized":
			m.appendEvent(sessionID, event.Kind, map[string]any{"payload": event.Payload})
		case "exit":
			m.handleExit(sessionID, process, event.Code)
		}
	}
	if process.Err() != nil {
		m.handleExit(sessionID, process, process.ExitCode())
	}
}

func (m *Manager) appendEvent(sessionID domain.ID, kind string, metadata any) {
	if m == nil || m.Store == nil {
		return
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return
	}
	m.mu.Lock()
	m.eventSeq[sessionID]++
	sequence := m.eventSeq[sessionID]
	m.mu.Unlock()
	id, err := domain.NewID("event")
	if err != nil {
		return
	}
	_ = m.Store.AppendRuntimeEvent(context.Background(), domain.RuntimeEvent{ID: id, SessionID: sessionID,
		Sequence: sequence, Kind: kind, Metadata: encoded, CreatedAt: m.now()})
}

func (m *Manager) handleExit(sessionID domain.ID, process *Process, code int) {
	ctx := context.Background()
	session, err := m.Store.Session(ctx, sessionID)
	if err != nil || session.Status.Terminal() {
		return
	}
	now := m.now()
	if session.Status == domain.SessionStopping {
		_, _ = m.Store.TransitionSession(ctx, sessionID, domain.SessionStopping, domain.SessionExited, now, &code, "")
	} else {
		_, _ = m.Store.TransitionSession(ctx, sessionID, session.Status, domain.SessionFailed, now, nil, string(domain.CodeProviderFailed))
	}
	m.mu.Lock()
	if current := m.processes[sessionID]; current == process {
		delete(m.processes, sessionID)
	}
	m.mu.Unlock()
}

func (m *Manager) process(sessionID domain.ID) (*Process, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	process := m.processes[sessionID]
	if process == nil {
		return nil, domain.NewError(domain.CodeNotFound, "session process not found")
	}
	return process, nil
}

func (m *Manager) Replay(ctx context.Context, sessionID domain.ID, from int64) (Replay, error) {
	if _, err := m.Store.Session(ctx, sessionID); err != nil {
		return Replay{}, err
	}
	m.mu.Lock()
	ring := m.rings[sessionID]
	m.mu.Unlock()
	if ring == nil {
		return Replay{}, domain.NewError(domain.CodeNotFound, "session replay not found")
	}
	return ring.Replay(from)
}

func (m *Manager) Attach(ctx context.Context, sessionID, clientID domain.ID, mode domain.AttachmentMode) error {
	session, err := m.Store.Session(ctx, sessionID)
	if err != nil {
		return err
	}
	if session.Status == domain.SessionStarting {
		return domain.NewError(domain.CodeConflict, "session is not attachable yet")
	}
	id, err := domain.NewID("attach")
	if err != nil {
		return err
	}
	now := m.now()
	err = m.Store.CreateAttachment(ctx, domain.SessionAttachment{ID: id, SessionID: sessionID,
		ClientDeviceID: clientID, Mode: mode, ConnectedAt: now, LastSeenAt: now})
	if domain.CodeOf(err) == domain.CodeConflict || domain.CodeOf(err) == domain.CodeAlreadyExists {
		return nil
	}
	return err
}

func (m *Manager) Detach(ctx context.Context, sessionID, clientID domain.ID) error {
	return m.Store.DeleteAttachment(ctx, sessionID, clientID)
}

func (m *Manager) Acquire(ctx context.Context, sessionID, clientID domain.ID) (domain.ControllerLease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var current *domain.ControllerLease
	loaded, err := m.Store.ControllerLease(ctx, sessionID)
	if err == nil {
		current = &loaded
	} else if domain.CodeOf(err) != domain.CodeNotFound {
		return domain.ControllerLease{}, err
	}
	lease, err := domain.AcquireControllerLease(current, sessionID, clientID, m.now(), m.LeaseDuration)
	if err != nil {
		return domain.ControllerLease{}, err
	}
	expected := int64(0)
	if current != nil {
		expected = current.Revision
	}
	if err := m.Store.SaveControllerLease(ctx, lease, expected); err != nil {
		return domain.ControllerLease{}, err
	}
	return lease, nil
}

func (m *Manager) Heartbeat(ctx context.Context, sessionID, clientID domain.ID, revision int64) (domain.ControllerLease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	lease, err := m.Store.ControllerLease(ctx, sessionID)
	if err != nil {
		return domain.ControllerLease{}, err
	}
	updated, err := lease.Heartbeat(clientID, revision, m.now(), m.LeaseDuration)
	if err != nil {
		return domain.ControllerLease{}, err
	}
	if err := m.Store.SaveControllerLease(ctx, updated, lease.Revision); err != nil {
		return domain.ControllerLease{}, err
	}
	return updated, nil
}

func (m *Manager) Release(ctx context.Context, sessionID, clientID domain.ID, revision int64) (domain.ControllerLease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	lease, err := m.Store.ControllerLease(ctx, sessionID)
	if err != nil {
		return domain.ControllerLease{}, err
	}
	updated, err := lease.Release(clientID, revision, m.now())
	if err != nil {
		return domain.ControllerLease{}, err
	}
	if err := m.Store.SaveControllerLease(ctx, updated, lease.Revision); err != nil {
		return domain.ControllerLease{}, err
	}
	return updated, nil
}

func (m *Manager) requireLease(ctx context.Context, sessionID, clientID domain.ID, revision int64) error {
	lease, err := m.Store.ControllerLease(ctx, sessionID)
	if err != nil {
		return err
	}
	return lease.RequireControl(clientID, revision, m.now())
}

func (m *Manager) Input(ctx context.Context, request InputRequest) (InputResult, error) {
	if request.Sequence < 1 {
		return InputResult{}, domain.NewError(domain.CodeInvalidArgument, "input sequence is invalid")
	}
	if err := m.requireLease(ctx, request.SessionID, request.ClientID, request.Revision); err != nil {
		return InputResult{}, err
	}
	key := string(request.SessionID) + ":" + string(request.ClientID)
	m.mu.Lock()
	expected := m.inputSeq[key]
	if expected == 0 {
		expected = 1
	}
	if request.Sequence < expected {
		m.mu.Unlock()
		return InputResult{Sequence: request.Sequence, Duplicate: true}, nil
	}
	if request.Sequence > expected {
		m.mu.Unlock()
		return InputResult{}, domain.NewError(domain.CodeConflict, "input sequence gap")
	}
	process := m.processes[request.SessionID]
	m.mu.Unlock()
	if process == nil {
		return InputResult{}, domain.NewError(domain.CodeNotFound, "session process not found")
	}
	if err := process.Input(request.Payload); err != nil {
		return InputResult{}, err
	}
	m.mu.Lock()
	m.inputSeq[key] = expected + 1
	m.mu.Unlock()
	return InputResult{Sequence: request.Sequence}, nil
}

func (m *Manager) Resize(ctx context.Context, request ResizeRequest) error {
	if request.Rows <= 0 || request.Cols <= 0 || request.Rows > 1000 || request.Cols > 1000 {
		return domain.NewError(domain.CodeInvalidArgument, "terminal size is invalid")
	}
	if err := m.requireLease(ctx, request.SessionID, request.ClientID, request.Revision); err != nil {
		return err
	}
	process, err := m.process(request.SessionID)
	if err != nil {
		return err
	}
	return process.Resize(request.Rows, request.Cols)
}

func (m *Manager) Stop(ctx context.Context, sessionID, clientID domain.ID, revision int64) (domain.Session, error) {
	if err := m.requireLease(ctx, sessionID, clientID, revision); err != nil {
		return domain.Session{}, err
	}
	session, err := m.Store.Session(ctx, sessionID)
	if err != nil {
		return domain.Session{}, err
	}
	if session.Status.Terminal() || session.Status == domain.SessionStopping {
		return session, nil
	}
	if _, err := m.Store.TransitionSession(ctx, sessionID, session.Status, domain.SessionStopping, m.now(), nil, ""); err != nil {
		return domain.Session{}, err
	}
	process, err := m.process(sessionID)
	if err != nil {
		return m.Store.Session(ctx, sessionID)
	}
	stopCtx, cancel := context.WithTimeout(ctx, m.StopTimeout)
	defer cancel()
	if err := process.Stop(stopCtx); err != nil && domain.CodeOf(err) != domain.CodeDeadlineExceeded {
		return domain.Session{}, err
	}
	return m.waitTerminal(ctx, sessionID)
}

func (m *Manager) waitTerminal(ctx context.Context, sessionID domain.ID) (domain.Session, error) {
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		session, err := m.Store.Session(ctx, sessionID)
		if err != nil {
			return domain.Session{}, err
		}
		if session.Status.Terminal() {
			return session, nil
		}
		select {
		case <-ctx.Done():
			return session, domain.WrapError(domain.CodeDeadlineExceeded, "session termination observation timed out", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (m *Manager) Kill(ctx context.Context, sessionID, clientID domain.ID, revision int64) (domain.Session, error) {
	if err := m.requireLease(ctx, sessionID, clientID, revision); err != nil {
		return domain.Session{}, err
	}
	session, err := m.Store.Session(ctx, sessionID)
	if err != nil {
		return domain.Session{}, err
	}
	if session.Status.Terminal() {
		return session, nil
	}
	process, processErr := m.process(sessionID)
	if processErr == nil {
		if err := process.Kill(); err != nil {
			return domain.Session{}, err
		}
	}
	if session.Status == domain.SessionStarting || session.Status == domain.SessionRunning || session.Status == domain.SessionStopping {
		if _, err := m.Store.TransitionSession(ctx, sessionID, session.Status, domain.SessionKilled, m.now(), nil, ""); err != nil {
			return domain.Session{}, err
		}
	}
	return m.Store.Session(ctx, sessionID)
}

func (m *Manager) Resume(ctx context.Context, sourceID domain.ID) (domain.Session, error) {
	source, err := m.Store.Session(ctx, sourceID)
	if err != nil {
		return domain.Session{}, err
	}
	if source.Provider != domain.ProviderFake {
		return domain.Session{}, domain.NewError(domain.CodeProviderResumeUnsupported, "provider continuation is not verified")
	}
	id, err := domain.NewID("session")
	if err != nil {
		return domain.Session{}, err
	}
	session, err := source.Resume(id, m.now())
	if err != nil {
		return domain.Session{}, err
	}
	return m.StartFake(ctx, StartRequest{DeviceID: session.DeviceID, CredentialInstanceID: session.CredentialInstanceID,
		RuntimeProfileID: session.RuntimeProfileID, WorkspaceID: session.WorkspaceID,
		Capabilities: session.CapabilitySnapshot, ResumedFromSessionID: source.ID})
}

func (m *Manager) Close() {
	m.mu.Lock()
	processes := make([]*Process, 0, len(m.processes))
	for _, process := range m.processes {
		processes = append(processes, process)
	}
	m.mu.Unlock()
	for _, process := range processes {
		_ = process.Kill()
	}
}
