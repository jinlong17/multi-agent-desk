package device_test

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/app"
	device "github.com/jinlong17/multi-agent-desk/internal/device"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
	runtimepkg "github.com/jinlong17/multi-agent-desk/internal/runtime"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
)

func TestNativeTwoClientFakeSessionControl(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	root := filepath.Join(t.TempDir(), "device-root")
	bootstrap, err := device.Bootstrap(ctx, root, "native-e2e", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(ctx, device.DeviceDatabasePath(root))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC().Truncate(time.Microsecond)
	second, _, err := device.CreateClient(ctx, store, "observer", []domain.Capability{domain.CapabilityMetadataRead, domain.CapabilitySessionObserve}, now)
	if err != nil {
		t.Fatal(err)
	}
	workspaceID, profileID, credentialID := nativeTestID(t, "workspace"), nativeTestID(t, "profile"), nativeTestID(t, "credential")
	if err := store.CreateWorkspace(ctx, domain.Workspace{ID: workspaceID, DeviceID: bootstrap.DeviceID, Path: t.TempDir(), Label: "native", Tags: []string{}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRuntimeProfile(ctx, domain.RuntimeProfile{ID: profileID, DeviceID: bootstrap.DeviceID, Name: "fake", Provider: "fake", Settings: []byte(`{}`), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateCredentialInstance(ctx, domain.CredentialInstance{ID: credentialID, DeviceID: bootstrap.DeviceID, Provider: "fake", AuthMethod: "fake", SecretRef: "fake:native", Status: domain.CredentialHealthy, CredentialRevision: 1, SecretDigest: "0123456789012345678901234567890123456789012345678901234567890123", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}

	executable := filepath.Join(t.TempDir(), "multidesk")
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	build := exec.Command("go", "build", "-o", executable, "../../cmd/multidesk")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build daemon binary: %v\n%s", err, output)
	}
	listener, err := device.Listen(root)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	daemon, err := device.LoadDaemonIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	authenticator, err := device.NewServerAuthenticator(daemon, mustEndpointInstance(t), store)
	if err != nil {
		t.Fatal(err)
	}
	manager := runtimepkg.NewManager(store, executable)
	defer manager.Close()
	service := app.NewSessionService(store, manager)
	server := &device.Server{Listener: listener, Authenticator: authenticator, Authorizer: (app.Authorizer{Clients: store}).Authorize, Handler: service}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(ctx) }()

	owner := LoadOwnerForE2E(t, root)
	ownerConnection, err := device.Dial(root, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer ownerConnection.Close()
	ownerAuth, err := (device.ClientAuthenticator{Identity: owner, RequestedCapabilities: e2eOwnerCapabilities}).Handshake(ctx, ownerConnection)
	if err != nil {
		t.Fatal(err)
	}
	ownerClient := &device.Client{Connection: ownerConnection, Auth: ownerAuth}
	observerConnection, err := device.Dial(root, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer observerConnection.Close()
	observerAuth, err := (device.ClientAuthenticator{Identity: second, RequestedCapabilities: []domain.Capability{domain.CapabilityMetadataRead, domain.CapabilitySessionObserve}}).Handshake(ctx, observerConnection)
	if err != nil {
		t.Fatal(err)
	}
	observerClient := &device.Client{Connection: observerConnection, Auth: observerAuth}

	startBody, _ := device.JSONBody(map[string]any{"device_id": bootstrap.DeviceID, "credential_instance_id": credentialID, "runtime_profile_id": profileID, "workspace_id": workspaceID, "capabilities": []domain.Capability{domain.CapabilitySessionResume}})
	start := mustCall(t, ownerClient, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "native-start-1", Method: "sessions.start", IdempotencyKey: "native-start-key", Body: startBody})
	var started struct {
		ID     domain.ID            `json:"id"`
		Status domain.SessionStatus `json:"status"`
	}
	decodeResult(t, start, &started)
	if started.Status != domain.SessionRunning {
		t.Fatalf("start status = %s", started.Status)
	}
	startReplay := mustCall(t, ownerClient, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "native-start-2", Method: "sessions.start", IdempotencyKey: "native-start-key", Body: startBody})
	var replayed struct {
		ID domain.ID `json:"id"`
	}
	decodeResult(t, startReplay, &replayed)
	if replayed.ID != started.ID {
		t.Fatalf("idempotent start IDs differ: %s vs %s", started.ID, replayed.ID)
	}

	observeBody, _ := device.JSONBody(map[string]any{"session_id": started.ID})
	mustCall(t, observerClient, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "native-list-1", Method: "sessions.list"})
	mustCall(t, observerClient, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "native-attach-1", Method: "sessions.attach", IdempotencyKey: "native-attach-key", Body: observeBody})
	mustCall(t, observerClient, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "native-observe-1", Method: "sessions.observe", Body: observeBody})
	response, observerErr := observerClient.Call(context.Background(), device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "native-observer-stop", Method: "sessions.stop", LeaseRevision: int64Ptr(1), Body: observeBody})
	if observerErr == nil || response.OK || response.Error == nil || domain.CodeOf(observerErr) != domain.CodeUnauthenticated {
		t.Fatal("observer mutation unexpectedly succeeded")
	}

	lease := mustCall(t, ownerClient, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "native-acquire-1", Method: "control.acquire", IdempotencyKey: "native-acquire-key", Body: observeBody})
	var leaseResult struct {
		Revision int64 `json:"revision"`
	}
	decodeResult(t, lease, &leaseResult)
	inputBody, _ := device.JSONBody(map[string]any{"session_id": started.ID, "sequence": 1, "payload": "hello"})
	mustCall(t, ownerClient, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "native-input-1", Method: "terminal.input", IdempotencyKey: "native-input-key", LeaseRevision: &leaseResult.Revision, Body: inputBody})
	resizeBody, _ := device.JSONBody(map[string]any{"session_id": started.ID, "rows": 24, "cols": 80})
	mustCall(t, ownerClient, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "native-resize-1", Method: "terminal.resize", IdempotencyKey: "native-resize-key", LeaseRevision: &leaseResult.Revision, Body: resizeBody})
	mustCall(t, ownerClient, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "native-stop-1", Method: "sessions.stop", IdempotencyKey: "native-stop-key", LeaseRevision: &leaseResult.Revision, Body: observeBody})
	resumed := mustCall(t, ownerClient, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "native-resume-1", Method: "sessions.resume", IdempotencyKey: "native-resume-key", Body: observeBody})
	var resumedResult struct {
		ID          domain.ID `json:"id"`
		ResumedFrom domain.ID `json:"resumed_from_session_id"`
	}
	decodeResult(t, resumed, &resumedResult)
	if resumedResult.ID == started.ID || resumedResult.ResumedFrom != started.ID {
		t.Fatalf("resume linkage = %+v", resumedResult)
	}
	resumedBody, _ := device.JSONBody(map[string]any{"session_id": resumedResult.ID})
	newLease := mustCall(t, ownerClient, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "native-acquire-2", Method: "control.acquire", IdempotencyKey: "native-acquire-key-2", Body: resumedBody})
	var newLeaseResult struct {
		Revision int64 `json:"revision"`
	}
	decodeResult(t, newLease, &newLeaseResult)
	mustCall(t, ownerClient, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "native-kill-1", Method: "sessions.kill", IdempotencyKey: "native-kill-key", LeaseRevision: &newLeaseResult.Revision, Body: resumedBody})
	cancel()
	server.Close()
	select {
	case <-serveErr:
	case <-time.After(10 * time.Second):
		t.Fatal("native daemon did not stop")
	}
}

func nativeTestID(t *testing.T, prefix string) domain.ID {
	t.Helper()
	id, err := domain.NewID(prefix)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustEndpointInstance(t *testing.T) []byte {
	t.Helper()
	instance, err := device.NewEndpointInstance()
	if err != nil {
		t.Fatal(err)
	}
	return instance
}

func LoadOwnerForE2E(t *testing.T, root string) device.ClientIdentity {
	t.Helper()
	identity, err := device.LoadOwnerIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	return identity
}

func mustCall(t *testing.T, client *device.Client, request device.Request) device.Response {
	t.Helper()
	response := call(t, client, request)
	if !response.OK {
		t.Fatalf("%s failed: %+v", request.Method, response.Error)
	}
	return response
}

func call(t *testing.T, client *device.Client, request device.Request) device.Response {
	t.Helper()
	response, err := client.Call(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func decodeResult(t *testing.T, response device.Response, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Result, target); err != nil {
		t.Fatal(err)
	}
}

func int64Ptr(value int64) *int64 { return &value }

var e2eOwnerCapabilities = []domain.Capability{
	domain.CapabilityMetadataRead, domain.CapabilitySessionObserve, domain.CapabilitySessionStart,
	domain.CapabilitySessionControlAcquire, domain.CapabilitySessionControl, domain.CapabilityTerminalControl,
	domain.CapabilitySessionResume,
}
