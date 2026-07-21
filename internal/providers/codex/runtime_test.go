package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
)

type fixtureRuntime struct {
	threads         atomic.Int64
	turns           atomic.Int64
	kills           atomic.Int64
	params          chan map[string]json.RawMessage
	responses       chan json.RawMessage
	done            chan struct{}
	client          net.Conn
	server          net.Conn
	failWrites      atomic.Bool
	blockWrites     atomic.Bool
	interruptReject atomic.Bool
	usageReject     atomic.Bool
	usageMu         sync.Mutex
	usageResult     json.RawMessage
	writeClosed     chan struct{}
	serverWriteMu   sync.Mutex
	killOnce        sync.Once
	doneOnce        sync.Once
	writeOnce       sync.Once
}

type fixtureWriter struct {
	connection net.Conn
	fail       *atomic.Bool
	block      *atomic.Bool
	closed     chan struct{}
	closeOnce  *sync.Once
}

func runtimeTestID(t *testing.T, prefix string) domain.ID {
	t.Helper()
	id, err := domain.NewID(prefix)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (w fixtureWriter) Write(value []byte) (int, error) {
	if w.fail.Load() {
		return 0, errors.New("fixture write failure")
	}
	if w.block.Load() {
		<-w.closed
		return 0, errors.New("fixture blocked write was closed")
	}
	return w.connection.Write(value)
}

func (w fixtureWriter) Close() error {
	w.closeOnce.Do(func() { close(w.closed) })
	return w.connection.Close()
}

func newFixtureRuntime(t *testing.T) (*RuntimeProcess, *fixtureRuntime) {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	fixture := &fixtureRuntime{params: make(chan map[string]json.RawMessage, 16), responses: make(chan json.RawMessage, 16),
		done: make(chan struct{}), client: clientConn, server: serverConn, writeClosed: make(chan struct{})}
	client := NewClient(clientConn, fixtureWriter{connection: clientConn, fail: &fixture.failWrites,
		block: &fixture.blockWrites, closed: fixture.writeClosed, closeOnce: &fixture.writeOnce})
	client.MaxWait = 100 * time.Millisecond
	go func() {
		reader := NewFrameReader(serverConn)
		for {
			frame, err := reader.Read()
			if err != nil {
				return
			}
			var envelope map[string]json.RawMessage
			if json.Unmarshal(frame, &envelope) == nil && len(envelope["method"]) == 0 && len(envelope["result"]) != 0 {
				fixture.responses <- append(json.RawMessage(nil), envelope["result"]...)
				continue
			}
			var request RPCRequest
			if DecodeObject(frame, &request) != nil || request.ID == 0 {
				continue
			}
			var result json.RawMessage
			switch request.Method {
			case MethodInitialize:
				result = json.RawMessage(`{"userAgent":"Codex/0.144.2","codexHome":"/fixture","platformFamily":"unix","platformOs":"linux"}`)
			case MethodThreadStart:
				var params map[string]json.RawMessage
				_ = json.Unmarshal(mustRawJSON(request.Params), &params)
				fixture.params <- params
				id := fixture.threads.Add(1)
				result = json.RawMessage(fmt.Sprintf(`{"thread":{"id":"thread-%d"}}`, id))
			case MethodTurnStart:
				id := fixture.turns.Add(1)
				result = json.RawMessage(fmt.Sprintf(`{"turn":{"id":"turn-%d"}}`, id))
			case MethodTurnInterrupt:
				if fixture.interruptReject.Load() {
					fixture.serverWriteMu.Lock()
					err := WriteFrame(serverConn, RPCResponse{JSONRPC: "2.0", ID: json.RawMessage(strconv.FormatUint(request.ID, 10)),
						Error: &RPCError{Code: -32000, Message: "fixture interrupt rejected"}})
					fixture.serverWriteMu.Unlock()
					if err != nil {
						return
					}
					continue
				}
				result = json.RawMessage(`{}`)
			case MethodAccountUsage:
				if fixture.usageReject.Load() {
					fixture.serverWriteMu.Lock()
					err := WriteFrame(serverConn, RPCResponse{JSONRPC: "2.0", ID: json.RawMessage(strconv.FormatUint(request.ID, 10)),
						Error: &RPCError{Code: -32001, Message: "fixture usage rejected"}})
					fixture.serverWriteMu.Unlock()
					if err != nil {
						return
					}
					continue
				}
				fixture.usageMu.Lock()
				result = append(json.RawMessage(nil), fixture.usageResult...)
				fixture.usageMu.Unlock()
				if len(result) == 0 {
					result = json.RawMessage(`{"dailyUsageBuckets":[],"summary":{"lifetimeTokens":1}}`)
				}
			default:
				result = json.RawMessage(`{}`)
			}
			fixture.serverWriteMu.Lock()
			err = WriteFrame(serverConn, RPCResponse{JSONRPC: "2.0", ID: json.RawMessage(strconv.FormatUint(request.ID, 10)), Result: result})
			fixture.serverWriteMu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	process := &RuntimeProcess{Client: client, Wait: func() error {
		<-fixture.done
		return nil
	}, Kill: func() error {
		fixture.killOnce.Do(func() {
			fixture.kills.Add(1)
			fixture.doneOnce.Do(func() {
				close(fixture.done)
				_ = clientConn.Close()
				_ = serverConn.Close()
			})
		})
		return nil
	}}
	return process, fixture
}

func (f *fixtureRuntime) notify(method string, params any) error {
	f.serverWriteMu.Lock()
	defer f.serverWriteMu.Unlock()
	return WriteFrame(f.server, RPCRequest{JSONRPC: "2.0", Method: method, Params: params})
}

func TestDecodeThreadStartResponseAllowsObservedRuntimeMetadataOnly(t *testing.T) {
	threadID, err := decodeThreadStartResponse(json.RawMessage(`{"thread":{"id":"thread-live"},"runtimeWorkspaceRoots":["/workspace"],"activePermissionProfile":null,"multiAgentMode":null}`))
	if err != nil || threadID != "thread-live" {
		t.Fatalf("thread=%q err=%v", threadID, err)
	}
	if _, err := decodeThreadStartResponse(json.RawMessage(`{"thread":{"id":"thread-live"},"unmapped":true}`)); domain.CodeOf(err) != domain.CodeProviderProtocolError {
		t.Fatalf("unmapped field code=%v err=%v", domain.CodeOf(err), err)
	}
}

func (f *fixtureRuntime) serverRequest(id uint64, method string, params any) error {
	f.serverWriteMu.Lock()
	defer f.serverWriteMu.Unlock()
	return WriteFrame(f.server, RPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
}

func (f *fixtureRuntime) setUsageResult(result string) {
	f.usageMu.Lock()
	f.usageResult = json.RawMessage(result)
	f.usageMu.Unlock()
}

func (f *fixtureRuntime) crash() {
	f.doneOnce.Do(func() {
		close(f.done)
		_ = f.server.Close()
		_ = f.client.Close()
	})
}

func prepareRuntimeManagerFixture(t *testing.T) (*RuntimeManager, RuntimeStartRequest, *storage.Store, *atomic.Int64, *[]*fixtureRuntime) {
	t.Helper()
	ctx := context.Background()
	store, credentialID, accountID := setupCodexCredential(t)
	t.Cleanup(func() { _ = store.Close() })
	credential, err := store.CredentialInstance(ctx, credentialID)
	if err != nil {
		t.Fatal(err)
	}
	profileID, _ := domain.NewID("profile")
	workspaceID, _ := domain.NewID("workspace")
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := store.CreateRuntimeProfile(ctx, domain.RuntimeProfile{ID: profileID, DeviceID: credential.DeviceID,
		AccountID: accountID, Name: "codex", Provider: domain.ProviderCodex,
		Settings: []byte(`{"model":"gpt-test","approval_policy":"on-request","sandbox":"workspace-write"}`), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	if err := store.CreateWorkspace(ctx, domain.Workspace{ID: workspaceID, DeviceID: credential.DeviceID,
		Path: workspace, Label: "codex", Tags: []string{}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	materializer := NewCredentialMaterializationManager(store, filepath.Join(t.TempDir(), "codex-home"),
		testVault{unlocked: true}, testCredentialSource{content: []byte(`{"token":"fixture"}`)})
	materializer.Sink = testMutationSink{store: store}
	manager := NewRuntimeManager(store, materializer)
	row := CompatibilityRows()[2]
	executable := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	manager.Discover = func(context.Context) (BinaryDescriptor, CapabilitySet, error) {
		descriptor := BinaryDescriptor{Provider: ProviderName, Path: executable, Version: row.Version, SchemaFingerprint: row.SchemaFingerprint}
		capabilities, err := CapabilitiesFor(descriptor)
		return descriptor, capabilities, err
	}
	spawnCount := &atomic.Int64{}
	fixtures := &[]*fixtureRuntime{}
	var fixtureMu sync.Mutex
	manager.Spawn = func(BinaryDescriptor, string) (*RuntimeProcess, error) {
		spawnCount.Add(1)
		process, fixture := newFixtureRuntime(t)
		fixtureMu.Lock()
		*fixtures = append(*fixtures, fixture)
		fixtureMu.Unlock()
		return process, nil
	}
	t.Cleanup(manager.Close)
	return manager, RuntimeStartRequest{DeviceID: credential.DeviceID, AccountID: accountID,
		CredentialInstanceID: credentialID, RuntimeProfileID: profileID, WorkspaceID: workspaceID}, store, spawnCount, fixtures
}

func runtimeManagerFixture(t *testing.T) (*RuntimeManager, RuntimeStartRequest, *storage.Store, *atomic.Int64, *[]*fixtureRuntime, context.Context) {
	t.Helper()
	return runtimeManagerFixtureAfterSetup(t, nil, 5*time.Second)
}

func runtimeManagerFixtureAfterSetup(t *testing.T, afterSetup func(), operationTimeout time.Duration) (*RuntimeManager, RuntimeStartRequest, *storage.Store, *atomic.Int64, *[]*fixtureRuntime, context.Context) {
	t.Helper()
	manager, request, store, spawnCount, fixtures := prepareRuntimeManagerFixture(t)
	if afterSetup != nil {
		afterSetup()
	}
	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	t.Cleanup(cancel)
	return manager, request, store, spawnCount, fixtures, ctx
}

func TestRuntimeManagerFixtureSetupDoesNotConsumeOperationBudget(t *testing.T) {
	const operationTimeout = time.Minute
	var setupFinished time.Time
	_, _, _, _, _, ctx := runtimeManagerFixtureAfterSetup(t, func() {
		setupFinished = time.Now()
	}, operationTimeout)
	deadline, ok := ctx.Deadline()
	if !ok || deadline.Before(setupFinished.Add(operationTimeout)) {
		t.Fatalf("operation budget started before setup completed: setup_finished=%v deadline=%v", setupFinished, deadline)
	}
}

func TestRuntimeManagerSharesCredentialRuntimeAndKeepsBindingsIndependent(t *testing.T) {
	manager, request, store, spawnCount, fixtures, ctx := runtimeManagerFixture(t)
	first, err := manager.Start(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.Start(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || first.ProviderSessionID == "" || second.ProviderSessionID == "" ||
		first.ProviderSessionID == second.ProviderSessionID || spawnCount.Load() != 1 || len(*fixtures) != 1 {
		t.Fatalf("sessions=%s/%s spawn=%d fixtures=%d", first.ID, second.ID, spawnCount.Load(), len(*fixtures))
	}
	fixture := (*fixtures)[0]
	if fixture.threads.Load() != 2 {
		t.Fatalf("thread starts=%d", fixture.threads.Load())
	}
	for range 2 {
		params := <-fixture.params
		if len(params) != 5 || string(params["approvalPolicy"]) != `"on-request"` ||
			string(params["sandbox"]) != `"workspace-write"` || string(params["ephemeral"]) != "false" ||
			string(params["model"]) != `"gpt-test"` || len(params["cwd"]) == 0 {
			t.Fatalf("daemon-owned thread params=%v", params)
		}
	}
	if err := manager.Stop(ctx, first.ID, false); err != nil {
		t.Fatal(err)
	}
	if fixture.kills.Load() != 0 {
		t.Fatal("stopping one binding killed the shared app-server")
	}
	if stopped, err := store.Session(ctx, first.ID); err != nil || stopped.Status != domain.SessionExited {
		t.Fatalf("first session=%+v err=%v", stopped, err)
	}
	if running, err := store.Session(ctx, second.ID); err != nil || running.Status != domain.SessionRunning {
		t.Fatalf("second session=%+v err=%v", running, err)
	}
	if err := manager.Input(ctx, RuntimeInputRequest{SessionID: second.ID, Payload: "hello"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Input(ctx, RuntimeInputRequest{SessionID: second.ID, Payload: "again"}); domain.CodeOf(err) != domain.CodeConflict {
		t.Fatalf("active-turn input err=%v", err)
	}
	if err := manager.Resize(second.ID); domain.CodeOf(err) != domain.CodeProviderControlUnsupported {
		t.Fatalf("resize err=%v", err)
	}
	if err := manager.Stop(ctx, second.ID, true); err != nil {
		t.Fatal(err)
	}
	if fixture.kills.Load() != 1 {
		t.Fatalf("last binding kill count=%d", fixture.kills.Load())
	}
	if killed, err := store.Session(ctx, second.ID); err != nil || killed.Status != domain.SessionKilled {
		t.Fatalf("second session=%+v err=%v", killed, err)
	}
}

func TestRuntimeManagerStartsReservedSessionOnceAndFailsPostReservationDrift(t *testing.T) {
	manager, request, store, spawnCount, fixtures, ctx := runtimeManagerFixture(t)
	executable := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\necho codex-cli 0.144.2\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	row := CompatibilityRows()[2]
	descriptor := BinaryDescriptor{Provider: ProviderName, Path: executable, Version: row.Version,
		Platform: "linux", Architecture: "amd64", SchemaFingerprint: row.SchemaFingerprint}
	capabilities, err := CapabilitiesFor(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	manager.Discover = func(context.Context) (BinaryDescriptor, CapabilitySet, error) {
		return descriptor, capabilities, nil
	}
	binaryFingerprint, err := BinaryFingerprint(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	newReserved := func() RuntimeReservedStartRequest {
		t.Helper()
		sessionID, idErr := domain.NewID("session")
		if idErr != nil {
			t.Fatal(idErr)
		}
		session := domain.Session{ID: sessionID, DeviceID: request.DeviceID, AccountID: request.AccountID,
			Provider: domain.ProviderCodex, CredentialInstanceID: request.CredentialInstanceID,
			RuntimeProfileID: request.RuntimeProfileID, WorkspaceID: request.WorkspaceID,
			Status: domain.SessionStarting, StartedAt: time.Now().UTC(),
			CapabilitySnapshot: []domain.Capability{domain.CapabilityProviderUsageRead, domain.CapabilitySessionControl}}
		if createErr := store.CreateSession(ctx, session); createErr != nil {
			t.Fatal(createErr)
		}
		return RuntimeReservedStartRequest{SessionID: sessionID, RuntimeStartRequest: request,
			ProviderVersion: descriptor.Version, BinaryFingerprint: binaryFingerprint,
			SchemaFingerprint: capabilities.SchemaFingerprint, CapabilityDigest: CapabilityDigest(capabilities)}
	}
	reserved := newReserved()
	start := make(chan struct{})
	results := make(chan domain.Session, 2)
	errors := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			result, startErr := manager.StartReserved(ctx, reserved)
			results <- result
			errors <- startErr
		}()
	}
	close(start)
	for range 2 {
		result, startErr := <-results, <-errors
		if startErr != nil || result.ID != reserved.SessionID || result.Status != domain.SessionRunning {
			t.Fatalf("reserved result=%+v err=%v", result, startErr)
		}
	}
	if spawnCount.Load() != 1 || len(*fixtures) != 1 || (*fixtures)[0].threads.Load() != 1 {
		t.Fatalf("reserved replay spawned=%d fixtures=%d threads=%d", spawnCount.Load(), len(*fixtures), (*fixtures)[0].threads.Load())
	}
	credentialBefore, err := store.CredentialInstance(ctx, request.CredentialInstanceID)
	if err != nil {
		t.Fatal(err)
	}
	drifted := newReserved()
	if err := os.WriteFile(executable, []byte("#!/bin/sh\necho replaced bytes\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.StartReserved(ctx, drifted); domain.CodeOf(err) != domain.CodeProviderVersionUnsupported {
		t.Fatalf("post-reservation drift code=%v err=%v", domain.CodeOf(err), err)
	}
	driftedSession, err := store.Session(ctx, drifted.SessionID)
	if err != nil || driftedSession.Status != domain.SessionFailed || driftedSession.FailureCode != string(domain.CodeProviderVersionUnsupported) {
		t.Fatalf("drifted Session=%+v err=%v", driftedSession, err)
	}
	original, err := store.Session(ctx, reserved.SessionID)
	if err != nil || original.Status != domain.SessionRunning {
		t.Fatalf("original Session=%+v err=%v", original, err)
	}
	credentialAfter, err := store.CredentialInstance(ctx, request.CredentialInstanceID)
	if err != nil || credentialAfter.CredentialRevision != credentialBefore.CredentialRevision {
		t.Fatalf("drift changed credential before=%+v after=%+v err=%v", credentialBefore, credentialAfter, err)
	}
	if spawnCount.Load() != 1 || (*fixtures)[0].threads.Load() != 1 {
		t.Fatalf("drift reached Provider spawn=%d threads=%d", spawnCount.Load(), (*fixtures)[0].threads.Load())
	}
	sharedRuntimeDrift := newReserved()
	replacementFingerprint, err := BinaryFingerprint(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	sharedRuntimeDrift.BinaryFingerprint = replacementFingerprint
	if _, err := manager.StartReserved(ctx, sharedRuntimeDrift); domain.CodeOf(err) != domain.CodeProviderVersionUnsupported {
		t.Fatalf("shared-runtime drift code=%v err=%v", domain.CodeOf(err), err)
	}
	sharedRuntimeDriftedSession, err := store.Session(ctx, sharedRuntimeDrift.SessionID)
	if err != nil || sharedRuntimeDriftedSession.Status != domain.SessionFailed ||
		sharedRuntimeDriftedSession.FailureCode != string(domain.CodeProviderVersionUnsupported) {
		t.Fatalf("shared-runtime drifted Session=%+v err=%v", sharedRuntimeDriftedSession, err)
	}
	if spawnCount.Load() != 1 || (*fixtures)[0].threads.Load() != 1 {
		t.Fatalf("shared-runtime drift reached Provider spawn=%d threads=%d", spawnCount.Load(), (*fixtures)[0].threads.Load())
	}
	if err := manager.Stop(ctx, reserved.SessionID, true); err != nil {
		t.Fatal(err)
	}
	descriptor.Platform, descriptor.Architecture = "darwin", "arm64"
	capabilities, err = CapabilitiesFor(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	binaryFingerprint, err = BinaryFingerprint(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	macOSReserved := newReserved()
	if _, err := manager.StartReserved(ctx, macOSReserved); domain.CodeOf(err) != domain.CodeProviderIdentityPending {
		t.Fatalf("macOS selector gate code=%v err=%v", domain.CodeOf(err), err)
	}
	macOSSession, err := store.Session(ctx, macOSReserved.SessionID)
	if err != nil || macOSSession.Status != domain.SessionFailed ||
		macOSSession.FailureCode != string(domain.CodeProviderIdentityPending) {
		t.Fatalf("macOS gated Session=%+v err=%v", macOSSession, err)
	}
	if spawnCount.Load() != 1 {
		t.Fatalf("macOS selector gate reached Provider spawn=%d", spawnCount.Load())
	}
}

func TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated(t *testing.T) {
	manager, requestA, store, spawnCount, fixtures, ctx := runtimeManagerFixture(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	accountB, credentialB, profileB, workspaceB := runtimeTestID(t, "account"), runtimeTestID(t, "credential"), runtimeTestID(t, "profile"), runtimeTestID(t, "workspace")
	if err := store.CreateAccount(ctx, domain.Account{ID: accountB, Provider: domain.ProviderCodex, DisplayName: "B",
		ProviderSubjectDigest: strings.Repeat("b", 64), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateCredentialInstance(ctx, domain.CredentialInstance{ID: credentialB, DeviceID: requestA.DeviceID,
		AccountID: accountB, Provider: domain.ProviderCodex, AuthMethod: domain.AuthMethodInteractive,
		SecretRef: "vault:" + string(credentialB), Status: domain.CredentialHealthy, CredentialRevision: 1,
		SecretDigest: strings.Repeat("b", 64), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRuntimeProfile(ctx, domain.RuntimeProfile{ID: profileB, DeviceID: requestA.DeviceID,
		AccountID: accountB, CredentialInstanceID: credentialB, Name: "B", Provider: domain.ProviderCodex,
		Settings: []byte(`{"approval_policy":"on-request","sandbox":"workspace-write"}`), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateWorkspace(ctx, domain.Workspace{ID: workspaceB, DeviceID: requestA.DeviceID,
		Path: t.TempDir(), Label: "B", Tags: []string{}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	requestB := RuntimeStartRequest{DeviceID: requestA.DeviceID, AccountID: accountB,
		CredentialInstanceID: credentialB, RuntimeProfileID: profileB, WorkspaceID: workspaceB}
	sessionA, err := manager.Start(ctx, requestA)
	if err != nil {
		t.Fatal(err)
	}
	sessionB, err := manager.Start(ctx, requestB)
	if err != nil {
		t.Fatal(err)
	}
	if sessionA.AccountID == sessionB.AccountID || sessionA.CredentialInstanceID == sessionB.CredentialInstanceID ||
		sessionA.ProviderSessionID == "" || sessionB.ProviderSessionID == "" || spawnCount.Load() != 2 || len(*fixtures) != 2 {
		t.Fatalf("A/B Sessions=%+v/%+v spawn=%d fixtures=%d", sessionA, sessionB, spawnCount.Load(), len(*fixtures))
	}
	(*fixtures)[0].setUsageResult(`{"dailyUsageBuckets":[{"startDate":"2026-07-16","tokens":11}],"summary":{"lifetimeTokens":11}}`)
	(*fixtures)[1].setUsageResult(`{"dailyUsageBuckets":[{"startDate":"2026-07-16","tokens":22}],"summary":{"lifetimeTokens":22}}`)
	usageA, err := manager.ReadUsage(ctx, requestA.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	usageB, err := manager.ReadUsage(ctx, requestB.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	if usageA.AccountID != requestA.AccountID || usageB.AccountID != requestB.AccountID ||
		usageA.RawReferenceHash == "" || usageB.RawReferenceHash == "" || usageA.RawReferenceHash == usageB.RawReferenceHash {
		t.Fatalf("A/B Usage=%+v/%+v", usageA, usageB)
	}
	credentialABefore, err := store.CredentialInstance(ctx, requestA.CredentialInstanceID)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Stop(ctx, sessionB.ID, true); err != nil {
		t.Fatal(err)
	}
	stillRunning, err := store.Session(ctx, sessionA.ID)
	if err != nil || stillRunning.Status != domain.SessionRunning {
		t.Fatalf("A changed after stopping B: %+v err=%v", stillRunning, err)
	}
	credentialAAfter, err := store.CredentialInstance(ctx, requestA.CredentialInstanceID)
	if err != nil || credentialAAfter.CredentialRevision != credentialABefore.CredentialRevision {
		t.Fatalf("A credential changed with B before=%+v after=%+v err=%v", credentialABefore, credentialAAfter, err)
	}
	if _, err := manager.ReadUsage(ctx, requestA.AccountID); err != nil {
		t.Fatalf("A Usage failed after stopping B: %v", err)
	}
	if err := manager.Stop(ctx, sessionA.ID, true); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeManagerRetiresStoppedTurnsWithoutFailingSharedRuntime(t *testing.T) {
	for _, test := range []struct {
		name   string
		killed bool
		reject bool
		status domain.SessionStatus
	}{
		{name: "graceful", status: domain.SessionExited},
		{name: "forced_after_interrupt_rejection", killed: true, reject: true, status: domain.SessionKilled},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager, request, store, _, fixtures, ctx := runtimeManagerFixture(t)
			first, err := manager.Start(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			second, err := manager.Start(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			if err := manager.Input(ctx, RuntimeInputRequest{SessionID: first.ID, Payload: "active turn"}); err != nil {
				t.Fatal(err)
			}
			fixture := (*fixtures)[0]
			fixture.interruptReject.Store(test.reject)
			if err := manager.Stop(ctx, first.ID, test.killed); err != nil {
				t.Fatal(err)
			}
			stopped, err := store.Session(ctx, first.ID)
			if err != nil || stopped.Status != test.status {
				t.Fatalf("stopped session=%+v err=%v", stopped, err)
			}
			if err := fixture.notify(MethodAgentMessageDelta, map[string]any{
				"threadId": "thread-1", "turnId": "turn-1", "itemId": "item-delayed", "delta": "late"}); err != nil {
				t.Fatal(err)
			}
			if err := fixture.notify(MethodTurnCompleted, map[string]any{
				"threadId": "thread-1", "turn": map[string]any{"id": "turn-1"}}); err != nil {
				t.Fatal(err)
			}
			fixture.interruptReject.Store(false)
			if err := manager.Input(ctx, RuntimeInputRequest{SessionID: second.ID, Payload: "still isolated"}); err != nil {
				t.Fatalf("surviving binding failed after delayed retired events: %v", err)
			}
			running, err := store.Session(ctx, second.ID)
			if err != nil || running.Status != domain.SessionRunning {
				t.Fatalf("surviving session=%+v err=%v", running, err)
			}
		})
	}
}

func TestRuntimeManagerGracefulInterruptRejectionRestoresBinding(t *testing.T) {
	manager, request, store, _, fixtures, ctx := runtimeManagerFixture(t)
	session, err := manager.Start(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Input(ctx, RuntimeInputRequest{SessionID: session.ID, Payload: "active turn"}); err != nil {
		t.Fatal(err)
	}
	(*fixtures)[0].interruptReject.Store(true)
	if err := manager.Stop(ctx, session.ID, false); domain.CodeOf(err) != domain.CodeProviderProtocolError {
		t.Fatalf("graceful interrupt rejection err=%v", err)
	}
	running, err := store.Session(ctx, session.ID)
	if err != nil || running.Status != domain.SessionRunning {
		t.Fatalf("restored session=%+v err=%v", running, err)
	}
}

func TestRuntimeManagerUnknownThreadStillFailsClosed(t *testing.T) {
	manager, request, store, _, fixtures, ctx := runtimeManagerFixture(t)
	session, err := manager.Start(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := (*fixtures)[0].notify(MethodTurnCompleted, map[string]any{
		"threadId": "thread-unknown", "turn": map[string]any{"id": "turn-unknown"}}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		failed, readErr := store.Session(ctx, session.ID)
		if readErr == nil && failed.Status == domain.SessionFailed {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("unknown thread did not fail the shared runtime closed")
}

func TestRuntimeManagerPersistsTruthfulUsageSuccessAndDegradation(t *testing.T) {
	for _, test := range []struct {
		name       string
		result     string
		reject     bool
		status     domain.UsageCapabilityStatus
		confidence domain.UsageConfidence
		errorCode  domain.ErrorCode
		window     string
		rawHash    bool
	}{
		{name: "supported", result: `{"dailyUsageBuckets":[{"startDate":"2026-07-15","tokens":42}],"summary":{"lifetimeTokens":42}}`,
			status: domain.UsageSupported, confidence: domain.UsageConfidenceHigh, window: "daily", rawHash: true},
		{name: "schema_changed", result: `{"dailyUsageBuckets":[{"startDate":"2026-07-15","tokenCount":42}],"summary":{"lifetimeTokens":42}}`,
			status: domain.UsageSchemaChanged, confidence: domain.UsageConfidenceLow, errorCode: domain.CodeProviderVersionUnsupported, window: "unknown"},
		{name: "provider_error", reject: true, status: domain.UsageError, confidence: domain.UsageConfidenceLow,
			errorCode: domain.CodeProviderProtocolError, window: "unknown"},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager, request, store, _, fixtures, ctx := runtimeManagerFixture(t)
			if _, err := manager.Start(ctx, request); err != nil {
				t.Fatal(err)
			}
			fixture := (*fixtures)[0]
			fixture.setUsageResult(test.result)
			fixture.usageReject.Store(test.reject)
			snapshot, err := manager.ReadUsage(ctx, request.AccountID)
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.CapabilityStatus != test.status || snapshot.Confidence != test.confidence ||
				snapshot.ErrorCode != test.errorCode || snapshot.WindowKind != test.window || snapshot.SourceVersion == "" ||
				snapshot.ProviderVersion != snapshot.SourceVersion || snapshot.CredentialInstanceID != request.CredentialInstanceID ||
				snapshot.StaleAt.IsZero() || !snapshot.StaleAt.Equal(snapshot.ObservedAt) || snapshot.ObservedAt.IsZero() ||
				(snapshot.RawReferenceHash != "") != test.rawHash {
				t.Fatalf("usage snapshot=%+v", snapshot)
			}
			expectedAvailability := domain.AvailabilityUnknown
			if test.status == domain.UsageSupported {
				expectedAvailability = domain.AvailabilityAvailable
			}
			if snapshot.Availability != expectedAvailability {
				t.Fatalf("usage availability=%s want=%s", snapshot.Availability, expectedAvailability)
			}
			stored, err := store.UsageSnapshot(ctx, snapshot.ID)
			if err != nil || stored.CapabilityStatus != test.status || stored.ErrorCode != test.errorCode ||
				stored.CredentialInstanceID != request.CredentialInstanceID || stored.ProviderVersion != snapshot.ProviderVersion ||
				stored.Availability != snapshot.Availability || !stored.StaleAt.Equal(snapshot.StaleAt) {
				t.Fatalf("stored usage=%+v err=%v", stored, err)
			}
		})
	}
}

func TestCodexV1ProfileRejectsUnknownAndDangerFullAccess(t *testing.T) {
	for _, raw := range []string{
		`{"approval_policy":"never","sandbox":"danger-full-access"}`,
		`{"approval_policy":"never","sandbox":"read-only","environment":{"TOKEN":"x"}}`,
		`{"model":"` + strings.Repeat("x", 129) + `","approval_policy":"never","sandbox":"read-only"}`,
	} {
		if _, err := decodeProfileSettings([]byte(raw)); err == nil {
			t.Fatalf("profile unexpectedly accepted: %q", raw)
		}
	}
}

func TestRuntimeManagerChildCrashFailsAllBindingsOnce(t *testing.T) {
	manager, request, store, _, fixtures, ctx := runtimeManagerFixture(t)
	first, err := manager.Start(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.Start(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	fixture := (*fixtures)[0]
	fixture.crash()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		left, leftErr := store.Session(ctx, first.ID)
		right, rightErr := store.Session(ctx, second.ID)
		if leftErr == nil && rightErr == nil && left.Status == domain.SessionFailed && right.Status == domain.SessionFailed {
			if left.FailureCode != string(domain.CodeProviderFailed) || right.FailureCode != string(domain.CodeProviderFailed) {
				t.Fatalf("failure codes=%s/%s", left.FailureCode, right.FailureCode)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("shared child crash did not fail both bindings")
}

func TestRuntimeManagerApprovalDispatchPersistsOnlyAfterProviderWrite(t *testing.T) {
	manager, request, store, _, fixtures, ctx := runtimeManagerFixture(t)
	session, err := manager.Start(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Input(ctx, RuntimeInputRequest{SessionID: session.ID, Payload: "needs approval"}); err != nil {
		t.Fatal(err)
	}
	fixture := (*fixtures)[0]
	if err := WriteFrame(fixture.server, RPCRequest{JSONRPC: "2.0", ID: 99, Method: MethodApprovalCommand,
		Params: map[string]any{"threadId": "thread-1", "turnId": "turn-1", "itemId": "item-1", "startedAtMs": 1}}); err != nil {
		t.Fatal(err)
	}
	var approval domain.Approval
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		approvals, readErr := store.ListApprovals(ctx, session.ID)
		if readErr == nil && len(approvals) == 1 {
			approval = approvals[0]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if approval.ID == "" || approval.Status != domain.ApprovalPending || approval.ResponseState != domain.ApprovalResponseIdle {
		t.Fatalf("pending approval=%+v", approval)
	}
	responderID, _ := domain.NewID("client")
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: responderID, Name: "controller", PublicKey: make([]byte, 32),
		Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityApprovalRespond}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	completed, err := manager.RespondApproval(ctx, ApprovalDispatchRequest{SessionID: session.ID, ApprovalID: approval.ID,
		ProviderApprovalID: approval.ProviderApprovalID, ResponderID: responderID, ResponseKey: "response-1",
		Decision: domain.ApprovalDecisionApprove, LeaseRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != domain.ApprovalApproved || completed.ResponseState != domain.ApprovalResponseWritten {
		t.Fatalf("completed approval=%+v", completed)
	}
	select {
	case response := <-fixture.responses:
		if string(response) != `{"decision":"accept"}` {
			t.Fatalf("provider response=%s", response)
		}
	case <-ctx.Done():
		t.Fatal("provider did not receive approval response")
	}
	replayed, err := manager.RespondApproval(ctx, ApprovalDispatchRequest{SessionID: session.ID, ApprovalID: approval.ID,
		ProviderApprovalID: approval.ProviderApprovalID, ResponderID: responderID, ResponseKey: "response-1",
		Decision: domain.ApprovalDecisionApprove, LeaseRevision: 1})
	if err != nil || replayed.ID != completed.ID || replayed.ResponseState != domain.ApprovalResponseWritten {
		t.Fatalf("approval replay=%+v err=%v", replayed, err)
	}
	select {
	case response := <-fixture.responses:
		t.Fatalf("duplicate provider response=%s", response)
	default:
	}
	if _, err := manager.RespondApproval(ctx, ApprovalDispatchRequest{SessionID: session.ID, ApprovalID: approval.ID,
		ProviderApprovalID: approval.ProviderApprovalID, ResponderID: responderID, ResponseKey: "different-key",
		Decision: domain.ApprovalDecisionApprove, LeaseRevision: 1}); domain.CodeOf(err) != domain.CodeConflict {
		t.Fatalf("different digest replay err=%v", err)
	}
}

func TestRuntimeManagerApprovalDecisionTable(t *testing.T) {
	for _, method := range []string{MethodApprovalCommand, MethodApprovalFileChange} {
		for _, decision := range []struct {
			local    domain.ApprovalDecision
			provider string
			status   domain.ApprovalStatus
		}{
			{local: domain.ApprovalDecisionApprove, provider: "accept", status: domain.ApprovalApproved},
			{local: domain.ApprovalDecisionDeny, provider: "decline", status: domain.ApprovalDenied},
			{local: domain.ApprovalDecisionCancel, provider: "cancel", status: domain.ApprovalCancelled},
		} {
			t.Run(method+"_"+string(decision.local), func(t *testing.T) {
				manager, request, store, _, fixtures, ctx := runtimeManagerFixture(t)
				session, err := manager.Start(ctx, request)
				if err != nil {
					t.Fatal(err)
				}
				if err := manager.Input(ctx, RuntimeInputRequest{SessionID: session.ID, Payload: "decision table"}); err != nil {
					t.Fatal(err)
				}
				fixture := (*fixtures)[0]
				if err := fixture.serverRequest(200, method, map[string]any{
					"threadId": "thread-1", "turnId": "turn-1", "itemId": "item-table", "startedAtMs": 1}); err != nil {
					t.Fatal(err)
				}
				var approval domain.Approval
				deadline := time.Now().Add(2 * time.Second)
				for time.Now().Before(deadline) {
					approvals, _ := store.ListApprovals(ctx, session.ID)
					if len(approvals) == 1 {
						approval = approvals[0]
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if approval.ID == "" {
					t.Fatal("approval was not persisted")
				}
				responderID, _ := domain.NewID("client")
				now := time.Now().UTC().Truncate(time.Microsecond)
				if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: responderID, Name: "controller", PublicKey: make([]byte, 32),
					Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityApprovalRespond}, CreatedAt: now, UpdatedAt: now}); err != nil {
					t.Fatal(err)
				}
				completed, err := manager.RespondApproval(ctx, ApprovalDispatchRequest{SessionID: session.ID, ApprovalID: approval.ID,
					ProviderApprovalID: approval.ProviderApprovalID, ResponderID: responderID, ResponseKey: "table-" + string(decision.local),
					Decision: decision.local, LeaseRevision: 1})
				if err != nil || completed.Status != decision.status || completed.ResponseState != domain.ApprovalResponseWritten {
					t.Fatalf("completed approval=%+v err=%v", completed, err)
				}
				select {
				case response := <-fixture.responses:
					if string(response) != `{"decision":"`+decision.provider+`"}` {
						t.Fatalf("provider response=%s", response)
					}
				case <-ctx.Done():
					t.Fatal("provider did not receive decision-table response")
				}
			})
		}
	}
}

func TestRuntimeManagerPermissionsApprovalFailsClosedWithoutResponse(t *testing.T) {
	manager, request, store, _, fixtures, ctx := runtimeManagerFixture(t)
	session, err := manager.Start(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Input(ctx, RuntimeInputRequest{SessionID: session.ID, Payload: "permissions"}); err != nil {
		t.Fatal(err)
	}
	fixture := (*fixtures)[0]
	if err := fixture.serverRequest(201, MethodApprovalPermissions, map[string]any{"threadId": "thread-1", "turnId": "turn-1",
		"itemId": "item-permissions", "startedAtMs": 1, "permissions": map[string]any{"network": true}}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		failed, readErr := store.Session(ctx, session.ID)
		if readErr == nil && failed.Status == domain.SessionFailed {
			approvals, _ := store.ListApprovals(ctx, session.ID)
			if len(approvals) != 0 {
				t.Fatalf("disabled permissions approval was persisted: %+v", approvals)
			}
			select {
			case response := <-fixture.responses:
				t.Fatalf("disabled permissions response reached provider: %s", response)
			default:
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("permissions approval did not fail closed")
}

func TestRuntimeManagerRejectsPersistentAndPolicyApprovalVariants(t *testing.T) {
	manager, request, store, _, fixtures, ctx := runtimeManagerFixture(t)
	session, err := manager.Start(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Input(ctx, RuntimeInputRequest{SessionID: session.ID, Payload: "variant"}); err != nil {
		t.Fatal(err)
	}
	fixture := (*fixtures)[0]
	if err := fixture.serverRequest(202, MethodApprovalCommand, map[string]any{
		"threadId": "thread-1", "turnId": "turn-1", "itemId": "item-variant", "startedAtMs": 1}); err != nil {
		t.Fatal(err)
	}
	var approval domain.Approval
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		approvals, _ := store.ListApprovals(ctx, session.ID)
		if len(approvals) == 1 {
			approval = approvals[0]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	responderID, _ := domain.NewID("client")
	for _, variant := range []domain.ApprovalDecision{"approve_for_session", "update_policy"} {
		if _, err := manager.RespondApproval(ctx, ApprovalDispatchRequest{SessionID: session.ID, ApprovalID: approval.ID,
			ProviderApprovalID: approval.ProviderApprovalID, ResponderID: responderID, ResponseKey: string(variant),
			Decision: variant, LeaseRevision: 1}); domain.CodeOf(err) != domain.CodeInvalidArgument {
			t.Fatalf("variant %q err=%v", variant, err)
		}
	}
	stored, err := store.Approval(ctx, approval.ID)
	if err != nil || stored.Status != domain.ApprovalPending || stored.ResponseState != domain.ApprovalResponseIdle {
		t.Fatalf("variant changed approval=%+v err=%v", stored, err)
	}
	select {
	case response := <-fixture.responses:
		t.Fatalf("disabled variant response reached provider: %s", response)
	default:
	}
}

func TestRuntimeManagerRejectsProviderPolicyAmendmentsWithoutResponse(t *testing.T) {
	for _, amendment := range []struct {
		name  string
		field string
		value any
	}{
		{name: "exec_policy", field: "proposedExecpolicyAmendment", value: map[string]any{"rule": "allow"}},
		{name: "network_policy", field: "proposedNetworkPolicyAmendments", value: []any{map[string]any{"host": "example.invalid"}}},
	} {
		t.Run(amendment.name, func(t *testing.T) {
			manager, request, store, _, fixtures, ctx := runtimeManagerFixture(t)
			session, err := manager.Start(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			if err := manager.Input(ctx, RuntimeInputRequest{SessionID: session.ID, Payload: "policy amendment"}); err != nil {
				t.Fatal(err)
			}
			fixture := (*fixtures)[0]
			params := map[string]any{"threadId": "thread-1", "turnId": "turn-1", "itemId": "item-amendment", "startedAtMs": 1,
				amendment.field: amendment.value}
			if err := fixture.serverRequest(203, MethodApprovalCommand, params); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(time.Second)
			for time.Now().Before(deadline) {
				failed, readErr := store.Session(ctx, session.ID)
				if readErr == nil && failed.Status == domain.SessionFailed {
					approvals, _ := store.ListApprovals(ctx, session.ID)
					if len(approvals) != 0 {
						t.Fatalf("disabled policy amendment was persisted: %+v", approvals)
					}
					select {
					case response := <-fixture.responses:
						t.Fatalf("disabled policy response reached provider: %s", response)
					default:
					}
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Fatal("policy amendment did not fail closed")
		})
	}
}

func TestRuntimeManagerApprovalWriteFailureBecomesAmbiguous(t *testing.T) {
	manager, request, store, _, fixtures, ctx := runtimeManagerFixture(t)
	session, err := manager.Start(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Input(ctx, RuntimeInputRequest{SessionID: session.ID, Payload: "needs approval"}); err != nil {
		t.Fatal(err)
	}
	fixture := (*fixtures)[0]
	if err := WriteFrame(fixture.server, RPCRequest{JSONRPC: "2.0", ID: 100, Method: MethodApprovalFileChange,
		Params: map[string]any{"threadId": "thread-1", "turnId": "turn-1", "itemId": "item-2", "startedAtMs": 1}}); err != nil {
		t.Fatal(err)
	}
	var approval domain.Approval
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		approvals, _ := store.ListApprovals(ctx, session.ID)
		if len(approvals) == 1 {
			approval = approvals[0]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	responderID, _ := domain.NewID("client")
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: responderID, Name: "controller", PublicKey: make([]byte, 32),
		Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityApprovalRespond}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	fixture.failWrites.Store(true)
	_, err = manager.RespondApproval(ctx, ApprovalDispatchRequest{SessionID: session.ID, ApprovalID: approval.ID,
		ProviderApprovalID: approval.ProviderApprovalID, ResponderID: responderID, ResponseKey: "response-2",
		Decision: domain.ApprovalDecisionCancel, LeaseRevision: 1})
	if domain.CodeOf(err) != domain.CodeApprovalDispatchAmbiguous {
		t.Fatalf("dispatch error=%v", err)
	}
	stored, err := store.Approval(ctx, approval.ID)
	if err != nil || stored.Status != domain.ApprovalExpired || stored.ResponseState != domain.ApprovalResponseAmbiguous ||
		stored.RequestedDecision != domain.ApprovalDecisionCancel {
		t.Fatalf("ambiguous approval=%+v err=%v", stored, err)
	}
}

func TestRuntimeManagerBlockedApprovalWriteIsBoundedAndCannotReplay(t *testing.T) {
	manager, request, store, _, fixtures, ctx := runtimeManagerFixture(t)
	session, err := manager.Start(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Input(ctx, RuntimeInputRequest{SessionID: session.ID, Payload: "needs approval"}); err != nil {
		t.Fatal(err)
	}
	fixture := (*fixtures)[0]
	if err := WriteFrame(fixture.server, RPCRequest{JSONRPC: "2.0", ID: 101, Method: MethodApprovalCommand,
		Params: map[string]any{"threadId": "thread-1", "turnId": "turn-1", "itemId": "item-3", "startedAtMs": 1}}); err != nil {
		t.Fatal(err)
	}
	var approval domain.Approval
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		approvals, _ := store.ListApprovals(ctx, session.ID)
		if len(approvals) == 1 {
			approval = approvals[0]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if approval.ID == "" {
		t.Fatal("approval was not persisted")
	}
	responderID, _ := domain.NewID("client")
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: responderID, Name: "controller", PublicKey: make([]byte, 32),
		Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityApprovalRespond}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	fixture.blockWrites.Store(true)
	dispatch := ApprovalDispatchRequest{SessionID: session.ID, ApprovalID: approval.ID,
		ProviderApprovalID: approval.ProviderApprovalID, ResponderID: responderID, ResponseKey: "response-blocked",
		Decision: domain.ApprovalDecisionDeny, LeaseRevision: 1}
	started := time.Now()
	_, err = manager.RespondApproval(context.Background(), dispatch)
	if domain.CodeOf(err) != domain.CodeApprovalDispatchAmbiguous {
		t.Fatalf("dispatch error=%v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("blocked write was not bounded: %s", elapsed)
	}
	stored, err := store.Approval(ctx, approval.ID)
	if err != nil || stored.Status != domain.ApprovalExpired || stored.ResponseState != domain.ApprovalResponseAmbiguous ||
		stored.RequestedDecision != domain.ApprovalDecisionDeny {
		t.Fatalf("ambiguous approval=%+v err=%v", stored, err)
	}
	if _, err := manager.RespondApproval(ctx, dispatch); domain.CodeOf(err) != domain.CodeApprovalDispatchAmbiguous {
		t.Fatalf("ambiguous replay err=%v", err)
	}
	select {
	case response := <-fixture.responses:
		t.Fatalf("blocked response reached provider: %s", response)
	default:
	}
}
