package vault

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
	_ "modernc.org/sqlite"
)

func remoteIdentityVault(t *testing.T) (*storage.Store, *Manager, string, time.Time) {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "device", "device.db")
	store, err := storage.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	now := time.Unix(1_800_000_000, 123_000_000).UTC()
	deviceID, clientID := vaultID(t, "device"), vaultID(t, "client")
	if err := store.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "remote", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: clientID, Name: "owner", PublicKey: bytes.Repeat([]byte{1}, 32), Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityVaultControl}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	manager, err := NewPersistentManager(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	password := []byte("remote-envelope-test-password")
	if _, err := manager.Initialize(ctx, clientID, "remote-envelope-init", password, now); err != nil {
		t.Fatal(err)
	}
	if err := manager.Unlock(password); err != nil {
		t.Fatal(err)
	}
	return store, manager, path, now
}

func TestRemoteIdentityEnvelopeOriginReuseActivationAndLockedGate(t *testing.T) {
	ctx := context.Background()
	store, manager, _, now := remoteIdentityVault(t)
	origin := "https://control.example.test"
	first, err := manager.PrepareRemoteIdentity(ctx, origin, RemoteIdentityOptions{}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer first.ZeroPrivateMaterial()
	if first.Record.Lifecycle != storage.RemoteIdentityPending || first.Record.RecordRevision != 1 || first.Envelope.Status != DeviceKeyEnvelopePending {
		t.Fatalf("pending identity=%+v envelope=%+v", first.Record, first.Envelope)
	}
	if first.Record.ID[:16] != "remote_identity_" || first.Record.ServerOrigin != origin || first.Envelope.ServerOrigin != origin {
		t.Fatalf("identity namespace/origin mismatch: %+v", first.Record)
	}
	mapping, err := store.ControlPlaneMapping(ctx, "device", first.Record.ID)
	if err != nil || mapping.ServerID != first.Record.ServerDeviceID {
		t.Fatalf("mapping=%+v err=%v", mapping, err)
	}
	replayed, err := manager.PrepareRemoteIdentity(ctx, origin, RemoteIdentityOptions{}, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer replayed.ZeroPrivateMaterial()
	if replayed.Record.ID != first.Record.ID || replayed.Record.ServerDeviceID != first.Record.ServerDeviceID || !bytes.Equal(replayed.Ed25519Seed, first.Ed25519Seed) || !bytes.Equal(replayed.X25519PrivateKey, first.X25519PrivateKey) {
		t.Fatal("pending prepare did not reuse the same exact identity")
	}
	other, err := manager.PrepareRemoteIdentity(ctx, "https://other.example.test", RemoteIdentityOptions{}, now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer other.ZeroPrivateMaterial()
	if other.Record.ID == first.Record.ID || other.Record.ServerDeviceID == first.Record.ServerDeviceID || bytes.Equal(other.SigningPublicKey, first.SigningPublicKey) || bytes.Equal(other.ExchangePublicKey, first.ExchangePublicKey) {
		t.Fatal("different origin reused remote identity material")
	}
	oldPayloadNonce := append([]byte(nil), first.Record.PayloadNonce...)
	oldWrapNonce := append([]byte(nil), first.Record.WrapNonce...)
	active, err := manager.ActivateRemoteIdentityCAS(ctx, first.Record.ID, origin, RemoteIdentityOptions{}, 1, []byte(`{"version":1,"type":"bootstrap_commit_receipt"}`), now.Add(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer active.ZeroPrivateMaterial()
	if active.Record.RecordRevision != 2 || active.Record.Lifecycle != storage.RemoteIdentityActive || active.Envelope.Status != DeviceKeyEnvelopeActive || len(active.Record.BootstrapReceiptDigest) != 32 {
		t.Fatalf("active identity=%+v envelope=%+v", active.Record, active.Envelope)
	}
	if !bytes.Equal(active.Ed25519Seed, first.Ed25519Seed) || !bytes.Equal(active.X25519PrivateKey, first.X25519PrivateKey) || bytes.Equal(active.Record.PayloadNonce, oldPayloadNonce) || bytes.Equal(active.Record.WrapNonce, oldWrapNonce) {
		t.Fatal("activation changed key material or reused envelope randomness")
	}
	if _, err := manager.ActivateRemoteIdentityCAS(ctx, first.Record.ID, origin, RemoteIdentityOptions{}, 1, []byte(`{"version":1}`), now.Add(5*time.Second)); domain.CodeOf(err) != domain.CodeCredentialRevisionConflict {
		t.Fatalf("stale activation code=%v err=%v", domain.CodeOf(err), err)
	}
	if err := manager.Lock(); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.OpenRemoteIdentity(ctx, first.Record.ID, origin, RemoteIdentityOptions{}); domain.CodeOf(err) != domain.CodeVaultLocked {
		t.Fatalf("locked open code=%v err=%v", domain.CodeOf(err), err)
	}
}

func TestRemoteIdentityEnvelopeTamperQuarantines(t *testing.T) {
	ctx := context.Background()
	store, manager, path, now := remoteIdentityVault(t)
	identity, err := manager.PrepareRemoteIdentity(ctx, "https://tamper.example.test", RemoteIdentityOptions{}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	identity.ZeroPrivateMaterial()
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `UPDATE remote_device_identities SET aad_digest=zeroblob(32) WHERE id=?`, identity.Record.ID); err != nil {
		raw.Close()
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.OpenRemoteIdentity(ctx, identity.Record.ID, identity.Record.ServerOrigin, RemoteIdentityOptions{}); domain.CodeOf(err) != domain.CodeVaultCorrupt {
		t.Fatalf("tampered open code=%v err=%v", domain.CodeOf(err), err)
	}
	if _, err := store.RemoteDeviceIdentity(ctx, identity.Record.ID); domain.CodeOf(err) != domain.CodeQuarantined {
		t.Fatalf("quarantine lookup code=%v err=%v", domain.CodeOf(err), err)
	}
}
