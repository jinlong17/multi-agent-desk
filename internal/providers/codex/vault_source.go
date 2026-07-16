package codex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/device"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/vault"
)

// VaultCredentialSource is the production provider-owned bridge from the
// neutral Vault API into Codex auth.json materialization and refresh CAS.
type VaultCredentialSource struct {
	Manager *vault.Manager
}

func NewVaultCredentialSource(manager *vault.Manager) *VaultCredentialSource {
	return &VaultCredentialSource{Manager: manager}
}

func (s *VaultCredentialSource) MaterializeCodexAuth(ctx context.Context, credential domain.CredentialInstance, home string) (AuthFile, error) {
	if s == nil || s.Manager == nil || credential.Provider != domain.ProviderCodex || credential.Status != domain.CredentialHealthy || home == "" {
		return AuthFile{}, domain.NewError(domain.CodeInvalidArgument, "Codex Vault source is invalid")
	}
	if err := device.ProtectPrivateDirectory(home); err != nil {
		return AuthFile{}, err
	}
	plain, revision, err := s.Manager.ReadCredential(ctx, credential.ID)
	if err != nil {
		return AuthFile{}, err
	}
	defer zeroCredentialBytes(plain)
	if revision != credential.CredentialRevision {
		return AuthFile{}, domain.NewError(domain.CodeCredentialRevisionConflict, "Vault credential revision changed")
	}
	digest := sha256.Sum256(plain)
	path := filepath.Join(home, "auth.json")
	if err := device.WritePrivateFileAtomic(path, plain); err != nil {
		return AuthFile{}, err
	}
	return AuthFile{RelativePath: "auth.json", Digest: hex.EncodeToString(digest[:]), Size: int64(len(plain))}, nil
}

func (s *VaultCredentialSource) CommitCodexAuth(ctx context.Context, credential domain.CredentialInstance, authPath string, at time.Time) (CredentialMutationResult, error) {
	if s == nil || s.Manager == nil || credential.Provider != domain.ProviderCodex || credential.Status != domain.CredentialHealthy ||
		filepath.Base(authPath) != "auth.json" || at.IsZero() {
		return CredentialMutationResult{}, domain.NewError(domain.CodeInvalidArgument, "Codex Vault mutation is invalid")
	}
	if err := device.VerifyPrivateDirectory(filepath.Dir(authPath)); err != nil {
		return CredentialMutationResult{}, err
	}
	info, err := os.Lstat(authPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 2 || info.Size() > MaxAuthFileBytes || device.VerifyPrivateFile(authPath) != nil {
		return CredentialMutationResult{}, domain.NewError(domain.CodeCredentialRecoveryRequired, "Codex auth mutation is invalid")
	}
	plain, err := os.ReadFile(authPath)
	if err != nil || !validJSONObject(plain) {
		zeroCredentialBytes(plain)
		return CredentialMutationResult{}, domain.NewError(domain.CodeCredentialRecoveryRequired, "Codex auth mutation is invalid")
	}
	defer zeroCredentialBytes(plain)
	digest := sha256.Sum256(plain)
	revision, err := s.Manager.SealCredential(ctx, vault.CredentialMetadata{CredentialInstanceID: credential.ID,
		AccountID: credential.AccountID, DeviceID: credential.DeviceID, Provider: credential.Provider,
		ExpectedRevision: credential.CredentialRevision, CreatedAt: credential.CreatedAt, UpdatedAt: at}, plain)
	if err != nil {
		return CredentialMutationResult{}, err
	}
	return CredentialMutationResult{Revision: revision, Digest: hex.EncodeToString(digest[:])}, nil
}

func zeroCredentialBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
