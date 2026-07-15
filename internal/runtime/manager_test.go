package runtime

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
)

func runtimeID(t *testing.T, prefix string) domain.ID {
	t.Helper()
	id, err := domain.NewID(prefix)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func runtimeFixture(t *testing.T) (*storage.Store, StartRequest, domain.ID) {
	t.Helper()
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "device", "device.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	now := time.Now().UTC().Truncate(time.Microsecond)
	deviceID, clientID := runtimeID(t, "device"), runtimeID(t, "client")
	if err := store.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "test", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: clientID, Name: "client", PublicKey: make([]byte, 32), Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilitySessionResume}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	workspaceID, profileID, credentialID := runtimeID(t, "workspace"), runtimeID(t, "profile"), runtimeID(t, "credential")
	if err := store.CreateWorkspace(ctx, domain.Workspace{ID: workspaceID, DeviceID: deviceID, Path: t.TempDir(), Label: "test", Tags: []string{}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRuntimeProfile(ctx, domain.RuntimeProfile{ID: profileID, DeviceID: deviceID, Name: "fake", Provider: "fake", Settings: []byte(`{}`), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateCredentialInstance(ctx, domain.CredentialInstance{ID: credentialID, DeviceID: deviceID, Provider: "fake", AuthMethod: "fake", SecretRef: "fake:test", Status: domain.CredentialHealthy, CredentialRevision: 1, SecretDigest: "0123456789012345678901234567890123456789012345678901234567890123", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	return store, StartRequest{DeviceID: deviceID, CredentialInstanceID: credentialID, RuntimeProfileID: profileID, WorkspaceID: workspaceID, Capabilities: []domain.Capability{domain.CapabilitySessionResume}}, clientID
}

func TestManagerRunsRealFakeProviderSubprocess(t *testing.T) {
	ctx := context.Background()
	store, request, clientID := runtimeFixture(t)
	executable := filepath.Join(t.TempDir(), "multidesk")
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	build := exec.Command("go", "build", "-o", executable, "../../cmd/multidesk")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fake provider binary: %v\n%s", err, output)
	}
	manager := NewManager(store, executable)
	manager.StopTimeout = 3 * time.Second
	defer manager.Close()
	session, err := manager.StartFake(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if session.Status != domain.SessionRunning {
		t.Fatalf("status = %s", session.Status)
	}
	lease, err := manager.Acquire(ctx, session.ID, clientID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Input(ctx, InputRequest{SessionID: session.ID, ClientID: clientID, Revision: lease.Revision, Sequence: 1, Payload: "hello"}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Input(ctx, InputRequest{SessionID: session.ID, ClientID: clientID, Revision: lease.Revision, Sequence: 1, Payload: "hello"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Resize(ctx, ResizeRequest{SessionID: session.ID, ClientID: clientID, Revision: lease.Revision, Rows: 24, Cols: 80}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	replay, err := manager.Replay(ctx, session.ID, 1)
	if err != nil && domain.CodeOf(err) != domain.CodeReplayUnavailable {
		t.Fatal(err)
	}
	if len(replay.Chunks) == 0 {
		t.Fatal("expected retained fake provider output")
	}
	stopped, err := manager.Stop(ctx, session.ID, clientID, lease.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Status != domain.SessionExited {
		t.Fatalf("stop status = %s", stopped.Status)
	}
	resumed, err := manager.Resume(ctx, stopped.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.ID == stopped.ID || resumed.ResumedFromSessionID != stopped.ID || resumed.Status != domain.SessionRunning {
		t.Fatalf("resume = %+v", resumed)
	}
	if _, err := manager.Kill(ctx, resumed.ID, clientID, lease.Revision); err == nil {
		t.Fatal("expected new lease to be required for resumed session")
	}
	newLease, err := manager.Acquire(ctx, resumed.ID, clientID)
	if err != nil {
		t.Fatal(err)
	}
	killed, err := manager.Kill(ctx, resumed.ID, clientID, newLease.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if killed.Status != domain.SessionKilled {
		t.Fatalf("kill status = %s", killed.Status)
	}
}
