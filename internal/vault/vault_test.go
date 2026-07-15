package vault

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
)

func vaultID(t *testing.T, prefix string) domain.ID {
	t.Helper()
	id, err := domain.NewID(prefix)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestManagerLockUnlockDoesNotRetainSecret(t *testing.T) {
	manager := NewManager()
	if manager.Status() != StateLocked || manager.RequireUnlocked() == nil {
		t.Fatal("new vault was not locked")
	}
	if err := manager.Unlock([]byte("test-only-secret")); err != nil {
		t.Fatal(err)
	}
	if manager.Status() != StateUnlocked || manager.Epoch() != 1 {
		t.Fatalf("unlocked state=%s epoch=%d", manager.Status(), manager.Epoch())
	}
	if err := manager.Lock(); err != nil || manager.Status() != StateLocked {
		t.Fatalf("lock failed: %v", err)
	}
	if NewManager().Status() != StateLocked {
		t.Fatal("new daemon vault did not reset to locked")
	}
}

func TestMaterializerAtomicCommitAndQuarantine(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "device", "device.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC().Truncate(time.Microsecond)
	deviceID, credentialID, leaseID := vaultID(t, "device"), vaultID(t, "credential"), vaultID(t, "lease")
	if err := store.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "vault", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateCredentialInstance(ctx, domain.CredentialInstance{ID: credentialID, DeviceID: deviceID, Provider: "fake", AuthMethod: "fake", SecretRef: "fake:vault", Status: domain.CredentialHealthy, CredentialRevision: 1, SecretDigest: "0123456789012345678901234567890123456789012345678901234567890123", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "runtime-home")
	manager := NewManager()
	if err := manager.Unlock([]byte("unlock")); err != nil {
		t.Fatal(err)
	}
	materializer := NewMaterializer(store, root, manager)
	materialization, err := materializer.Materialize(ctx, MaterializationRequest{LeaseID: leaseID, CredentialInstanceID: credentialID, CredentialRevision: 1, Content: []byte("fake-credential"), RefCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if materialization.State != domain.MaterializationActive {
		t.Fatalf("state=%s", materialization.State)
	}
	contentPath := filepath.Join(root, "leases", string(leaseID), "credential.fake")
	info, err := os.Stat(contentPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("credential mode=%o", info.Mode().Perm())
	}
	repeated, err := materializer.Materialize(ctx, MaterializationRequest{LeaseID: leaseID, CredentialInstanceID: credentialID, CredentialRevision: 1, Content: []byte("fake-credential"), RefCount: 1})
	if err != nil || repeated.ManifestDigest != materialization.ManifestDigest {
		t.Fatalf("idempotent materialization: %+v %v", repeated, err)
	}
	if _, err := materializer.Materialize(ctx, MaterializationRequest{LeaseID: vaultID(t, "lease"), CredentialInstanceID: credentialID, CredentialRevision: 2, Content: []byte("stale"), RefCount: 1}); domain.CodeOf(err) != domain.CodeMaterializationConflict {
		t.Fatalf("stale revision code=%v", domain.CodeOf(err))
	}
	manifestPath := filepath.Join(root, "leases", string(leaseID), "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"manifest_version":1,"lease_id":"bad"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := materializer.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	quarantineEntries, err := os.ReadDir(filepath.Join(root, "quarantine"))
	if err != nil || len(quarantineEntries) != 1 {
		t.Fatalf("quarantine entries=%d err=%v", len(quarantineEntries), err)
	}
}
