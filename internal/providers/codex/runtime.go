package codex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
)

const MaxTurnInputBytes = 64 * 1024

type RuntimeStartRequest struct {
	DeviceID             domain.ID
	AccountID            domain.ID
	CredentialInstanceID domain.ID
	RuntimeProfileID     domain.ID
	WorkspaceID          domain.ID
}

type RuntimeReservedStartRequest struct {
	SessionID domain.ID
	RuntimeStartRequest
	ProviderVersion   string
	BinaryFingerprint string
	SchemaFingerprint string
	CapabilityDigest  string
}

type RuntimeInputRequest struct {
	SessionID domain.ID
	Payload   string
}

type ApprovalDispatchRequest struct {
	SessionID          domain.ID
	ApprovalID         domain.ID
	ProviderApprovalID string
	ResponderID        domain.ID
	ResponseKey        string
	Decision           domain.ApprovalDecision
	LeaseRevision      int64
}

type RuntimeEventSink func(domain.ID, string, any, []byte)

// RuntimeProcess is the bounded lifecycle returned by a production or fixture
// app-server spawn. Client owns the only JSON-RPC reader.
type RuntimeProcess struct {
	Client *Client
	Wait   func() error
	Kill   func() error
}

type DiscoverRuntime func(context.Context) (BinaryDescriptor, CapabilitySet, error)
type SpawnRuntime func(BinaryDescriptor, string) (*RuntimeProcess, error)

type RuntimeManager struct {
	Store        *storage.Store
	Materializer *CredentialMaterializationManager
	Discover     DiscoverRuntime
	Spawn        SpawnRuntime
	EventSink    RuntimeEventSink
	Now          func() time.Time

	mu       sync.Mutex
	workers  sync.WaitGroup
	runtimes map[domain.ID]*CredentialRuntime
	bindings map[domain.ID]*SessionBinding
	starting map[domain.ID]chan struct{}
	closed   bool
}

type CredentialRuntime struct {
	CredentialInstanceID domain.ID
	CredentialRevision   int64
	Descriptor           BinaryDescriptor
	BinaryFingerprint    string
	Capabilities         CapabilitySet
	Process              *RuntimeProcess
	Materialization      *MaterializationHandle
	Bindings             map[domain.ID]*SessionBinding
	Retired              map[string]retiredBinding
	finalizing           bool
	pumpStarted          bool
}

type retiredBinding struct {
	TurnID    string
	ExpiresAt time.Time
}

type SessionBinding struct {
	SessionID        domain.ID
	AccountID        domain.ID
	RuntimeProfileID domain.ID
	WorkspaceID      domain.ID
	ThreadID         string
	ActiveTurnID     string
	Pending          map[string]pendingRuntimeApproval
	CompletedTurnID  string
	state            string
}

type pendingRuntimeApproval struct {
	ApprovalID domain.ID
	RequestID  json.RawMessage
	Method     string
	Digest     string
}

type codexProfileSettings struct {
	Model          string `json:"model,omitempty"`
	ApprovalPolicy string `json:"approval_policy"`
	Sandbox        string `json:"sandbox"`
}

func NewRuntimeManager(store *storage.Store, materializer *CredentialMaterializationManager) *RuntimeManager {
	return &RuntimeManager{Store: store, Materializer: materializer, Discover: discoverRuntime,
		Spawn: spawnRuntime, Now: func() time.Time { return time.Now().UTC() },
		runtimes: make(map[domain.ID]*CredentialRuntime), bindings: make(map[domain.ID]*SessionBinding),
		starting: make(map[domain.ID]chan struct{})}
}

func discoverRuntime(ctx context.Context) (BinaryDescriptor, CapabilitySet, error) {
	descriptor, err := Discover(ctx, DiscoverOptions{})
	if err != nil {
		return BinaryDescriptor{}, CapabilitySet{}, err
	}
	capabilities, err := Probe(ctx, descriptor, ProbeOptions{})
	return descriptor, capabilities, err
}

func spawnRuntime(descriptor BinaryDescriptor, home string) (*RuntimeProcess, error) {
	if descriptor.Path == "" || !filepath.IsAbs(descriptor.Path) || home == "" || !filepath.IsAbs(home) {
		return nil, domain.NewError(domain.CodeInvalidArgument, "codex runtime spawn is incomplete")
	}
	command := exec.Command(descriptor.Path, "app-server")
	command.Env = append([]string{"CODEX_HOME=" + home, "HOME=" + home, "PATH=" + os.Getenv("PATH")}, NetworkEnvironment(os.Getenv)...)
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, domain.WrapError(domain.CodeProviderFailed, "codex stdin could not be opened", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, domain.WrapError(domain.CodeProviderFailed, "codex stdout could not be opened", err)
	}
	command.Stderr = io.Discard
	if err := command.Start(); err != nil {
		return nil, domain.WrapError(domain.CodeProviderFailed, "codex app-server could not start", err)
	}
	return &RuntimeProcess{Client: NewClient(stdout, stdin), Wait: command.Wait, Kill: func() error {
		if command.Process == nil {
			return nil
		}
		err := command.Process.Kill()
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return err
	}}, nil
}

func (m *RuntimeManager) now() time.Time {
	if m != nil && m.Now != nil {
		return m.Now().UTC()
	}
	return time.Now().UTC()
}

func (m *RuntimeManager) Start(ctx context.Context, request RuntimeStartRequest) (domain.Session, error) {
	return m.start(ctx, request, nil)
}

func (m *RuntimeManager) StartReserved(ctx context.Context, request RuntimeReservedStartRequest) (domain.Session, error) {
	return m.start(ctx, request.RuntimeStartRequest, &request)
}

func (m *RuntimeManager) start(ctx context.Context, request RuntimeStartRequest, reserved *RuntimeReservedStartRequest) (domain.Session, error) {
	if m == nil || m.Store == nil || m.Materializer == nil || m.Discover == nil || m.Spawn == nil || ctx == nil {
		return domain.Session{}, domain.NewError(domain.CodeInvalidArgument, "codex runtime manager is incomplete")
	}
	for _, id := range []domain.ID{request.DeviceID, request.AccountID, request.CredentialInstanceID, request.RuntimeProfileID, request.WorkspaceID} {
		if err := domain.ValidateID(id); err != nil {
			return domain.Session{}, err
		}
	}
	profile, err := m.Store.RuntimeProfile(ctx, request.RuntimeProfileID)
	if err != nil {
		return domain.Session{}, err
	}
	settings, err := decodeProfileSettings(profile.Settings)
	if err != nil {
		return domain.Session{}, err
	}
	workspace, err := m.Store.Workspace(ctx, request.WorkspaceID)
	if err != nil {
		return domain.Session{}, err
	}
	canonicalWorkspace, err := canonicalWorkspacePath(workspace.Path)
	if err != nil {
		return domain.Session{}, err
	}
	credential, err := m.Store.CredentialInstance(ctx, request.CredentialInstanceID)
	if err != nil {
		return domain.Session{}, err
	}
	if profile.Provider != domain.ProviderCodex || profile.DeviceID != request.DeviceID || profile.AccountID != request.AccountID ||
		credential.Provider != domain.ProviderCodex || credential.DeviceID != request.DeviceID || credential.AccountID != request.AccountID ||
		workspace.DeviceID != request.DeviceID || credential.Status != domain.CredentialHealthy {
		return domain.Session{}, domain.NewError(domain.CodeConflict, "codex runtime links do not match")
	}
	capabilitySnapshot := []domain.Capability{domain.CapabilityApprovalRead, domain.CapabilityApprovalRespond,
		domain.CapabilityProviderUsageRead, domain.CapabilitySessionControl, domain.CapabilitySessionObserve, domain.CapabilityTerminalControl}
	var session domain.Session
	if reserved == nil {
		sessionID, idErr := domain.NewID("session")
		if idErr != nil {
			return domain.Session{}, idErr
		}
		session = domain.Session{ID: sessionID, DeviceID: request.DeviceID, AccountID: request.AccountID,
			Provider: domain.ProviderCodex, CredentialInstanceID: request.CredentialInstanceID,
			RuntimeProfileID: request.RuntimeProfileID, WorkspaceID: request.WorkspaceID,
			Status: domain.SessionStarting, StartedAt: m.now(), CapabilitySnapshot: capabilitySnapshot}
		if err := m.Store.CreateSession(ctx, session); err != nil {
			return domain.Session{}, err
		}
	} else {
		if domain.ValidateID(reserved.SessionID) != nil || reserved.ProviderVersion == "" ||
			!validRuntimeFingerprint(reserved.BinaryFingerprint) || !validRuntimeFingerprint(reserved.SchemaFingerprint) ||
			!validRuntimeFingerprint(reserved.CapabilityDigest) {
			return domain.Session{}, domain.NewError(domain.CodeInvalidArgument, "reserved Codex compatibility is invalid")
		}
		session, err = m.Store.Session(ctx, reserved.SessionID)
		if err != nil {
			return domain.Session{}, err
		}
		if session.DeviceID != request.DeviceID || session.AccountID != request.AccountID ||
			session.CredentialInstanceID != request.CredentialInstanceID || session.RuntimeProfileID != request.RuntimeProfileID ||
			session.WorkspaceID != request.WorkspaceID || session.Provider != domain.ProviderCodex {
			return domain.Session{}, domain.NewError(domain.CodeProfileBindingChanged, "reserved Session tuple changed")
		}
		if session.Status != domain.SessionStarting {
			return session, nil
		}
		m.mu.Lock()
		if waiting := m.starting[session.ID]; waiting != nil {
			m.mu.Unlock()
			select {
			case <-waiting:
				return m.Store.Session(ctx, session.ID)
			case <-ctx.Done():
				return domain.Session{}, domain.NewError(domain.CodeDeadlineExceeded, "reserved Codex start wait expired")
			}
		}
		waiting := make(chan struct{})
		m.starting[session.ID] = waiting
		m.mu.Unlock()
		defer func() {
			m.mu.Lock()
			if current := m.starting[session.ID]; current == waiting {
				delete(m.starting, session.ID)
				close(waiting)
			}
			m.mu.Unlock()
		}()
	}
	sessionID := session.ID
	failStart := func(cause error) (domain.Session, error) {
		_, _ = m.Store.TransitionSession(context.Background(), sessionID, domain.SessionStarting, domain.SessionFailed, m.now(), nil, string(domain.CodeOf(cause)))
		return domain.Session{}, cause
	}
	var reservedDescriptor BinaryDescriptor
	var reservedCapabilities CapabilitySet
	if reserved != nil {
		reservedDescriptor, reservedCapabilities, err = m.Discover(ctx)
		if err != nil {
			return failStart(err)
		}
		if err := RequireSelectorPlatform(reservedDescriptor); err != nil {
			return failStart(err)
		}
		binaryFingerprint, fingerprintErr := BinaryFingerprint(reservedDescriptor)
		if fingerprintErr != nil {
			return failStart(fingerprintErr)
		}
		if reservedDescriptor.Version != reserved.ProviderVersion || binaryFingerprint != reserved.BinaryFingerprint ||
			reservedCapabilities.SchemaFingerprint != reserved.SchemaFingerprint || CapabilityDigest(reservedCapabilities) != reserved.CapabilityDigest {
			return failStart(domain.NewError(domain.CodeProviderVersionUnsupported, "Codex runtime changed after Session reservation"))
		}
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return failStart(domain.NewError(domain.CodeProviderFailed, "codex runtime manager is closed"))
	}
	runtime := m.runtimes[request.CredentialInstanceID]
	created := false
	if runtime != nil && (runtime.finalizing || runtime.CredentialRevision != credential.CredentialRevision) {
		m.mu.Unlock()
		return failStart(domain.NewError(domain.CodeCredentialRevisionConflict, "codex shared runtime revision changed"))
	}
	if runtime == nil {
		descriptor, capabilities := reservedDescriptor, reservedCapabilities
		binaryFingerprint := ""
		if reserved == nil {
			var discoverErr error
			descriptor, capabilities, discoverErr = m.Discover(ctx)
			if discoverErr != nil {
				m.mu.Unlock()
				return failStart(discoverErr)
			}
			binaryFingerprint, discoverErr = BinaryFingerprint(descriptor)
			if discoverErr != nil {
				m.mu.Unlock()
				return failStart(discoverErr)
			}
		} else {
			binaryFingerprint = reserved.BinaryFingerprint
		}
		materialization, acquireErr := m.Materializer.Acquire(ctx, request.CredentialInstanceID, request.RuntimeProfileID)
		if acquireErr != nil {
			m.mu.Unlock()
			return failStart(acquireErr)
		}
		process, spawnErr := m.Spawn(descriptor, materialization.AuthHomePath())
		if spawnErr != nil {
			_ = materialization.Release(context.Background())
			m.mu.Unlock()
			return failStart(spawnErr)
		}
		if process == nil || process.Client == nil || process.Wait == nil || process.Kill == nil {
			_ = materialization.Release(context.Background())
			m.mu.Unlock()
			return failStart(domain.NewError(domain.CodeProviderFailed, "codex spawned runtime is incomplete"))
		}
		if configureErr := process.Client.ConfigureMethods(capabilities.Methods); configureErr != nil {
			_ = process.Kill()
			_ = materialization.Release(context.Background())
			m.mu.Unlock()
			return failStart(configureErr)
		}
		if _, handshakeErr := process.Client.Handshake(ctx, InitializeParams{ClientInfo: ClientInfo{Name: "multi-agent-desk", Version: "phase2"}, Capabilities: &InitializeCapabilities{}}); handshakeErr != nil {
			_ = process.Kill()
			_ = materialization.Release(context.Background())
			m.mu.Unlock()
			return failStart(handshakeErr)
		}
		runtime = &CredentialRuntime{CredentialInstanceID: request.CredentialInstanceID,
			CredentialRevision: credential.CredentialRevision, Descriptor: descriptor,
			BinaryFingerprint: binaryFingerprint, Capabilities: capabilities,
			Process: process, Materialization: materialization, Bindings: make(map[domain.ID]*SessionBinding),
			Retired: make(map[string]retiredBinding)}
		m.runtimes[request.CredentialInstanceID] = runtime
		created = true
	} else if reserved != nil && (runtime.Descriptor.Path != reservedDescriptor.Path ||
		runtime.BinaryFingerprint != reserved.BinaryFingerprint ||
		runtime.Descriptor.Version != reservedDescriptor.Version || runtime.Capabilities.SchemaFingerprint != reservedCapabilities.SchemaFingerprint ||
		CapabilityDigest(runtime.Capabilities) != reserved.CapabilityDigest) {
		m.mu.Unlock()
		return failStart(domain.NewError(domain.CodeProviderVersionUnsupported, "Codex shared runtime does not match Session reservation"))
	}
	threadParams := map[string]any{"cwd": canonicalWorkspace, "approvalPolicy": settings.ApprovalPolicy,
		"sandbox": settings.Sandbox, "ephemeral": false}
	if settings.Model != "" {
		threadParams["model"] = settings.Model
	}
	var raw json.RawMessage
	if callErr := runtime.Process.Client.Call(ctx, MethodThreadStart, threadParams, &raw); callErr != nil {
		if created {
			delete(m.runtimes, request.CredentialInstanceID)
			runtime.finalizing = true
		}
		m.mu.Unlock()
		if created {
			_ = m.finalize(runtime)
		}
		return failStart(callErr)
	}
	threadID, decodeErr := decodeThreadStartResponse(raw)
	if decodeErr != nil {
		if created {
			delete(m.runtimes, request.CredentialInstanceID)
			runtime.finalizing = true
		}
		m.mu.Unlock()
		if created {
			_ = m.finalize(runtime)
		}
		return failStart(decodeErr)
	}
	binding := &SessionBinding{SessionID: sessionID, AccountID: request.AccountID,
		RuntimeProfileID: request.RuntimeProfileID, WorkspaceID: request.WorkspaceID,
		ThreadID: threadID, Pending: make(map[string]pendingRuntimeApproval), state: "running"}
	runtime.Bindings[sessionID] = binding
	m.bindings[sessionID] = binding
	startPump := created && !runtime.pumpStarted
	if startPump {
		runtime.pumpStarted = true
		m.workers.Add(3)
	}
	m.mu.Unlock()
	if _, err := m.Store.SetSessionProviderSessionID(ctx, sessionID, domain.SessionStarting, threadID); err != nil {
		m.releaseBinding(sessionID, domain.SessionFailed, string(domain.CodeProviderFailed))
		return domain.Session{}, err
	}
	if _, err := m.Store.TransitionSession(ctx, sessionID, domain.SessionStarting, domain.SessionRunning, m.now(), nil, ""); err != nil {
		m.releaseBinding(sessionID, domain.SessionFailed, string(domain.CodeProviderFailed))
		return domain.Session{}, err
	}
	if startPump {
		go func() {
			defer m.workers.Done()
			m.eventPump(runtime)
		}()
		go func() {
			defer m.workers.Done()
			m.leasePump(runtime)
		}()
		go func() {
			defer m.workers.Done()
			if err := runtime.Process.Wait(); err != nil {
				m.failRuntime(runtime, domain.CodeProviderFailed)
				return
			}
			m.failRuntime(runtime, domain.CodeProviderFailed)
		}()
	}
	return m.Store.Session(ctx, sessionID)
}

func validRuntimeFingerprint(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func decodeProfileSettings(raw json.RawMessage) (codexProfileSettings, error) {
	if len(raw) == 0 {
		return codexProfileSettings{}, domain.NewError(domain.CodeInvalidArgument, "codex.v1 profile settings are required")
	}
	var settings codexProfileSettings
	if err := DecodeObject(raw, &settings); err != nil {
		return codexProfileSettings{}, err
	}
	if len(settings.Model) > 128 || (settings.ApprovalPolicy != "untrusted" && settings.ApprovalPolicy != "on-request" && settings.ApprovalPolicy != "never") ||
		(settings.Sandbox != "read-only" && settings.Sandbox != "workspace-write") {
		return codexProfileSettings{}, domain.NewError(domain.CodeInvalidArgument, "codex.v1 profile settings are invalid")
	}
	return settings, nil
}

func canonicalWorkspacePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", domain.NewError(domain.CodeInvalidArgument, "workspace path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", domain.NewError(domain.CodeConflict, "workspace path could not be resolved")
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", domain.NewError(domain.CodeConflict, "workspace path could not be canonicalized")
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() {
		return "", domain.NewError(domain.CodeConflict, "workspace path is not a directory")
	}
	return filepath.Clean(canonical), nil
}

func decodeThreadStartResponse(raw json.RawMessage) (string, error) {
	var response map[string]json.RawMessage
	if json.Unmarshal(raw, &response) != nil {
		return "", domain.NewError(domain.CodeProviderProtocolError, "codex thread/start response is invalid")
	}
	allowed := map[string]struct{}{"activePermissionProfile": {}, "approvalPolicy": {}, "approvalsReviewer": {}, "cwd": {},
		"instructionSources": {}, "model": {}, "modelProvider": {}, "multiAgentMode": {}, "reasoningEffort": {},
		"runtimeWorkspaceRoots": {}, "sandbox": {}, "serviceTier": {}, "thread": {}}
	for key := range response {
		if _, ok := allowed[key]; !ok {
			return "", domain.NewError(domain.CodeProviderProtocolError, "codex thread/start response field is unmapped")
		}
	}
	return boundedObjectID(response["thread"], "thread")
}

func decodeTurnStartResponse(raw json.RawMessage) (string, error) {
	var response struct {
		Turn json.RawMessage `json:"turn"`
	}
	if err := DecodeObject(raw, &response); err != nil {
		return "", err
	}
	return boundedObjectID(response.Turn, "turn")
}

func (m *RuntimeManager) Input(ctx context.Context, request RuntimeInputRequest) error {
	if ctx == nil || domain.ValidateID(request.SessionID) != nil || request.Payload == "" || len(request.Payload) > MaxTurnInputBytes {
		return domain.NewError(domain.CodeInvalidArgument, "codex turn input is invalid")
	}
	m.mu.Lock()
	binding := m.bindings[request.SessionID]
	if binding == nil || binding.state != "running" {
		m.mu.Unlock()
		return domain.NewError(domain.CodeNotFound, "codex session binding is unavailable")
	}
	if binding.ActiveTurnID != "" {
		m.mu.Unlock()
		return domain.NewError(domain.CodeConflict, "codex session already has an active turn")
	}
	runtime := m.runtimeForBindingLocked(binding)
	if runtime == nil || !runtime.Capabilities.Allows(MethodTurnStart) {
		m.mu.Unlock()
		return domain.NewError(domain.CodeProviderVersionUnsupported, "codex turn/start is not enabled")
	}
	binding.ActiveTurnID = "starting"
	threadID := binding.ThreadID
	client := runtime.Process.Client
	m.mu.Unlock()
	var raw json.RawMessage
	err := client.Call(ctx, MethodTurnStart, map[string]any{"threadId": threadID,
		"input": []any{map[string]any{"type": "text", "text": request.Payload}}}, &raw)
	if err != nil {
		m.mu.Lock()
		if current := m.bindings[request.SessionID]; current == binding && current.ActiveTurnID == "starting" {
			current.ActiveTurnID = ""
		}
		m.mu.Unlock()
		return err
	}
	turnID, err := decodeTurnStartResponse(raw)
	if err != nil {
		m.failRuntime(runtime, domain.CodeProviderProtocolError)
		return err
	}
	m.mu.Lock()
	if current := m.bindings[request.SessionID]; current != binding || current.state != "running" {
		m.mu.Unlock()
		return domain.NewError(domain.CodeProviderFailed, "codex session ended during turn start")
	}
	if binding.CompletedTurnID == turnID {
		binding.ActiveTurnID = ""
	} else {
		binding.ActiveTurnID = turnID
	}
	m.mu.Unlock()
	m.emit(request.SessionID, "turn_started", map[string]any{"turn_id": turnID}, nil)
	return nil
}

func (m *RuntimeManager) Resize(domain.ID) error {
	return domain.NewError(domain.CodeProviderControlUnsupported, "codex conversation resize is unsupported")
}

func (m *RuntimeManager) Stop(ctx context.Context, sessionID domain.ID, killed bool) error {
	if ctx == nil || domain.ValidateID(sessionID) != nil {
		return domain.NewError(domain.CodeInvalidArgument, "codex stop request is invalid")
	}
	m.mu.Lock()
	binding := m.bindings[sessionID]
	if binding == nil {
		m.mu.Unlock()
		return domain.NewError(domain.CodeNotFound, "codex session binding is unavailable")
	}
	binding.state = "stopping"
	runtime := m.runtimeForBindingLocked(binding)
	if runtime == nil {
		binding.state = "running"
		m.mu.Unlock()
		return domain.NewError(domain.CodeProviderFailed, "codex shared runtime is unavailable")
	}
	turnID, threadID := binding.ActiveTurnID, binding.ThreadID
	client := runtime.Process.Client
	canInterrupt := runtime.Capabilities.Allows(MethodTurnInterrupt)
	m.mu.Unlock()
	if turnID != "" && turnID != "starting" {
		if !canInterrupt {
			if !killed {
				m.mu.Lock()
				if current := m.bindings[sessionID]; current == binding {
					current.state = "running"
				}
				m.mu.Unlock()
				return domain.NewError(domain.CodeProviderControlUnsupported, "codex turn interrupt is unsupported")
			}
		} else if err := client.Call(ctx, MethodTurnInterrupt, map[string]any{"threadId": threadID, "turnId": turnID}, nil); err != nil && !killed {
			m.mu.Lock()
			if current := m.bindings[sessionID]; current == binding {
				current.state = "running"
			}
			m.mu.Unlock()
			return err
		}
	}
	status := domain.SessionExited
	if killed {
		status = domain.SessionKilled
	}
	m.releaseBinding(sessionID, status, "")
	return nil
}

func (m *RuntimeManager) releaseBinding(sessionID domain.ID, status domain.SessionStatus, failureCode string) {
	m.mu.Lock()
	binding := m.bindings[sessionID]
	if binding == nil {
		m.mu.Unlock()
		return
	}
	runtime := m.runtimeForBindingLocked(binding)
	if runtime != nil && binding.ThreadID != "" && binding.ActiveTurnID != "" && binding.ActiveTurnID != "starting" {
		runtime.Retired[binding.ThreadID] = retiredBinding{TurnID: binding.ActiveTurnID, ExpiresAt: m.now().Add(30 * time.Second)}
	}
	delete(m.bindings, sessionID)
	if runtime != nil {
		delete(runtime.Bindings, sessionID)
	}
	var finalize *CredentialRuntime
	if runtime != nil && len(runtime.Bindings) == 0 && !runtime.finalizing {
		runtime.finalizing = true
		delete(m.runtimes, runtime.CredentialInstanceID)
		finalize = runtime
	}
	m.mu.Unlock()
	current, err := m.Store.Session(context.Background(), sessionID)
	if err == nil && !current.Status.Terminal() {
		if status == domain.SessionExited && current.Status != domain.SessionStopping {
			_, _ = m.Store.TransitionSession(context.Background(), sessionID, current.Status, domain.SessionStopping, m.now(), nil, "")
			current.Status = domain.SessionStopping
		}
		_, _ = m.Store.TransitionSession(context.Background(), sessionID, current.Status, status, m.now(), nil, failureCode)
	}
	if finalize != nil {
		_ = m.finalize(finalize)
	}
}

func (m *RuntimeManager) runtimeForBindingLocked(binding *SessionBinding) *CredentialRuntime {
	for _, runtime := range m.runtimes {
		if runtime.Bindings[binding.SessionID] == binding {
			return runtime
		}
	}
	return nil
}

func (m *RuntimeManager) finalize(runtime *CredentialRuntime) error {
	if runtime == nil {
		return nil
	}
	runtime.Process.Client.Close()
	firstErr := runtime.Process.Kill()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := runtime.Materialization.ObserveAndCommit(ctx); err != nil {
		if firstErr == nil {
			firstErr = err
		}
		_ = runtime.Materialization.Quarantine(ctx, "runtime finalization failed")
	}
	if err := runtime.Materialization.Release(ctx); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (m *RuntimeManager) eventPump(runtime *CredentialRuntime) {
	for {
		frame, err := runtime.Process.Client.ReadInbound(context.Background())
		if err != nil {
			if domain.CodeOf(err) == domain.CodeDeadlineExceeded {
				select {
				case <-runtime.Process.Client.done:
					m.failRuntime(runtime, domain.CodeProviderFailed)
					return
				default:
					continue
				}
			}
			m.failRuntime(runtime, domain.CodeProviderFailed)
			return
		}
		if err := m.routeInbound(runtime, frame); err != nil {
			m.failRuntime(runtime, domain.CodeProviderProtocolError)
			return
		}
	}
}

func (m *RuntimeManager) leasePump(runtime *CredentialRuntime) {
	ticker := time.NewTicker(LeaseTTL / 3)
	defer ticker.Stop()
	for {
		select {
		case <-runtime.Process.Client.done:
			return
		case <-ticker.C:
		}
		m.mu.Lock()
		active := runtime != nil && !runtime.finalizing && m.runtimes[runtime.CredentialInstanceID] == runtime
		m.mu.Unlock()
		if !active {
			return
		}
		if _, err := runtime.Materialization.RefreshLease(context.Background()); err != nil {
			m.failRuntime(runtime, domain.CodeCredentialWriterConflict)
			return
		}
	}
}

func (m *RuntimeManager) routeInbound(runtime *CredentialRuntime, frame json.RawMessage) error {
	var envelope struct {
		JSONRPC string          `json:"jsonrpc,omitempty"`
		ID      json.RawMessage `json:"id,omitempty"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := DecodeObject(frame, &envelope); err != nil {
		return err
	}
	if envelope.Method == MethodApprovalPermissions {
		return domain.NewError(domain.CodeProviderVersionUnsupported, "codex permissions Approval is disabled")
	}
	event, err := MapEvent(envelope.Method, envelope.Params, runtime.Descriptor.Version, m.now())
	if err != nil {
		return err
	}
	if event.Method == MethodConfigWarning || event.Method == MethodRemoteControlStatus || event.Method == MethodAccountRateLimitsUpdated {
		if len(envelope.ID) != 0 {
			return domain.NewError(domain.CodeProviderProtocolError, "codex config warning cannot request a response")
		}
		return nil
	}
	m.mu.Lock()
	for threadID, retired := range runtime.Retired {
		if !retired.ExpiresAt.After(m.now()) {
			delete(runtime.Retired, threadID)
		}
	}
	var binding *SessionBinding
	for _, candidate := range runtime.Bindings {
		if candidate.ThreadID == event.ThreadID {
			binding = candidate
			break
		}
	}
	if binding == nil {
		if retired, ok := runtime.Retired[event.ThreadID]; ok && event.TurnID == retired.TurnID &&
			(event.Method == MethodAgentMessageDelta || event.Method == MethodTurnCompleted) {
			if event.Method == MethodTurnCompleted {
				delete(runtime.Retired, event.ThreadID)
			}
			m.mu.Unlock()
			return nil
		}
		m.mu.Unlock()
		return domain.NewError(domain.CodeProviderProtocolError, "codex event thread is unknown")
	}
	if event.TurnID != "" && binding.ActiveTurnID != "starting" && binding.ActiveTurnID != "" && event.TurnID != binding.ActiveTurnID {
		m.mu.Unlock()
		return domain.NewError(domain.CodeProviderProtocolError, "codex event turn is unknown")
	}
	sessionID := binding.SessionID
	switch event.Method {
	case MethodTurnStarted:
		if binding.ActiveTurnID == "" || binding.ActiveTurnID == "starting" {
			binding.ActiveTurnID = event.TurnID
		}
	case MethodTurnCompleted:
		binding.CompletedTurnID = event.TurnID
		binding.ActiveTurnID = ""
	case MethodApprovalCommand, MethodApprovalFileChange:
		if len(envelope.ID) == 0 || !runtime.Capabilities.Allows(event.Method) {
			m.mu.Unlock()
			return domain.NewError(domain.CodeProviderVersionUnsupported, "codex Approval method is not enabled")
		}
		if _, exists := binding.Pending[event.ProviderApprovalID]; exists {
			m.mu.Unlock()
			return domain.NewError(domain.CodeConflict, "codex Approval is already pending")
		}
		approvalID, idErr := domain.NewID("approval")
		if idErr != nil {
			m.mu.Unlock()
			return idErr
		}
		binding.Pending[event.ProviderApprovalID] = pendingRuntimeApproval{ApprovalID: approvalID,
			RequestID: append(json.RawMessage(nil), envelope.ID...), Method: event.Method, Digest: event.PayloadDigest}
		m.mu.Unlock()
		approval := domain.Approval{ID: approvalID, SessionID: sessionID, ProviderApprovalID: event.ProviderApprovalID,
			Kind: event.Kind, PayloadDigest: event.PayloadDigest, Summary: event.Summary, Status: domain.ApprovalPending,
			ResponseState: domain.ApprovalResponseIdle, IdempotencyKey: "provider-pending", RequestedAt: m.now()}
		if err := m.Store.CreateApproval(context.Background(), approval); err != nil {
			return err
		}
		m.emit(sessionID, "approval_requested", map[string]any{"approval_id": approvalID,
			"provider_approval_id": event.ProviderApprovalID, "kind": event.Kind}, nil)
		return nil
	}
	m.mu.Unlock()
	switch event.Method {
	case MethodThreadStarted:
		m.emit(sessionID, "thread_started", map[string]any{"thread_id": event.ThreadID}, nil)
	case MethodTurnStarted:
		m.emit(sessionID, "turn_started", map[string]any{"turn_id": event.TurnID}, nil)
	case MethodTurnCompleted:
		m.emit(sessionID, "turn_completed", map[string]any{"turn_id": event.TurnID}, nil)
	case MethodAgentMessageDelta:
		m.emit(sessionID, "output", map[string]any{"item_id": event.ProviderItemID, "bytes": len(event.Text)}, []byte(event.Text))
	}
	return nil
}

func (m *RuntimeManager) emit(sessionID domain.ID, kind string, metadata any, output []byte) {
	if m != nil && m.EventSink != nil {
		m.EventSink(sessionID, kind, metadata, output)
	}
}

func (m *RuntimeManager) failRuntime(runtime *CredentialRuntime, code domain.ErrorCode) {
	m.mu.Lock()
	if runtime == nil || runtime.finalizing {
		m.mu.Unlock()
		return
	}
	runtime.finalizing = true
	delete(m.runtimes, runtime.CredentialInstanceID)
	sessions := make([]domain.ID, 0, len(runtime.Bindings))
	for sessionID := range runtime.Bindings {
		delete(m.bindings, sessionID)
		sessions = append(sessions, sessionID)
	}
	runtime.Bindings = make(map[domain.ID]*SessionBinding)
	m.mu.Unlock()
	for _, sessionID := range sessions {
		current, err := m.Store.Session(context.Background(), sessionID)
		if err == nil && !current.Status.Terminal() {
			_, _ = m.Store.TransitionSession(context.Background(), sessionID, current.Status, domain.SessionFailed, m.now(), nil, string(code))
			m.emit(sessionID, "error", map[string]any{"code": code}, nil)
		}
	}
	_ = m.finalize(runtime)
}

func (m *RuntimeManager) RespondApproval(ctx context.Context, request ApprovalDispatchRequest) (domain.Approval, error) {
	if ctx == nil || request.LeaseRevision < 1 || request.ResponseKey == "" || len(request.ResponseKey) > 128 {
		return domain.Approval{}, domain.NewError(domain.CodeInvalidArgument, "codex Approval dispatch is invalid")
	}
	for _, id := range []domain.ID{request.SessionID, request.ApprovalID, request.ResponderID} {
		if err := domain.ValidateID(id); err != nil {
			return domain.Approval{}, err
		}
	}
	if request.Decision != domain.ApprovalDecisionApprove && request.Decision != domain.ApprovalDecisionDeny && request.Decision != domain.ApprovalDecisionCancel {
		return domain.Approval{}, domain.NewError(domain.CodeInvalidArgument, "codex Approval decision is invalid")
	}
	stored, err := m.Store.Approval(ctx, request.ApprovalID)
	if err != nil {
		return domain.Approval{}, err
	}
	if stored.SessionID != request.SessionID || stored.ProviderApprovalID != request.ProviderApprovalID {
		return domain.Approval{}, domain.NewError(domain.CodeApprovalUnknown, "codex Approval request does not match")
	}
	decisionText := map[domain.ApprovalDecision]string{domain.ApprovalDecisionApprove: "accept", domain.ApprovalDecisionDeny: "decline", domain.ApprovalDecisionCancel: "cancel"}[request.Decision]
	method := ""
	switch stored.Kind {
	case "commandExecution":
		method = MethodApprovalCommand
	case "fileChange":
		method = MethodApprovalFileChange
	default:
		return domain.Approval{}, domain.NewError(domain.CodeProviderVersionUnsupported, "codex Approval kind is disabled")
	}
	digestInput := strings.Join([]string{string(request.SessionID), string(request.ApprovalID), request.ProviderApprovalID,
		method, stored.PayloadDigest, string(request.Decision), string(request.ResponderID),
		strconv.FormatInt(request.LeaseRevision, 10), request.ResponseKey}, "\x00")
	digestBytes := sha256.Sum256([]byte(digestInput))
	dispatchDigest := hex.EncodeToString(digestBytes[:])
	if stored.ResponseState != domain.ApprovalResponseIdle {
		if stored.DispatchDigest != dispatchDigest {
			return domain.Approval{}, domain.NewError(domain.CodeConflict, "codex Approval dispatch already has another decision")
		}
		if stored.ResponseState == domain.ApprovalResponseWritten {
			return stored, nil
		}
		return domain.Approval{}, domain.NewError(domain.CodeApprovalDispatchAmbiguous, "codex Approval dispatch is already in flight or ambiguous")
	}
	m.mu.Lock()
	binding := m.bindings[request.SessionID]
	if binding == nil {
		m.mu.Unlock()
		return domain.Approval{}, domain.NewError(domain.CodeApprovalDispatchAmbiguous, "codex Approval binding is unavailable")
	}
	pending, ok := binding.Pending[request.ProviderApprovalID]
	runtime := m.runtimeForBindingLocked(binding)
	if !ok || pending.ApprovalID != request.ApprovalID || runtime == nil || pending.Method != method || pending.Digest != stored.PayloadDigest {
		m.mu.Unlock()
		return domain.Approval{}, domain.NewError(domain.CodeApprovalUnknown, "codex Approval request is not pending")
	}
	client := runtime.Process.Client
	m.mu.Unlock()
	if _, err := m.Store.ClaimApprovalDispatch(ctx, request.ApprovalID, request.ProviderApprovalID, request.ResponderID,
		request.ResponseKey, request.Decision, dispatchDigest, m.now()); err != nil {
		return domain.Approval{}, err
	}
	if err := client.RespondServerRequest(ctx, pending.RequestID, map[string]any{"decision": decisionText}); err != nil {
		m.mu.Lock()
		if current := m.bindings[request.SessionID]; current == binding {
			delete(current.Pending, request.ProviderApprovalID)
		}
		m.mu.Unlock()
		_, _ = m.Store.FailApprovalDispatch(context.Background(), request.ApprovalID, dispatchDigest, string(domain.CodeApprovalDispatchAmbiguous), m.now())
		return domain.Approval{}, domain.NewError(domain.CodeApprovalDispatchAmbiguous, "codex Approval response write is ambiguous")
	}
	m.mu.Lock()
	if current := m.bindings[request.SessionID]; current == binding {
		delete(current.Pending, request.ProviderApprovalID)
	}
	m.mu.Unlock()
	completed, err := m.Store.CompleteApprovalDispatch(ctx, request.ApprovalID, dispatchDigest, m.now())
	if err != nil {
		_, _ = m.Store.FailApprovalDispatch(context.Background(), request.ApprovalID, dispatchDigest, string(domain.CodeApprovalDispatchAmbiguous), m.now())
		return domain.Approval{}, domain.NewError(domain.CodeApprovalDispatchAmbiguous, "codex Approval completion is ambiguous")
	}
	m.emit(request.SessionID, "approval_resolved", map[string]any{"approval_id": request.ApprovalID, "decision": request.Decision}, nil)
	return completed, nil
}

func (m *RuntimeManager) ReadUsage(ctx context.Context, accountID domain.ID) (domain.UsageSnapshot, error) {
	if ctx == nil || domain.ValidateID(accountID) != nil {
		return domain.UsageSnapshot{}, domain.NewError(domain.CodeInvalidArgument, "codex usage account is invalid")
	}
	m.mu.Lock()
	var runtime *CredentialRuntime
	var deviceID domain.ID
	var credentialID domain.ID
	for _, binding := range m.bindings {
		if binding.AccountID == accountID && binding.state == "running" {
			runtime = m.runtimeForBindingLocked(binding)
			if runtime == nil {
				continue
			}
			session, _ := m.Store.Session(context.Background(), binding.SessionID)
			deviceID = session.DeviceID
			credentialID = runtime.CredentialInstanceID
			break
		}
	}
	if runtime == nil {
		m.mu.Unlock()
		return domain.UsageSnapshot{}, domain.NewError(domain.CodeUsageUnavailable, "codex usage is unavailable without an active supported runtime")
	}
	client, version := runtime.Process.Client, runtime.Descriptor.Version
	allowsUsage := runtime.Capabilities.Allows(MethodAccountUsage)
	m.mu.Unlock()
	if !allowsUsage {
		return m.persistUsageSnapshot(context.Background(), accountID, credentialID, deviceID, version, "unknown", "", domain.UsageUnavailable,
			domain.CodeProviderVersionUnsupported, domain.UsageConfidenceLow)
	}
	var raw json.RawMessage
	if err := client.Call(ctx, MethodAccountUsage, map[string]any{}, &raw); err != nil {
		code := domain.CodeOf(err)
		if code == "" {
			code = domain.CodeProviderFailed
		}
		return m.persistUsageSnapshot(context.Background(), accountID, credentialID, deviceID, version, "unknown", "", domain.UsageError,
			code, domain.UsageConfidenceLow)
	}
	projection, err := DecodeUsageResponse(raw, version, m.now())
	if err != nil {
		code := domain.CodeOf(err)
		if code == "" {
			code = domain.CodeProviderVersionUnsupported
		}
		return m.persistUsageSnapshot(context.Background(), accountID, credentialID, deviceID, version, "unknown", "", domain.UsageSchemaChanged,
			code, domain.UsageConfidenceLow)
	}
	return m.persistUsageSnapshot(ctx, accountID, credentialID, deviceID, projection.SourceVersion, "daily", projection.RawReferenceHash,
		domain.UsageCapabilityStatus(projection.CapabilityStatus), "", domain.UsageConfidence(projection.Confidence))
}

func (m *RuntimeManager) persistUsageSnapshot(ctx context.Context, accountID, credentialID, deviceID domain.ID, version, windowKind, rawHash string,
	status domain.UsageCapabilityStatus, errorCode domain.ErrorCode, confidence domain.UsageConfidence) (domain.UsageSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	persistCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	id, err := domain.NewID("usage")
	if err != nil {
		return domain.UsageSnapshot{}, err
	}
	observedAt := m.now()
	availability := domain.AvailabilityUnknown
	if status == domain.UsageSupported {
		availability = domain.AvailabilityAvailable
	}
	snapshot := domain.UsageSnapshot{ID: id, Provider: domain.ProviderCodex, AccountID: accountID,
		CredentialInstanceID: credentialID, DeviceID: deviceID,
		Source: domain.UsageSourceOfficial, Confidence: confidence, WindowKind: windowKind, ObservedAt: observedAt,
		RawReferenceHash: rawHash, SourceVersion: version, CapabilityStatus: status, ErrorCode: errorCode,
		ProviderVersion: version, Availability: availability, StaleAt: observedAt}
	if err := m.Store.CreateUsageSnapshot(persistCtx, snapshot); err != nil {
		return domain.UsageSnapshot{}, err
	}
	return snapshot, nil
}

func (m *RuntimeManager) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	runtimes := make([]*CredentialRuntime, 0, len(m.runtimes))
	for _, runtime := range m.runtimes {
		runtime.finalizing = true
		runtimes = append(runtimes, runtime)
	}
	m.runtimes = make(map[domain.ID]*CredentialRuntime)
	m.bindings = make(map[domain.ID]*SessionBinding)
	m.mu.Unlock()
	for _, runtime := range runtimes {
		for sessionID := range runtime.Bindings {
			current, err := m.Store.Session(context.Background(), sessionID)
			if err == nil && !current.Status.Terminal() {
				_, _ = m.Store.TransitionSession(context.Background(), sessionID, current.Status, domain.SessionFailed, m.now(), nil, string(domain.CodeDaemonUnavailable))
			}
		}
		_ = m.finalize(runtime)
	}
	m.workers.Wait()
}
