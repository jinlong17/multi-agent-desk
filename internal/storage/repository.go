package storage

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

func (s *Store) CreateDevice(ctx context.Context, device domain.Device) error {
	if device.Kind != domain.DeviceKindDaemon || len(device.SigningPublicKey) != 32 || device.DisplayName == "" ||
		!validCreatedUpdated(device.CreatedAt, device.UpdatedAt) {
		return domain.NewError(domain.CodeInvalidArgument, "invalid daemon device")
	}
	if err := domain.ValidateID(device.ID); err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM device_identity").Scan(&count); err != nil {
			return writeError("device identity could not be checked", err)
		}
		if count != 0 {
			return domain.NewError(domain.CodeAlreadyExists, "device identity already exists")
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO device_identity(id, kind, display_name, signing_public_key, created_at, updated_at)
			VALUES(?, ?, ?, ?, ?, ?)`,
			device.ID, device.Kind, device.DisplayName, device.SigningPublicKey,
			formatTime(device.CreatedAt), formatTime(device.UpdatedAt),
		)
		return writeError("device identity could not be created", err)
	})
}

func (s *Store) Device(ctx context.Context) (domain.Device, error) {
	var device domain.Device
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, kind, display_name, signing_public_key, created_at, updated_at
		FROM device_identity LIMIT 1`).Scan(
		&device.ID, &device.Kind, &device.DisplayName, &device.SigningPublicKey, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Device{}, domain.NewError(domain.CodeNotFound, "device identity not found")
	}
	if err != nil {
		return domain.Device{}, domain.WrapError(domain.CodeConflict, "device identity could not be read", err)
	}
	device.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return domain.Device{}, err
	}
	device.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return domain.Device{}, err
	}
	return device, nil
}

func (s *Store) CreateClientIdentity(ctx context.Context, client domain.ClientIdentity) error {
	if err := domain.ValidateID(client.ID); err != nil {
		return err
	}
	if client.Name == "" || len(client.PublicKey) != 32 || client.Revision < 1 || client.Status != domain.ClientIdentityActive ||
		!validCreatedUpdated(client.CreatedAt, client.UpdatedAt) {
		return domain.NewError(domain.CodeInvalidArgument, "invalid client identity")
	}
	capabilities, err := domain.CapabilitiesJSON(client.Caps)
	if err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO client_identities(id, name, public_key, revision, status, capabilities_json, created_at, updated_at)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
			client.ID, client.Name, client.PublicKey, client.Revision, client.Status, capabilities,
			formatTime(client.CreatedAt), formatTime(client.UpdatedAt),
		)
		return writeError("client identity could not be created", err)
	})
}

func (s *Store) ClientIdentity(ctx context.Context, id domain.ID) (domain.ClientIdentity, error) {
	var client domain.ClientIdentity
	var capabilities []byte
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, public_key, revision, status, capabilities_json, created_at, updated_at
		FROM client_identities WHERE id = ?`, id).Scan(
		&client.ID, &client.Name, &client.PublicKey, &client.Revision, &client.Status,
		&capabilities, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ClientIdentity{}, domain.NewError(domain.CodeNotFound, "client identity not found")
	}
	if err != nil {
		return domain.ClientIdentity{}, domain.WrapError(domain.CodeConflict, "client identity could not be read", err)
	}
	if err := json.Unmarshal(capabilities, &client.Caps); err != nil {
		return domain.ClientIdentity{}, domain.WrapError(domain.CodeSchemaIncompatible, "stored client capabilities are invalid", err)
	}
	client.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return domain.ClientIdentity{}, err
	}
	client.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return domain.ClientIdentity{}, err
	}
	return client, nil
}

func (s *Store) ListClientIdentities(ctx context.Context) ([]domain.ClientIdentity, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, public_key, revision, status, capabilities_json, created_at, updated_at
		FROM client_identities ORDER BY id`)
	if err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "client identities could not be read", err)
	}
	defer rows.Close()
	clients := make([]domain.ClientIdentity, 0)
	for rows.Next() {
		var client domain.ClientIdentity
		var capabilities []byte
		var createdAt, updatedAt string
		if err := rows.Scan(&client.ID, &client.Name, &client.PublicKey, &client.Revision,
			&client.Status, &capabilities, &createdAt, &updatedAt); err != nil {
			return nil, domain.WrapError(domain.CodeConflict, "client identity could not be read", err)
		}
		if err := json.Unmarshal(capabilities, &client.Caps); err != nil {
			return nil, domain.WrapError(domain.CodeSchemaIncompatible, "stored client capabilities are invalid", err)
		}
		client.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		client.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, err
		}
		clients = append(clients, client)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "client identities could not be read", err)
	}
	return clients, nil
}

func (s *Store) RotateClientIdentity(ctx context.Context, id domain.ID, expectedRevision int64, publicKey []byte, at time.Time) (domain.ClientIdentity, error) {
	if err := domain.ValidateID(id); err != nil {
		return domain.ClientIdentity{}, err
	}
	if expectedRevision < 1 || len(publicKey) != 32 || at.IsZero() {
		return domain.ClientIdentity{}, domain.NewError(domain.CodeInvalidArgument, "invalid client identity rotation")
	}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `
			UPDATE client_identities
			SET public_key = ?, revision = revision + 1, updated_at = ?
			WHERE id = ? AND status = 'active' AND revision = ?`,
			publicKey, formatTime(at), id, expectedRevision)
		if err != nil {
			return writeError("client identity could not be rotated", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return writeError("client identity rotation result could not be read", err)
		}
		if changed != 1 {
			return domain.NewError(domain.CodeConflict, "client identity revision changed")
		}
		return nil
	})
	if err != nil {
		return domain.ClientIdentity{}, err
	}
	return s.ClientIdentity(ctx, id)
}

func (s *Store) RevokeClientIdentity(ctx context.Context, id domain.ID, expectedRevision int64, at time.Time) (domain.ClientIdentity, error) {
	if err := domain.ValidateID(id); err != nil {
		return domain.ClientIdentity{}, err
	}
	if expectedRevision < 1 || at.IsZero() {
		return domain.ClientIdentity{}, domain.NewError(domain.CodeInvalidArgument, "invalid client identity revocation")
	}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `
			UPDATE client_identities
			SET status = 'revoked', revision = revision + 1, updated_at = ?
			WHERE id = ? AND status = 'active' AND revision = ?`,
			formatTime(at), id, expectedRevision)
		if err != nil {
			return writeError("client identity could not be revoked", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return writeError("client identity revocation result could not be read", err)
		}
		if changed != 1 {
			return domain.NewError(domain.CodeConflict, "client identity revision changed")
		}
		return nil
	})
	if err != nil {
		return domain.ClientIdentity{}, err
	}
	return s.ClientIdentity(ctx, id)
}

func (s *Store) CreateWorkspace(ctx context.Context, workspace domain.Workspace) error {
	if err := domain.ValidateID(workspace.ID); err != nil {
		return err
	}
	if err := domain.ValidateID(workspace.DeviceID); err != nil {
		return err
	}
	if workspace.Path == "" || !validCreatedUpdated(workspace.CreatedAt, workspace.UpdatedAt) {
		return domain.NewError(domain.CodeInvalidArgument, "workspace path is required")
	}
	tags, err := json.Marshal(workspace.Tags)
	if err != nil {
		return domain.WrapError(domain.CodeInvalidArgument, "workspace tags are invalid", err)
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO workspaces(id, device_id, path, label, tags_json, created_at, updated_at)
			VALUES(?, ?, ?, ?, ?, ?, ?)`,
			workspace.ID, workspace.DeviceID, workspace.Path, workspace.Label, tags,
			formatTime(workspace.CreatedAt), formatTime(workspace.UpdatedAt),
		)
		return writeError("workspace could not be created", err)
	})
}

func (s *Store) Workspace(ctx context.Context, id domain.ID) (domain.Workspace, error) {
	var workspace domain.Workspace
	var tags []byte
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, device_id, path, label, tags_json, created_at, updated_at
		FROM workspaces WHERE id = ?`, id).Scan(
		&workspace.ID, &workspace.DeviceID, &workspace.Path, &workspace.Label, &tags, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Workspace{}, domain.NewError(domain.CodeNotFound, "workspace not found")
	}
	if err != nil {
		return domain.Workspace{}, domain.WrapError(domain.CodeConflict, "workspace could not be read", err)
	}
	if err := json.Unmarshal(tags, &workspace.Tags); err != nil {
		return domain.Workspace{}, domain.WrapError(domain.CodeSchemaIncompatible, "stored workspace tags are invalid", err)
	}
	workspace.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return domain.Workspace{}, err
	}
	workspace.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return domain.Workspace{}, err
	}
	return workspace, nil
}

func (s *Store) CreateRuntimeProfile(ctx context.Context, profile domain.RuntimeProfile) error {
	if err := domain.ValidateID(profile.ID); err != nil {
		return err
	}
	if err := domain.ValidateID(profile.DeviceID); err != nil {
		return err
	}
	if profile.Name == "" || profile.Provider != "fake" || !json.Valid(profile.Settings) ||
		!validCreatedUpdated(profile.CreatedAt, profile.UpdatedAt) {
		return domain.NewError(domain.CodeInvalidArgument, "invalid runtime profile")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO runtime_profiles(id, device_id, name, provider, settings_json, created_at, updated_at)
			VALUES(?, ?, ?, ?, ?, ?, ?)`,
			profile.ID, profile.DeviceID, profile.Name, profile.Provider, profile.Settings,
			formatTime(profile.CreatedAt), formatTime(profile.UpdatedAt),
		)
		return writeError("runtime profile could not be created", err)
	})
}

func (s *Store) RuntimeProfile(ctx context.Context, id domain.ID) (domain.RuntimeProfile, error) {
	var profile domain.RuntimeProfile
	var settings []byte
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, device_id, name, provider, settings_json, created_at, updated_at
		FROM runtime_profiles WHERE id = ?`, id).Scan(
		&profile.ID, &profile.DeviceID, &profile.Name, &profile.Provider, &settings, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RuntimeProfile{}, domain.NewError(domain.CodeNotFound, "runtime profile not found")
	}
	if err != nil {
		return domain.RuntimeProfile{}, domain.WrapError(domain.CodeConflict, "runtime profile could not be read", err)
	}
	if !json.Valid(settings) {
		return domain.RuntimeProfile{}, domain.NewError(domain.CodeSchemaIncompatible, "stored runtime profile settings are invalid")
	}
	profile.Settings = append([]byte(nil), settings...)
	profile.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return domain.RuntimeProfile{}, err
	}
	profile.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return domain.RuntimeProfile{}, err
	}
	return profile, nil
}

func (s *Store) CreateCredentialInstance(ctx context.Context, credential domain.CredentialInstance) error {
	if err := domain.ValidateID(credential.ID); err != nil {
		return err
	}
	if err := domain.ValidateID(credential.DeviceID); err != nil {
		return err
	}
	digest, digestErr := hex.DecodeString(credential.SecretDigest)
	if credential.Provider != "fake" || credential.AuthMethod != "fake" || !strings.HasPrefix(credential.SecretRef, "fake:") ||
		digestErr != nil || len(digest) != 32 || credential.CredentialRevision < 0 ||
		!validCredentialStatus(credential.Status) || !validCreatedUpdated(credential.CreatedAt, credential.UpdatedAt) {
		return domain.NewError(domain.CodeInvalidArgument, "invalid fake credential instance")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO credential_instances(
				id, device_id, provider, auth_method, secret_ref, status,
				credential_revision, secret_digest, created_at, updated_at
			) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			credential.ID, credential.DeviceID, credential.Provider, credential.AuthMethod,
			credential.SecretRef, credential.Status, credential.CredentialRevision,
			credential.SecretDigest, formatTime(credential.CreatedAt), formatTime(credential.UpdatedAt),
		)
		return writeError("credential instance could not be created", err)
	})
}

func (s *Store) CredentialInstance(ctx context.Context, id domain.ID) (domain.CredentialInstance, error) {
	var credential domain.CredentialInstance
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, device_id, provider, auth_method, secret_ref, status,
			credential_revision, secret_digest, created_at, updated_at
		FROM credential_instances WHERE id = ?`, id).Scan(
		&credential.ID, &credential.DeviceID, &credential.Provider, &credential.AuthMethod,
		&credential.SecretRef, &credential.Status, &credential.CredentialRevision,
		&credential.SecretDigest, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CredentialInstance{}, domain.NewError(domain.CodeNotFound, "credential instance not found")
	}
	if err != nil {
		return domain.CredentialInstance{}, domain.WrapError(domain.CodeConflict, "credential instance could not be read", err)
	}
	credential.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return domain.CredentialInstance{}, err
	}
	credential.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return domain.CredentialInstance{}, err
	}
	return credential, nil
}

func (s *Store) CreateSession(ctx context.Context, session domain.Session) error {
	validated, err := domain.NewSession(session)
	if err != nil {
		return err
	}
	capabilities, err := domain.CapabilitiesJSON(validated.CapabilitySnapshot)
	if err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if validated.ResumedFromSessionID != "" {
			if err := validateResumeSource(ctx, tx, validated); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO sessions(
				id, device_id, provider, credential_instance_id, runtime_profile_id,
				workspace_id, provider_session_id, resumed_from_session_id, status,
				started_at, ended_at, exit_code, capability_snapshot_json, failure_code
			) VALUES(?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, NULL, NULL, ?, '')`,
			validated.ID, validated.DeviceID, validated.Provider, validated.CredentialInstanceID,
			validated.RuntimeProfileID, validated.WorkspaceID, validated.ProviderSessionID,
			validated.ResumedFromSessionID, validated.Status, formatTime(validated.StartedAt), capabilities,
		)
		return writeError("session could not be created", err)
	})
}

func (s *Store) Session(ctx context.Context, id domain.ID) (domain.Session, error) {
	var session domain.Session
	var startedAt string
	var endedAt sql.NullString
	var exitCode sql.NullInt64
	var capabilities []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT id, device_id, provider, credential_instance_id, runtime_profile_id,
			workspace_id, coalesce(provider_session_id, ''), coalesce(resumed_from_session_id, ''),
			status, started_at, ended_at, exit_code, capability_snapshot_json, failure_code
		FROM sessions WHERE id = ?`, id).Scan(
		&session.ID, &session.DeviceID, &session.Provider, &session.CredentialInstanceID,
		&session.RuntimeProfileID, &session.WorkspaceID, &session.ProviderSessionID,
		&session.ResumedFromSessionID, &session.Status, &startedAt, &endedAt, &exitCode,
		&capabilities, &session.FailureCode,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Session{}, domain.NewError(domain.CodeNotFound, "session not found")
	}
	if err != nil {
		return domain.Session{}, domain.WrapError(domain.CodeConflict, "session could not be read", err)
	}
	session.StartedAt, err = parseTime(startedAt)
	if err != nil {
		return domain.Session{}, err
	}
	session.EndedAt, err = parseOptionalTime(endedAt)
	if err != nil {
		return domain.Session{}, err
	}
	if exitCode.Valid {
		value := int(exitCode.Int64)
		session.ExitCode = &value
	}
	if err := json.Unmarshal(capabilities, &session.CapabilitySnapshot); err != nil {
		return domain.Session{}, domain.WrapError(domain.CodeSchemaIncompatible, "stored session capabilities are invalid", err)
	}
	return session, nil
}

// TransitionSession applies the domain edge and persists it with a status CAS.
func (s *Store) TransitionSession(ctx context.Context, id domain.ID, expected, next domain.SessionStatus, at time.Time, exitCode *int, failureCode string) (domain.Session, error) {
	session, err := s.Session(ctx, id)
	if err != nil {
		return domain.Session{}, err
	}
	if session.Status != expected {
		return domain.Session{}, domain.NewError(domain.CodeConflict, "session status changed")
	}
	if err := session.Transition(next, at, exitCode, failureCode); err != nil {
		return domain.Session{}, err
	}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		var endedAt any
		if session.EndedAt != nil {
			endedAt = formatTime(*session.EndedAt)
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE sessions SET status = ?, ended_at = ?, exit_code = ?, failure_code = ?
			WHERE id = ? AND status = ?`,
			session.Status, endedAt, session.ExitCode, session.FailureCode, session.ID, expected,
		)
		if err != nil {
			return writeError("session transition could not be persisted", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return writeError("session transition result could not be read", err)
		}
		if changed != 1 {
			return domain.NewError(domain.CodeConflict, "session status changed")
		}
		return nil
	})
	if err != nil {
		return domain.Session{}, err
	}
	return session, nil
}

func (s *Store) CreateAttachment(ctx context.Context, attachment domain.SessionAttachment) error {
	if err := domain.ValidateID(attachment.ID); err != nil {
		return err
	}
	if err := domain.ValidateID(attachment.SessionID); err != nil {
		return err
	}
	if err := domain.ValidateID(attachment.ClientDeviceID); err != nil {
		return err
	}
	if attachment.Mode != domain.AttachmentObserver && attachment.Mode != domain.AttachmentController {
		return domain.NewError(domain.CodeInvalidArgument, "invalid attachment mode")
	}
	if attachment.ConnectedAt.IsZero() || attachment.LastSeenAt.Before(attachment.ConnectedAt) {
		return domain.NewError(domain.CodeInvalidArgument, "invalid attachment timestamps")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO session_attachments(id, session_id, client_device_id, mode, connected_at, last_seen_at)
			VALUES(?, ?, ?, ?, ?, ?)`,
			attachment.ID, attachment.SessionID, attachment.ClientDeviceID, attachment.Mode,
			formatTime(attachment.ConnectedAt), formatTime(attachment.LastSeenAt),
		)
		return writeError("session attachment could not be created", err)
	})
}

func (s *Store) DeleteAttachment(ctx context.Context, sessionID, clientID domain.ID) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "DELETE FROM session_attachments WHERE session_id = ? AND client_device_id = ?", sessionID, clientID)
		return writeError("session attachment could not be deleted", err)
	})
}

// SaveControllerLease performs a compare-and-swap against the stored revision.
// expectedRevision zero inserts the first lease.
func (s *Store) SaveControllerLease(ctx context.Context, lease domain.ControllerLease, expectedRevision int64) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var releasedAt any
		if lease.ReleasedAt != nil {
			releasedAt = formatTime(*lease.ReleasedAt)
		}
		if expectedRevision == 0 {
			if lease.Revision != 1 {
				return domain.NewError(domain.CodeStaleLease, "initial lease revision must be one")
			}
			_, err := tx.ExecContext(ctx, `
				INSERT INTO controller_leases(
					session_id, holder_device_id, lease_revision, expires_at, last_heartbeat_at, released_at
				) VALUES(?, ?, ?, ?, ?, ?)`,
				lease.SessionID, lease.HolderDeviceID, lease.Revision, formatTime(lease.ExpiresAt),
				formatTime(lease.LastHeartbeat), releasedAt,
			)
			if err != nil {
				return domain.WrapError(domain.CodeStaleLease, "controller lease changed", err)
			}
			return nil
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE controller_leases SET holder_device_id = ?, lease_revision = ?, expires_at = ?,
				last_heartbeat_at = ?, released_at = ?
			WHERE session_id = ? AND lease_revision = ?`,
			lease.HolderDeviceID, lease.Revision, formatTime(lease.ExpiresAt),
			formatTime(lease.LastHeartbeat), releasedAt, lease.SessionID, expectedRevision,
		)
		if err != nil {
			return writeError("controller lease could not be saved", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return writeError("controller lease result could not be read", err)
		}
		if changed != 1 {
			return domain.NewError(domain.CodeStaleLease, "controller lease revision is stale")
		}
		return nil
	})
}

func (s *Store) ControllerLease(ctx context.Context, sessionID domain.ID) (domain.ControllerLease, error) {
	var lease domain.ControllerLease
	var expiresAt, heartbeatAt string
	var releasedAt sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT session_id, holder_device_id, lease_revision, expires_at, last_heartbeat_at, released_at
		FROM controller_leases WHERE session_id = ?`, sessionID).Scan(
		&lease.SessionID, &lease.HolderDeviceID, &lease.Revision, &expiresAt, &heartbeatAt, &releasedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ControllerLease{}, domain.NewError(domain.CodeNotFound, "controller lease not found")
	}
	if err != nil {
		return domain.ControllerLease{}, domain.WrapError(domain.CodeConflict, "controller lease could not be read", err)
	}
	lease.ExpiresAt, err = parseTime(expiresAt)
	if err != nil {
		return domain.ControllerLease{}, err
	}
	lease.LastHeartbeat, err = parseTime(heartbeatAt)
	if err != nil {
		return domain.ControllerLease{}, err
	}
	lease.ReleasedAt, err = parseOptionalTime(releasedAt)
	if err != nil {
		return domain.ControllerLease{}, err
	}
	return lease, nil
}

func (s *Store) AppendRuntimeEvent(ctx context.Context, event domain.RuntimeEvent) error {
	if err := domain.ValidateID(event.ID); err != nil {
		return err
	}
	if err := domain.ValidateID(event.SessionID); err != nil {
		return err
	}
	if event.Sequence < 1 || event.Kind == "" || !json.Valid(event.Metadata) || event.CreatedAt.IsZero() {
		return domain.NewError(domain.CodeInvalidArgument, "invalid runtime event")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO session_events(id, session_id, sequence, kind, metadata_json, created_at)
			VALUES(?, ?, ?, ?, ?, ?)`, event.ID, event.SessionID, event.Sequence, event.Kind, event.Metadata, formatTime(event.CreatedAt))
		return writeError("runtime event could not be appended", err)
	})
}

func (s *Store) AppendAuditEvent(ctx context.Context, event domain.AuditEvent) error {
	for _, id := range []domain.ID{event.ID, event.ActorID, event.TargetID} {
		if err := domain.ValidateID(id); err != nil {
			return err
		}
	}
	if event.Action == "" || event.TargetType == "" ||
		(event.Decision != "allowed" && event.Decision != "denied" && event.Decision != "failed") ||
		!json.Valid(event.Metadata) || event.CreatedAt.IsZero() {
		return domain.NewError(domain.CodeInvalidArgument, "invalid audit event")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO audit_events(
				id, actor_id, action, target_type, target_id, decision, error_code, metadata_json, created_at
			) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			event.ID, event.ActorID, event.Action, event.TargetType, event.TargetID,
			event.Decision, event.ErrorCode, event.Metadata, formatTime(event.CreatedAt),
		)
		return writeError("audit event could not be appended", err)
	})
}

// IdempotencyRecord is the bounded replay metadata for one client mutation.
// ResponseMetadata contains an already-encoded, redacted service result; it
// never contains raw terminal bytes or credential material.
type IdempotencyRecord struct {
	ClientID         domain.ID
	Method           string
	IdempotencyKey   string
	RequestDigest    string
	ResponseCode     domain.ErrorCode
	ResponseMetadata json.RawMessage
	CreatedAt        time.Time
}

func (s *Store) IdempotencyRecord(ctx context.Context, clientID domain.ID, method, key string) (IdempotencyRecord, error) {
	var record IdempotencyRecord
	var metadata, createdAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT client_id, method, idempotency_key, request_digest, response_code,
			response_metadata_json, created_at
		FROM idempotency_records WHERE client_id = ? AND method = ? AND idempotency_key = ?`,
		clientID, method, key).Scan(&record.ClientID, &record.Method, &record.IdempotencyKey,
		&record.RequestDigest, &record.ResponseCode, &metadata, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return IdempotencyRecord{}, domain.NewError(domain.CodeNotFound, "idempotency record not found")
	}
	if err != nil {
		return IdempotencyRecord{}, domain.WrapError(domain.CodeConflict, "idempotency record could not be read", err)
	}
	if !json.Valid([]byte(metadata)) {
		return IdempotencyRecord{}, domain.NewError(domain.CodeSchemaIncompatible, "idempotency metadata is invalid")
	}
	record.ResponseMetadata = json.RawMessage(metadata)
	record.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return IdempotencyRecord{}, err
	}
	return record, nil
}

// SaveIdempotencyRecord inserts one record. The primary key is intentionally
// not replaced: a concurrent or reused key must be compared by the caller.
func (s *Store) SaveIdempotencyRecord(ctx context.Context, record IdempotencyRecord) error {
	if err := domain.ValidateID(record.ClientID); err != nil {
		return err
	}
	if record.Method == "" || record.IdempotencyKey == "" || len(record.Method) > 128 || len(record.IdempotencyKey) > 128 ||
		len(record.RequestDigest) != 64 || record.ResponseCode == "" || !json.Valid(record.ResponseMetadata) || record.CreatedAt.IsZero() {
		return domain.NewError(domain.CodeInvalidArgument, "invalid idempotency record")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO idempotency_records(
				client_id, method, idempotency_key, request_digest, response_code,
				response_metadata_json, created_at
			) VALUES(?, ?, ?, ?, ?, ?, ?)`, record.ClientID, record.Method, record.IdempotencyKey,
			record.RequestDigest, record.ResponseCode, record.ResponseMetadata, formatTime(record.CreatedAt))
		return writeError("idempotency record could not be saved", err)
	})
}

// ListSessions returns bounded metadata in deterministic order for the
// application service and CLI. It deliberately exposes no provider payload.
func (s *Store) ListSessions(ctx context.Context) ([]domain.Session, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM sessions ORDER BY started_at, id`)
	if err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "sessions could not be listed", err)
	}
	ids := make([]domain.ID, 0)
	for rows.Next() {
		var id domain.ID
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, domain.WrapError(domain.CodeConflict, "session list could not be read", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, domain.WrapError(domain.CodeConflict, "sessions could not be listed", err)
	}
	if err := rows.Close(); err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "sessions could not be listed", err)
	}
	sessions := make([]domain.Session, 0, len(ids))
	for _, id := range ids {
		session, err := s.Session(ctx, id)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (s *Store) CreateCredentialMaterialization(ctx context.Context, materialization domain.CredentialMaterialization) error {
	if err := domain.ValidateID(materialization.LeaseID); err != nil {
		return err
	}
	if err := domain.ValidateID(materialization.CredentialInstanceID); err != nil {
		return err
	}
	if materialization.CredentialRevision < 1 || materialization.ManifestVersion != 1 || len(materialization.ManifestDigest) != 64 ||
		materialization.State != domain.MaterializationPending || materialization.RefCount < 0 || materialization.CreatedAt.IsZero() || materialization.UpdatedAt.Before(materialization.CreatedAt) {
		return domain.NewError(domain.CodeInvalidArgument, "invalid credential materialization")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO credential_materializations(
				lease_id, credential_instance_id, credential_revision, manifest_version,
				manifest_digest, state, ref_count, created_at, updated_at
			) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`, materialization.LeaseID, materialization.CredentialInstanceID,
			materialization.CredentialRevision, materialization.ManifestVersion, materialization.ManifestDigest,
			materialization.State, materialization.RefCount, formatTime(materialization.CreatedAt), formatTime(materialization.UpdatedAt))
		return writeError("credential materialization could not be created", err)
	})
}

func (s *Store) CredentialMaterialization(ctx context.Context, leaseID domain.ID) (domain.CredentialMaterialization, error) {
	var materialization domain.CredentialMaterialization
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT lease_id, credential_instance_id, credential_revision, manifest_version,
			manifest_digest, state, ref_count, created_at, updated_at
		FROM credential_materializations WHERE lease_id = ?`, leaseID).Scan(&materialization.LeaseID,
		&materialization.CredentialInstanceID, &materialization.CredentialRevision, &materialization.ManifestVersion,
		&materialization.ManifestDigest, &materialization.State, &materialization.RefCount, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CredentialMaterialization{}, domain.NewError(domain.CodeNotFound, "credential materialization not found")
	}
	if err != nil {
		return domain.CredentialMaterialization{}, domain.WrapError(domain.CodeConflict, "credential materialization could not be read", err)
	}
	materialization.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return domain.CredentialMaterialization{}, err
	}
	materialization.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return domain.CredentialMaterialization{}, err
	}
	return materialization, nil
}

func (s *Store) ListCredentialMaterializations(ctx context.Context) ([]domain.CredentialMaterialization, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT lease_id FROM credential_materializations ORDER BY created_at, lease_id`)
	if err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "credential materializations could not be listed", err)
	}
	var ids []domain.ID
	for rows.Next() {
		var id domain.ID
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, domain.WrapError(domain.CodeConflict, "credential materialization list could not be read", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, domain.WrapError(domain.CodeConflict, "credential materializations could not be listed", err)
	}
	if err := rows.Close(); err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "credential materializations could not be listed", err)
	}
	result := make([]domain.CredentialMaterialization, 0, len(ids))
	for _, id := range ids {
		materialization, err := s.CredentialMaterialization(ctx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, materialization)
	}
	return result, nil
}

func (s *Store) TransitionCredentialMaterialization(ctx context.Context, leaseID domain.ID, expected, next domain.MaterializationState, refCount int64, at time.Time) (domain.CredentialMaterialization, error) {
	if refCount < 0 || at.IsZero() || next == "" {
		return domain.CredentialMaterialization{}, domain.NewError(domain.CodeInvalidArgument, "invalid materialization transition")
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE credential_materializations SET state = ?, ref_count = ?, updated_at = ?
		WHERE lease_id = ? AND state = ?`, next, refCount, formatTime(at), leaseID, expected)
	if err != nil {
		return domain.CredentialMaterialization{}, domain.WrapError(domain.CodeMaterializationConflict, "credential materialization transition failed", err)
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return domain.CredentialMaterialization{}, domain.NewError(domain.CodeMaterializationConflict, "credential materialization state changed")
	}
	return s.CredentialMaterialization(ctx, leaseID)
}

func validCreatedUpdated(createdAt, updatedAt time.Time) bool {
	return !createdAt.IsZero() && !updatedAt.IsZero() && !updatedAt.Before(createdAt)
}

func validCredentialStatus(status domain.CredentialStatus) bool {
	switch status {
	case domain.CredentialHealthy, domain.CredentialExpired, domain.CredentialRevoked, domain.CredentialUnknown:
		return true
	default:
		return false
	}
}

func validateResumeSource(ctx context.Context, tx *sql.Tx, resumed domain.Session) error {
	var source domain.Session
	var endedAt sql.NullString
	var sourceCapabilitiesJSON []byte
	err := tx.QueryRowContext(ctx, `
		SELECT device_id, provider, credential_instance_id, runtime_profile_id,
			workspace_id, coalesce(provider_session_id, ''), status, ended_at,
			capability_snapshot_json
		FROM sessions WHERE id = ?`, resumed.ResumedFromSessionID).Scan(
		&source.DeviceID, &source.Provider, &source.CredentialInstanceID,
		&source.RuntimeProfileID, &source.WorkspaceID, &source.ProviderSessionID,
		&source.Status, &endedAt, &sourceCapabilitiesJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NewError(domain.CodeNotFound, "resume source session not found")
	}
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "resume source could not be read", err)
	}
	if !source.Status.Terminal() || !endedAt.Valid {
		return domain.NewError(domain.CodeInvalidTransition, "resume source session is not terminal")
	}
	sourceEndedAt, err := parseTime(endedAt.String)
	if err != nil {
		return err
	}
	if resumed.StartedAt.Before(sourceEndedAt) {
		return domain.NewError(domain.CodeInvalidArgument, "resumed session cannot precede source end")
	}
	if err := json.Unmarshal(sourceCapabilitiesJSON, &source.CapabilitySnapshot); err != nil {
		return domain.WrapError(domain.CodeSchemaIncompatible, "resume source capabilities are invalid", err)
	}
	canonicalSource, err := domain.CanonicalCapabilities(source.CapabilitySnapshot)
	if err != nil {
		return domain.WrapError(domain.CodeSchemaIncompatible, "resume source capabilities are invalid", err)
	}
	if !slices.Equal(canonicalSource, source.CapabilitySnapshot) {
		return domain.NewError(domain.CodeSchemaIncompatible, "resume source capabilities are not canonical")
	}
	if !domain.HasCapability(canonicalSource, domain.CapabilitySessionResume) {
		return domain.NewError(domain.CodePermissionDenied, "resume source capability is required")
	}
	if !slices.Equal(canonicalSource, resumed.CapabilitySnapshot) {
		return domain.NewError(domain.CodeConflict, "resumed session changed capability snapshot")
	}
	if source.DeviceID != resumed.DeviceID || source.Provider != resumed.Provider ||
		source.CredentialInstanceID != resumed.CredentialInstanceID || source.RuntimeProfileID != resumed.RuntimeProfileID ||
		source.WorkspaceID != resumed.WorkspaceID || source.ProviderSessionID != resumed.ProviderSessionID {
		return domain.NewError(domain.CodeConflict, "resumed session changed frozen source fields")
	}
	return nil
}
