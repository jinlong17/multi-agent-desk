package storage

import (
	"context"
	"crypto/sha256"
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

func (s *Store) CreateAccount(ctx context.Context, account domain.Account) error {
	if !account.Enabled {
		// A newly created Account is enabled by default; disabling is an
		// explicit subsequent operation through SetAccountEnabled.
		account.Enabled = true
	}
	if err := domain.ValidateAccount(account); err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO accounts(id, provider, display_name, provider_subject_digest, enabled, created_at, updated_at)
			VALUES(?, ?, ?, NULLIF(?, ''), ?, ?, ?)`,
			account.ID, account.Provider, account.DisplayName, account.ProviderSubjectDigest, boolInt(account.Enabled),
			formatTime(account.CreatedAt), formatTime(account.UpdatedAt))
		return writeError("account could not be created", err)
	})
}

func (s *Store) Account(ctx context.Context, id domain.ID) (domain.Account, error) {
	var account domain.Account
	var digest sql.NullString
	var enabled int
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, provider, display_name, provider_subject_digest, enabled, created_at, updated_at
		FROM accounts WHERE id = ?`, id).Scan(&account.ID, &account.Provider, &account.DisplayName, &digest, &enabled, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Account{}, domain.NewError(domain.CodeNotFound, "account not found")
	}
	if err != nil {
		return domain.Account{}, domain.WrapError(domain.CodeConflict, "account could not be read", err)
	}
	if digest.Valid {
		account.ProviderSubjectDigest = digest.String
	}
	account.Enabled = enabled == 1
	account.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return domain.Account{}, err
	}
	account.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return domain.Account{}, err
	}
	return account, nil
}

func (s *Store) ListAccounts(ctx context.Context) ([]domain.Account, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM accounts ORDER BY display_name, id`)
	if err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "accounts could not be listed", err)
	}
	defer rows.Close()
	var ids []domain.ID
	for rows.Next() {
		var id domain.ID
		if err := rows.Scan(&id); err != nil {
			return nil, domain.WrapError(domain.CodeConflict, "account list could not be read", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "accounts could not be listed", err)
	}
	result := make([]domain.Account, 0, len(ids))
	for _, id := range ids {
		account, err := s.Account(ctx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, account)
	}
	return result, nil
}

func (s *Store) SetAccountEnabled(ctx context.Context, id domain.ID, enabled bool, at time.Time) (domain.Account, error) {
	if err := domain.ValidateID(id); err != nil {
		return domain.Account{}, err
	}
	if at.IsZero() {
		return domain.Account{}, domain.NewError(domain.CodeInvalidArgument, "account update requires a timestamp")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE accounts SET enabled = ?, updated_at = ? WHERE id = ?`, boolInt(enabled), formatTime(at), id)
	if err != nil {
		return domain.Account{}, writeError("account status could not be updated", err)
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return domain.Account{}, domain.NewError(domain.CodeNotFound, "account not found")
	}
	return s.Account(ctx, id)
}

func (s *Store) CreateRuntimeProfile(ctx context.Context, profile domain.RuntimeProfile) error {
	if err := domain.ValidateID(profile.ID); err != nil {
		return err
	}
	if err := domain.ValidateID(profile.DeviceID); err != nil {
		return err
	}
	if profile.AccountID != "" {
		if err := domain.ValidateID(profile.AccountID); err != nil {
			return err
		}
	}
	if profile.Name == "" || !domain.ProviderKnown(profile.Provider) ||
		(profile.Provider == domain.ProviderCodex && profile.AccountID == "") || !json.Valid(profile.Settings) ||
		!validCreatedUpdated(profile.CreatedAt, profile.UpdatedAt) {
		return domain.NewError(domain.CodeInvalidArgument, "invalid runtime profile")
	}
	if profile.AccountID == "" {
		profile.AccountID = fakeAccountID(profile.DeviceID)
	}
	if domain.ValidateID(profile.AccountID) != nil {
		return domain.NewError(domain.CodeInvalidArgument, "invalid runtime profile account")
	}
	profile.CredentialInstanceID = ""
	profile.SelectorAlias, profile.SelectorKey = "", ""
	profile.Internal, profile.Enabled = true, true
	if profile.Revision == 0 {
		profile.Revision = 1
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if err := validateAccountForProvider(ctx, tx, profile.Provider, profile.AccountID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO runtime_profiles(id, device_id, account_id, name, provider, settings_json, created_at, updated_at)
			VALUES(?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?)`,
			profile.ID, profile.DeviceID, profile.AccountID, profile.Name, profile.Provider, profile.Settings,
			formatTime(profile.CreatedAt), formatTime(profile.UpdatedAt),
		)
		return writeError("runtime profile could not be created", err)
	})
}

func (s *Store) RuntimeProfile(ctx context.Context, id domain.ID) (domain.RuntimeProfile, error) {
	var profile domain.RuntimeProfile
	var settings []byte
	var accountID sql.NullString
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, device_id, account_id, name, provider, settings_json, created_at, updated_at
		FROM runtime_profiles WHERE id = ?`, id).Scan(
		&profile.ID, &profile.DeviceID, &accountID, &profile.Name, &profile.Provider, &settings, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RuntimeProfile{}, domain.NewError(domain.CodeNotFound, "runtime profile not found")
	}
	if err != nil {
		return domain.RuntimeProfile{}, domain.WrapError(domain.CodeConflict, "runtime profile could not be read", err)
	}
	if accountID.Valid {
		profile.AccountID = domain.ID(accountID.String)
	}
	if !json.Valid(settings) {
		return domain.RuntimeProfile{}, domain.NewError(domain.CodeSchemaIncompatible, "stored runtime profile settings are invalid")
	}
	profile.Settings = append([]byte(nil), settings...)
	if credentialID.Valid {
		profile.CredentialInstanceID = domain.ID(credentialID.String)
	}
	if alias.Valid {
		profile.SelectorAlias = alias.String
	}
	if aliasKey.Valid {
		profile.SelectorKey = aliasKey.String
	}
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

func (s *Store) ListRuntimeProfiles(ctx context.Context) ([]domain.RuntimeProfile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM runtime_profiles ORDER BY name, id`)
	if err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "runtime profiles could not be listed", err)
	}
	defer rows.Close()
	var ids []domain.ID
	for rows.Next() {
		var id domain.ID
		if err := rows.Scan(&id); err != nil {
			return nil, domain.WrapError(domain.CodeConflict, "runtime profile list could not be read", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "runtime profiles could not be listed", err)
	}
	profiles := make([]domain.RuntimeProfile, 0, len(ids))
	for _, id := range ids {
		profile, err := s.RuntimeProfile(ctx, id)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func (s *Store) UpdateRuntimeProfile(ctx context.Context, profile domain.RuntimeProfile) (domain.RuntimeProfile, error) {
	if err := domain.ValidateID(profile.ID); err != nil {
		return domain.RuntimeProfile{}, err
	}
	if err := domain.ValidateID(profile.DeviceID); err != nil {
		return domain.RuntimeProfile{}, err
	}
	if profile.AccountID != "" {
		if err := domain.ValidateID(profile.AccountID); err != nil {
			return domain.RuntimeProfile{}, err
		}
	}
	if profile.Name == "" || !domain.ProviderKnown(profile.Provider) ||
		(profile.Provider == domain.ProviderCodex && profile.AccountID == "") || !json.Valid(profile.Settings) ||
		!validCreatedUpdated(profile.CreatedAt, profile.UpdatedAt) {
		return domain.RuntimeProfile{}, domain.NewError(domain.CodeInvalidArgument, "invalid runtime profile")
	}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if err := validateAccountForProvider(ctx, tx, profile.Provider, profile.AccountID); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE runtime_profiles SET device_id = ?, account_id = NULLIF(?, ''), name = ?, provider = ?,
				settings_json = ?, updated_at = ? WHERE id = ?`,
			profile.DeviceID, profile.AccountID, profile.Name, profile.Provider, profile.Settings,
			formatTime(profile.UpdatedAt), profile.ID)
		if err != nil {
			return writeError("runtime profile could not be updated", err)
		}
		changed, err := result.RowsAffected()
		if err != nil || changed != 1 {
			return domain.NewError(domain.CodeNotFound, "runtime profile not found")
		}
		return nil
	})
	if err != nil {
		return domain.RuntimeProfile{}, err
	}
	return s.RuntimeProfile(ctx, profile.ID)
}

func (s *Store) DeleteRuntimeProfile(ctx context.Context, id domain.ID) error {
	if err := domain.ValidateID(id); err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, "DELETE FROM runtime_profiles WHERE id = ?", id)
		if err != nil {
			return writeError("runtime profile could not be deleted", err)
		}
		changed, err := result.RowsAffected()
		if err != nil || changed != 1 {
			return domain.NewError(domain.CodeNotFound, "runtime profile not found")
		}
		return nil
	})
}

func (s *Store) CreateCredentialInstance(ctx context.Context, credential domain.CredentialInstance) error {
	if err := domain.ValidateID(credential.ID); err != nil {
		return err
	}
	if err := domain.ValidateID(credential.DeviceID); err != nil {
		return err
	}
	digest, digestErr := hex.DecodeString(credential.SecretDigest)
	validFake := credential.Provider == domain.ProviderFake && credential.AuthMethod == domain.AuthMethodFake &&
		credential.AccountID == "" && strings.HasPrefix(credential.SecretRef, "fake:") && credential.CredentialRevision >= 0
	validCodex := credential.Provider == domain.ProviderCodex &&
		(credential.AuthMethod == domain.AuthMethodInteractive || credential.AuthMethod == domain.AuthMethodDeviceCode) &&
		credential.AccountID != "" && strings.HasPrefix(credential.SecretRef, "vault:") && credential.CredentialRevision >= 1
	if (!validFake && !validCodex) || digestErr != nil || len(digest) != 32 ||
		!validCredentialStatus(credential.Status) || !validCreatedUpdated(credential.CreatedAt, credential.UpdatedAt) {
		return domain.NewError(domain.CodeInvalidArgument, "invalid credential instance")
	}
	if credential.AccountID == "" {
		credential.AccountID = fakeAccountID(credential.DeviceID)
	}
	if domain.ValidateID(credential.AccountID) != nil {
		return domain.NewError(domain.CodeInvalidArgument, "invalid credential account")
	}
	if credential.Availability == "" {
		credential.Availability = domain.AvailabilityUnknown
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if err := validateAccountForProvider(ctx, tx, credential.Provider, credential.AccountID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO credential_instances(
				id, device_id, account_id, provider, auth_method, secret_ref, status,
				credential_revision, secret_digest, created_at, updated_at
			) VALUES(?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?)`,
			credential.ID, credential.DeviceID, credential.AccountID, credential.Provider, credential.AuthMethod,
			credential.SecretRef, credential.Status, credential.CredentialRevision,
			credential.SecretDigest, formatTime(credential.CreatedAt), formatTime(credential.UpdatedAt),
		)
		return writeError("credential instance could not be created", err)
	})
}

func (s *Store) CredentialInstance(ctx context.Context, id domain.ID) (domain.CredentialInstance, error) {
	var credential domain.CredentialInstance
	var accountID sql.NullString
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, device_id, account_id, provider, auth_method, secret_ref, status,
			credential_revision, secret_digest, created_at, updated_at
		FROM credential_instances WHERE id = ?`, id).Scan(
		&credential.ID, &credential.DeviceID, &accountID, &credential.Provider, &credential.AuthMethod,
		&credential.SecretRef, &credential.Status, &credential.CredentialRevision,
		&credential.SecretDigest, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CredentialInstance{}, domain.NewError(domain.CodeNotFound, "credential instance not found")
	}
	if err != nil {
		return domain.CredentialInstance{}, domain.WrapError(domain.CodeConflict, "credential instance could not be read", err)
	}
	if accountID.Valid {
		credential.AccountID = domain.ID(accountID.String)
	}
	credential.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return domain.CredentialInstance{}, err
	}
	credential.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return domain.CredentialInstance{}, err
	}
	credential.LastValidatedAt, err = parseOptionalTime(validatedAt)
	if err != nil {
		return domain.CredentialInstance{}, err
	}
	return credential, nil
}

func (s *Store) ListCredentialInstances(ctx context.Context) ([]domain.CredentialInstance, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM credential_instances ORDER BY id`)
	if err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "credential instances could not be listed", err)
	}
	defer rows.Close()
	var ids []domain.ID
	for rows.Next() {
		var id domain.ID
		if err := rows.Scan(&id); err != nil {
			return nil, domain.WrapError(domain.CodeConflict, "credential instance list could not be read", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "credential instances could not be listed", err)
	}
	result := make([]domain.CredentialInstance, 0, len(ids))
	for _, id := range ids {
		credential, err := s.CredentialInstance(ctx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, credential)
	}
	return result, nil
}

func (s *Store) UpdateCredentialStatus(ctx context.Context, id domain.ID, status domain.CredentialStatus, at time.Time) (domain.CredentialInstance, error) {
	if err := domain.ValidateID(id); err != nil {
		return domain.CredentialInstance{}, err
	}
	if !validCredentialStatus(status) || at.IsZero() {
		return domain.CredentialInstance{}, domain.NewError(domain.CodeInvalidArgument, "invalid credential status")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE credential_instances SET status = ?, updated_at = ? WHERE id = ?`, status, formatTime(at), id)
	if err != nil {
		return domain.CredentialInstance{}, writeError("credential status could not be updated", err)
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return domain.CredentialInstance{}, domain.NewError(domain.CodeNotFound, "credential instance not found")
	}
	return s.CredentialInstance(ctx, id)
}

// UpdateCredentialRevisionCAS commits a provider-auth digest only when the
// caller still owns the expected credential revision. Raw credential bytes
// never enter the Store; only the bounded integrity digest is persisted.
func (s *Store) UpdateCredentialRevisionCAS(ctx context.Context, id domain.ID, expectedRevision int64, secretDigest string, at time.Time) (domain.CredentialInstance, error) {
	if err := domain.ValidateID(id); err != nil {
		return domain.CredentialInstance{}, err
	}
	if expectedRevision < 1 || len(secretDigest) != 64 || at.IsZero() {
		return domain.CredentialInstance{}, domain.NewError(domain.CodeInvalidArgument, "invalid credential revision update")
	}
	if _, err := hex.DecodeString(secretDigest); err != nil {
		return domain.CredentialInstance{}, domain.NewError(domain.CodeInvalidArgument, "credential digest is invalid")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE credential_instances SET credential_revision = ?, secret_digest = ?, updated_at = ? WHERE id = ? AND credential_revision = ? AND status = ?`, expectedRevision+1, secretDigest, formatTime(at), id, expectedRevision, domain.CredentialHealthy)
	if err != nil {
		return domain.CredentialInstance{}, writeError("credential revision could not be committed", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return domain.CredentialInstance{}, writeError("credential revision result could not be read", err)
	}
	if changed != 1 {
		return domain.CredentialInstance{}, domain.NewError(domain.CodeCredentialRevisionConflict, "credential revision is stale or unavailable")
	}
	return s.CredentialInstance(ctx, id)
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
		if err := validateSessionLinks(ctx, tx, validated); err != nil {
			return err
		}
		if validated.ResumedFromSessionID != "" {
			if err := validateResumeSource(ctx, tx, validated); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO sessions(
				id, device_id, account_id, provider, credential_instance_id, runtime_profile_id,
				workspace_id, provider_session_id, resumed_from_session_id, status,
				started_at, ended_at, exit_code, capability_snapshot_json, failure_code
			) VALUES(?, ?, NULLIF(?, ''), ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, NULL, NULL, ?, '')`,
			validated.ID, validated.DeviceID, validated.AccountID, validated.Provider, validated.CredentialInstanceID,
			validated.RuntimeProfileID, validated.WorkspaceID, validated.ProviderSessionID,
			validated.ResumedFromSessionID, validated.Status, formatTime(validated.StartedAt), capabilities,
		)
		return writeError("session could not be created", err)
	})
}

func (s *Store) Session(ctx context.Context, id domain.ID) (domain.Session, error) {
	var session domain.Session
	var accountID sql.NullString
	var startedAt string
	var endedAt sql.NullString
	var exitCode sql.NullInt64
	var capabilities []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT id, device_id, account_id, provider, credential_instance_id, runtime_profile_id,
			workspace_id, coalesce(provider_session_id, ''), coalesce(resumed_from_session_id, ''),
			status, started_at, ended_at, exit_code, capability_snapshot_json, failure_code
		FROM sessions WHERE id = ?`, id).Scan(
		&session.ID, &session.DeviceID, &accountID, &session.Provider, &session.CredentialInstanceID,
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
	if accountID.Valid {
		session.AccountID = domain.ID(accountID.String)
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

// SetSessionProviderSessionID records the bounded Provider thread identity
// while the local Session is still starting. It cannot be rewritten after the
// Session advances or by a second thread/start result.
func (s *Store) SetSessionProviderSessionID(ctx context.Context, id domain.ID, expected domain.SessionStatus, providerSessionID string) (domain.Session, error) {
	if err := domain.ValidateID(id); err != nil {
		return domain.Session{}, err
	}
	if expected != domain.SessionStarting || providerSessionID == "" || len(providerSessionID) > 256 {
		return domain.Session{}, domain.NewError(domain.CodeInvalidArgument, "provider session identity is invalid")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE sessions SET provider_session_id=? WHERE id=? AND provider='codex' AND status=? AND coalesce(provider_session_id,'')=''`, providerSessionID, id, expected)
	if err != nil {
		return domain.Session{}, writeError("provider session identity could not be persisted", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return domain.Session{}, writeError("provider session identity result could not be read", err)
	}
	if changed != 1 {
		return domain.Session{}, domain.NewError(domain.CodeConflict, "provider session identity changed")
	}
	return s.Session(ctx, id)
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

func (s *Store) CreateApproval(ctx context.Context, approval domain.Approval) error {
	if approval.ResponseState == "" {
		approval.ResponseState = domain.ApprovalResponseIdle
	}
	if err := domain.ValidateApproval(approval); err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var sessionProvider string
		if err := tx.QueryRowContext(ctx, "SELECT provider FROM sessions WHERE id = ?", approval.SessionID).Scan(&sessionProvider); errors.Is(err, sql.ErrNoRows) {
			return domain.NewError(domain.CodeNotFound, "session not found")
		} else if err != nil {
			return domain.WrapError(domain.CodeConflict, "approval session could not be read", err)
		} else if sessionProvider != domain.ProviderCodex {
			return domain.NewError(domain.CodeConflict, "approval session provider is unsupported")
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO approvals(
				id, session_id, provider_approval_id, kind, payload_digest, summary, status, response_state,
				requested_decision, responded_by_device_id, idempotency_key, dispatch_digest,
				requested_at, dispatch_started_at, responded_at, dispatch_error_code
			) VALUES(?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, NULLIF(?, ''), ?, ?, ?, ?)`,
			approval.ID, approval.SessionID, approval.ProviderApprovalID, approval.Kind, approval.PayloadDigest,
			approval.Summary, approval.Status, approval.ResponseState, approval.RequestedDecision,
			approval.RespondedByDeviceID, approval.IdempotencyKey, approval.DispatchDigest,
			formatTime(approval.RequestedAt), optionalTimeValue(approval.DispatchStartedAt),
			optionalTimeValue(approval.RespondedAt), approval.DispatchErrorCode)
		return writeError("approval could not be created", err)
	})
}

func (s *Store) Approval(ctx context.Context, id domain.ID) (domain.Approval, error) {
	var approval domain.Approval
	var respondedBy sql.NullString
	var requestedAt string
	var dispatchStartedAt, respondedAt sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, session_id, provider_approval_id, kind, payload_digest, summary, status,
			response_state, coalesce(requested_decision, ''), coalesce(responded_by_device_id, ''),
			idempotency_key, coalesce(dispatch_digest, ''), requested_at, dispatch_started_at,
			responded_at, dispatch_error_code
		FROM approvals WHERE id = ?`, id).Scan(
		&approval.ID, &approval.SessionID, &approval.ProviderApprovalID, &approval.Kind, &approval.PayloadDigest,
		&approval.Summary, &approval.Status, &approval.ResponseState, &approval.RequestedDecision,
		&respondedBy, &approval.IdempotencyKey, &approval.DispatchDigest, &requestedAt,
		&dispatchStartedAt, &respondedAt, &approval.DispatchErrorCode)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Approval{}, domain.NewError(domain.CodeNotFound, "approval not found")
	}
	if err != nil {
		return domain.Approval{}, domain.WrapError(domain.CodeConflict, "approval could not be read", err)
	}
	if respondedBy.Valid {
		approval.RespondedByDeviceID = domain.ID(respondedBy.String)
	}
	approval.RequestedAt, err = parseTime(requestedAt)
	if err != nil {
		return domain.Approval{}, err
	}
	approval.RespondedAt, err = parseOptionalTime(respondedAt)
	if err != nil {
		return domain.Approval{}, err
	}
	approval.DispatchStartedAt, err = parseOptionalTime(dispatchStartedAt)
	if err != nil {
		return domain.Approval{}, err
	}
	return approval, nil
}

func (s *Store) ListApprovals(ctx context.Context, sessionID domain.ID) ([]domain.Approval, error) {
	if err := domain.ValidateID(sessionID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM approvals WHERE session_id = ? ORDER BY requested_at, id`, sessionID)
	if err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "approvals could not be listed", err)
	}
	defer rows.Close()
	var ids []domain.ID
	for rows.Next() {
		var id domain.ID
		if err := rows.Scan(&id); err != nil {
			return nil, domain.WrapError(domain.CodeConflict, "approval list could not be read", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "approvals could not be listed", err)
	}
	result := make([]domain.Approval, 0, len(ids))
	for _, id := range ids {
		approval, err := s.Approval(ctx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, approval)
	}
	return result, nil
}

func approvalDecision(status domain.ApprovalStatus) domain.ApprovalDecision {
	switch status {
	case domain.ApprovalApproved:
		return domain.ApprovalDecisionApprove
	case domain.ApprovalDenied:
		return domain.ApprovalDecisionDeny
	case domain.ApprovalCancelled:
		return domain.ApprovalDecisionCancel
	default:
		return ""
	}
}

func approvalStatusForDecision(decision domain.ApprovalDecision) domain.ApprovalStatus {
	switch decision {
	case domain.ApprovalDecisionApprove:
		return domain.ApprovalApproved
	case domain.ApprovalDecisionDeny:
		return domain.ApprovalDenied
	case domain.ApprovalDecisionCancel:
		return domain.ApprovalCancelled
	default:
		return ""
	}
}

// ClaimApprovalDispatch durably records the exact decision and dispatch digest
// before the caller writes to the Provider transport. Replays with the same
// digest return the stored state; another decision or key conflicts.
func (s *Store) ClaimApprovalDispatch(ctx context.Context, approvalID domain.ID, providerApprovalID string, responderID domain.ID, responseKey string, decision domain.ApprovalDecision, dispatchDigest string, at time.Time) (domain.Approval, error) {
	for _, id := range []domain.ID{approvalID, responderID} {
		if err := domain.ValidateID(id); err != nil {
			return domain.Approval{}, err
		}
	}
	if providerApprovalID == "" || responseKey == "" || len(responseKey) > 128 ||
		approvalStatusForDecision(decision) == "" || len(dispatchDigest) != 64 || at.IsZero() {
		return domain.Approval{}, domain.NewError(domain.CodeInvalidArgument, "invalid approval dispatch claim")
	}
	if _, err := hex.DecodeString(dispatchDigest); err != nil {
		return domain.Approval{}, domain.NewError(domain.CodeInvalidArgument, "approval dispatch digest is invalid")
	}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var storedProviderID, storedKey, storedDigest string
		var status domain.ApprovalStatus
		var responseState domain.ApprovalResponseState
		var storedDecision domain.ApprovalDecision
		var storedResponder domain.ID
		err := tx.QueryRowContext(ctx, `SELECT provider_approval_id, status, response_state, coalesce(requested_decision,''), coalesce(responded_by_device_id,''), idempotency_key, coalesce(dispatch_digest,'') FROM approvals WHERE id=?`, approvalID).
			Scan(&storedProviderID, &status, &responseState, &storedDecision, &storedResponder, &storedKey, &storedDigest)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.NewError(domain.CodeApprovalUnknown, "approval request is unknown")
		}
		if err != nil {
			return domain.WrapError(domain.CodeConflict, "approval could not be checked", err)
		}
		if storedProviderID != providerApprovalID {
			return domain.NewError(domain.CodeApprovalUnknown, "approval provider request is unknown")
		}
		if responseState != domain.ApprovalResponseIdle {
			if storedDecision == decision && storedResponder == responderID && storedKey == responseKey && storedDigest == dispatchDigest {
				return nil
			}
			return domain.NewError(domain.CodeConflict, "approval dispatch already has another decision")
		}
		if status != domain.ApprovalPending {
			return domain.NewError(domain.CodeConflict, "approval is already terminal")
		}
		result, err := tx.ExecContext(ctx, `UPDATE approvals SET response_state='dispatching', requested_decision=?, responded_by_device_id=?, idempotency_key=?, dispatch_digest=?, dispatch_started_at=? WHERE id=? AND status='pending' AND response_state='idle'`, decision, responderID, responseKey, dispatchDigest, formatTime(at), approvalID)
		if err != nil {
			return writeError("approval dispatch could not be claimed", err)
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return domain.NewError(domain.CodeConflict, "approval changed before dispatch")
		}
		return nil
	})
	if err != nil {
		return domain.Approval{}, err
	}
	return s.Approval(ctx, approvalID)
}

// CompleteApprovalDispatch records a Provider write only after the caller has
// received success for the previously claimed digest.
func (s *Store) CompleteApprovalDispatch(ctx context.Context, approvalID domain.ID, dispatchDigest string, at time.Time) (domain.Approval, error) {
	if err := domain.ValidateID(approvalID); err != nil {
		return domain.Approval{}, err
	}
	if len(dispatchDigest) != 64 || at.IsZero() {
		return domain.Approval{}, domain.NewError(domain.CodeInvalidArgument, "invalid approval dispatch completion")
	}
	approval, err := s.Approval(ctx, approvalID)
	if err != nil {
		return domain.Approval{}, err
	}
	if approval.DispatchDigest != dispatchDigest {
		return domain.Approval{}, domain.NewError(domain.CodeConflict, "approval dispatch digest changed")
	}
	if approval.ResponseState == domain.ApprovalResponseWritten {
		return approval, nil
	}
	if approval.ResponseState != domain.ApprovalResponseDispatching {
		return domain.Approval{}, domain.NewError(domain.CodeConflict, "approval dispatch is not in flight")
	}
	status := approvalStatusForDecision(approval.RequestedDecision)
	result, err := s.db.ExecContext(ctx, `UPDATE approvals SET status=?, response_state='written', responded_at=?, dispatch_error_code='' WHERE id=? AND status='pending' AND response_state='dispatching' AND dispatch_digest=?`, status, formatTime(at), approvalID, dispatchDigest)
	if err != nil {
		return domain.Approval{}, writeError("approval dispatch could not be completed", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return domain.Approval{}, domain.NewError(domain.CodeConflict, "approval changed before dispatch completion")
	}
	return s.Approval(ctx, approvalID)
}

// FailApprovalDispatch makes an in-flight Provider write durably ambiguous;
// it is never retried automatically because the Provider may have applied it.
func (s *Store) FailApprovalDispatch(ctx context.Context, approvalID domain.ID, dispatchDigest, errorCode string, at time.Time) (domain.Approval, error) {
	if err := domain.ValidateID(approvalID); err != nil {
		return domain.Approval{}, err
	}
	if len(dispatchDigest) != 64 || errorCode == "" || len(errorCode) > 64 || at.IsZero() {
		return domain.Approval{}, domain.NewError(domain.CodeInvalidArgument, "invalid approval dispatch failure")
	}
	approval, err := s.Approval(ctx, approvalID)
	if err != nil {
		return domain.Approval{}, err
	}
	if approval.DispatchDigest != dispatchDigest {
		return domain.Approval{}, domain.NewError(domain.CodeConflict, "approval dispatch digest changed")
	}
	if approval.ResponseState == domain.ApprovalResponseAmbiguous && approval.DispatchErrorCode == errorCode {
		return approval, nil
	}
	if approval.ResponseState != domain.ApprovalResponseDispatching {
		return domain.Approval{}, domain.NewError(domain.CodeConflict, "approval dispatch is not in flight")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE approvals SET status='expired', response_state='ambiguous', responded_at=?, dispatch_error_code=? WHERE id=? AND status='pending' AND response_state='dispatching' AND dispatch_digest=?`, formatTime(at), errorCode, approvalID, dispatchDigest)
	if err != nil {
		return domain.Approval{}, writeError("approval dispatch failure could not be saved", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return domain.Approval{}, domain.NewError(domain.CodeConflict, "approval changed before dispatch failure")
	}
	return s.Approval(ctx, approvalID)
}

// RespondApproval records a response only after its caller has successfully
// written the exact Provider result. P3A owns that runtime write.
func (s *Store) RespondApproval(ctx context.Context, approvalID domain.ID, providerApprovalID string, responderID domain.ID, responseKey string, decision domain.ApprovalStatus, at time.Time) (domain.Approval, error) {
	requestedDecision := approvalDecision(decision)
	if requestedDecision == "" {
		return domain.Approval{}, domain.NewError(domain.CodeInvalidArgument, "invalid approval response")
	}
	digestBytes := sha256.Sum256([]byte(string(approvalID) + "\x00" + providerApprovalID + "\x00" + string(responderID) + "\x00" + responseKey + "\x00" + string(requestedDecision)))
	dispatchDigest := hex.EncodeToString(digestBytes[:])
	if _, err := s.ClaimApprovalDispatch(ctx, approvalID, providerApprovalID, responderID, responseKey, requestedDecision, dispatchDigest, at); err != nil {
		return domain.Approval{}, err
	}
	return s.CompleteApprovalDispatch(ctx, approvalID, dispatchDigest, at)
}

// ExpirePendingApprovals is called during daemon recovery. It never writes a
// Provider mutation; the local state only records that a pending request can
// no longer safely be replayed after restart.
func (s *Store) ExpirePendingApprovals(ctx context.Context, at time.Time) error {
	if at.IsZero() {
		return domain.NewError(domain.CodeInvalidArgument, "approval recovery requires a timestamp")
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE approvals SET status='expired', response_state='ambiguous',
			requested_decision=coalesce(requested_decision, 'cancel'),
			responded_by_device_id=coalesce(responded_by_device_id, (SELECT id FROM client_identities ORDER BY id LIMIT 1)),
			dispatch_digest=coalesce(dispatch_digest, lower(hex(randomblob(32)))),
			dispatch_started_at=coalesce(dispatch_started_at, requested_at), responded_at=?,
			dispatch_error_code=CASE WHEN response_state='idle' THEN 'daemon_restart_before_dispatch' ELSE 'daemon_restart' END
		WHERE status='pending' AND response_state IN ('idle','dispatching')`, formatTime(at))
	if err != nil {
		return writeError("pending approvals could not be expired", err)
	}
	return nil
}

func (s *Store) CreateUsageSnapshot(ctx context.Context, snapshot domain.UsageSnapshot) error {
	if err := domain.ValidateUsageSnapshot(snapshot); err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if err := validateAccountForProvider(ctx, tx, snapshot.Provider, snapshot.AccountID); err != nil {
			return err
		}
		var deviceExists int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM device_identity WHERE id = ?", snapshot.DeviceID).Scan(&deviceExists); err != nil {
			return writeError("usage device could not be checked", err)
		}
		if deviceExists != 1 {
			return domain.NewError(domain.CodeNotFound, "device not found")
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO usage_snapshots(
				id, provider, account_id, device_id, source, confidence, window_kind,
				used_value, limit_value, used_percent, resets_at, observed_at,
				raw_reference_hash, source_version, capability_status, error_code
			) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?)`,
			snapshot.ID, snapshot.Provider, snapshot.AccountID, snapshot.DeviceID, snapshot.Source, snapshot.Confidence,
			snapshot.WindowKind, optionalFloat(snapshot.UsedValue), optionalFloat(snapshot.LimitValue), optionalFloat(snapshot.UsedPercent),
			optionalTimeValue(snapshot.ResetsAt), formatTime(snapshot.ObservedAt), snapshot.RawReferenceHash, snapshot.SourceVersion,
			snapshot.CapabilityStatus, snapshot.ErrorCode)
		return writeError("usage snapshot could not be created", err)
	})
}

func (s *Store) UsageSnapshot(ctx context.Context, id domain.ID) (domain.UsageSnapshot, error) {
	var snapshot domain.UsageSnapshot
	var used, limit, percent sql.NullFloat64
	var resetsAt sql.NullString
	var rawHash sql.NullString
	var observedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, provider, account_id, device_id, source, confidence, window_kind,
			used_value, limit_value, used_percent, resets_at, observed_at,
			coalesce(raw_reference_hash, ''), source_version, capability_status, error_code
		FROM usage_snapshots WHERE id = ?`, id).Scan(
		&snapshot.ID, &snapshot.Provider, &snapshot.AccountID, &snapshot.DeviceID, &snapshot.Source, &snapshot.Confidence,
		&snapshot.WindowKind, &used, &limit, &percent, &resetsAt, &observedAt, &rawHash, &snapshot.SourceVersion,
		&snapshot.CapabilityStatus, &snapshot.ErrorCode)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.UsageSnapshot{}, domain.NewError(domain.CodeNotFound, "usage snapshot not found")
	}
	if err != nil {
		return domain.UsageSnapshot{}, domain.WrapError(domain.CodeConflict, "usage snapshot could not be read", err)
	}
	if used.Valid {
		snapshot.UsedValue = &used.Float64
	}
	if limit.Valid {
		snapshot.LimitValue = &limit.Float64
	}
	if percent.Valid {
		snapshot.UsedPercent = &percent.Float64
	}
	snapshot.ResetsAt, err = parseOptionalTime(resetsAt)
	if err != nil {
		return domain.UsageSnapshot{}, err
	}
	snapshot.ObservedAt, err = parseTime(observedAt)
	if err != nil {
		return domain.UsageSnapshot{}, err
	}
	snapshot.RawReferenceHash = rawHash.String
	return snapshot, nil
}

func (s *Store) ListUsageSnapshots(ctx context.Context, accountID domain.ID) ([]domain.UsageSnapshot, error) {
	if err := domain.ValidateID(accountID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM usage_snapshots WHERE account_id = ? ORDER BY observed_at DESC, id`, accountID)
	if err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "usage snapshots could not be listed", err)
	}
	defer rows.Close()
	var ids []domain.ID
	for rows.Next() {
		var id domain.ID
		if err := rows.Scan(&id); err != nil {
			return nil, domain.WrapError(domain.CodeConflict, "usage snapshot list could not be read", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "usage snapshots could not be listed", err)
	}
	result := make([]domain.UsageSnapshot, 0, len(ids))
	for _, id := range ids {
		snapshot, err := s.UsageSnapshot(ctx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, snapshot)
	}
	return result, nil
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

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func validateAccountForProvider(ctx context.Context, tx *sql.Tx, provider string, accountID domain.ID) error {
	if provider == domain.ProviderCodex && accountID == "" {
		return domain.NewError(domain.CodeInvalidArgument, "codex records require an account")
	}
	if accountID == "" {
		return nil
	}
	if err := domain.ValidateID(accountID); err != nil {
		return err
	}
	var accountProvider string
	var enabled int
	err := tx.QueryRowContext(ctx, "SELECT provider, enabled FROM accounts WHERE id = ?", accountID).Scan(&accountProvider, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NewError(domain.CodeNotFound, "account not found")
	}
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "account linkage could not be checked", err)
	}
	if accountProvider != provider {
		return domain.NewError(domain.CodeConflict, "account provider does not match record")
	}
	if enabled != 1 {
		return domain.NewError(domain.CodePermissionDenied, "account is disabled")
	}
	return nil
}

func validateSessionLinks(ctx context.Context, tx *sql.Tx, session domain.Session) error {
	if err := validateAccountForProvider(ctx, tx, session.Provider, session.AccountID); err != nil {
		return err
	}
	var profileDevice, profileProvider string
	var profileAccount sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT device_id, provider, account_id FROM runtime_profiles WHERE id = ?`, session.RuntimeProfileID).
		Scan(&profileDevice, &profileProvider, &profileAccount)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NewError(domain.CodeNotFound, "runtime profile not found")
	}
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "runtime profile linkage could not be checked", err)
	}
	if profileDevice != string(session.DeviceID) || profileProvider != session.Provider {
		return domain.NewError(domain.CodeConflict, "session profile does not match device or provider")
	}
	if (profileAccount.Valid && domain.ID(profileAccount.String) != session.AccountID) || (!profileAccount.Valid && session.Provider == domain.ProviderCodex) {
		return domain.NewError(domain.CodeConflict, "session profile account does not match")
	}

	var credentialDevice, credentialProvider string
	var credentialAccount sql.NullString
	var credentialStatus domain.CredentialStatus
	var credentialRevision int64
	err = tx.QueryRowContext(ctx, `SELECT device_id, provider, account_id, status, credential_revision FROM credential_instances WHERE id = ?`, session.CredentialInstanceID).
		Scan(&credentialDevice, &credentialProvider, &credentialAccount, &credentialStatus, &credentialRevision)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NewError(domain.CodeNotFound, "credential instance not found")
	}
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "credential linkage could not be checked", err)
	}
	if credentialDevice != string(session.DeviceID) || credentialProvider != session.Provider {
		return domain.NewError(domain.CodeConflict, "session credential does not match device or provider")
	}
	if credentialStatus == domain.CredentialRevoked || credentialStatus == domain.CredentialExpired {
		return domain.NewError(domain.CodePermissionDenied, "credential instance is not usable")
	}
	if session.Provider == domain.ProviderCodex && (credentialRevision < 1 || !credentialAccount.Valid || domain.ID(credentialAccount.String) != session.AccountID) {
		return domain.NewError(domain.CodeConflict, "session credential account or revision does not match")
	}
	if session.Provider == domain.ProviderFake && credentialAccount.Valid {
		return domain.NewError(domain.CodeConflict, "legacy fake credential cannot link an account")
	}
	var revocationReserved int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM credential_revocations WHERE credential_instance_id = ?`, session.CredentialInstanceID).Scan(&revocationReserved); err != nil {
		return domain.WrapError(domain.CodeConflict, "credential revocation state could not be checked", err)
	}
	if revocationReserved != 0 {
		return domain.NewError(domain.CodePermissionDenied, "credential revocation is in progress")
	}
	var workspaceDevice string
	err = tx.QueryRowContext(ctx, "SELECT device_id FROM workspaces WHERE id = ?", session.WorkspaceID).Scan(&workspaceDevice)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NewError(domain.CodeNotFound, "workspace not found")
	}
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "workspace linkage could not be checked", err)
	}
	if workspaceDevice != string(session.DeviceID) {
		return domain.NewError(domain.CodeConflict, "session workspace does not match device")
	}
	return nil
}

func validCreatedUpdated(createdAt, updatedAt time.Time) bool {
	return !createdAt.IsZero() && !updatedAt.IsZero() && !updatedAt.Before(createdAt)
}

func optionalTimeValue(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}

func optionalFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
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
	var sourceAccountID sql.NullString
	var endedAt sql.NullString
	var sourceCapabilitiesJSON []byte
	err := tx.QueryRowContext(ctx, `
		SELECT device_id, account_id, provider, credential_instance_id, runtime_profile_id,
			workspace_id, coalesce(provider_session_id, ''), status, ended_at,
			capability_snapshot_json
		FROM sessions WHERE id = ?`, resumed.ResumedFromSessionID).Scan(
		&source.DeviceID, &sourceAccountID, &source.Provider, &source.CredentialInstanceID,
		&source.RuntimeProfileID, &source.WorkspaceID, &source.ProviderSessionID,
		&source.Status, &endedAt, &sourceCapabilitiesJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NewError(domain.CodeNotFound, "resume source session not found")
	}
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "resume source could not be read", err)
	}
	if sourceAccountID.Valid {
		source.AccountID = domain.ID(sourceAccountID.String)
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
	if source.DeviceID != resumed.DeviceID || source.AccountID != resumed.AccountID || source.Provider != resumed.Provider ||
		source.CredentialInstanceID != resumed.CredentialInstanceID || source.RuntimeProfileID != resumed.RuntimeProfileID ||
		source.WorkspaceID != resumed.WorkspaceID || source.ProviderSessionID != resumed.ProviderSessionID {
		return domain.NewError(domain.CodeConflict, "resumed session changed frozen source fields")
	}
	return nil
}

func fakeAccountID(deviceID domain.ID) domain.ID {
	value := string(deviceID)
	if !strings.HasPrefix(value, "device_") {
		return ""
	}
	return domain.ID("account_" + strings.TrimPrefix(value, "device_"))
}

func ensureFakeAccountTx(ctx context.Context, tx *sql.Tx, accountID, deviceID domain.ID, createdAt, updatedAt time.Time) error {
	if accountID != fakeAccountID(deviceID) || domain.ValidateID(accountID) != nil {
		return domain.NewError(domain.CodeInvalidArgument, "invalid internal fake account")
	}
	_, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO accounts(
			id, provider, display_name, provider_subject_digest, subscription_hint,
			internal, enabled, revision, created_at, updated_at
		) VALUES(?, 'fake', 'Legacy Fake', '', '', 1, 1, 1, ?, ?)`,
		accountID, formatTime(createdAt), formatTime(updatedAt))
	if err != nil {
		return writeError("internal fake account could not be created", err)
	}
	var provider string
	var internal bool
	if err := tx.QueryRowContext(ctx, "SELECT provider, internal FROM accounts WHERE id = ?", accountID).Scan(&provider, &internal); err != nil {
		return writeError("internal fake account could not be verified", err)
	}
	if provider != domain.ProviderFake || !internal {
		return domain.NewError(domain.CodeSchemaIncompatible, "internal fake account conflicts with stored data")
	}
	return nil
}
