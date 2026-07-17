package storage

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

type VaultConfig struct {
	FormatVersion       int
	KDFName             string
	KDFSalt             []byte
	ArgonTime           uint32
	ArgonMemoryKiB      uint32
	ArgonParallelism    uint8
	KeyCheckNonce       []byte
	KeyCheckCiphertext  []byte
	InitializedAt       time.Time
	InitializedByDevice domain.ID
	InitRequestDigest   string
}

type VaultItem struct {
	CredentialInstanceID domain.ID
	AccountID            domain.ID
	DeviceID             domain.ID
	Provider             string
	EnvelopeVersion      int
	CredentialRevision   int64
	CipherName           string
	PayloadNonce         []byte
	PayloadCiphertext    []byte
	WrapName             string
	WrapNonce            []byte
	WrappedDEK           []byte
	AADDigest            string
	SecretDigest         string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (s *Store) VaultConfig(ctx context.Context) (VaultConfig, error) {
	var value VaultConfig
	var initializedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT format_version, kdf_name, kdf_salt, argon_time, argon_memory_kib,
		       argon_parallelism, key_check_nonce, key_check_ciphertext,
		       initialized_at, initialized_by_device_id, init_request_digest
		FROM vault_config WHERE singleton_id = 1`).Scan(
		&value.FormatVersion, &value.KDFName, &value.KDFSalt, &value.ArgonTime,
		&value.ArgonMemoryKiB, &value.ArgonParallelism, &value.KeyCheckNonce,
		&value.KeyCheckCiphertext, &initializedAt, &value.InitializedByDevice,
		&value.InitRequestDigest)
	if errors.Is(err, sql.ErrNoRows) {
		return VaultConfig{}, domain.NewError(domain.CodeNotFound, "vault is uninitialized")
	}
	if err != nil {
		return VaultConfig{}, domain.WrapError(domain.CodeVaultCorrupt, "vault configuration is invalid", err)
	}
	value.InitializedAt, err = parseTime(initializedAt)
	if err != nil {
		return VaultConfig{}, domain.NewError(domain.CodeVaultCorrupt, "vault configuration time is invalid")
	}
	if !validVaultConfig(value) {
		return VaultConfig{}, domain.NewError(domain.CodeVaultCorrupt, "vault configuration is invalid")
	}
	return value, nil
}

func validVaultConfig(value VaultConfig) bool {
	if value.FormatVersion != 1 || value.KDFName != "argon2id-v19" || len(value.KDFSalt) != 16 || value.ArgonTime != 3 || value.ArgonMemoryKiB != 65536 ||
		value.ArgonParallelism < 1 || value.ArgonParallelism > 4 || len(value.KeyCheckNonce) != 12 || len(value.KeyCheckCiphertext) != 49 ||
		domain.ValidateID(value.InitializedByDevice) != nil || len(value.InitRequestDigest) != 64 || value.InitializedAt.IsZero() {
		return false
	}
	_, err := hex.DecodeString(value.InitRequestDigest)
	return err == nil
}

// InitializeVault stores the one password-bound singleton. The caller derives
// the key-check before this transaction; no password or body digest is passed.
func (s *Store) InitializeVault(ctx context.Context, config VaultConfig) (bool, error) {
	if !validVaultConfig(config) {
		return false, domain.NewError(domain.CodeInvalidArgument, "vault initialization is invalid")
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return false, domain.WrapError(domain.CodeConflict, "vault initialization connection could not be acquired", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return false, domain.WrapError(domain.CodeConflict, "vault initialization transaction could not start", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()

	var existing VaultConfig
	var initializedAt string
	err = conn.QueryRowContext(ctx, `SELECT format_version, kdf_name, kdf_salt, argon_time, argon_memory_kib, argon_parallelism, key_check_nonce, key_check_ciphertext, initialized_at, initialized_by_device_id, init_request_digest FROM vault_config WHERE singleton_id=1`).Scan(
		&existing.FormatVersion, &existing.KDFName, &existing.KDFSalt, &existing.ArgonTime,
		&existing.ArgonMemoryKiB, &existing.ArgonParallelism, &existing.KeyCheckNonce,
		&existing.KeyCheckCiphertext, &initializedAt, &existing.InitializedByDevice,
		&existing.InitRequestDigest)
	if err == nil {
		existing.InitializedAt, err = parseTime(initializedAt)
		if err != nil || !validVaultConfig(existing) {
			return false, domain.NewError(domain.CodeVaultCorrupt, "vault configuration is invalid")
		}
		if existing.InitializedByDevice != config.InitializedByDevice || existing.InitRequestDigest != config.InitRequestDigest {
			return false, domain.NewError(domain.CodeVaultAlreadyInitialized, "vault is already initialized")
		}
		if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
			return false, domain.WrapError(domain.CodeConflict, "vault initialization replay could not commit", err)
		}
		committed = true
		return true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, domain.WrapError(domain.CodeVaultCorrupt, "vault configuration could not be checked", err)
	}
	var dependencies int
	if err := conn.QueryRowContext(ctx, `
			SELECT
			  (SELECT count(*) FROM credential_instances WHERE provider = 'codex' AND (secret_ref <> '' OR credential_revision > 0)) +
			  (SELECT count(*) FROM auth_enrollments WHERE state IN ('begun','validating','awaiting_confirmation')) +
			  (SELECT count(*) FROM sessions WHERE provider = 'codex' AND status IN ('starting','running','stopping'))`).Scan(&dependencies); err != nil {
		return false, domain.WrapError(domain.CodeConflict, "vault dependencies could not be checked", err)
	}
	if dependencies != 0 {
		return false, domain.NewError(domain.CodeConflict, "vault initialization is blocked by Codex state")
	}
	result, err := conn.ExecContext(ctx, `
			INSERT INTO vault_config(singleton_id, format_version, kdf_name, kdf_salt,
				argon_time, argon_memory_kib, argon_parallelism, key_check_nonce,
				key_check_ciphertext, initialized_at, initialized_by_device_id,
				init_request_digest, created_at, updated_at)
			SELECT 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
			WHERE NOT EXISTS (SELECT 1 FROM vault_config WHERE singleton_id=1)`,
		config.FormatVersion, config.KDFName, config.KDFSalt, config.ArgonTime,
		config.ArgonMemoryKiB, config.ArgonParallelism, config.KeyCheckNonce,
		config.KeyCheckCiphertext, formatTime(config.InitializedAt),
		config.InitializedByDevice, config.InitRequestDigest,
		formatTime(config.InitializedAt), formatTime(config.InitializedAt))
	if err != nil {
		return false, writeError("vault could not be initialized", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return false, domain.NewError(domain.CodeVaultAlreadyInitialized, "vault is already initialized")
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return false, domain.WrapError(domain.CodeConflict, "vault initialization could not commit", err)
	}
	committed = true
	return false, nil
}

func (s *Store) VaultItem(ctx context.Context, credentialID domain.ID) (VaultItem, error) {
	var value VaultItem
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT credential_instance_id, account_id, device_id, provider,
		       envelope_version, credential_revision, cipher_name, payload_nonce,
		       payload_ciphertext, wrap_name, wrap_nonce, wrapped_dek, aad_digest,
		       secret_digest, created_at, updated_at
		FROM vault_items WHERE credential_instance_id = ?`, credentialID).Scan(
		&value.CredentialInstanceID, &value.AccountID, &value.DeviceID, &value.Provider,
		&value.EnvelopeVersion, &value.CredentialRevision, &value.CipherName,
		&value.PayloadNonce, &value.PayloadCiphertext, &value.WrapName, &value.WrapNonce,
		&value.WrappedDEK, &value.AADDigest, &value.SecretDigest, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return VaultItem{}, domain.NewError(domain.CodeNotFound, "vault item not found")
	}
	if err != nil {
		return VaultItem{}, domain.WrapError(domain.CodeVaultCorrupt, "vault item could not be read", err)
	}
	value.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return VaultItem{}, err
	}
	value.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return VaultItem{}, err
	}
	if !validVaultItem(value) {
		return VaultItem{}, domain.NewError(domain.CodeVaultCorrupt, "vault item structure is invalid")
	}
	return value, nil
}

func validVaultItem(value VaultItem) bool {
	if domain.ValidateID(value.CredentialInstanceID) != nil || domain.ValidateID(value.AccountID) != nil || domain.ValidateID(value.DeviceID) != nil ||
		value.Provider != domain.ProviderCodex || value.EnvelopeVersion != 1 || value.CredentialRevision < 1 ||
		value.CipherName != "aes-256-gcm" || value.WrapName != "aes-256-gcm" || len(value.PayloadNonce) != 12 ||
		len(value.PayloadCiphertext) < 18 || len(value.PayloadCiphertext) > 65552 || len(value.WrapNonce) != 12 || len(value.WrappedDEK) != 48 ||
		len(value.AADDigest) != 64 || len(value.SecretDigest) != 64 || value.CreatedAt.IsZero() || value.UpdatedAt.Before(value.CreatedAt) {
		return false
	}
	if _, err := hex.DecodeString(value.AADDigest); err != nil {
		return false
	}
	if _, err := hex.DecodeString(value.SecretDigest); err != nil {
		return false
	}
	return true
}

// ReplaceVaultItemCAS atomically advances encrypted bytes and the credential
// metadata. The credential row must already exist and match the item identity.
func (s *Store) ReplaceVaultItemCAS(ctx context.Context, expectedRevision int64, item VaultItem, status domain.CredentialStatus) error {
	return s.replaceVaultItemCAS(ctx, expectedRevision, item, status, "", "", "")
}

func (s *Store) ReplaceVaultItemEnrollmentCAS(ctx context.Context, enrollmentID, clientID domain.ID, completionDigest string, expectedRevision int64, item VaultItem, status domain.CredentialStatus) error {
	if enrollmentID == "" || clientID == "" {
		return domain.NewError(domain.CodeInvalidArgument, "enrollment commit identity is required")
	}
	if len(completionDigest) != 64 {
		return domain.NewError(domain.CodeInvalidArgument, "enrollment completion digest is invalid")
	}
	return s.replaceVaultItemCAS(ctx, expectedRevision, item, status, enrollmentID, clientID, completionDigest)
}

func (s *Store) replaceVaultItemCAS(ctx context.Context, expectedRevision int64, item VaultItem, status domain.CredentialStatus, enrollmentID, clientID domain.ID, completionDigest string) error {
	if !validVaultItem(item) {
		return domain.NewError(domain.CodeInvalidArgument, "vault item structure is invalid")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var accountID, deviceID domain.ID
		var provider string
		var currentStatus domain.CredentialStatus
		var revision int64
		if err := tx.QueryRowContext(ctx, `SELECT account_id, device_id, provider, status, credential_revision FROM credential_instances WHERE id = ?`, item.CredentialInstanceID).Scan(&accountID, &deviceID, &provider, &currentStatus, &revision); err != nil {
			return domain.WrapError(domain.CodeNotFound, "credential instance was not found", err)
		}
		if revision != expectedRevision || accountID != item.AccountID || deviceID != item.DeviceID || provider != domain.ProviderCodex || item.CredentialRevision != expectedRevision+1 {
			return domain.NewError(domain.CodeCredentialRevisionConflict, "credential revision changed")
		}
		if currentStatus == domain.CredentialRevoked || currentStatus == domain.CredentialExpired {
			return domain.NewError(domain.CodeCredentialRevisionConflict, "credential is not sealable")
		}
		var revoking int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM credential_revocations WHERE credential_instance_id=?`, item.CredentialInstanceID).Scan(&revoking); err != nil {
			return domain.WrapError(domain.CodeConflict, "credential revocation state could not be checked", err)
		}
		if revoking != 0 {
			return domain.NewError(domain.CodeCredentialRevisionConflict, "credential revocation is in progress")
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO vault_items(credential_instance_id, account_id, device_id, provider,
				envelope_version, credential_revision, cipher_name, payload_nonce,
				payload_ciphertext, wrap_name, wrap_nonce, wrapped_dek, aad_digest,
				secret_digest, created_at, updated_at)
			VALUES(?, ?, ?, 'codex', 1, ?, 'aes-256-gcm', ?, ?, 'aes-256-gcm', ?, ?, ?, ?, ?, ?)
			ON CONFLICT(credential_instance_id) DO UPDATE SET
				credential_revision=excluded.credential_revision, payload_nonce=excluded.payload_nonce,
				payload_ciphertext=excluded.payload_ciphertext, wrap_nonce=excluded.wrap_nonce,
				wrapped_dek=excluded.wrapped_dek, aad_digest=excluded.aad_digest,
				secret_digest=excluded.secret_digest, updated_at=excluded.updated_at`,
			item.CredentialInstanceID, item.AccountID, item.DeviceID, item.CredentialRevision,
			item.PayloadNonce, item.PayloadCiphertext, item.WrapNonce, item.WrappedDEK,
			item.AADDigest, item.SecretDigest, formatTime(item.CreatedAt), formatTime(item.UpdatedAt))
		if err != nil {
			return writeError("vault item could not be saved", err)
		}
		result, err := tx.ExecContext(ctx, `UPDATE credential_instances SET credential_revision=?, secret_digest=?, status=?, updated_at=? WHERE id=? AND credential_revision=?`, item.CredentialRevision, item.SecretDigest, status, formatTime(item.UpdatedAt), item.CredentialInstanceID, expectedRevision)
		if err != nil {
			return writeError("credential revision could not be saved", err)
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return domain.NewError(domain.CodeCredentialRevisionConflict, "credential revision changed")
		}
		if enrollmentID != "" {
			var profileID domain.ID
			if err := tx.QueryRowContext(ctx, `SELECT runtime_profile_id FROM auth_enrollments
					WHERE id=? AND client_device_id=? AND credential_instance_id=?
						AND state='awaiting_confirmation' AND confirmed_by_client_id=?
						AND completion_idempotency_digest=?`, enrollmentID, clientID,
				item.CredentialInstanceID, clientID, completionDigest).Scan(&profileID); err != nil {
				return domain.NewError(domain.CodeIdentityConfirmationRequired, "auth confirmation is required before credential seal")
			}
			profileResult, err := tx.ExecContext(ctx, `UPDATE runtime_profiles SET
					credential_instance_id=?, revision=CASE WHEN credential_instance_id IS NULL THEN revision+1 ELSE revision END,
					updated_at=? WHERE id=? AND account_id=? AND device_id=? AND provider='codex'
						AND (credential_instance_id IS NULL OR credential_instance_id=?)`, item.CredentialInstanceID,
				formatTime(item.UpdatedAt), profileID, item.AccountID, item.DeviceID, item.CredentialInstanceID)
			if err != nil {
				return writeError("auth profile binding could not be committed", err)
			}
			profileChanged, _ := profileResult.RowsAffected()
			if profileChanged != 1 {
				return domain.NewError(domain.CodeProfileBindingChanged, "auth profile binding changed before commit")
			}
			result, err := tx.ExecContext(ctx, `UPDATE auth_enrollments SET state='succeeded', updated_at=? WHERE id=? AND client_device_id=? AND credential_instance_id=? AND state='awaiting_confirmation' AND confirmed_by_client_id=? AND completion_idempotency_digest=?`, formatTime(item.UpdatedAt), enrollmentID, clientID, item.CredentialInstanceID, clientID, completionDigest)
			if err != nil {
				return writeError("auth enrollment could not be committed", err)
			}
			changed, _ := result.RowsAffected()
			if changed != 1 {
				return domain.NewError(domain.CodeConflict, "auth enrollment changed before commit")
			}
		}
		return nil
	})
}

// EnsureVaultCredentialRevocable is a metadata-only diagnostic. Callers that
// will mutate external state must use ReserveVaultCredentialRevocation first.
func (s *Store) EnsureVaultCredentialRevocable(ctx context.Context, credentialID domain.ID) error {
	if err := domain.ValidateID(credentialID); err != nil {
		return err
	}
	var provider string
	if err := s.db.QueryRowContext(ctx, `SELECT provider FROM credential_instances WHERE id=?`, credentialID).Scan(&provider); errors.Is(err, sql.ErrNoRows) {
		return domain.NewError(domain.CodeNotFound, "credential instance not found")
	} else if err != nil {
		return domain.WrapError(domain.CodeConflict, "credential could not be checked", err)
	} else if provider != domain.ProviderCodex {
		return domain.NewError(domain.CodeConflict, "credential provider is unsupported")
	}
	var active int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM sessions WHERE credential_instance_id=? AND status IN ('starting','running','stopping')`, credentialID).Scan(&active); err != nil {
		return domain.WrapError(domain.CodeConflict, "credential sessions could not be checked", err)
	}
	if active != 0 {
		return domain.NewError(domain.CodeConflict, "credential is used by an active session")
	}
	return nil
}

// ReserveVaultCredentialRevocation atomically excludes future Session inserts
// before logout touches the canonical credential home. The reservation is
// durable and idempotent so a crash or cleanup failure remains fail-closed and
// a later logout retry can finish the operation.
func (s *Store) ReserveVaultCredentialRevocation(ctx context.Context, credentialID domain.ID, at time.Time) error {
	if err := domain.ValidateID(credentialID); err != nil {
		return err
	}
	if at.IsZero() {
		return domain.NewError(domain.CodeInvalidArgument, "credential revocation requires a timestamp")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var provider string
		var status domain.CredentialStatus
		if err := tx.QueryRowContext(ctx, `SELECT provider, status FROM credential_instances WHERE id=?`, credentialID).Scan(&provider, &status); errors.Is(err, sql.ErrNoRows) {
			return domain.NewError(domain.CodeNotFound, "credential instance not found")
		} else if err != nil {
			return domain.WrapError(domain.CodeConflict, "credential could not be checked", err)
		} else if provider != domain.ProviderCodex {
			return domain.NewError(domain.CodeConflict, "credential provider is unsupported")
		}
		if status == domain.CredentialRevoked {
			return nil
		}
		var active int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM sessions WHERE credential_instance_id=? AND status IN ('starting','running','stopping')`, credentialID).Scan(&active); err != nil {
			return domain.WrapError(domain.CodeConflict, "credential sessions could not be checked", err)
		}
		if active != 0 {
			return domain.NewError(domain.CodeConflict, "credential is used by an active session")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO credential_revocations(credential_instance_id, requested_at) VALUES(?, ?) ON CONFLICT(credential_instance_id) DO NOTHING`, credentialID, formatTime(at)); err != nil {
			return writeError("credential revocation could not be reserved", err)
		}
		return nil
	})
}

// FinalizeVaultCredentialRevocation consumes a prior reservation, removes the
// encrypted item, and marks metadata revoked in one transaction. An already
// revoked credential is a successful replay after a lost response.
func (s *Store) FinalizeVaultCredentialRevocation(ctx context.Context, credentialID domain.ID, at time.Time) (domain.CredentialInstance, error) {
	if err := domain.ValidateID(credentialID); err != nil {
		return domain.CredentialInstance{}, err
	}
	if at.IsZero() {
		return domain.CredentialInstance{}, domain.NewError(domain.CodeInvalidArgument, "credential revocation requires a timestamp")
	}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var provider string
		var status domain.CredentialStatus
		if err := tx.QueryRowContext(ctx, `SELECT provider, status FROM credential_instances WHERE id=?`, credentialID).Scan(&provider, &status); errors.Is(err, sql.ErrNoRows) {
			return domain.NewError(domain.CodeNotFound, "credential instance not found")
		} else if err != nil {
			return domain.WrapError(domain.CodeConflict, "credential could not be checked", err)
		} else if provider != domain.ProviderCodex {
			return domain.NewError(domain.CodeConflict, "credential provider is unsupported")
		}
		if status == domain.CredentialRevoked {
			if _, err := tx.ExecContext(ctx, `UPDATE runtime_profiles SET credential_instance_id=NULL,
				revision=revision+1, updated_at=? WHERE credential_instance_id=? AND internal=0`, formatTime(at), credentialID); err != nil {
				return writeError("revoked credential profile bindings could not be cleared", err)
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM credential_revocations WHERE credential_instance_id=?`, credentialID); err != nil {
				return writeError("credential revocation replay could not be cleared", err)
			}
			return nil
		}
		var reserved int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM credential_revocations WHERE credential_instance_id=?`, credentialID).Scan(&reserved); err != nil {
			return domain.WrapError(domain.CodeConflict, "credential revocation reservation could not be checked", err)
		}
		if reserved != 1 {
			return domain.NewError(domain.CodeConflict, "credential revocation is not reserved")
		}
		var active int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM sessions WHERE credential_instance_id=? AND status IN ('starting','running','stopping')`, credentialID).Scan(&active); err != nil {
			return domain.WrapError(domain.CodeConflict, "credential sessions could not be checked", err)
		}
		if active != 0 {
			return domain.NewError(domain.CodeConflict, "credential is used by an active session")
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM vault_items WHERE credential_instance_id=?`, credentialID); err != nil {
			return writeError("vault item could not be removed", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE runtime_profiles SET credential_instance_id=NULL,
				revision=revision+1, updated_at=? WHERE credential_instance_id=? AND internal=0`, formatTime(at), credentialID); err != nil {
			return writeError("credential profile bindings could not be cleared", err)
		}
		result, err := tx.ExecContext(ctx, `UPDATE credential_instances SET status='revoked', secret_digest=?, updated_at=? WHERE id=?`, strings.Repeat("0", 64), formatTime(at), credentialID)
		if err != nil {
			return writeError("credential could not be revoked", err)
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return domain.NewError(domain.CodeConflict, "credential changed before revocation")
		}
		result, err = tx.ExecContext(ctx, `DELETE FROM credential_revocations WHERE credential_instance_id=?`, credentialID)
		if err != nil {
			return writeError("credential revocation reservation could not be cleared", err)
		}
		changed, _ = result.RowsAffected()
		if changed != 1 {
			return domain.NewError(domain.CodeConflict, "credential revocation reservation changed before commit")
		}
		return nil
	})
	if err != nil {
		return domain.CredentialInstance{}, err
	}
	return s.CredentialInstance(ctx, credentialID)
}

// RevokeVaultCredential is the filesystem-free convenience path used by Store
// callers. Application logout reserves before deleting external state and then
// calls FinalizeVaultCredentialRevocation directly.
func (s *Store) RevokeVaultCredential(ctx context.Context, credentialID domain.ID, at time.Time) (domain.CredentialInstance, error) {
	if err := s.ReserveVaultCredentialRevocation(ctx, credentialID, at); err != nil {
		return domain.CredentialInstance{}, err
	}
	return s.FinalizeVaultCredentialRevocation(ctx, credentialID, at)
}
