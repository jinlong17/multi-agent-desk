package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

const (
	DefaultPageLimit = 50
	MaxPageLimit     = 200
)

type AccountListOptions struct {
	Provider string
	Limit    int
	Cursor   string
}

type ProfileListOptions struct {
	AccountID domain.ID
	Limit     int
	Cursor    string
}

type AccountPage struct {
	Items      []domain.Account
	NextCursor string
}

type ProfilePage struct {
	Items      []domain.RuntimeProfile
	NextCursor string
}

type AccountPatch struct {
	DisplayName      *string
	SubscriptionHint *string
}

type ProfilePatch struct {
	Name          *string
	SelectorAlias *string
}

type ProfileBinding struct {
	Account    domain.Account
	Profile    domain.RuntimeProfile
	Credential *domain.CredentialInstance
}

type pageCursor struct {
	Version int    `json:"v"`
	At      string `json:"at"`
	ID      string `json:"id"`
	Filter  string `json:"filter"`
}

func (s *Store) CreateAccountWithDefaultProfile(ctx context.Context, account domain.Account, profile domain.RuntimeProfile) (domain.Account, domain.RuntimeProfile, error) {
	validated, err := domain.NewAccount(account)
	if err != nil || !domain.PublicProvider(account.Provider) || account.Internal {
		return domain.Account{}, domain.RuntimeProfile{}, domain.NewError(domain.CodeInvalidArgument, "invalid public account")
	}
	account = validated
	profile, err = validatePublicProfile(profile, account)
	if err != nil {
		return domain.Account{}, domain.RuntimeProfile{}, err
	}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		existing, existingProfile, found, err := accountByAliasTx(ctx, tx, profile.SelectorKey)
		if err != nil {
			return err
		}
		if found {
			if existing.Provider == account.Provider && existing.DisplayName == account.DisplayName &&
				existing.SubscriptionHint == account.SubscriptionHint && existingProfile.Name == profile.Name &&
				existingProfile.SelectorAlias == profile.SelectorAlias {
				account, profile = existing, existingProfile
				return nil
			}
			return domain.NewError(domain.CodeAliasConflict, "profile alias already exists")
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO accounts(
				id, provider, display_name, provider_subject_digest, subscription_hint,
				internal, enabled, revision, created_at, updated_at
			) VALUES(?, ?, ?, ?, ?, 0, ?, ?, ?, ?)`,
			account.ID, account.Provider, account.DisplayName, account.ProviderSubjectDigest,
			account.SubscriptionHint, account.Enabled, account.Revision,
			formatTime(account.CreatedAt), formatTime(account.UpdatedAt)); err != nil {
			return writeError("account could not be created", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO runtime_profiles(
				id, account_id, credential_instance_id, device_id, name, provider,
				selector_alias, selector_key, settings_json, internal, enabled, revision,
				created_at, updated_at
			) VALUES(?, ?, NULL, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?)`,
			profile.ID, profile.AccountID, profile.DeviceID, profile.Name, profile.Provider,
			profile.SelectorAlias, profile.SelectorKey, profile.Settings, profile.Enabled,
			profile.Revision, formatTime(profile.CreatedAt), formatTime(profile.UpdatedAt)); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				return domain.NewError(domain.CodeAliasConflict, "profile alias already exists")
			}
			return writeError("default profile could not be created", err)
		}
		return nil
	})
	return account, profile, err
}

func (s *Store) AccountBySelector(ctx context.Context, selector string) (domain.Account, error) {
	if strings.HasPrefix(selector, "@") {
		binding, err := s.ResolveProfile(ctx, selector)
		if err != nil {
			return domain.Account{}, err
		}
		return binding.Account, nil
	}
	return s.PublicAccount(ctx, domain.ID(selector))
}

func (s *Store) PublicAccount(ctx context.Context, id domain.ID) (domain.Account, error) {
	account, err := s.Account(ctx, id)
	if err != nil {
		return domain.Account{}, err
	}
	if account.Internal {
		return domain.Account{}, domain.NewError(domain.CodeAccountNotFound, "account not found")
	}
	return account, nil
}

func (s *Store) ListAccountPage(ctx context.Context, options AccountListOptions) (AccountPage, error) {
	limit, err := pageLimit(options.Limit)
	if err != nil {
		return AccountPage{}, err
	}
	if options.Provider != "" && !domain.PublicProvider(options.Provider) {
		return AccountPage{}, domain.NewError(domain.CodeInvalidArgument, "account provider filter is invalid")
	}
	filter := cursorFilter("accounts", options.Provider)
	cursor, err := decodePageCursor(options.Cursor, filter)
	if err != nil {
		return AccountPage{}, err
	}
	query := `SELECT id, provider, display_name, provider_subject_digest, subscription_hint,
		internal, enabled, revision, created_at, updated_at FROM accounts
		WHERE internal = 0 AND (? = '' OR provider = ?)
		AND (? = '' OR created_at > ? OR (created_at = ? AND id > ?))
		ORDER BY created_at, id LIMIT ?`
	rows, err := s.db.QueryContext(ctx, query, options.Provider, options.Provider,
		cursor.At, cursor.At, cursor.At, cursor.ID, limit+1)
	if err != nil {
		return AccountPage{}, domain.WrapError(domain.CodeConflict, "accounts could not be listed", err)
	}
	defer rows.Close()
	items := make([]domain.Account, 0, limit+1)
	for rows.Next() {
		item, err := scanAccount(rows)
		if err != nil {
			return AccountPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return AccountPage{}, domain.WrapError(domain.CodeConflict, "accounts could not be listed", err)
	}
	page := AccountPage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor, err = encodePageCursor(last.CreatedAt, last.ID, filter)
		if err != nil {
			return AccountPage{}, err
		}
	}
	return page, nil
}

func (s *Store) UpdateAccount(ctx context.Context, id domain.ID, expectedRevision int64, patch AccountPatch, at time.Time) (domain.Account, error) {
	if expectedRevision < 1 || at.IsZero() || (patch.DisplayName == nil && patch.SubscriptionHint == nil) {
		return domain.Account{}, domain.NewError(domain.CodeInvalidArgument, "invalid account update")
	}
	account, err := s.Account(ctx, id)
	if err != nil {
		return domain.Account{}, err
	}
	if account.Revision != expectedRevision {
		return domain.Account{}, domain.NewError(domain.CodeSyncConflict, "account revision changed")
	}
	if patch.DisplayName != nil {
		account.DisplayName = *patch.DisplayName
	}
	if patch.SubscriptionHint != nil {
		account.SubscriptionHint = *patch.SubscriptionHint
	}
	account.Revision++
	account.UpdatedAt = at.UTC()
	if _, err := domain.NewAccount(account); err != nil {
		return domain.Account{}, err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE accounts SET display_name = ?, subscription_hint = ?,
		revision = ?, updated_at = ? WHERE id = ? AND internal = 0 AND revision = ?`,
		account.DisplayName, account.SubscriptionHint, account.Revision, formatTime(account.UpdatedAt), id, expectedRevision)
	if err != nil {
		return domain.Account{}, writeError("account could not be updated", err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return domain.Account{}, domain.NewError(domain.CodeSyncConflict, "account revision changed")
	}
	return account, nil
}

func (s *Store) SetAccountEnabledRevision(ctx context.Context, id domain.ID, expectedRevision int64, enabled bool, at time.Time) (domain.Account, error) {
	if expectedRevision < 1 || at.IsZero() {
		return domain.Account{}, domain.NewError(domain.CodeInvalidArgument, "invalid account state update")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE accounts SET enabled = ?, revision = revision + 1, updated_at = ?
		WHERE id = ? AND internal = 0 AND revision = ?`, enabled, formatTime(at), id, expectedRevision)
	if err != nil {
		return domain.Account{}, writeError("account state could not be updated", err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		if _, lookupErr := s.Account(ctx, id); lookupErr != nil {
			return domain.Account{}, lookupErr
		}
		return domain.Account{}, domain.NewError(domain.CodeSyncConflict, "account revision changed")
	}
	return s.Account(ctx, id)
}

func (s *Store) CreateProfile(ctx context.Context, account domain.Account, profile domain.RuntimeProfile) (domain.RuntimeProfile, error) {
	profile, err := validatePublicProfile(profile, account)
	if err != nil {
		return domain.RuntimeProfile{}, err
	}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		var provider string
		var internal bool
		if err := tx.QueryRowContext(ctx, "SELECT provider, internal FROM accounts WHERE id = ?", account.ID).Scan(&provider, &internal); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return domain.NewError(domain.CodeAccountNotFound, "account not found")
			}
			return writeError("profile account could not be read", err)
		}
		if internal || provider != profile.Provider {
			return domain.NewError(domain.CodeInvalidArgument, "profile provider does not match account")
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO runtime_profiles(
			id, account_id, credential_instance_id, device_id, name, provider,
			selector_alias, selector_key, settings_json, internal, enabled, revision,
			created_at, updated_at
		) VALUES(?, ?, NULL, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?)`,
			profile.ID, profile.AccountID, profile.DeviceID, profile.Name, profile.Provider,
			profile.SelectorAlias, profile.SelectorKey, profile.Settings, profile.Enabled,
			profile.Revision, formatTime(profile.CreatedAt), formatTime(profile.UpdatedAt))
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "unique") {
			return domain.NewError(domain.CodeAliasConflict, "profile alias already exists")
		}
		return writeError("profile could not be created", err)
	})
	return profile, err
}

func (s *Store) ResolveProfile(ctx context.Context, selector string) (ProfileBinding, error) {
	key, err := domain.ParseProfileSelector(selector)
	if err != nil {
		return ProfileBinding{}, err
	}
	profile, err := scanProfile(s.db.QueryRowContext(ctx, `SELECT
		id, account_id, credential_instance_id, device_id, name, provider,
		selector_alias, selector_key, settings_json, internal, enabled, revision,
		created_at, updated_at FROM runtime_profiles
		WHERE selector_key = ? AND internal = 0`, key))
	if err != nil {
		if domain.CodeOf(err) == domain.CodeNotFound {
			return ProfileBinding{}, domain.NewError(domain.CodeProfileNotFound, "profile not found")
		}
		return ProfileBinding{}, err
	}
	account, err := s.Account(ctx, profile.AccountID)
	if err != nil {
		return ProfileBinding{}, err
	}
	binding := ProfileBinding{Account: account, Profile: profile}
	if profile.CredentialInstanceID != "" {
		credential, err := s.CredentialInstance(ctx, profile.CredentialInstanceID)
		if err != nil {
			return ProfileBinding{}, err
		}
		binding.Credential = &credential
	}
	return binding, nil
}

func (s *Store) Profile(ctx context.Context, id domain.ID) (domain.RuntimeProfile, error) {
	profile, err := scanProfile(s.db.QueryRowContext(ctx, `SELECT
		id, account_id, credential_instance_id, device_id, name, provider,
		selector_alias, selector_key, settings_json, internal, enabled, revision,
		created_at, updated_at FROM runtime_profiles WHERE id = ? AND internal = 0`, id))
	if domain.CodeOf(err) == domain.CodeNotFound {
		return domain.RuntimeProfile{}, domain.NewError(domain.CodeProfileNotFound, "profile not found")
	}
	return profile, err
}

func (s *Store) ListProfiles(ctx context.Context, options ProfileListOptions) (ProfilePage, error) {
	limit, err := pageLimit(options.Limit)
	if err != nil {
		return ProfilePage{}, err
	}
	if options.AccountID != "" && domain.ValidateID(options.AccountID) != nil {
		return ProfilePage{}, domain.NewError(domain.CodeInvalidArgument, "profile account filter is invalid")
	}
	filter := cursorFilter("profiles", string(options.AccountID))
	cursor, err := decodePageCursor(options.Cursor, filter)
	if err != nil {
		return ProfilePage{}, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, account_id, credential_instance_id,
		device_id, name, provider, selector_alias, selector_key, settings_json,
		internal, enabled, revision, created_at, updated_at FROM runtime_profiles
		WHERE internal = 0 AND (? = '' OR account_id = ?)
		AND (? = '' OR created_at > ? OR (created_at = ? AND id > ?))
		ORDER BY created_at, id LIMIT ?`, options.AccountID, options.AccountID,
		cursor.At, cursor.At, cursor.At, cursor.ID, limit+1)
	if err != nil {
		return ProfilePage{}, domain.WrapError(domain.CodeConflict, "profiles could not be listed", err)
	}
	defer rows.Close()
	items := make([]domain.RuntimeProfile, 0, limit+1)
	for rows.Next() {
		profile, err := scanProfile(rows)
		if err != nil {
			return ProfilePage{}, err
		}
		items = append(items, profile)
	}
	if err := rows.Err(); err != nil {
		return ProfilePage{}, domain.WrapError(domain.CodeConflict, "profiles could not be listed", err)
	}
	page := ProfilePage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor, err = encodePageCursor(last.CreatedAt, last.ID, filter)
		if err != nil {
			return ProfilePage{}, err
		}
	}
	return page, nil
}

func (s *Store) UpdateProfile(ctx context.Context, id domain.ID, expectedRevision int64, patch ProfilePatch, at time.Time) (domain.RuntimeProfile, error) {
	if expectedRevision < 1 || at.IsZero() || (patch.Name == nil && patch.SelectorAlias == nil) {
		return domain.RuntimeProfile{}, domain.NewError(domain.CodeInvalidArgument, "invalid profile update")
	}
	profile, err := s.Profile(ctx, id)
	if err != nil {
		return domain.RuntimeProfile{}, err
	}
	if profile.Revision != expectedRevision {
		return domain.RuntimeProfile{}, domain.NewError(domain.CodeSyncConflict, "profile revision changed")
	}
	if patch.Name != nil {
		profile.Name = *patch.Name
	}
	if patch.SelectorAlias != nil {
		key, err := domain.CanonicalSelectorAlias(*patch.SelectorAlias)
		if err != nil {
			return domain.RuntimeProfile{}, err
		}
		profile.SelectorAlias, profile.SelectorKey = *patch.SelectorAlias, key
	}
	if len(profile.Name) < 1 || len(profile.Name) > 128 {
		return domain.RuntimeProfile{}, domain.NewError(domain.CodeInvalidArgument, "invalid profile name")
	}
	profile.Revision++
	profile.UpdatedAt = at.UTC()
	result, err := s.db.ExecContext(ctx, `UPDATE runtime_profiles SET name = ?, selector_alias = ?,
		selector_key = ?, revision = ?, updated_at = ? WHERE id = ? AND internal = 0 AND revision = ?`,
		profile.Name, profile.SelectorAlias, profile.SelectorKey, profile.Revision,
		formatTime(profile.UpdatedAt), id, expectedRevision)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return domain.RuntimeProfile{}, domain.NewError(domain.CodeAliasConflict, "profile alias already exists")
		}
		return domain.RuntimeProfile{}, writeError("profile could not be updated", err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return domain.RuntimeProfile{}, domain.NewError(domain.CodeSyncConflict, "profile revision changed")
	}
	return profile, nil
}

func (s *Store) SetProfileEnabled(ctx context.Context, id domain.ID, expectedRevision int64, enabled bool, at time.Time) (domain.RuntimeProfile, error) {
	if expectedRevision < 1 || at.IsZero() {
		return domain.RuntimeProfile{}, domain.NewError(domain.CodeInvalidArgument, "invalid profile state update")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE runtime_profiles SET enabled = ?, revision = revision + 1,
		updated_at = ? WHERE id = ? AND internal = 0 AND revision = ?`, enabled, formatTime(at), id, expectedRevision)
	if err != nil {
		return domain.RuntimeProfile{}, writeError("profile state could not be updated", err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		if _, lookupErr := s.Profile(ctx, id); lookupErr != nil {
			return domain.RuntimeProfile{}, lookupErr
		}
		return domain.RuntimeProfile{}, domain.NewError(domain.CodeSyncConflict, "profile revision changed")
	}
	return s.Profile(ctx, id)
}

func (s *Store) DeleteProfile(ctx context.Context, id domain.ID, expectedRevision int64, at time.Time) error {
	if expectedRevision < 1 || at.IsZero() {
		return domain.NewError(domain.CodeInvalidArgument, "invalid profile deletion")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		profile, err := profileTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if profile.Internal {
			return domain.NewError(domain.CodePermissionDenied, "internal profile cannot be deleted")
		}
		if profile.Revision != expectedRevision {
			return domain.NewError(domain.CodeSyncConflict, "profile revision changed")
		}
		if profile.Enabled {
			return domain.NewError(domain.CodeConflict, "profile must be disabled before deletion")
		}
		if profile.CredentialInstanceID != "" {
			return domain.NewError(domain.CodeProviderCleanupRequired, "profile credential cleanup is required")
		}
		var references int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sessions WHERE runtime_profile_id = ?", id).Scan(&references); err != nil {
			return writeError("profile session references could not be checked", err)
		}
		if references != 0 {
			return domain.NewError(domain.CodeProfileInUse, "profile is referenced by sessions")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO metadata_tombstones(
			entity_type, entity_id, provider, final_revision, deleted_at
		) VALUES('profile', ?, ?, ?, ?)`, id, profile.Provider, profile.Revision+1, formatTime(at)); err != nil {
			return writeError("profile tombstone could not be created", err)
		}
		result, err := tx.ExecContext(ctx, "DELETE FROM runtime_profiles WHERE id = ? AND revision = ?", id, expectedRevision)
		if err != nil {
			return writeError("profile could not be deleted", err)
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return domain.NewError(domain.CodeSyncConflict, "profile revision changed")
		}
		return nil
	})
}

func (s *Store) DeleteAccount(ctx context.Context, id domain.ID, expectedRevision int64, at time.Time) error {
	if expectedRevision < 1 || at.IsZero() {
		return domain.NewError(domain.CodeInvalidArgument, "invalid account deletion")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		account, err := accountTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if account.Internal {
			return domain.NewError(domain.CodePermissionDenied, "internal account cannot be deleted")
		}
		if account.Revision != expectedRevision {
			return domain.NewError(domain.CodeSyncConflict, "account revision changed")
		}
		if account.Enabled {
			return domain.NewError(domain.CodeConflict, "account must be disabled before deletion")
		}
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sessions WHERE account_id = ?", id).Scan(&count); err != nil {
			return writeError("account session references could not be checked", err)
		}
		if count != 0 {
			return domain.NewError(domain.CodeActiveSessions, "account is referenced by sessions")
		}
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM credential_instances WHERE account_id = ?", id).Scan(&count); err != nil {
			return writeError("account credentials could not be checked", err)
		}
		if count != 0 {
			return domain.NewError(domain.CodeProviderCleanupRequired, "account credential cleanup is required")
		}
		rows, err := tx.QueryContext(ctx, "SELECT id, provider, revision FROM runtime_profiles WHERE account_id = ? AND internal = 0", id)
		if err != nil {
			return writeError("account profiles could not be read", err)
		}
		type tombstone struct {
			id       domain.ID
			provider string
			revision int64
		}
		var profiles []tombstone
		for rows.Next() {
			var item tombstone
			if err := rows.Scan(&item.id, &item.provider, &item.revision); err != nil {
				_ = rows.Close()
				return writeError("account profiles could not be read", err)
			}
			profiles = append(profiles, item)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return writeError("account profiles could not be read", err)
		}
		if err := rows.Close(); err != nil {
			return writeError("account profiles could not be closed", err)
		}
		for _, profile := range profiles {
			if _, err := tx.ExecContext(ctx, `INSERT INTO metadata_tombstones(
				entity_type, entity_id, provider, final_revision, deleted_at
			) VALUES('profile', ?, ?, ?, ?)`, profile.id, profile.provider, profile.revision+1, formatTime(at)); err != nil {
				return writeError("profile tombstone could not be created", err)
			}
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM usage_snapshots WHERE account_id = ?", id); err != nil {
			return writeError("account usage could not be deleted", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM runtime_profiles WHERE account_id = ? AND internal = 0", id); err != nil {
			return writeError("account profiles could not be deleted", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO metadata_tombstones(
			entity_type, entity_id, provider, final_revision, deleted_at
		) VALUES('account', ?, ?, ?, ?)`, id, account.Provider, account.Revision+1, formatTime(at)); err != nil {
			return writeError("account tombstone could not be created", err)
		}
		result, err := tx.ExecContext(ctx, "DELETE FROM accounts WHERE id = ? AND revision = ?", id, expectedRevision)
		if err != nil {
			return writeError("account could not be deleted", err)
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return domain.NewError(domain.CodeSyncConflict, "account revision changed")
		}
		return nil
	})
}

func (s *Store) CreateUsageSnapshotWithWindows(ctx context.Context, snapshot domain.UsageSnapshot) error {
	validated, err := domain.NewUsageSnapshot(snapshot)
	if err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var provider string
		var internal bool
		if err := tx.QueryRowContext(ctx, "SELECT provider, internal FROM accounts WHERE id = ?", validated.AccountID).Scan(&provider, &internal); err != nil {
			return writeError("usage account could not be read", err)
		}
		if internal || provider != validated.Provider {
			return domain.NewError(domain.CodeInvalidArgument, "usage provider does not match account")
		}
		var replayID domain.ID
		err := tx.QueryRowContext(ctx, `SELECT id FROM usage_snapshots
			WHERE account_id = ? AND device_id = ? AND provider_version = ?
				AND observed_at = ? AND raw_reference_hash = ?`,
			validated.AccountID, validated.DeviceID, validated.ProviderVersion,
			formatTime(validated.ObservedAt), validated.RawReferenceHash).Scan(&replayID)
		if err == nil {
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return domain.WrapError(domain.CodeConflict, "usage replay could not be checked", err)
		}
		var credential any
		if validated.CredentialInstanceID != "" {
			credential = validated.CredentialInstanceID
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO usage_snapshots(
			id, account_id, credential_instance_id, device_id, provider,
			provider_version, source, confidence, availability, observed_at,
			stale_at, raw_reference_hash
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, validated.ID,
			validated.AccountID, credential, validated.DeviceID, validated.Provider,
			validated.ProviderVersion, validated.Source, validated.Confidence,
			validated.Availability, formatTime(validated.ObservedAt),
			formatTime(validated.StaleAt), validated.RawReferenceHash); err != nil {
			return writeError("usage snapshot could not be created", err)
		}
		for index, window := range validated.Windows {
			if _, err := tx.ExecContext(ctx, `INSERT INTO usage_windows(
				snapshot_id, position, provider_limit_id, kind, label,
				duration_seconds, used_value, limit_value, used_percent,
				remaining_percent, resets_at
			) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, validated.ID, index,
				window.ProviderLimitID, window.Kind, window.Label, window.DurationSeconds,
				window.UsedValue, window.LimitValue, window.UsedPercent,
				window.RemainingPercent, optionalUsageTimeValue(window.ResetsAt)); err != nil {
				return writeError("usage window could not be created", err)
			}
		}
		return nil
	})
}

func (s *Store) ListUsageSnapshotsWithWindows(ctx context.Context, accountID domain.ID) ([]domain.UsageSnapshot, error) {
	if accountID != "" && domain.ValidateID(accountID) != nil {
		return nil, domain.NewError(domain.CodeInvalidArgument, "usage account filter is invalid")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, account_id,
		coalesce(credential_instance_id, ''), device_id, provider, provider_version,
		source, confidence, availability, observed_at, stale_at, raw_reference_hash
		FROM usage_snapshots WHERE (? = '' OR account_id = ?)
		ORDER BY observed_at DESC, id DESC`, accountID, accountID)
	if err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "usage snapshots could not be listed", err)
	}
	var snapshots []domain.UsageSnapshot
	for rows.Next() {
		var snapshot domain.UsageSnapshot
		var observedAt, staleAt string
		if err := rows.Scan(&snapshot.ID, &snapshot.AccountID, &snapshot.CredentialInstanceID,
			&snapshot.DeviceID, &snapshot.Provider, &snapshot.ProviderVersion,
			&snapshot.Source, &snapshot.Confidence, &snapshot.Availability,
			&observedAt, &staleAt, &snapshot.RawReferenceHash); err != nil {
			_ = rows.Close()
			return nil, domain.WrapError(domain.CodeConflict, "usage snapshot could not be read", err)
		}
		snapshot.ObservedAt, err = parseTime(observedAt)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		snapshot.StaleAt, err = parseTime(staleAt)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, domain.WrapError(domain.CodeConflict, "usage snapshots could not be listed", err)
	}
	if err := rows.Close(); err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "usage snapshots could not be closed", err)
	}
	for index := range snapshots {
		windows, err := s.usageWindows(ctx, snapshots[index].ID)
		if err != nil {
			return nil, err
		}
		snapshots[index].Windows = windows
	}
	return snapshots, nil
}

func (s *Store) usageWindows(ctx context.Context, snapshotID domain.ID) ([]domain.UsageWindow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT provider_limit_id, kind, label,
		duration_seconds, used_value, limit_value, used_percent, remaining_percent,
		resets_at FROM usage_windows WHERE snapshot_id = ? ORDER BY position`, snapshotID)
	if err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "usage windows could not be listed", err)
	}
	defer rows.Close()
	var windows []domain.UsageWindow
	for rows.Next() {
		var window domain.UsageWindow
		var duration sql.NullInt64
		var used, limit, usedPercent, remainingPercent sql.NullFloat64
		var resetsAt sql.NullString
		if err := rows.Scan(&window.ProviderLimitID, &window.Kind, &window.Label,
			&duration, &used, &limit, &usedPercent, &remainingPercent, &resetsAt); err != nil {
			return nil, domain.WrapError(domain.CodeConflict, "usage window could not be read", err)
		}
		window.DurationSeconds = optionalInt64(duration)
		window.UsedValue = optionalFloat64(used)
		window.LimitValue = optionalFloat64(limit)
		window.UsedPercent = optionalFloat64(usedPercent)
		window.RemainingPercent = optionalFloat64(remainingPercent)
		window.ResetsAt, err = parseOptionalTime(resetsAt)
		if err != nil {
			return nil, err
		}
		windows = append(windows, window)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "usage windows could not be listed", err)
	}
	return windows, nil
}

func validatePublicProfile(profile domain.RuntimeProfile, account domain.Account) (domain.RuntimeProfile, error) {
	if domain.ValidateID(profile.ID) != nil || domain.ValidateID(profile.DeviceID) != nil ||
		profile.AccountID != account.ID || profile.Provider != account.Provider ||
		!domain.PublicProvider(profile.Provider) || profile.Internal ||
		len(profile.Name) < 1 || len(profile.Name) > 128 || profile.Revision < 1 ||
		profile.CreatedAt.IsZero() || profile.UpdatedAt.Before(profile.CreatedAt) ||
		profile.CredentialInstanceID != "" {
		return domain.RuntimeProfile{}, domain.NewError(domain.CodeInvalidArgument, "invalid public profile")
	}
	key, err := domain.CanonicalSelectorAlias(profile.SelectorAlias)
	if err != nil {
		return domain.RuntimeProfile{}, err
	}
	settings, err := domain.CanonicalSettings(profile.Settings)
	if err != nil {
		return domain.RuntimeProfile{}, err
	}
	profile.SelectorKey, profile.Settings = key, settings
	return profile, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanAccount(row rowScanner) (domain.Account, error) {
	var account domain.Account
	var createdAt, updatedAt string
	err := row.Scan(&account.ID, &account.Provider, &account.DisplayName,
		&account.ProviderSubjectDigest, &account.SubscriptionHint, &account.Internal,
		&account.Enabled, &account.Revision, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Account{}, domain.NewError(domain.CodeAccountNotFound, "account not found")
	}
	if err != nil {
		return domain.Account{}, domain.WrapError(domain.CodeConflict, "account could not be read", err)
	}
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

func scanProfile(row rowScanner) (domain.RuntimeProfile, error) {
	var profile domain.RuntimeProfile
	var credentialID, alias, aliasKey sql.NullString
	var settings []byte
	var createdAt, updatedAt string
	err := row.Scan(&profile.ID, &profile.AccountID, &credentialID, &profile.DeviceID,
		&profile.Name, &profile.Provider, &alias, &aliasKey, &settings,
		&profile.Internal, &profile.Enabled, &profile.Revision, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RuntimeProfile{}, domain.NewError(domain.CodeNotFound, "profile not found")
	}
	if err != nil {
		return domain.RuntimeProfile{}, domain.WrapError(domain.CodeConflict, "profile could not be read", err)
	}
	profile.CredentialInstanceID = domain.ID(credentialID.String)
	profile.SelectorAlias, profile.SelectorKey = alias.String, aliasKey.String
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

func accountTx(ctx context.Context, tx *sql.Tx, id domain.ID) (domain.Account, error) {
	return scanAccount(tx.QueryRowContext(ctx, `SELECT id, provider, display_name,
		provider_subject_digest, subscription_hint, internal, enabled, revision,
		created_at, updated_at FROM accounts WHERE id = ?`, id))
}

func profileTx(ctx context.Context, tx *sql.Tx, id domain.ID) (domain.RuntimeProfile, error) {
	return scanProfile(tx.QueryRowContext(ctx, `SELECT id, account_id,
		credential_instance_id, device_id, name, provider, selector_alias,
		selector_key, settings_json, internal, enabled, revision, created_at,
		updated_at FROM runtime_profiles WHERE id = ?`, id))
}

func accountByAliasTx(ctx context.Context, tx *sql.Tx, key string) (domain.Account, domain.RuntimeProfile, bool, error) {
	var accountID, profileID domain.ID
	err := tx.QueryRowContext(ctx, `SELECT account_id, id FROM runtime_profiles
		WHERE selector_key = ? AND internal = 0`, key).Scan(&accountID, &profileID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Account{}, domain.RuntimeProfile{}, false, nil
	}
	if err != nil {
		return domain.Account{}, domain.RuntimeProfile{}, false, writeError("profile alias could not be checked", err)
	}
	account, err := accountTx(ctx, tx, accountID)
	if err != nil {
		return domain.Account{}, domain.RuntimeProfile{}, false, err
	}
	profile, err := profileTx(ctx, tx, profileID)
	return account, profile, true, err
}

func pageLimit(limit int) (int, error) {
	if limit == 0 {
		return DefaultPageLimit, nil
	}
	if limit < 1 || limit > MaxPageLimit {
		return 0, domain.NewError(domain.CodeInvalidArgument, "page limit must be between 1 and 200")
	}
	return limit, nil
}

func cursorFilter(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(digest[:])
}

func encodePageCursor(at time.Time, id domain.ID, filter string) (string, error) {
	payload, err := json.Marshal(pageCursor{Version: 1, At: formatTime(at), ID: string(id), Filter: filter})
	if err != nil {
		return "", domain.WrapError(domain.CodeConflict, "page cursor could not be encoded", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodePageCursor(value, filter string) (pageCursor, error) {
	if value == "" {
		return pageCursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) > 1024 {
		return pageCursor{}, domain.NewError(domain.CodeInvalidArgument, "page cursor is invalid")
	}
	var cursor pageCursor
	if err := json.Unmarshal(raw, &cursor); err != nil || cursor.Version != 1 ||
		cursor.Filter != filter || domain.ValidateID(domain.ID(cursor.ID)) != nil {
		return pageCursor{}, domain.NewError(domain.CodeInvalidArgument, "page cursor is invalid for this filter")
	}
	if _, err := time.Parse(time.RFC3339Nano, cursor.At); err != nil {
		return pageCursor{}, domain.NewError(domain.CodeInvalidArgument, "page cursor timestamp is invalid")
	}
	return cursor, nil
}

func optionalUsageTimeValue(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}

func optionalInt64(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func optionalFloat64(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	result := value.Float64
	return &result
}
