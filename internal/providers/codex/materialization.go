package codex

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/device"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
)

const (
	MaxAuthFileBytes = 64 * 1024
	LeaseTTL         = 2 * time.Minute
)

// CredentialSource is the typed Vault-to-provider boundary. It receives a
// private staging home and writes only provider material there; callers never
// pass raw credential bytes through the manager or IPC.
type CredentialSource interface {
	MaterializeCodexAuth(context.Context, domain.CredentialInstance, string) (AuthFile, error)
}

type CredentialMutationResult struct {
	Revision int64
	Digest   string
}

type CredentialMutationSink interface {
	CommitCodexAuth(context.Context, domain.CredentialInstance, string, time.Time) (CredentialMutationResult, error)
}

type AuthFile struct {
	RelativePath string
	Digest       string
	Size         int64
}

type LeaseState struct {
	CredentialInstanceID domain.ID
	OwnerID              string
	CredentialRevision   int64
	AcquiredAt           time.Time
	ExpiresAt            time.Time
}

type CommitResult struct {
	Changed  bool
	Revision int64
	Digest   string
}

type CredentialMaterializationManager struct {
	Store  *storage.Store
	Root   string
	Vault  interface{ RequireUnlocked() error }
	Source CredentialSource
	Sink   CredentialMutationSink
	Now    func() time.Time
}

type MaterializationHandle struct {
	manager      *CredentialMaterializationManager
	credentialID domain.ID
	ownerID      string
	home         string
	lockPath     string
	expectedRev  int64
	authDigest   string
	acquiredAt   time.Time
	released     bool
}

type lockRecord struct {
	CredentialInstanceID domain.ID `json:"credential_instance_id"`
	OwnerID              string    `json:"owner_id"`
	CredentialRevision   int64     `json:"credential_revision"`
	AcquiredAt           time.Time `json:"acquired_at"`
	ExpiresAt            time.Time `json:"expires_at"`
}

type homeManifest struct {
	Version              int64     `json:"version"`
	CredentialInstanceID domain.ID `json:"credential_instance_id"`
	CredentialRevision   int64     `json:"credential_revision"`
	AuthPath             string    `json:"auth_path"`
	AuthDigest           string    `json:"auth_digest"`
}

func NewCredentialMaterializationManager(store *storage.Store, root string, vault interface{ RequireUnlocked() error }, source CredentialSource) *CredentialMaterializationManager {
	manager := &CredentialMaterializationManager{Store: store, Root: root, Vault: vault, Source: source, Now: func() time.Time { return time.Now().UTC() }}
	manager.Sink, _ = source.(CredentialMutationSink)
	return manager
}

func (m *CredentialMaterializationManager) now() time.Time {
	if m != nil && m.Now != nil {
		return m.Now().UTC()
	}
	return time.Now().UTC()
}

// Acquire returns the one process/filesystem writer for a credential. Runtime
// profiles intentionally share the canonical home and writer.
func (m *CredentialMaterializationManager) Acquire(ctx context.Context, credentialID, runtimeProfileID domain.ID) (*MaterializationHandle, error) {
	if m == nil || m.Store == nil || m.Root == "" || m.Source == nil || ctx == nil {
		return nil, domain.NewError(domain.CodeInvalidArgument, "credential materialization manager is incomplete")
	}
	if err := domain.ValidateID(credentialID); err != nil {
		return nil, err
	}
	if runtimeProfileID != "" {
		if err := domain.ValidateID(runtimeProfileID); err != nil {
			return nil, err
		}
	}
	if m.Vault != nil {
		if err := m.Vault.RequireUnlocked(); err != nil {
			return nil, err
		}
	}
	credential, err := m.Store.CredentialInstance(ctx, credentialID)
	if err != nil {
		return nil, err
	}
	if credential.Provider != domain.ProviderCodex || credential.Status != domain.CredentialHealthy || credential.CredentialRevision < 1 {
		return nil, domain.NewError(domain.CodeCredentialRecoveryRequired, "credential is not available for Codex materialization")
	}
	if err := m.ensureRoot(); err != nil {
		return nil, err
	}
	home := filepath.Join(m.Root, string(credentialID))
	lockPath := filepath.Join(m.Root, string(credentialID)+".writer.lock")
	ownerID := fmt.Sprintf("%d-%x", os.Getpid(), m.now().UnixNano())
	lock := lockRecord{CredentialInstanceID: credentialID, OwnerID: ownerID, CredentialRevision: credential.CredentialRevision, AcquiredAt: m.now(), ExpiresAt: m.now().Add(LeaseTTL)}
	lockBytes, _ := json.Marshal(lock)
	if err := writePrivate(lockPath, lockBytes); err != nil {
		if credentialWriterLockContended(lockPath, err) {
			return nil, domain.NewError(domain.CodeCredentialWriterConflict, "another canonical credential writer owns this CredentialInstance")
		}
		return nil, domain.WrapError(domain.CodeConflict, "credential writer lock could not be acquired", err)
	}
	h := &MaterializationHandle{manager: m, credentialID: credentialID, ownerID: ownerID, home: home, lockPath: lockPath, expectedRev: credential.CredentialRevision, acquiredAt: lock.AcquiredAt}
	if err := m.prepareHome(ctx, h, credential); err != nil {
		_ = os.Remove(lockPath)
		return nil, err
	}
	return h, nil
}

func (m *CredentialMaterializationManager) prepareHome(ctx context.Context, h *MaterializationHandle, credential domain.CredentialInstance) error {
	if info, err := os.Lstat(h.home); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			if quarantineErr := m.quarantinePath(h.home); quarantineErr != nil {
				return quarantineErr
			}
			return domain.NewError(domain.CodeCredentialRecoveryRequired, "credential auth home is not a private directory")
		}
		if err := device.VerifyPrivateDirectory(h.home); err != nil {
			if quarantineErr := m.quarantinePath(h.home); quarantineErr != nil {
				return quarantineErr
			}
			return domain.NewError(domain.CodeCredentialRecoveryRequired, "credential auth home is not a private directory")
		}
		manifest, authDigest, err := validateHome(h.home, h.credentialID, credential.CredentialRevision)
		if err != nil {
			if quarantineErr := m.quarantinePath(h.home); quarantineErr != nil {
				return quarantineErr
			}
			return domain.WrapError(domain.CodeCredentialRecoveryRequired, "credential auth home requires official re-login", err)
		}
		h.authDigest = authDigest
		if manifest.CredentialRevision != credential.CredentialRevision {
			return domain.NewError(domain.CodeCredentialRevisionConflict, "credential auth home revision is stale")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return domain.WrapError(domain.CodeConflict, "credential auth home could not be inspected", err)
	}
	staging, err := os.MkdirTemp(m.Root, ".staging-")
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "credential auth staging home could not be created", err)
	}
	defer os.RemoveAll(staging)
	if err := restrictPrivateDir(staging); err != nil {
		return err
	}
	authFile, err := m.Source.MaterializeCodexAuth(ctx, credential, staging)
	if err != nil {
		return domain.WrapError(domain.CodeCredentialRecoveryRequired, "official Codex login could not materialize credentials", err)
	}
	if authFile.RelativePath != "auth.json" {
		return domain.NewError(domain.CodeCredentialRecoveryRequired, "credential source returned an unsupported auth path")
	}
	validatedDigest, size, err := validateAuthFile(filepath.Join(staging, authFile.RelativePath))
	if err != nil || validatedDigest != authFile.Digest || size != authFile.Size {
		return domain.NewError(domain.CodeCredentialRecoveryRequired, "materialized credential structure or digest is invalid")
	}
	manifest := homeManifest{Version: 1, CredentialInstanceID: h.credentialID, CredentialRevision: credential.CredentialRevision, AuthPath: "auth.json", AuthDigest: validatedDigest}
	manifestBytes, _ := json.Marshal(manifest)
	if err := writePrivate(filepath.Join(staging, "manifest.json"), manifestBytes); err != nil {
		return err
	}
	if err := os.Rename(staging, h.home); err != nil {
		return domain.WrapError(domain.CodeConflict, "credential auth home could not be committed", err)
	}
	if err := device.VerifyPrivateDirectory(h.home); err != nil {
		return domain.WrapError(domain.CodeConflict, "credential auth home boundary was not preserved", err)
	}
	h.authDigest = validatedDigest
	return nil
}

func (h *MaterializationHandle) AuthHomePath() string {
	if h == nil || h.released {
		return ""
	}
	return h.home
}

func (h *MaterializationHandle) RefreshLease(ctx context.Context) (LeaseState, error) {
	if h == nil || h.manager == nil || h.released || ctx == nil {
		return LeaseState{}, domain.NewError(domain.CodeInvalidArgument, "materialization handle is unavailable")
	}
	if err := h.verifyLock(); err != nil {
		return LeaseState{}, err
	}
	now := h.manager.now()
	record := lockRecord{CredentialInstanceID: h.credentialID, OwnerID: h.ownerID, CredentialRevision: h.expectedRev, AcquiredAt: h.acquiredAt, ExpiresAt: now.Add(LeaseTTL)}
	data, _ := json.Marshal(record)
	if err := writeLock(h.lockPath, data); err != nil {
		return LeaseState{}, domain.WrapError(domain.CodeConflict, "credential writer lease could not be refreshed", err)
	}
	return LeaseState{CredentialInstanceID: h.credentialID, OwnerID: h.ownerID, CredentialRevision: h.expectedRev, AcquiredAt: h.acquiredAt, ExpiresAt: record.ExpiresAt}, nil
}

func (h *MaterializationHandle) ObserveAndCommit(ctx context.Context) (CommitResult, error) {
	if h == nil || h.manager == nil || h.released || ctx == nil {
		return CommitResult{}, domain.NewError(domain.CodeInvalidArgument, "materialization handle is unavailable")
	}
	if err := h.verifyLock(); err != nil {
		return CommitResult{}, err
	}
	digest, _, err := validateAuthFile(filepath.Join(h.home, "auth.json"))
	if err != nil {
		_ = h.quarantine(ctx)
		return CommitResult{}, domain.WrapError(domain.CodeCredentialRecoveryRequired, "credential auth file requires re-login", err)
	}
	if digest == h.authDigest {
		return CommitResult{Changed: false, Revision: h.expectedRev, Digest: digest}, nil
	}
	credential, err := h.manager.Store.CredentialInstance(ctx, h.credentialID)
	if err != nil {
		return CommitResult{}, err
	}
	if credential.CredentialRevision != h.expectedRev {
		_ = h.quarantine(ctx)
		return CommitResult{}, domain.NewError(domain.CodeCredentialRevisionConflict, "credential revision is stale")
	}
	if h.manager.Sink == nil {
		_ = h.quarantine(ctx)
		return CommitResult{}, domain.NewError(domain.CodeCredentialRecoveryRequired, "Vault mutation sink is unavailable")
	}
	var updated domain.CredentialInstance
	{
		committed, commitErr := h.manager.Sink.CommitCodexAuth(ctx, credential, filepath.Join(h.home, "auth.json"), h.manager.now())
		if commitErr != nil {
			err = commitErr
		} else if committed.Revision != h.expectedRev+1 || committed.Digest != digest {
			err = domain.NewError(domain.CodeCredentialRecoveryRequired, "Vault mutation result is inconsistent")
		} else {
			updated, err = h.manager.Store.CredentialInstance(ctx, h.credentialID)
		}
	}
	if err != nil {
		_ = h.quarantine(ctx)
		if domain.CodeOf(err) == domain.CodeCredentialRevisionConflict {
			return CommitResult{}, err
		}
		return CommitResult{}, domain.WrapError(domain.CodeCredentialRecoveryRequired, "credential revision could not be committed", err)
	}
	manifest := homeManifest{Version: 1, CredentialInstanceID: h.credentialID, CredentialRevision: updated.CredentialRevision, AuthPath: "auth.json", AuthDigest: digest}
	data, _ := json.Marshal(manifest)
	if err := replacePrivate(filepath.Join(h.home, "manifest.json"), data); err != nil {
		_ = h.quarantine(ctx)
		return CommitResult{}, domain.NewError(domain.CodeCredentialRecoveryRequired, "credential commit became ambiguous")
	}
	h.expectedRev, h.authDigest = updated.CredentialRevision, digest
	return CommitResult{Changed: true, Revision: updated.CredentialRevision, Digest: digest}, nil
}

func (h *MaterializationHandle) Quarantine(ctx context.Context, reason string) error {
	if h == nil || h.manager == nil || h.released {
		return domain.NewError(domain.CodeInvalidArgument, "materialization handle is unavailable")
	}
	return h.quarantine(ctx)
}

func (h *MaterializationHandle) quarantine(ctx context.Context) error {
	if err := h.verifyLock(); err != nil {
		return err
	}
	if err := h.manager.quarantinePath(h.home); err != nil {
		return err
	}
	_ = os.Remove(h.lockPath)
	h.released = true
	return nil
}

func (h *MaterializationHandle) Release(ctx context.Context) error {
	if h == nil || h.manager == nil || h.released {
		return nil
	}
	if err := h.verifyLock(); err != nil {
		return err
	}
	if err := os.RemoveAll(h.home); err != nil {
		return domain.WrapError(domain.CodeConflict, "credential auth home could not be released", err)
	}
	if err := os.Remove(h.lockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return domain.WrapError(domain.CodeConflict, "credential writer lock could not be released", err)
	}
	h.released = true
	return nil
}

func (h *MaterializationHandle) verifyLock() error {
	if err := device.VerifyPrivateFile(h.lockPath); err != nil {
		return domain.NewError(domain.CodeCredentialWriterConflict, "canonical credential writer lease is not private")
	}
	data, err := os.ReadFile(h.lockPath)
	if err != nil {
		return domain.NewError(domain.CodeCredentialWriterConflict, "canonical credential writer lease is unavailable")
	}
	var record lockRecord
	if decodeStrict(data, &record) != nil || record.OwnerID != h.ownerID || record.CredentialInstanceID != h.credentialID {
		return domain.NewError(domain.CodeCredentialWriterConflict, "canonical credential writer lease is owned by another process")
	}
	if h.manager.now().After(record.ExpiresAt) {
		return domain.NewError(domain.CodeCredentialWriterConflict, "canonical credential writer lease expired")
	}
	return nil
}

// Recover quarantines stale writer residue left by a prior process and any
// malformed home. A valid unlocked home without a lock remains reusable.
func (m *CredentialMaterializationManager) Recover(ctx context.Context) error {
	if m == nil || m.Root == "" || ctx == nil {
		return domain.NewError(domain.CodeInvalidArgument, "credential materialization manager is incomplete")
	}
	if err := m.ensureRoot(); err != nil {
		return err
	}
	entries, err := os.ReadDir(m.Root)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "credential auth root could not be listed", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".staging-") ||
			strings.HasSuffix(entry.Name(), ".writer.lock") ||
			strings.HasSuffix(entry.Name(), ".writer.lock.new") ||
			strings.HasSuffix(entry.Name(), ".writer.lock.replace") {
			if err := m.quarantinePath(filepath.Join(m.Root, entry.Name())); err != nil {
				return err
			}
			continue
		}
		if entry.Name() == "quarantine" {
			continue
		}
		if !entry.IsDir() || domain.ValidateID(domain.ID(entry.Name())) != nil {
			if err := m.quarantinePath(filepath.Join(m.Root, entry.Name())); err != nil {
				return err
			}
			continue
		}
		if _, _, err := validateHome(filepath.Join(m.Root, entry.Name()), domain.ID(entry.Name()), 0); err != nil {
			if err := m.quarantinePath(filepath.Join(m.Root, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *CredentialMaterializationManager) ensureRoot() error {
	if err := os.MkdirAll(m.Root, 0o700); err != nil {
		return domain.WrapError(domain.CodeConflict, "credential auth root could not be created", err)
	}
	if err := restrictPrivateDir(m.Root); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(m.Root, "quarantine"), 0o700); err != nil {
		return domain.WrapError(domain.CodeConflict, "credential quarantine root could not be created", err)
	}
	return restrictPrivateDir(filepath.Join(m.Root, "quarantine"))
}

func (m *CredentialMaterializationManager) quarantinePath(path string) error {
	if err := os.MkdirAll(filepath.Join(m.Root, "quarantine"), 0o700); err != nil {
		return err
	}
	if err := restrictPrivateDir(filepath.Join(m.Root, "quarantine")); err != nil {
		return err
	}
	target := filepath.Join(m.Root, "quarantine", filepath.Base(path)+"-"+m.now().Format("20060102T150405.000000000Z"))
	if err := os.Rename(path, target); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return domain.WrapError(domain.CodeQuarantined, "credential auth residue could not be quarantined", err)
	}
	return verifyQuarantinedPath(target)
}

func verifyQuarantinedPath(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return domain.WrapError(domain.CodeQuarantined, "quarantined credential residue could not be inspected", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return domain.NewError(domain.CodeQuarantined, "quarantined credential residue is a symbolic link")
	}
	if info.IsDir() {
		if err := device.ProtectPrivateDirectory(path); err != nil {
			return domain.WrapError(domain.CodeQuarantined, "quarantined credential directory could not be restricted", err)
		}
		if err := device.VerifyPrivateDirectory(path); err != nil {
			return domain.WrapError(domain.CodeQuarantined, "quarantined credential directory is not private", err)
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return domain.NewError(domain.CodeQuarantined, "quarantined credential residue has an unsupported type")
	}
	if err := device.VerifyPrivateFile(path); err != nil {
		return domain.WrapError(domain.CodeQuarantined, "quarantined credential file is not private", err)
	}
	return nil
}

func restrictPrivateDir(path string) error {
	if err := device.ProtectPrivateDirectory(path); err != nil {
		return domain.WrapError(domain.CodeConflict, "credential auth directory permissions could not be restricted", err)
	}
	return nil
}

func writePrivate(path string, data []byte) error { return device.WritePrivateFileAtomic(path, data) }

func writeLock(path string, data []byte) error {
	return device.ReplacePrivateFileAtomic(path, data)
}

func replacePrivate(path string, data []byte) error {
	return device.ReplacePrivateFileAtomic(path, data)
}

func validateHome(home string, credentialID domain.ID, revision int64) (homeManifest, string, error) {
	if err := device.VerifyPrivateDirectory(home); err != nil {
		return homeManifest{}, "", errors.New("auth home is not private")
	}
	if err := filepath.WalkDir(home, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == home {
			return nil
		}
		relative, err := filepath.Rel(home, path)
		if err != nil || strings.Contains(relative, ".."+string(filepath.Separator)) || entry.Type()&os.ModeSymlink != 0 {
			return errors.New("auth home contains an unsafe path")
		}
		if entry.IsDir() {
			return device.VerifyPrivateDirectory(path)
		}
		if relative == "auth.json" || relative == "manifest.json" {
			return nil
		}
		return errors.New("auth home contains an unexpected file")
	}); err != nil {
		return homeManifest{}, "", err
	}
	manifestPath := filepath.Join(home, "manifest.json")
	if err := device.VerifyPrivateFile(manifestPath); err != nil {
		return homeManifest{}, "", errors.New("manifest is not private")
	}
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil || len(manifestData) > 4096 {
		return homeManifest{}, "", errors.New("manifest unavailable")
	}
	var manifest homeManifest
	if decodeStrict(manifestData, &manifest) != nil || manifest.Version != 1 || manifest.CredentialInstanceID != credentialID || manifest.AuthPath != "auth.json" || len(manifest.AuthDigest) != 64 {
		return homeManifest{}, "", errors.New("manifest invalid")
	}
	if revision > 0 && manifest.CredentialRevision != revision {
		return manifest, "", errors.New("manifest revision mismatch")
	}
	digest, _, err := validateAuthFile(filepath.Join(home, "auth.json"))
	if err != nil || digest != manifest.AuthDigest {
		return manifest, "", errors.New("auth digest mismatch")
	}
	return manifest, digest, nil
}

func credentialWriterLockContended(path string, cause error) bool {
	if domain.CodeOf(cause) == domain.CodeAlreadyExists {
		return true
	}
	for _, candidate := range []string{path, path + ".new", path + ".replace"} {
		if _, err := os.Lstat(candidate); err == nil {
			return true
		}
	}
	return false
}

func validateAuthFile(path string) (string, int64, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > MaxAuthFileBytes || device.VerifyPrivateFile(path) != nil {
		return "", 0, errors.New("auth file permissions or size invalid")
	}
	data, err := os.ReadFile(path)
	if err != nil || !validJSONObject(data) {
		return "", 0, errors.New("auth file structure invalid")
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), int64(len(data)), nil
}

// ReadEnrollmentAuth validates the daemon-owned login staging directory and
// returns only the bounded auth.json bytes for immediate Vault import.
func ReadEnrollmentAuth(home string) ([]byte, error) {
	if device.VerifyPrivateDirectory(home) != nil {
		return nil, domain.NewError(domain.CodeCredentialRecoveryRequired, "enrollment staging is not private")
	}
	// Official login and app-server validation both create non-credential
	// runtime state under CODEX_HOME. None of it is imported: validation is
	// intentionally scoped to the exact auth.json path, and the daemon removes
	// the complete private staging directory after the terminal transition.
	path := filepath.Join(home, "auth.json")
	if _, _, err := validateAuthFile(path); err != nil {
		return nil, domain.NewError(domain.CodeCredentialRecoveryRequired, "enrollment credential is invalid")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, domain.NewError(domain.CodeCredentialRecoveryRequired, "enrollment credential is unavailable")
	}
	return data, nil
}

func validJSONObject(data []byte) bool {
	structureDecoder := json.NewDecoder(bytes.NewReader(data))
	if err := validateValue(structureDecoder); err != nil {
		return false
	}
	var structureTrailing any
	if err := structureDecoder.Decode(&structureTrailing); !errors.Is(err, io.EOF) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var value map[string]json.RawMessage
	if decoder.Decode(&value) != nil || value == nil {
		return false
	}
	var trailing any
	return errors.Is(decoder.Decode(&trailing), io.EOF)
}

func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	return nil
}
