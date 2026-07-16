package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
	"github.com/jinlong17/multi-agent-desk/internal/vault"
)

func TestVaultCredentialSourceMaterializesAndAtomicallyCommitsRefresh(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "device", "device.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Unix(2100, 0).UTC()
	deviceID, clientID := codexSourceID(t, "device"), codexSourceID(t, "client")
	accountID, profileID, credentialID := codexSourceID(t, "account"), codexSourceID(t, "profile"), codexSourceID(t, "credential")
	if err := store.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "source", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: clientID, Name: "owner", PublicKey: make([]byte, 32), Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityVaultControl}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	manager, err := vault.NewPersistentManager(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Initialize(ctx, clientID, "source-init", []byte("source-password"), now); err != nil {
		t.Fatal(err)
	}
	if err := manager.Unlock([]byte("source-password")); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAccount(ctx, domain.Account{ID: accountID, Provider: domain.ProviderCodex, DisplayName: "source", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRuntimeProfile(ctx, domain.RuntimeProfile{ID: profileID, DeviceID: deviceID, AccountID: accountID, Name: "source", Provider: domain.ProviderCodex, Settings: []byte(`{}`), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateCredentialInstance(ctx, domain.CredentialInstance{ID: credentialID, DeviceID: deviceID, AccountID: accountID, Provider: domain.ProviderCodex, AuthMethod: domain.AuthMethodInteractive, SecretRef: "vault:" + string(credentialID), Status: domain.CredentialUnknown, CredentialRevision: 1, SecretDigest: strings.Repeat("0", 64), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	initial := []byte(`{"tokens":{"access":"initial"}}`)
	if _, err := manager.SealCredential(ctx, vault.CredentialMetadata{CredentialInstanceID: credentialID, AccountID: accountID, DeviceID: deviceID, Provider: domain.ProviderCodex, ExpectedRevision: 1, CreatedAt: now, UpdatedAt: now.Add(time.Second)}, initial); err != nil {
		t.Fatal(err)
	}
	source := NewVaultCredentialSource(manager)
	materializer := NewCredentialMaterializationManager(store, filepath.Join(t.TempDir(), "codex-home"), manager, source)
	handle, err := materializer.Acquire(ctx, credentialID, profileID)
	if err != nil {
		t.Fatal(err)
	}
	refreshed := []byte(`{"tokens":{"access":"refreshed"}}`)
	if err := os.WriteFile(filepath.Join(handle.AuthHomePath(), "auth.json"), refreshed, 0o600); err != nil {
		t.Fatal(err)
	}
	commit, err := handle.ObserveAndCommit(ctx)
	if err != nil || !commit.Changed || commit.Revision != 3 {
		t.Fatalf("refresh commit=%+v err=%v", commit, err)
	}
	plain, revision, err := manager.ReadCredential(ctx, credentialID)
	if err != nil || revision != 3 || string(plain) != string(refreshed) {
		t.Fatalf("Vault refresh revision=%d payload=%q err=%v", revision, plain, err)
	}
	zeroCredentialBytes(plain)
	if err := handle.Release(ctx); err != nil {
		t.Fatal(err)
	}
}

func codexSourceID(t *testing.T, prefix string) domain.ID {
	t.Helper()
	id, err := domain.NewID(prefix)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
