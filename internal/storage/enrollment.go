package storage

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

type EnrollmentState string

const (
	EnrollmentBegun                EnrollmentState = "begun"
	EnrollmentValidating           EnrollmentState = "validating"
	EnrollmentAwaitingConfirmation EnrollmentState = "awaiting_confirmation"
	EnrollmentSucceeded            EnrollmentState = "succeeded"
	EnrollmentCancelled            EnrollmentState = "cancelled"
	EnrollmentExpired              EnrollmentState = "expired"
	EnrollmentFailed               EnrollmentState = "failed"
)

type AuthEnrollment struct {
	ID                             domain.ID
	ClientDeviceID                 domain.ID
	RuntimeProfileID               domain.ID
	CredentialInstanceID           domain.ID
	BinaryFingerprint              string
	StagingPath                    string
	State                          EnrollmentState
	IdempotencyDigest              string
	CompletionIdempotencyDigest    string
	ConfirmationAccountID          domain.ID
	ConfirmationAccountRevision    int64
	ConfirmationProfileID          domain.ID
	ConfirmationProfileRevision    int64
	ConfirmationCredentialID       domain.ID
	ConfirmationCredentialRevision int64
	ConfirmationAliasDigest        string
	ConfirmedByClientID            domain.ID
	ConfirmedAt                    *time.Time
	ExpiresAt                      time.Time
	CreatedAt                      time.Time
	UpdatedAt                      time.Time
}

func (s *Store) BeginAuthEnrollment(ctx context.Context, enrollment AuthEnrollment, credential *domain.CredentialInstance) error {
	if enrollment.ID == "" || enrollment.ClientDeviceID == "" || enrollment.RuntimeProfileID == "" ||
		enrollment.CredentialInstanceID == "" || len(enrollment.BinaryFingerprint) != 64 || enrollment.StagingPath == "" ||
		enrollment.State != EnrollmentBegun || len(enrollment.IdempotencyDigest) != 64 || enrollment.ExpiresAt.IsZero() ||
		enrollment.CreatedAt.IsZero() || enrollment.UpdatedAt.IsZero() {
		return domain.NewError(domain.CodeInvalidArgument, "auth enrollment is invalid")
	}
	if _, err := hex.DecodeString(enrollment.IdempotencyDigest); err != nil {
		return domain.NewError(domain.CodeInvalidArgument, "auth enrollment idempotency digest is invalid")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if credential != nil {
			if err := domain.ValidateID(credential.ID); err != nil {
				return err
			}
			if credential.Provider != domain.ProviderCodex || credential.AuthMethod != domain.AuthMethodInteractive || credential.AccountID == "" || credential.CredentialRevision != 1 || len(credential.SecretDigest) != 64 {
				return domain.NewError(domain.CodeInvalidArgument, "enrollment credential is invalid")
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO credential_instances(id, device_id, account_id, provider, auth_method, secret_ref, status, credential_revision, secret_digest, created_at, updated_at) VALUES(?, ?, ?, 'codex', 'interactive', ?, ?, ?, ?, ?, ?)`,
				credential.ID, credential.DeviceID, credential.AccountID, credential.SecretRef, credential.Status,
				credential.CredentialRevision, credential.SecretDigest, formatTime(credential.CreatedAt), formatTime(credential.UpdatedAt))
			if err != nil {
				return writeError("enrollment credential could not be created", err)
			}
		} else {
			var profileID domain.ID
			if err := tx.QueryRowContext(ctx, `SELECT rp.id FROM credential_instances ci JOIN runtime_profiles rp ON rp.device_id=ci.device_id AND rp.account_id=ci.account_id WHERE ci.id=? AND ci.provider='codex' AND rp.id=?`, enrollment.CredentialInstanceID, enrollment.RuntimeProfileID).Scan(&profileID); err != nil {
				return domain.NewError(domain.CodeConflict, "enrollment credential does not match profile")
			}
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO auth_enrollments(id, client_device_id, runtime_profile_id, credential_instance_id, binary_fingerprint, staging_path, state, idempotency_digest, completion_idempotency_digest, expires_at, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, 'begun', ?, NULL, ?, ?, ?)`,
			enrollment.ID, enrollment.ClientDeviceID, enrollment.RuntimeProfileID, enrollment.CredentialInstanceID,
			enrollment.BinaryFingerprint, enrollment.StagingPath, enrollment.IdempotencyDigest,
			formatTime(enrollment.ExpiresAt), formatTime(enrollment.CreatedAt), formatTime(enrollment.UpdatedAt))
		return writeError("auth enrollment could not be started", err)
	})
}

func (s *Store) AuthEnrollment(ctx context.Context, id domain.ID) (AuthEnrollment, error) {
	var value AuthEnrollment
	var credentialID, confirmationAccountID, confirmationProfileID, confirmationCredentialID sql.NullString
	var confirmationAccountRevision, confirmationProfileRevision, confirmationCredentialRevision sql.NullInt64
	var confirmationAliasDigest, confirmedByClientID, confirmedAt sql.NullString
	var expiresAt, createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `SELECT id, client_device_id, runtime_profile_id, credential_instance_id, binary_fingerprint, staging_path, state, idempotency_digest, coalesce(completion_idempotency_digest,''), confirmation_account_id, confirmation_account_revision, confirmation_profile_id, confirmation_profile_revision, confirmation_credential_id, confirmation_credential_revision, confirmation_alias_digest, confirmed_by_client_id, confirmed_at, expires_at, created_at, updated_at FROM auth_enrollments WHERE id=?`, id).Scan(
		&value.ID, &value.ClientDeviceID, &value.RuntimeProfileID, &credentialID,
		&value.BinaryFingerprint, &value.StagingPath, &value.State, &value.IdempotencyDigest, &value.CompletionIdempotencyDigest,
		&confirmationAccountID, &confirmationAccountRevision, &confirmationProfileID, &confirmationProfileRevision,
		&confirmationCredentialID, &confirmationCredentialRevision, &confirmationAliasDigest, &confirmedByClientID, &confirmedAt,
		&expiresAt, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AuthEnrollment{}, domain.NewError(domain.CodeNotFound, "auth enrollment not found")
	}
	if err != nil {
		return AuthEnrollment{}, domain.WrapError(domain.CodeConflict, "auth enrollment could not be read", err)
	}
	if credentialID.Valid {
		value.CredentialInstanceID = domain.ID(credentialID.String)
	}
	if confirmationAccountID.Valid {
		value.ConfirmationAccountID = domain.ID(confirmationAccountID.String)
	}
	if confirmationAccountRevision.Valid {
		value.ConfirmationAccountRevision = confirmationAccountRevision.Int64
	}
	if confirmationProfileID.Valid {
		value.ConfirmationProfileID = domain.ID(confirmationProfileID.String)
	}
	if confirmationProfileRevision.Valid {
		value.ConfirmationProfileRevision = confirmationProfileRevision.Int64
	}
	if confirmationCredentialID.Valid {
		value.ConfirmationCredentialID = domain.ID(confirmationCredentialID.String)
	}
	if confirmationCredentialRevision.Valid {
		value.ConfirmationCredentialRevision = confirmationCredentialRevision.Int64
	}
	if confirmationAliasDigest.Valid {
		value.ConfirmationAliasDigest = confirmationAliasDigest.String
	}
	if confirmedByClientID.Valid {
		value.ConfirmedByClientID = domain.ID(confirmedByClientID.String)
	}
	value.ConfirmedAt, err = parseOptionalTime(confirmedAt)
	if err != nil {
		return AuthEnrollment{}, err
	}
	value.ExpiresAt, err = parseTime(expiresAt)
	if err != nil {
		return AuthEnrollment{}, err
	}
	value.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return AuthEnrollment{}, err
	}
	value.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return AuthEnrollment{}, err
	}
	return value, nil
}

func (s *Store) AuthEnrollmentByBeginDigest(ctx context.Context, clientID domain.ID, digest string) (AuthEnrollment, error) {
	if err := domain.ValidateID(clientID); err != nil {
		return AuthEnrollment{}, err
	}
	if len(digest) != 64 {
		return AuthEnrollment{}, domain.NewError(domain.CodeInvalidArgument, "begin idempotency digest is invalid")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return AuthEnrollment{}, domain.NewError(domain.CodeInvalidArgument, "begin idempotency digest is invalid")
	}
	var id domain.ID
	err := s.db.QueryRowContext(ctx, `SELECT id FROM auth_enrollments WHERE client_device_id=? AND idempotency_digest=?`, clientID, digest).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return AuthEnrollment{}, domain.NewError(domain.CodeNotFound, "auth enrollment replay not found")
	}
	if err != nil {
		return AuthEnrollment{}, domain.WrapError(domain.CodeConflict, "auth enrollment replay could not be read", err)
	}
	return s.AuthEnrollment(ctx, id)
}

func (s *Store) ClaimAuthEnrollment(ctx context.Context, id, clientID domain.ID, completionDigest string, at time.Time) (AuthEnrollment, error) {
	if len(completionDigest) != 64 || at.IsZero() {
		return AuthEnrollment{}, domain.NewError(domain.CodeInvalidArgument, "completion idempotency digest is invalid")
	}
	if _, err := hex.DecodeString(completionDigest); err != nil {
		return AuthEnrollment{}, domain.NewError(domain.CodeInvalidArgument, "completion idempotency digest is invalid")
	}
	existing, err := s.AuthEnrollment(ctx, id)
	if err != nil {
		return AuthEnrollment{}, err
	}
	if existing.ClientDeviceID != clientID {
		return AuthEnrollment{}, domain.NewError(domain.CodePermissionDenied, "auth enrollment owner is required")
	}
	if (existing.State == EnrollmentBegun || existing.State == EnrollmentValidating || existing.State == EnrollmentAwaitingConfirmation) && !at.Before(existing.ExpiresAt) {
		if _, finishErr := s.FinishAuthEnrollment(ctx, id, clientID, EnrollmentExpired, at); finishErr != nil {
			return AuthEnrollment{}, finishErr
		}
		return AuthEnrollment{}, domain.NewError(domain.CodeDeadlineExceeded, "auth enrollment expired")
	}
	if (existing.State == EnrollmentSucceeded || existing.State == EnrollmentValidating || existing.State == EnrollmentAwaitingConfirmation) && existing.CompletionIdempotencyDigest == completionDigest {
		return existing, nil
	}
	result, err := s.db.ExecContext(ctx, `UPDATE auth_enrollments SET state='validating', completion_idempotency_digest=?, updated_at=? WHERE id=? AND client_device_id=? AND state='begun' AND expires_at>?`, completionDigest, formatTime(at), id, clientID, formatTime(at))
	if err != nil {
		return AuthEnrollment{}, writeError("auth enrollment could not be claimed", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return AuthEnrollment{}, domain.NewError(domain.CodeConflict, "auth enrollment is not claimable")
	}
	return s.AuthEnrollment(ctx, id)
}

// AwaitAuthEnrollmentConfirmation records only the internal tuple and alias
// digest after official login validation. Credential bytes remain in the
// private staging Home until the owning client explicitly confirms.
func (s *Store) AwaitAuthEnrollmentConfirmation(ctx context.Context, id, clientID, accountID, profileID, credentialID domain.ID,
	accountRevision, profileRevision, credentialRevision int64, aliasDigest string, at time.Time) (AuthEnrollment, error) {
	for _, value := range []domain.ID{id, clientID, accountID, profileID, credentialID} {
		if domain.ValidateID(value) != nil {
			return AuthEnrollment{}, domain.NewError(domain.CodeInvalidArgument, "auth confirmation tuple is invalid")
		}
	}
	if accountRevision < 1 || profileRevision < 1 || credentialRevision < 1 || !validFingerprint(aliasDigest) || at.IsZero() {
		return AuthEnrollment{}, domain.NewError(domain.CodeInvalidArgument, "auth confirmation digest is invalid")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE auth_enrollments SET
		state='awaiting_confirmation', confirmation_account_id=?, confirmation_account_revision=?,
		confirmation_profile_id=?, confirmation_profile_revision=?, confirmation_credential_id=?,
		confirmation_credential_revision=?, confirmation_alias_digest=?, updated_at=?
		WHERE id=? AND client_device_id=? AND runtime_profile_id=? AND credential_instance_id=?
			AND state='validating' AND expires_at>?`, accountID, accountRevision, profileID, profileRevision, credentialID, credentialRevision,
		aliasDigest, formatTime(at), id, clientID, profileID, credentialID, formatTime(at))
	if err != nil {
		return AuthEnrollment{}, writeError("auth confirmation could not be prepared", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return AuthEnrollment{}, domain.NewError(domain.CodeConflict, "auth enrollment changed before confirmation")
	}
	return s.AuthEnrollment(ctx, id)
}

// ConfirmAuthEnrollmentAttestation binds the explicit operator attestation to
// the enrollment owner. Vault sealing is a later CAS in the same application
// operation and requires this state.
func (s *Store) ConfirmAuthEnrollmentAttestation(ctx context.Context, id, clientID domain.ID, aliasDigest string, at time.Time) (AuthEnrollment, error) {
	if domain.ValidateID(id) != nil || domain.ValidateID(clientID) != nil || !validFingerprint(aliasDigest) || at.IsZero() {
		return AuthEnrollment{}, domain.NewError(domain.CodeInvalidArgument, "auth confirmation is invalid")
	}
	existing, err := s.AuthEnrollment(ctx, id)
	if err != nil {
		return AuthEnrollment{}, err
	}
	if existing.ClientDeviceID != clientID {
		return AuthEnrollment{}, domain.NewError(domain.CodePermissionDenied, "auth enrollment owner is required")
	}
	if !at.Before(existing.ExpiresAt) {
		_, _ = s.FinishAuthEnrollment(ctx, id, clientID, EnrollmentExpired, at)
		return AuthEnrollment{}, domain.NewError(domain.CodeConfirmationExpired, "auth confirmation expired")
	}
	if existing.State == EnrollmentSucceeded && existing.ConfirmedByClientID == clientID && existing.ConfirmationAliasDigest == aliasDigest {
		return existing, nil
	}
	if existing.State != EnrollmentAwaitingConfirmation || existing.ConfirmationAliasDigest != aliasDigest {
		return AuthEnrollment{}, domain.NewError(domain.CodeIdentityConfirmationMismatch, "auth confirmation does not match enrollment")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE auth_enrollments SET confirmed_by_client_id=?, confirmed_at=?, updated_at=?
		WHERE id=? AND client_device_id=? AND state='awaiting_confirmation' AND confirmation_alias_digest=?
			AND confirmed_by_client_id IS NULL AND expires_at>?`, clientID, formatTime(at), formatTime(at),
		id, clientID, aliasDigest, formatTime(at))
	if err != nil {
		return AuthEnrollment{}, writeError("auth confirmation could not be recorded", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return AuthEnrollment{}, domain.NewError(domain.CodeConflict, "auth confirmation changed before commit")
	}
	return s.AuthEnrollment(ctx, id)
}

func (s *Store) FinishAuthEnrollment(ctx context.Context, id, clientID domain.ID, state EnrollmentState, at time.Time) (AuthEnrollment, error) {
	if (state != EnrollmentCancelled && state != EnrollmentExpired && state != EnrollmentFailed) || at.IsZero() {
		return AuthEnrollment{}, domain.NewError(domain.CodeInvalidArgument, "auth enrollment terminal state is invalid")
	}
	returnValue := AuthEnrollment{}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var credentialID domain.ID
		var current EnrollmentState
		if err := tx.QueryRowContext(ctx, `SELECT credential_instance_id, state FROM auth_enrollments WHERE id=? AND client_device_id=?`, id, clientID).Scan(&credentialID, &current); err != nil {
			return domain.NewError(domain.CodeNotFound, "auth enrollment not found")
		}
		if current == state {
			return nil
		}
		if current != EnrollmentBegun && current != EnrollmentValidating && current != EnrollmentAwaitingConfirmation {
			return domain.NewError(domain.CodeConflict, "auth enrollment is terminal")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE auth_enrollments SET state=?, updated_at=? WHERE id=?`, state, formatTime(at), id); err != nil {
			return writeError("auth enrollment could not be finished", err)
		}
		var items int
		_ = tx.QueryRowContext(ctx, `SELECT count(*) FROM vault_items WHERE credential_instance_id=?`, credentialID).Scan(&items)
		if items == 0 {
			if _, err := tx.ExecContext(ctx, `UPDATE auth_enrollments SET credential_instance_id=NULL WHERE id=?`, id); err != nil {
				return err
			}
			_, _ = tx.ExecContext(ctx, `DELETE FROM credential_instances WHERE id=? AND provider='codex' AND status='unknown' AND credential_revision=1`, credentialID)
		}
		return nil
	})
	if err != nil {
		return AuthEnrollment{}, err
	}
	returnValue, err = s.AuthEnrollment(ctx, id)
	return returnValue, err
}

func (s *Store) ExpireAuthEnrollments(ctx context.Context, at time.Time) ([]string, error) {
	if at.IsZero() {
		return nil, domain.NewError(domain.CodeInvalidArgument, "auth enrollment recovery requires a timestamp")
	}
	var paths []string
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT id, credential_instance_id, staging_path FROM auth_enrollments WHERE state IN ('begun','validating','awaiting_confirmation')`)
		if err != nil {
			return domain.WrapError(domain.CodeConflict, "auth enrollments could not be read", err)
		}
		type expiredEnrollment struct {
			id           domain.ID
			credentialID domain.ID
			path         string
		}
		var values []expiredEnrollment
		for rows.Next() {
			var value expiredEnrollment
			if err := rows.Scan(&value.id, &value.credentialID, &value.path); err != nil {
				_ = rows.Close()
				return err
			}
			values = append(values, value)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE auth_enrollments SET state='expired', updated_at=? WHERE state IN ('begun','validating','awaiting_confirmation')`, formatTime(at)); err != nil {
			return writeError("auth enrollments could not be expired", err)
		}
		for _, value := range values {
			var items int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM vault_items WHERE credential_instance_id=?`, value.credentialID).Scan(&items); err != nil {
				return err
			}
			if items != 0 {
				continue
			}
			if _, err := tx.ExecContext(ctx, `UPDATE auth_enrollments SET credential_instance_id=NULL WHERE id=?`, value.id); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM credential_instances WHERE id=? AND provider='codex' AND status='unknown' AND credential_revision=1`, value.credentialID); err != nil {
				return err
			}
		}
		pathRows, err := tx.QueryContext(ctx, `SELECT staging_path FROM auth_enrollments ORDER BY id`)
		if err != nil {
			return domain.WrapError(domain.CodeConflict, "auth enrollment staging paths could not be read", err)
		}
		for pathRows.Next() {
			var path string
			if err := pathRows.Scan(&path); err != nil {
				_ = pathRows.Close()
				return err
			}
			paths = append(paths, path)
		}
		if err := pathRows.Close(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}

// ExpireDueAuthEnrollments clears deadline-expired operations without
// disturbing other profiles. It is called before a new begin so an abandoned
// enrollment cannot hold the one-active-per-profile index forever.
func (s *Store) ExpireDueAuthEnrollments(ctx context.Context, at time.Time) ([]string, error) {
	if at.IsZero() {
		return nil, domain.NewError(domain.CodeInvalidArgument, "auth enrollment expiry requires a timestamp")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, client_device_id, staging_path FROM auth_enrollments WHERE state IN ('begun','validating','awaiting_confirmation') AND expires_at<=? ORDER BY id`, formatTime(at))
	if err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "due auth enrollments could not be read", err)
	}
	type dueEnrollment struct {
		id       domain.ID
		clientID domain.ID
		path     string
	}
	var due []dueEnrollment
	for rows.Next() {
		var value dueEnrollment
		if err := rows.Scan(&value.id, &value.clientID, &value.path); err != nil {
			_ = rows.Close()
			return nil, err
		}
		due = append(due, value)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(due))
	for _, value := range due {
		if _, err := s.FinishAuthEnrollment(ctx, value.id, value.clientID, EnrollmentExpired, at); err != nil {
			return nil, err
		}
		paths = append(paths, value.path)
	}
	return paths, nil
}
