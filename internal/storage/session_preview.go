package storage

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

// SessionStartPreview is the durable authority for one confirmed Codex
// Session reservation. It deliberately contains no Provider identity or
// credential material.
type SessionStartPreview struct {
	ID                    domain.ID
	ClientID              domain.ID
	Provider              string
	AccountID             domain.ID
	AccountRevision       int64
	RuntimeProfileID      domain.ID
	ProfileRevision       int64
	CredentialInstanceID  domain.ID
	CredentialRevision    int64
	DeviceID              domain.ID
	WorkspaceID           domain.ID
	WorkspaceUpdatedAt    time.Time
	UsageSnapshotID       domain.ID
	ProviderVersion       string
	BinaryFingerprint     string
	SchemaFingerprint     string
	CapabilityDigest      string
	CreatedAt             time.Time
	ExpiresAt             time.Time
	ConsumedAt            *time.Time
	ConsumedRequestDigest string
	SessionID             domain.ID
}

type SessionStartConfirmation struct {
	Confirmed            bool
	AccountID            domain.ID
	AccountRevision      int64
	RuntimeProfileID     domain.ID
	ProfileRevision      int64
	CredentialInstanceID domain.ID
	CredentialRevision   int64
	DeviceID             domain.ID
	WorkspaceID          domain.ID
	UsageSnapshotID      domain.ID
	ProviderVersion      string
}

type ConsumeSessionStartPreviewRequest struct {
	PreviewID         domain.ID
	ClientID          domain.ID
	RequestDigest     string
	At                time.Time
	BinaryFingerprint string
	SchemaFingerprint string
	CapabilityDigest  string
	Confirmation      SessionStartConfirmation
	Session           domain.Session
}

func validFingerprint(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validSessionStartPreview(value SessionStartPreview) bool {
	for _, id := range []domain.ID{value.ID, value.ClientID, value.AccountID, value.RuntimeProfileID,
		value.CredentialInstanceID, value.DeviceID, value.WorkspaceID} {
		if domain.ValidateID(id) != nil {
			return false
		}
	}
	if value.UsageSnapshotID != "" && domain.ValidateID(value.UsageSnapshotID) != nil {
		return false
	}
	return value.Provider == domain.ProviderCodex && value.AccountRevision >= 1 && value.ProfileRevision >= 1 &&
		value.CredentialRevision >= 1 && len(value.ProviderVersion) >= 1 && len(value.ProviderVersion) <= 128 &&
		validFingerprint(value.BinaryFingerprint) && validFingerprint(value.SchemaFingerprint) &&
		validFingerprint(value.CapabilityDigest) && !value.WorkspaceUpdatedAt.IsZero() &&
		!value.CreatedAt.IsZero() && value.ExpiresAt.After(value.CreatedAt)
}

// CreateSessionStartPreview persists an owner-bound preview only while the
// complete public selector tuple is still enabled, healthy, linked and sealed.
func (s *Store) CreateSessionStartPreview(ctx context.Context, value SessionStartPreview) error {
	if !validSessionStartPreview(value) || value.ConsumedAt != nil || value.ConsumedRequestDigest != "" || value.SessionID != "" {
		return domain.NewError(domain.CodeInvalidArgument, "session preview is invalid")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var accountProvider string
		var accountEnabled, accountInternal int
		var accountRevision int64
		if err := tx.QueryRowContext(ctx, `SELECT provider, enabled, internal, revision FROM accounts WHERE id=?`, value.AccountID).
			Scan(&accountProvider, &accountEnabled, &accountInternal, &accountRevision); err != nil {
			return domain.NewError(domain.CodeProfileBindingChanged, "preview account is unavailable")
		}
		var profileAccount, profileDevice, profileCredential domain.ID
		var profileProvider string
		var profileEnabled, profileInternal int
		var profileRevision int64
		if err := tx.QueryRowContext(ctx, `SELECT account_id, device_id, coalesce(credential_instance_id,''), provider, enabled, internal, revision FROM runtime_profiles WHERE id=?`, value.RuntimeProfileID).
			Scan(&profileAccount, &profileDevice, &profileCredential, &profileProvider, &profileEnabled, &profileInternal, &profileRevision); err != nil {
			return domain.NewError(domain.CodeProfileBindingChanged, "preview profile is unavailable")
		}
		var credentialAccount, credentialDevice domain.ID
		var credentialProvider string
		var credentialStatus domain.CredentialStatus
		var credentialRevision int64
		if err := tx.QueryRowContext(ctx, `SELECT account_id, device_id, provider, status, credential_revision FROM credential_instances WHERE id=?`, value.CredentialInstanceID).
			Scan(&credentialAccount, &credentialDevice, &credentialProvider, &credentialStatus, &credentialRevision); err != nil {
			return domain.NewError(domain.CodeProfileBindingChanged, "preview credential is unavailable")
		}
		if accountProvider != domain.ProviderCodex || accountEnabled != 1 || accountInternal != 0 ||
			profileAccount != value.AccountID || profileDevice != value.DeviceID || profileCredential != value.CredentialInstanceID ||
			profileProvider != domain.ProviderCodex || profileEnabled != 1 || profileInternal != 0 ||
			credentialAccount != value.AccountID || credentialDevice != value.DeviceID || credentialProvider != domain.ProviderCodex ||
			credentialStatus != domain.CredentialHealthy || accountRevision != value.AccountRevision ||
			profileRevision != value.ProfileRevision || credentialRevision != value.CredentialRevision {
			return domain.NewError(domain.CodeProfileBindingChanged, "profile binding changed before preview")
		}
		var workspaceDevice domain.ID
		var workspaceUpdatedAt string
		if err := tx.QueryRowContext(ctx, `SELECT device_id, updated_at FROM workspaces WHERE id=?`, value.WorkspaceID).Scan(&workspaceDevice, &workspaceUpdatedAt); err != nil || workspaceDevice != value.DeviceID || workspaceUpdatedAt != formatTime(value.WorkspaceUpdatedAt) {
			return domain.NewError(domain.CodeProfileBindingChanged, "preview workspace is unavailable")
		}
		var sealed, revoking int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM vault_items WHERE credential_instance_id=? AND credential_revision=?`, value.CredentialInstanceID, value.CredentialRevision).Scan(&sealed); err != nil || sealed != 1 {
			return domain.NewError(domain.CodeProfileBindingChanged, "preview credential is not sealed")
		}
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM credential_revocations WHERE credential_instance_id=?`, value.CredentialInstanceID).Scan(&revoking); err != nil || revoking != 0 {
			return domain.NewError(domain.CodePermissionDenied, "credential revocation is in progress")
		}
		if value.UsageSnapshotID != "" {
			var usageAccount, usageDevice domain.ID
			var usageCredential sql.NullString
			if err := tx.QueryRowContext(ctx, `SELECT account_id, device_id, credential_instance_id FROM usage_snapshots WHERE id=?`, value.UsageSnapshotID).
				Scan(&usageAccount, &usageDevice, &usageCredential); err != nil || usageAccount != value.AccountID || usageDevice != value.DeviceID ||
				(usageCredential.Valid && domain.ID(usageCredential.String) != value.CredentialInstanceID) {
				return domain.NewError(domain.CodeProfileBindingChanged, "preview usage binding is invalid")
			}
		}
		var usage any
		if value.UsageSnapshotID != "" {
			usage = value.UsageSnapshotID
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO session_start_previews(
			id, client_id, provider, account_id, account_revision, runtime_profile_id,
			profile_revision, credential_instance_id, credential_revision, device_id,
			workspace_id, workspace_updated_at, usage_snapshot_id, provider_version, binary_fingerprint,
			schema_fingerprint, capability_digest, created_at, expires_at
		) VALUES(?, ?, 'codex', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			value.ID, value.ClientID, value.AccountID, value.AccountRevision, value.RuntimeProfileID,
			value.ProfileRevision, value.CredentialInstanceID, value.CredentialRevision, value.DeviceID,
			value.WorkspaceID, formatTime(value.WorkspaceUpdatedAt), usage, value.ProviderVersion, value.BinaryFingerprint,
			value.SchemaFingerprint, value.CapabilityDigest, formatTime(value.CreatedAt), formatTime(value.ExpiresAt))
		return writeError("session preview could not be created", err)
	})
}

func (s *Store) SessionStartPreview(ctx context.Context, id domain.ID) (SessionStartPreview, error) {
	var value SessionStartPreview
	var usage, consumedAt, consumedDigest, sessionID sql.NullString
	var workspaceUpdatedAt, createdAt, expiresAt string
	err := s.db.QueryRowContext(ctx, `SELECT id, client_id, provider, account_id, account_revision,
		runtime_profile_id, profile_revision, credential_instance_id, credential_revision,
		device_id, workspace_id, workspace_updated_at, usage_snapshot_id, provider_version, binary_fingerprint,
		schema_fingerprint, capability_digest, created_at, expires_at, consumed_at,
		consumed_request_digest, session_id FROM session_start_previews WHERE id=?`, id).Scan(
		&value.ID, &value.ClientID, &value.Provider, &value.AccountID, &value.AccountRevision,
		&value.RuntimeProfileID, &value.ProfileRevision, &value.CredentialInstanceID, &value.CredentialRevision,
		&value.DeviceID, &value.WorkspaceID, &workspaceUpdatedAt, &usage, &value.ProviderVersion, &value.BinaryFingerprint,
		&value.SchemaFingerprint, &value.CapabilityDigest, &createdAt, &expiresAt, &consumedAt, &consumedDigest, &sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionStartPreview{}, domain.NewError(domain.CodeNotFound, "session preview not found")
	}
	if err != nil {
		return SessionStartPreview{}, domain.WrapError(domain.CodeConflict, "session preview could not be read", err)
	}
	if usage.Valid {
		value.UsageSnapshotID = domain.ID(usage.String)
	}
	if consumedDigest.Valid {
		value.ConsumedRequestDigest = consumedDigest.String
	}
	if sessionID.Valid {
		value.SessionID = domain.ID(sessionID.String)
	}
	value.WorkspaceUpdatedAt, err = parseTime(workspaceUpdatedAt)
	if err != nil {
		return SessionStartPreview{}, err
	}
	value.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return SessionStartPreview{}, err
	}
	value.ExpiresAt, err = parseTime(expiresAt)
	if err != nil {
		return SessionStartPreview{}, err
	}
	value.ConsumedAt, err = parseOptionalTime(consumedAt)
	if err != nil {
		return SessionStartPreview{}, err
	}
	return value, nil
}

// ConsumeSessionStartPreview validates and consumes a preview in the same
// transaction that inserts the immutable starting Session.
func (s *Store) ConsumeSessionStartPreview(ctx context.Context, request ConsumeSessionStartPreviewRequest) (domain.Session, error) {
	if domain.ValidateID(request.PreviewID) != nil || domain.ValidateID(request.ClientID) != nil ||
		!validFingerprint(request.RequestDigest) || request.At.IsZero() || !validFingerprint(request.BinaryFingerprint) ||
		!validFingerprint(request.SchemaFingerprint) || !validFingerprint(request.CapabilityDigest) || !request.Confirmation.Confirmed {
		return domain.Session{}, domain.NewError(domain.CodeIdentityConfirmationRequired, "valid session confirmation is required")
	}
	validated, err := domain.NewSession(request.Session)
	if err != nil {
		return domain.Session{}, err
	}
	capabilities, err := domain.CapabilitiesJSON(validated.CapabilitySnapshot)
	if err != nil {
		return domain.Session{}, err
	}
	var resultID domain.ID
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		var preview SessionStartPreview
		var usage, consumedAt, consumedDigest, sessionID sql.NullString
		var workspaceUpdatedAt, expiresAt string
		if err := tx.QueryRowContext(ctx, `SELECT client_id, provider, account_id, account_revision,
			runtime_profile_id, profile_revision, credential_instance_id, credential_revision,
			device_id, workspace_id, workspace_updated_at, usage_snapshot_id, provider_version, binary_fingerprint,
			schema_fingerprint, capability_digest, expires_at, consumed_at,
			consumed_request_digest, session_id FROM session_start_previews WHERE id=?`, request.PreviewID).Scan(
			&preview.ClientID, &preview.Provider, &preview.AccountID, &preview.AccountRevision,
			&preview.RuntimeProfileID, &preview.ProfileRevision, &preview.CredentialInstanceID, &preview.CredentialRevision,
			&preview.DeviceID, &preview.WorkspaceID, &workspaceUpdatedAt, &usage, &preview.ProviderVersion, &preview.BinaryFingerprint,
			&preview.SchemaFingerprint, &preview.CapabilityDigest, &expiresAt, &consumedAt, &consumedDigest, &sessionID); errors.Is(err, sql.ErrNoRows) {
			return domain.NewError(domain.CodeIdentityConfirmationRequired, "daemon-issued session preview is required")
		} else if err != nil {
			return domain.WrapError(domain.CodeConflict, "session preview could not be read", err)
		}
		if usage.Valid {
			preview.UsageSnapshotID = domain.ID(usage.String)
		}
		preview.WorkspaceUpdatedAt, err = parseTime(workspaceUpdatedAt)
		if err != nil {
			return err
		}
		if preview.ClientID != request.ClientID {
			return domain.NewError(domain.CodePermissionDenied, "session preview owner is required")
		}
		if consumedAt.Valid {
			if consumedDigest.Valid && consumedDigest.String == request.RequestDigest && sessionID.Valid {
				resultID = domain.ID(sessionID.String)
				return nil
			}
			return domain.NewError(domain.CodeConflict, "session preview was already consumed")
		}
		expires, parseErr := parseTime(expiresAt)
		if parseErr != nil {
			return parseErr
		}
		if !request.At.Before(expires) {
			return domain.NewError(domain.CodeConfirmationExpired, "session confirmation expired")
		}
		confirmation := request.Confirmation
		if !confirmation.Confirmed || confirmation.AccountID != preview.AccountID || confirmation.AccountRevision != preview.AccountRevision ||
			confirmation.RuntimeProfileID != preview.RuntimeProfileID || confirmation.ProfileRevision != preview.ProfileRevision ||
			confirmation.CredentialInstanceID != preview.CredentialInstanceID || confirmation.CredentialRevision != preview.CredentialRevision ||
			confirmation.DeviceID != preview.DeviceID || confirmation.WorkspaceID != preview.WorkspaceID ||
			confirmation.UsageSnapshotID != preview.UsageSnapshotID || confirmation.ProviderVersion != preview.ProviderVersion ||
			request.BinaryFingerprint != preview.BinaryFingerprint || request.SchemaFingerprint != preview.SchemaFingerprint ||
			request.CapabilityDigest != preview.CapabilityDigest {
			return domain.NewError(domain.CodeProfileBindingChanged, "profile binding changed after preview")
		}
		if validated.AccountID != preview.AccountID || validated.RuntimeProfileID != preview.RuntimeProfileID ||
			validated.CredentialInstanceID != preview.CredentialInstanceID || validated.DeviceID != preview.DeviceID ||
			validated.WorkspaceID != preview.WorkspaceID || validated.Provider != domain.ProviderCodex {
			return domain.NewError(domain.CodeProfileBindingChanged, "session does not match preview")
		}
		var accountEnabled, profileEnabled int
		var accountRevision, profileRevision, credentialRevision int64
		var profileCredential domain.ID
		var credentialStatus domain.CredentialStatus
		if err := tx.QueryRowContext(ctx, `SELECT enabled, revision FROM accounts WHERE id=? AND provider='codex' AND internal=0`, preview.AccountID).Scan(&accountEnabled, &accountRevision); err != nil || accountEnabled != 1 || accountRevision != preview.AccountRevision {
			return domain.NewError(domain.CodeProfileBindingChanged, "account changed after preview")
		}
		if err := tx.QueryRowContext(ctx, `SELECT enabled, revision, coalesce(credential_instance_id,'') FROM runtime_profiles WHERE id=? AND account_id=? AND device_id=? AND provider='codex' AND internal=0`, preview.RuntimeProfileID, preview.AccountID, preview.DeviceID).Scan(&profileEnabled, &profileRevision, &profileCredential); err != nil || profileEnabled != 1 || profileRevision != preview.ProfileRevision || profileCredential != preview.CredentialInstanceID {
			return domain.NewError(domain.CodeProfileBindingChanged, "profile changed after preview")
		}
		if err := tx.QueryRowContext(ctx, `SELECT status, credential_revision FROM credential_instances WHERE id=? AND account_id=? AND device_id=? AND provider='codex'`, preview.CredentialInstanceID, preview.AccountID, preview.DeviceID).Scan(&credentialStatus, &credentialRevision); err != nil || credentialStatus != domain.CredentialHealthy || credentialRevision != preview.CredentialRevision {
			return domain.NewError(domain.CodeProfileBindingChanged, "credential changed after preview")
		}
		var latestUsage sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT id FROM usage_snapshots WHERE account_id=? ORDER BY observed_at DESC, id DESC LIMIT 1`, preview.AccountID).Scan(&latestUsage); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return domain.WrapError(domain.CodeConflict, "usage binding could not be checked", err)
		}
		if (latestUsage.Valid && domain.ID(latestUsage.String) != preview.UsageSnapshotID) || (!latestUsage.Valid && preview.UsageSnapshotID != "") {
			return domain.NewError(domain.CodeProfileBindingChanged, "usage snapshot changed after preview")
		}
		if err := validateSessionLinks(ctx, tx, validated); err != nil {
			return err
		}
		var currentWorkspaceUpdatedAt string
		if err := tx.QueryRowContext(ctx, `SELECT updated_at FROM workspaces WHERE id=?`, preview.WorkspaceID).Scan(&currentWorkspaceUpdatedAt); err != nil || currentWorkspaceUpdatedAt != formatTime(preview.WorkspaceUpdatedAt) {
			return domain.NewError(domain.CodeProfileBindingChanged, "workspace changed after preview")
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO sessions(
			id, device_id, account_id, provider, credential_instance_id, runtime_profile_id,
			workspace_id, provider_session_id, resumed_from_session_id, status, started_at,
			ended_at, exit_code, capability_snapshot_json, failure_code
		) VALUES(?, ?, ?, 'codex', ?, ?, ?, NULL, NULL, 'starting', ?, NULL, NULL, ?, '')`,
			validated.ID, validated.DeviceID, validated.AccountID, validated.CredentialInstanceID,
			validated.RuntimeProfileID, validated.WorkspaceID, formatTime(validated.StartedAt), capabilities)
		if err != nil {
			return writeError("reserved session could not be created", err)
		}
		updated, err := tx.ExecContext(ctx, `UPDATE session_start_previews SET consumed_at=?, consumed_request_digest=?, session_id=? WHERE id=? AND consumed_at IS NULL`, formatTime(request.At), request.RequestDigest, validated.ID, request.PreviewID)
		if err != nil {
			return writeError("session preview could not be consumed", err)
		}
		changed, _ := updated.RowsAffected()
		if changed != 1 {
			return domain.NewError(domain.CodeConflict, "session preview changed before consumption")
		}
		resultID = validated.ID
		return nil
	})
	if err != nil {
		return domain.Session{}, err
	}
	return s.Session(ctx, resultID)
}

func (s *Store) DeleteExpiredSessionStartPreviews(ctx context.Context, before time.Time) error {
	if before.IsZero() {
		return domain.NewError(domain.CodeInvalidArgument, "preview cleanup timestamp is required")
	}
	cutoff := before.Add(-SessionStartPreviewRetention)
	_, err := s.db.ExecContext(ctx, `DELETE FROM session_start_previews WHERE expires_at<? AND (consumed_at IS NULL OR consumed_at<?)`, formatTime(cutoff), formatTime(cutoff))
	return writeError("expired session previews could not be deleted", err)
}

const SessionStartPreviewRetention = 24 * time.Hour
