package vault

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/device"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
)

const (
	ManifestVersion        = int64(1)
	MaxFakeCredentialBytes = 64 * 1024
)

type MaterializationRequest struct {
	LeaseID              domain.ID
	CredentialInstanceID domain.ID
	CredentialRevision   int64
	Content              []byte
	RefCount             int64
}

type MaterializationManifest struct {
	ManifestVersion      int64     `json:"manifest_version"`
	LeaseID              domain.ID `json:"lease_id"`
	CredentialInstanceID domain.ID `json:"credential_instance_id"`
	CredentialRevision   int64     `json:"credential_revision"`
	ContentDigest        string    `json:"content_digest"`
	State                string    `json:"state"`
}

type Materializer struct {
	Store *storage.Store
	Root  string
	Vault interface{ RequireUnlocked() error }
	Now   func() time.Time
	mu    sync.Mutex
}

func NewMaterializer(store *storage.Store, root string, gate interface{ RequireUnlocked() error }) *Materializer {
	return &Materializer{Store: store, Root: root, Vault: gate, Now: func() time.Time { return time.Now().UTC() }}
}

func (m *Materializer) now() time.Time {
	if m != nil && m.Now != nil {
		return m.Now().UTC()
	}
	return time.Now().UTC()
}

func (m *Materializer) Materialize(ctx context.Context, request MaterializationRequest) (domain.CredentialMaterialization, error) {
	if m == nil || m.Store == nil || m.Root == "" || ctx == nil {
		return domain.CredentialMaterialization{}, domain.NewError(domain.CodeInvalidArgument, "materializer is incomplete")
	}
	if m.Vault != nil {
		if err := m.Vault.RequireUnlocked(); err != nil {
			return domain.CredentialMaterialization{}, err
		}
	}
	if err := domain.ValidateID(request.LeaseID); err != nil {
		return domain.CredentialMaterialization{}, err
	}
	if request.CredentialRevision < 1 || request.RefCount < 0 || len(request.Content) == 0 || len(request.Content) > MaxFakeCredentialBytes {
		return domain.CredentialMaterialization{}, domain.NewError(domain.CodeInvalidArgument, "materialization request is invalid")
	}
	credential, err := m.Store.CredentialInstance(ctx, request.CredentialInstanceID)
	if err != nil {
		return domain.CredentialMaterialization{}, err
	}
	if credential.CredentialRevision != request.CredentialRevision || credential.Status != domain.CredentialHealthy {
		return domain.CredentialMaterialization{}, domain.NewError(domain.CodeMaterializationConflict, "credential revision is stale or unavailable")
	}
	contentDigest := sha256.Sum256(request.Content)
	manifest := MaterializationManifest{ManifestVersion: ManifestVersion, LeaseID: request.LeaseID,
		CredentialInstanceID: request.CredentialInstanceID, CredentialRevision: request.CredentialRevision,
		ContentDigest: hex.EncodeToString(contentDigest[:]), State: string(domain.MaterializationActive)}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return domain.CredentialMaterialization{}, domain.WrapError(domain.CodeConflict, "materialization manifest could not be encoded", err)
	}
	manifestDigest := sha256.Sum256(manifestBytes)
	digestText := hex.EncodeToString(manifestDigest[:])

	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	existing, existingErr := m.Store.CredentialMaterialization(ctx, request.LeaseID)
	if existingErr == nil {
		if existing.ManifestDigest == digestText && existing.State == domain.MaterializationActive {
			return existing, nil
		}
		return domain.CredentialMaterialization{}, domain.NewError(domain.CodeMaterializationConflict, "materialization lease already has a different manifest")
	}
	if domain.CodeOf(existingErr) != domain.CodeNotFound {
		return domain.CredentialMaterialization{}, existingErr
	}
	if err := m.ensureRoots(); err != nil {
		return domain.CredentialMaterialization{}, err
	}
	materialization := domain.CredentialMaterialization{LeaseID: request.LeaseID, CredentialInstanceID: request.CredentialInstanceID,
		CredentialRevision: request.CredentialRevision, ManifestVersion: ManifestVersion, ManifestDigest: digestText,
		State: domain.MaterializationPending, RefCount: request.RefCount, CreatedAt: now, UpdatedAt: now}
	if err := m.Store.CreateCredentialMaterialization(ctx, materialization); err != nil {
		return domain.CredentialMaterialization{}, err
	}
	staging, err := os.MkdirTemp(filepath.Join(m.Root, "leases"), ".staging-")
	if err != nil {
		return domain.CredentialMaterialization{}, domain.WrapError(domain.CodeConflict, "materialization staging directory could not be created", err)
	}
	if err := restrictDir(staging); err != nil {
		_ = os.RemoveAll(staging)
		return domain.CredentialMaterialization{}, err
	}
	stagingPath := filepath.Join(staging, "materialization")
	if err := os.Mkdir(stagingPath, 0o700); err != nil {
		_ = os.RemoveAll(staging)
		return domain.CredentialMaterialization{}, domain.WrapError(domain.CodeConflict, "materialization staging path could not be created", err)
	}
	if err := restrictDir(stagingPath); err != nil {
		_ = os.RemoveAll(staging)
		return domain.CredentialMaterialization{}, err
	}
	if err := writePrivate(filepath.Join(stagingPath, "manifest.json"), manifestBytes); err != nil {
		_ = os.RemoveAll(staging)
		return domain.CredentialMaterialization{}, err
	}
	if err := writePrivate(filepath.Join(stagingPath, "credential.fake"), request.Content); err != nil {
		_ = os.RemoveAll(staging)
		return domain.CredentialMaterialization{}, err
	}
	finalPath := m.finalPath(request.LeaseID)
	if _, err := os.Lstat(finalPath); err == nil {
		_ = os.RemoveAll(staging)
		return domain.CredentialMaterialization{}, domain.NewError(domain.CodeMaterializationConflict, "materialization path already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		_ = os.RemoveAll(staging)
		return domain.CredentialMaterialization{}, domain.WrapError(domain.CodeConflict, "materialization path could not be inspected", err)
	}
	if err := os.Rename(stagingPath, finalPath); err != nil {
		_ = os.RemoveAll(staging)
		return domain.CredentialMaterialization{}, domain.WrapError(domain.CodeConflict, "materialization could not be committed", err)
	}
	_ = os.RemoveAll(staging)
	active, err := m.Store.TransitionCredentialMaterialization(ctx, request.LeaseID, domain.MaterializationPending, domain.MaterializationActive, request.RefCount, m.now())
	if err != nil {
		return domain.CredentialMaterialization{}, err
	}
	return active, nil
}

func (m *Materializer) Recover(ctx context.Context) error {
	if m == nil || m.Store == nil || m.Root == "" {
		return domain.NewError(domain.CodeInvalidArgument, "materializer is incomplete")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureRoots(); err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(m.Root, "leases"))
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "materialization directory could not be listed", err)
	}
	seen := make(map[domain.ID]struct{})
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".staging-") {
			if err := m.quarantine(entry.Name()); err != nil {
				return err
			}
			continue
		}
		leaseID := domain.ID(entry.Name())
		if domain.ValidateID(leaseID) != nil || !entry.IsDir() {
			if err := m.quarantine(entry.Name()); err != nil {
				return err
			}
			continue
		}
		seen[leaseID] = struct{}{}
		materialization, err := m.Store.CredentialMaterialization(ctx, leaseID)
		if domain.CodeOf(err) == domain.CodeNotFound {
			if err := m.quarantine(entry.Name()); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		manifest, err := readManifest(filepath.Join(m.finalPath(leaseID), "manifest.json"))
		if err != nil || !manifestMatches(manifest, materialization, filepath.Join(m.finalPath(leaseID), "credential.fake")) {
			if materialization.State == domain.MaterializationPending || materialization.State == domain.MaterializationActive {
				_, _ = m.Store.TransitionCredentialMaterialization(ctx, leaseID, materialization.State, domain.MaterializationQuarantined, 0, m.now())
			}
			if qErr := m.quarantine(entry.Name()); qErr != nil {
				return qErr
			}
			continue
		}
		if materialization.State == domain.MaterializationPending {
			if _, err := m.Store.TransitionCredentialMaterialization(ctx, leaseID, domain.MaterializationPending, domain.MaterializationActive, materialization.RefCount, m.now()); err != nil {
				return err
			}
		}
	}
	rows, err := m.Store.ListCredentialMaterializations(ctx)
	if err != nil {
		return err
	}
	for _, materialization := range rows {
		if materialization.State == domain.MaterializationPending {
			if _, exists := seen[materialization.LeaseID]; !exists {
				if _, err := m.Store.TransitionCredentialMaterialization(ctx, materialization.LeaseID, domain.MaterializationPending, domain.MaterializationQuarantined, 0, m.now()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (m *Materializer) Release(ctx context.Context, leaseID domain.ID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	materialization, err := m.Store.CredentialMaterialization(ctx, leaseID)
	if err != nil {
		return err
	}
	if materialization.State == domain.MaterializationReleased {
		return nil
	}
	if _, err := m.Store.TransitionCredentialMaterialization(ctx, leaseID, materialization.State, domain.MaterializationReleased, 0, m.now()); err != nil {
		return err
	}
	if err := os.RemoveAll(m.finalPath(leaseID)); err != nil {
		return domain.WrapError(domain.CodeConflict, "materialization could not be released", err)
	}
	return nil
}

func (m *Materializer) ensureRoots() error {
	if err := os.MkdirAll(m.Root, 0o700); err != nil {
		return domain.WrapError(domain.CodeConflict, "materialization root could not be created", err)
	}
	if err := restrictDir(m.Root); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(m.Root, "leases"), 0o700); err != nil {
		return domain.WrapError(domain.CodeConflict, "materialization root could not be created", err)
	}
	if err := os.MkdirAll(filepath.Join(m.Root, "quarantine"), 0o700); err != nil {
		return domain.WrapError(domain.CodeConflict, "materialization quarantine could not be created", err)
	}
	if err := restrictDir(filepath.Join(m.Root, "leases")); err != nil {
		return err
	}
	return restrictDir(filepath.Join(m.Root, "quarantine"))
}

func (m *Materializer) finalPath(leaseID domain.ID) string {
	return filepath.Join(m.Root, "leases", string(leaseID))
}

func (m *Materializer) quarantine(name string) error {
	source := filepath.Join(m.Root, "leases", name)
	target := filepath.Join(m.Root, "quarantine", name+"-"+m.now().Format("20060102T150405.000000000Z"))
	if err := os.Rename(source, target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return domain.WrapError(domain.CodeQuarantined, "ambiguous materialization could not be quarantined", err)
	}
	return nil
}

func writePrivate(path string, data []byte) error {
	return device.WritePrivateFileAtomic(path, data)
}

func restrictDir(path string) error {
	return device.ProtectPrivateDirectory(path)
}

func readManifest(path string) (MaterializationManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) > 4096 {
		return MaterializationManifest{}, domain.NewError(domain.CodeSchemaIncompatible, "materialization manifest is unavailable")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest MaterializationManifest
	if err := decoder.Decode(&manifest); err != nil {
		return MaterializationManifest{}, domain.NewError(domain.CodeSchemaIncompatible, "materialization manifest is invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return MaterializationManifest{}, domain.NewError(domain.CodeSchemaIncompatible, "materialization manifest is invalid")
	}
	return manifest, nil
}

func manifestMatches(manifest MaterializationManifest, materialization domain.CredentialMaterialization, contentPath string) bool {
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return false
	}
	digest := sha256.Sum256(encoded)
	content, err := os.ReadFile(contentPath)
	if err != nil {
		return false
	}
	contentDigest := sha256.Sum256(content)
	return manifest.ManifestVersion == ManifestVersion && manifest.LeaseID == materialization.LeaseID &&
		manifest.CredentialInstanceID == materialization.CredentialInstanceID && manifest.CredentialRevision == materialization.CredentialRevision &&
		manifest.State == string(domain.MaterializationActive) && hex.EncodeToString(digest[:]) == materialization.ManifestDigest &&
		manifest.ContentDigest == hex.EncodeToString(contentDigest[:])
}
