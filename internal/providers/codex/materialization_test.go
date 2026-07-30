package codex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/device"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
)

type testCredentialSource struct {
	content []byte
}

type invalidPathCredentialSource struct{}

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
	if err := device.VerifyPrivateDirectory(home); err != nil {
		return AuthFile{}, err
	}
	path := "auth.json"
	full := filepath.Join(home, path)
	if err := device.WritePrivateFileAtomic(full, s.content); err != nil {
		return AuthFile{}, err
	}
	digest := sha256.Sum256(s.content)
	return AuthFile{RelativePath: path, Digest: hex.EncodeToString(digest[:]), Size: int64(len(s.content))}, nil
}

func (invalidPathCredentialSource) MaterializeCodexAuth(_ context.Context, _ domain.CredentialInstance, home string) (AuthFile, error) {
	if err := device.VerifyPrivateDirectory(home); err != nil {
		return AuthFile{}, err
	}
	return AuthFile{RelativePath: "../auth.json", Digest: strings.Repeat("0", 64), Size: 2}, nil
}

func overwriteExistingPrivateFile(t *testing.T, path string, data []byte) {
	t.Helper()
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := device.VerifyPrivateFile(path); err != nil {
		t.Fatalf("pre-write private boundary: %v", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("provider refresh replaced the auth file instead of truncating it in place")
	}
	if err := device.VerifyPrivateFile(path); err != nil {
		t.Fatalf("post-write private boundary: %v", err)
	}
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
	for _, directory := range []string{m.Root, filepath.Join(m.Root, "quarantine"), h.AuthHomePath()} {
		if err := device.VerifyPrivateDirectory(directory); err != nil {
			t.Fatalf("private directory %s: %v", directory, err)
		}
	}
	for _, path := range []string{
		filepath.Join(m.Root, string(credentialID)+".writer.lock"),
		filepath.Join(h.AuthHomePath(), "auth.json"),
		filepath.Join(h.AuthHomePath(), "manifest.json"),
	} {
		if err := device.VerifyPrivateFile(path); err != nil {
			t.Fatalf("private file %s: %v", path, err)
		}
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
	if err := device.VerifyPrivateFile(filepath.Join(m.Root, string(credentialID)+".writer.lock")); err != nil {
		t.Fatalf("refreshed lock boundary: %v", err)
	}
	overwriteExistingPrivateFile(t, filepath.Join(h.AuthHomePath(), "auth.json"), []byte(`{"access_token":"rotated"}`))
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
	bad := NewCredentialMaterializationManager(store, root, testVault{unlocked: true}, invalidPathCredentialSource{})
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
	overwriteExistingPrivateFile(t, filepath.Join(handle.AuthHomePath(), "auth.json"), []byte(`{"token":"changed"}`))
	if _, err := handle.ObserveAndCommit(ctx); domain.CodeOf(err) != domain.CodeCredentialRecoveryRequired {
		t.Fatalf("metadata-only refresh was not rejected: %v", err)
	}
	credential, err := store.CredentialInstance(ctx, credentialID)
	if err != nil || credential.CredentialRevision != 1 {
		t.Fatalf("metadata advanced without Vault sink: %+v err=%v", credential, err)
	}
}

func TestReadEnrollmentAuthAllowsOfficialRuntimeResidue(t *testing.T) {
	home := t.TempDir()
	if err := device.ProtectPrivateDirectory(home); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"log", "tmp", ".tmp", "skills"} {
		path := filepath.Join(home, name)
		if err := os.Mkdir(path, 0o770); err != nil {
			t.Fatal(err)
		}
	}
	arg0 := filepath.Join(home, "tmp", "arg0")
	if err := os.Mkdir(arg0, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "log", "codex-login.log"), []byte("runtime residue is ignored"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"state_5.sqlite", "goals_1.sqlite", "installation_id", "config.toml"} {
		if err := os.WriteFile(filepath.Join(home, name), []byte("runtime residue is ignored"), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	want := []byte(`{"tokens":{"access":"synthetic"}}`)
	authPath := filepath.Join(home, "auth.json")
	if err := device.WritePrivateFileAtomic(authPath, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if err := device.WritePrivateFileAtomic(authPath, []byte(`{"tokens":{"access":"replacement"}}`)); domain.CodeOf(err) != domain.CodeAlreadyExists {
		t.Fatalf("create-only auth replacement code=%s err=%v", domain.CodeOf(err), err)
	}
	overwriteExistingPrivateFile(t, authPath, want)
	got, err := ReadEnrollmentAuth(home)
	if err != nil || string(got) != string(want) {
		t.Fatalf("auth=%q err=%v", got, err)
	}
}

func TestReadEnrollmentAuthRejectsMissingOrInvalidCredential(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, string)
	}{
		{name: "missing auth", setup: func(t *testing.T, home string) {
			path := filepath.Join(home, "log")
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := device.ProtectPrivateDirectory(path); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "auth is a directory", setup: func(t *testing.T, home string) {
			path := filepath.Join(home, "auth.json")
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := device.ProtectPrivateDirectory(path); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "invalid auth", setup: func(t *testing.T, home string) {
			if err := device.WritePrivateFileAtomic(filepath.Join(home, "auth.json"), []byte("not-json")); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			if err := device.ProtectPrivateDirectory(home); err != nil {
				t.Fatal(err)
			}
			test.setup(t, home)
			if _, err := ReadEnrollmentAuth(home); domain.CodeOf(err) != domain.CodeCredentialRecoveryRequired {
				t.Fatalf("code=%v err=%v", domain.CodeOf(err), err)
			}
		})
	}
}

func writeEnrollmentAuthFixture(t *testing.T, home string) {
	t.Helper()
	if err := device.WritePrivateFileAtomic(filepath.Join(home, "auth.json"), []byte(`{"token":"synthetic"}`)); err != nil {
		t.Fatal(err)
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
	if err := device.WritePrivateFileAtomic(filepath.Join(h.AuthHomePath(), "unexpected.txt"), []byte("residue")); err != nil {
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
	if err := device.WritePrivateFileAtomic(filepath.Join(h2.AuthHomePath(), "unexpected.txt"), []byte("residue")); err != nil {
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

func TestCredentialMaterializationMapsFixedLockCandidateAndRecoversResidue(t *testing.T) {
	ctx := context.Background()
	store, credentialID, _ := setupCodexCredential(t)
	defer store.Close()
	manager := NewCredentialMaterializationManager(store, filepath.Join(t.TempDir(), "codex-home"), testVault{unlocked: true}, testCredentialSource{content: []byte(`{"token":"synthetic"}`)})
	if err := manager.ensureRoot(); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(manager.Root, string(credentialID)+".writer.lock")
	if err := device.WritePrivateFileAtomic(lockPath+".new", []byte("crash residue")); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Acquire(ctx, credentialID, ""); domain.CodeOf(err) != domain.CodeCredentialWriterConflict {
		t.Fatalf("fixed lock candidate code=%s err=%v", domain.CodeOf(err), err)
	}
	if err := manager.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(lockPath + ".new"); !os.IsNotExist(err) {
		t.Fatalf("lock candidate was not quarantined: %v", err)
	}
	handle, err := manager.Acquire(ctx, credentialID, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := handle.Release(ctx); err != nil {
		t.Fatal(err)
	}
}
