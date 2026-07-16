package codex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
)

type testCredentialSource struct {
	content []byte
	path    string
}

type testMutationSink struct{ store *storage.Store }

func (s testMutationSink) CommitCodexAuth(ctx context.Context, credential domain.CredentialInstance, path string, at time.Time) (CredentialMutationResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CredentialMutationResult{}, err
	}
	digest := sha256.Sum256(data)
	digestText := hex.EncodeToString(digest[:])
	updated, err := s.store.UpdateCredentialRevisionCAS(ctx, credential.ID, credential.CredentialRevision, digestText, at)
	if err != nil {
		return CredentialMutationResult{}, err
	}
	return CredentialMutationResult{Revision: updated.CredentialRevision, Digest: digestText}, nil
}

func (s testCredentialSource) MaterializeCodexAuth(_ context.Context, _ domain.CredentialInstance, home string) (AuthFile, error) {
	path := s.path
	if path == "" {
		path = "auth.json"
	}
	full := filepath.Join(home, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
		return AuthFile{}, err
	}
	if err := os.WriteFile(full, s.content, 0o600); err != nil {
		return AuthFile{}, err
	}
	digest := sha256.Sum256(s.content)
	return AuthFile{RelativePath: path, Digest: hex.EncodeToString(digest[:]), Size: int64(len(s.content))}, nil
}

type testVault struct{ unlocked bool }

func (v testVault) RequireUnlocked() error {
	if !v.unlocked {
		return domain.NewError(domain.CodeVaultLocked, "vault is locked")
	}
	return nil
}

func setupCodexCredential(t *testing.T) (*storage.Store, domain.ID, domain.ID) {
	t.Helper()
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "device", "device.db"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	deviceID, _ := domain.NewID("device")
	accountID, _ := domain.NewID("account")
	credentialID, _ := domain.NewID("credential")
	if err := store.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "test", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAccount(ctx, domain.Account{ID: accountID, Provider: domain.ProviderCodex, DisplayName: "test", ProviderSubjectDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateCredentialInstance(ctx, domain.CredentialInstance{ID: credentialID, DeviceID: deviceID, AccountID: accountID, Provider: domain.ProviderCodex, AuthMethod: domain.AuthMethodInteractive, SecretRef: "vault:test", Status: domain.CredentialHealthy, CredentialRevision: 1, SecretDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	return store, credentialID, accountID
}

func TestCredentialMaterializationSingleWriterCASAndPermissions(t *testing.T) {
	ctx := context.Background()
	store, credentialID, _ := setupCodexCredential(t)
	defer store.Close()
	profileID, _ := domain.NewID("profile")
	content := []byte(`{"access_token":"synthetic"}`)
	m := NewCredentialMaterializationManager(store, filepath.Join(t.TempDir(), "codex-home"), testVault{unlocked: true}, testCredentialSource{content: content})
	m.Sink = testMutationSink{store: store}
	h, err := m.Acquire(ctx, credentialID, profileID)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Release(ctx)
	if h.AuthHomePath() == "" {
		t.Fatal("missing private auth home")
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(h.AuthHomePath()); err != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("home mode=%o err=%v", info.Mode().Perm(), err)
		}
		if info, err := os.Stat(filepath.Join(h.AuthHomePath(), "auth.json")); err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("auth mode=%o err=%v", info.Mode().Perm(), err)
		}
	}
	second := NewCredentialMaterializationManager(store, m.Root, testVault{unlocked: true}, testCredentialSource{content: content})
	if _, err := second.Acquire(ctx, credentialID, profileID); domain.CodeOf(err) != domain.CodeCredentialWriterConflict {
		t.Fatalf("second writer err=%v", err)
	}
	state, err := h.RefreshLease(ctx)
	if err != nil || state.CredentialRevision != 1 || state.OwnerID == "" {
		t.Fatalf("lease=%+v err=%v", state, err)
	}
	if err := os.WriteFile(filepath.Join(h.AuthHomePath(), "auth.json"), []byte(`{"access_token":"rotated"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	committed, err := h.ObserveAndCommit(ctx)
	if err != nil || !committed.Changed || committed.Revision != 2 {
		t.Fatalf("commit=%+v err=%v", committed, err)
	}
	credential, err := store.CredentialInstance(ctx, credentialID)
	if err != nil || credential.CredentialRevision != 2 || credential.SecretDigest != committed.Digest {
		t.Fatalf("credential=%+v err=%v", credential, err)
	}
}

func TestCredentialMaterializationRejectsRawPathAndRecoversCrashResidue(t *testing.T) {
	ctx := context.Background()
	store, credentialID, _ := setupCodexCredential(t)
	defer store.Close()
	root := filepath.Join(t.TempDir(), "codex-home")
	bad := NewCredentialMaterializationManager(store, root, testVault{unlocked: true}, testCredentialSource{content: []byte(`{}`), path: "../auth.json"})
	if _, err := bad.Acquire(ctx, credentialID, ""); domain.CodeOf(err) != domain.CodeCredentialRecoveryRequired {
		t.Fatalf("path traversal err=%v", err)
	}
	good := NewCredentialMaterializationManager(store, root, testVault{unlocked: true}, testCredentialSource{content: []byte(`{"token":"synthetic"}`)})
	h, err := good.Acquire(ctx, credentialID, "")
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a process crash: a fresh manager sees the stale lock and moves
	// both lock and home to quarantine before any new writer can start.
	restarted := NewCredentialMaterializationManager(store, root, testVault{unlocked: true}, testCredentialSource{content: []byte(`{"token":"synthetic"}`)})
	if err := restarted.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "quarantine")); err != nil {
		t.Fatal(err)
	}
	if replacement, err := good.Acquire(ctx, credentialID, ""); err != nil {
		t.Fatalf("post-recovery acquire err=%v", err)
	} else {
		_ = replacement.Release(ctx)
	}
	_ = h.Release(ctx)
}

func TestCredentialMaterializationRequiresUnlockedVault(t *testing.T) {
	ctx := context.Background()
	store, credentialID, _ := setupCodexCredential(t)
	defer store.Close()
	m := NewCredentialMaterializationManager(store, filepath.Join(t.TempDir(), "codex-home"), testVault{}, testCredentialSource{content: []byte(`{}`)})
	if _, err := m.Acquire(ctx, credentialID, ""); domain.CodeOf(err) != domain.CodeVaultLocked {
		t.Fatalf("locked vault err=%v", err)
	}
}

func TestCredentialMaterializationRefreshRequiresAtomicVaultSink(t *testing.T) {
	ctx := context.Background()
	store, credentialID, _ := setupCodexCredential(t)
	defer store.Close()
	m := NewCredentialMaterializationManager(store, filepath.Join(t.TempDir(), "codex-home"), testVault{unlocked: true}, testCredentialSource{content: []byte(`{"token":"initial"}`)})
	handle, err := m.Acquire(ctx, credentialID, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(handle.AuthHomePath(), "auth.json"), []byte(`{"token":"changed"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.ObserveAndCommit(ctx); domain.CodeOf(err) != domain.CodeCredentialRecoveryRequired {
		t.Fatalf("metadata-only refresh was not rejected: %v", err)
	}
	credential, err := store.CredentialInstance(ctx, credentialID)
	if err != nil || credential.CredentialRevision != 1 {
		t.Fatalf("metadata advanced without Vault sink: %+v err=%v", credential, err)
	}
}

func TestCredentialMaterializationRejectsDuplicateJSONAndUnexpectedFiles(t *testing.T) {
	ctx := context.Background()
	store, credentialID, _ := setupCodexCredential(t)
	defer store.Close()
	root := filepath.Join(t.TempDir(), "codex-home")
	duplicate := NewCredentialMaterializationManager(store, root, testVault{unlocked: true}, testCredentialSource{content: []byte(`{"token":1,"token":2}`)})
	if _, err := duplicate.Acquire(ctx, credentialID, ""); domain.CodeOf(err) != domain.CodeCredentialRecoveryRequired {
		t.Fatalf("duplicate auth JSON err=%v", err)
	}
	valid := NewCredentialMaterializationManager(store, root, testVault{unlocked: true}, testCredentialSource{content: []byte(`{"token":1}`)})
	h, err := valid.Acquire(ctx, credentialID, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.AuthHomePath(), "unexpected.txt"), []byte("residue"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := h.Release(ctx); err != nil {
		t.Fatal(err)
	}
	recovered := NewCredentialMaterializationManager(store, root, testVault{unlocked: true}, testCredentialSource{content: []byte(`{"token":1}`)})
	// Recreate a valid home, then add an unexpected file before the next
	// acquisition to exercise quarantine rather than silent reuse.
	h2, err := recovered.Acquire(ctx, credentialID, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h2.AuthHomePath(), "unexpected.txt"), []byte("residue"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, string(credentialID)+".writer.lock")); err != nil {
		t.Fatal(err)
	}
	if _, err := recovered.Acquire(ctx, credentialID, ""); domain.CodeOf(err) != domain.CodeCredentialRecoveryRequired {
		t.Fatalf("unexpected file was not quarantined: %v", err)
	}
	_ = h2.Release(ctx)
}
