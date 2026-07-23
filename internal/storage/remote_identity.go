package storage

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

var remoteIdentityIDPattern = regexp.MustCompile(`^remote_identity_[0-9a-f]{32}$`)

type RemoteIdentityLifecycle string

const (
	RemoteIdentityPending RemoteIdentityLifecycle = "pending"
	RemoteIdentityActive  RemoteIdentityLifecycle = "active"
	RemoteIdentityRetired RemoteIdentityLifecycle = "retired"
)

type ControlPlaneIDMapping struct {
	EntityType string
	LocalID    string
	ServerID   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type RemoteDeviceIdentityRecord struct {
	ID                     string
	ServerOrigin           string
	ServerDeviceID         string
	SigningPublicKey       []byte
	ExchangePublicKey      []byte
	SigningKeyDigest       []byte
	ExchangeKeyDigest      []byte
	KeyRevision            int64
	RecordRevision         int64
	Lifecycle              RemoteIdentityLifecycle
	PayloadAlgorithm       string
	PayloadNonce           []byte
	PayloadCiphertext      []byte
	WrapAlgorithm          string
	WrapNonce              []byte
	WrappedDEK             []byte
	AADDigest              []byte
	PlaintextDigest        []byte
	BootstrapReceiptJSON   []byte
	BootstrapReceiptDigest []byte
	QuarantineReason       string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type RemoteIdentityReseal struct {
	ExpectedRecordRevision int64
	NextRecordRevision     int64
	Lifecycle              RemoteIdentityLifecycle
	PayloadNonce           []byte
	PayloadCiphertext      []byte
	WrapNonce              []byte
	WrappedDEK             []byte
	AADDigest              []byte
	PlaintextDigest        []byte
	BootstrapReceiptJSON   []byte
	BootstrapReceiptDigest []byte
	UpdatedAt              time.Time
}

func validRemoteIdentityRecord(value RemoteDeviceIdentityRecord) bool {
	if !remoteIdentityIDPattern.MatchString(value.ID) || value.KeyRevision != 1 || value.RecordRevision < 1 ||
		(value.Lifecycle != RemoteIdentityPending && value.Lifecycle != RemoteIdentityActive && value.Lifecycle != RemoteIdentityRetired) ||
		value.PayloadAlgorithm != "aes-256-gcm" || value.WrapAlgorithm != "aes-256-gcm" ||
		len(value.SigningPublicKey) != 32 || len(value.ExchangePublicKey) != 32 || len(value.SigningKeyDigest) != 32 || len(value.ExchangeKeyDigest) != 32 ||
		len(value.PayloadNonce) != 12 || len(value.PayloadCiphertext) < 17 || len(value.PayloadCiphertext) > 4112 ||
		len(value.WrapNonce) != 12 || len(value.WrappedDEK) != 48 || len(value.AADDigest) != 32 || len(value.PlaintextDigest) != 32 ||
		value.CreatedAt.IsZero() || value.UpdatedAt.IsZero() || value.UpdatedAt.Before(value.CreatedAt) || len(value.QuarantineReason) > 64 {
		return false
	}
	if _, err := transport.ParseCanonicalServerOriginV1(value.ServerOrigin, transport.CanonicalServerOriginOptions{AllowDevelopmentLocalhost: true}); err != nil {
		return false
	}
	if _, err := transport.ParseUUIDv7(value.ServerDeviceID); err != nil {
		return false
	}
	hasReceipt := len(value.BootstrapReceiptJSON) != 0 || len(value.BootstrapReceiptDigest) != 0
	if hasReceipt && (len(value.BootstrapReceiptJSON) < 2 || len(value.BootstrapReceiptJSON) > 4096 || len(value.BootstrapReceiptDigest) != 32) {
		return false
	}
	return value.Lifecycle != RemoteIdentityActive || hasReceipt
}

func validRemoteMapping(value ControlPlaneIDMapping) bool {
	if value.EntityType != "device" || !remoteIdentityIDPattern.MatchString(value.LocalID) || value.CreatedAt.IsZero() || value.UpdatedAt.IsZero() || value.UpdatedAt.Before(value.CreatedAt) {
		return false
	}
	_, err := transport.ParseUUIDv7(value.ServerID)
	return err == nil
}

// CreateRemoteDeviceIdentity commits the immutable server UUID mapping and
// encrypted envelope metadata in one BEGIN IMMEDIATE transaction. The mapping
// is inserted first to satisfy the migration's relation trigger; any later
// failure rolls both rows back.
func (s *Store) CreateRemoteDeviceIdentity(ctx context.Context, value RemoteDeviceIdentityRecord, mapping ControlPlaneIDMapping) error {
	if ctx == nil || !validRemoteIdentityRecord(value) || value.Lifecycle != RemoteIdentityPending || len(value.BootstrapReceiptJSON) != 0 || len(value.BootstrapReceiptDigest) != 0 || !validRemoteMapping(mapping) || mapping.LocalID != value.ID || mapping.ServerID != value.ServerDeviceID {
		return domain.NewError(domain.CodeInvalidArgument, "remote identity record is invalid")
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "remote identity connection could not be acquired", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return domain.WrapError(domain.CodeConflict, "remote identity transaction could not start", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	if _, err := conn.ExecContext(ctx, `INSERT INTO controlplane_id_mappings(entity_type,local_id,server_id,created_at,updated_at) VALUES('device',?,?,?,?)`,
		mapping.LocalID, mapping.ServerID, formatTime(mapping.CreatedAt), formatTime(mapping.UpdatedAt)); err != nil {
		return writeError("remote identity mapping could not be created", err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO remote_device_identities(
		id,server_origin,server_device_id,signing_public_key,exchange_public_key,
		signing_key_digest,exchange_key_digest,key_revision,record_revision,lifecycle,
		payload_algorithm,payload_nonce,payload_ciphertext,wrap_algorithm,wrap_nonce,
		wrapped_dek,aad_digest,plaintext_digest,bootstrap_receipt_json,
		bootstrap_receipt_digest,quarantine_reason,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,'aes-256-gcm',?,?,'aes-256-gcm',?,?,?,?,NULL,NULL,NULL,?,?)`,
		value.ID, value.ServerOrigin, value.ServerDeviceID, value.SigningPublicKey,
		value.ExchangePublicKey, value.SigningKeyDigest, value.ExchangeKeyDigest,
		value.KeyRevision, value.RecordRevision, value.Lifecycle, value.PayloadNonce,
		value.PayloadCiphertext, value.WrapNonce, value.WrappedDEK, value.AADDigest,
		value.PlaintextDigest, formatTime(value.CreatedAt), formatTime(value.UpdatedAt)); err != nil {
		return writeError("remote identity envelope could not be created", err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return domain.WrapError(domain.CodeConflict, "remote identity transaction could not commit", err)
	}
	committed = true
	return nil
}

func (s *Store) RemoteDeviceIdentity(ctx context.Context, id string) (RemoteDeviceIdentityRecord, error) {
	return scanRemoteIdentity(s.db.QueryRowContext(ctx, remoteIdentitySelect+" WHERE id=?", id))
}

func (s *Store) PendingRemoteDeviceIdentityForOrigin(ctx context.Context, serverOrigin string) (RemoteDeviceIdentityRecord, error) {
	return scanRemoteIdentity(s.db.QueryRowContext(ctx, remoteIdentitySelect+" WHERE server_origin=? AND lifecycle='pending' AND quarantine_reason IS NULL ORDER BY created_at LIMIT 1", serverOrigin))
}

func (s *Store) ActiveRemoteDeviceIdentityForOrigin(ctx context.Context, serverOrigin string) (RemoteDeviceIdentityRecord, error) {
	return scanRemoteIdentity(s.db.QueryRowContext(ctx, remoteIdentitySelect+" WHERE server_origin=? AND lifecycle='active' AND quarantine_reason IS NULL ORDER BY created_at DESC LIMIT 1", serverOrigin))
}

func (s *Store) ControlPlaneMapping(ctx context.Context, entityType, localID string) (ControlPlaneIDMapping, error) {
	var value ControlPlaneIDMapping
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `SELECT entity_type,local_id,server_id,created_at,updated_at FROM controlplane_id_mappings WHERE entity_type=? AND local_id=?`, entityType, localID).
		Scan(&value.EntityType, &value.LocalID, &value.ServerID, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ControlPlaneIDMapping{}, domain.NewError(domain.CodeNotFound, "control plane mapping was not found")
	}
	if err != nil {
		return ControlPlaneIDMapping{}, domain.WrapError(domain.CodeConflict, "control plane mapping could not be read", err)
	}
	value.CreatedAt, err = parseTime(createdAt)
	if err == nil {
		value.UpdatedAt, err = parseTime(updatedAt)
	}
	if err != nil || !validRemoteMapping(value) {
		return ControlPlaneIDMapping{}, domain.NewError(domain.CodeVaultCorrupt, "control plane mapping is invalid")
	}
	return value, nil
}

func (s *Store) ActivateRemoteDeviceIdentityCAS(ctx context.Context, id string, reseal RemoteIdentityReseal) error {
	if ctx == nil || !remoteIdentityIDPattern.MatchString(id) || reseal.ExpectedRecordRevision < 1 || reseal.NextRecordRevision != reseal.ExpectedRecordRevision+1 || reseal.Lifecycle != RemoteIdentityActive ||
		len(reseal.PayloadNonce) != 12 || len(reseal.PayloadCiphertext) < 17 || len(reseal.PayloadCiphertext) > 4112 || len(reseal.WrapNonce) != 12 || len(reseal.WrappedDEK) != 48 ||
		len(reseal.AADDigest) != 32 || len(reseal.PlaintextDigest) != 32 || len(reseal.BootstrapReceiptJSON) < 2 || len(reseal.BootstrapReceiptJSON) > 4096 || len(reseal.BootstrapReceiptDigest) != 32 || reseal.UpdatedAt.IsZero() {
		return domain.NewError(domain.CodeInvalidArgument, "remote identity activation is invalid")
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "remote identity activation connection could not be acquired", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return domain.WrapError(domain.CodeConflict, "remote identity activation transaction could not start", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	var currentLifecycle string
	var currentRevision int64
	var quarantine sql.NullString
	if err := conn.QueryRowContext(ctx, `SELECT lifecycle,record_revision,quarantine_reason FROM remote_device_identities WHERE id=?`, id).Scan(&currentLifecycle, &currentRevision, &quarantine); err != nil {
		return domain.NewError(domain.CodeNotFound, "remote identity was not found")
	}
	if currentLifecycle != string(RemoteIdentityPending) || currentRevision != reseal.ExpectedRecordRevision || quarantine.Valid {
		return domain.NewError(domain.CodeCredentialRevisionConflict, "remote identity record revision changed")
	}
	result, err := conn.ExecContext(ctx, `UPDATE remote_device_identities SET
		record_revision=?,lifecycle='active',payload_nonce=?,payload_ciphertext=?,
		wrap_nonce=?,wrapped_dek=?,aad_digest=?,plaintext_digest=?,
		bootstrap_receipt_json=?,bootstrap_receipt_digest=?,updated_at=?
		WHERE id=? AND record_revision=? AND lifecycle='pending' AND quarantine_reason IS NULL`,
		reseal.NextRecordRevision, reseal.PayloadNonce, reseal.PayloadCiphertext,
		reseal.WrapNonce, reseal.WrappedDEK, reseal.AADDigest, reseal.PlaintextDigest,
		string(reseal.BootstrapReceiptJSON), reseal.BootstrapReceiptDigest,
		formatTime(reseal.UpdatedAt), id, reseal.ExpectedRecordRevision)
	if err != nil {
		return writeError("remote identity activation could not be saved", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return domain.NewError(domain.CodeCredentialRevisionConflict, "remote identity record revision changed")
	}
	if _, err := conn.ExecContext(ctx, `UPDATE controlplane_id_mappings SET updated_at=? WHERE entity_type='device' AND local_id=?`, formatTime(reseal.UpdatedAt), id); err != nil {
		return writeError("remote identity mapping timestamp could not be updated", err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return domain.WrapError(domain.CodeConflict, "remote identity activation could not commit", err)
	}
	committed = true
	return nil
}

func (s *Store) QuarantineRemoteDeviceIdentity(ctx context.Context, id, reason string) error {
	if !remoteIdentityIDPattern.MatchString(id) || reason == "" || len(reason) > 64 {
		return domain.NewError(domain.CodeInvalidArgument, "remote identity quarantine input is invalid")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE remote_device_identities SET quarantine_reason=? WHERE id=? AND quarantine_reason IS NULL`, reason, id)
	if err != nil {
		return writeError("remote identity could not be quarantined", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return domain.NewError(domain.CodeQuarantined, "remote identity is already quarantined or missing")
	}
	return nil
}

const remoteIdentitySelect = `SELECT id,server_origin,server_device_id,
	signing_public_key,exchange_public_key,signing_key_digest,exchange_key_digest,
	key_revision,record_revision,lifecycle,payload_algorithm,payload_nonce,
	payload_ciphertext,wrap_algorithm,wrap_nonce,wrapped_dek,aad_digest,
	plaintext_digest,bootstrap_receipt_json,bootstrap_receipt_digest,
	quarantine_reason,created_at,updated_at FROM remote_device_identities`

type remoteIdentityScanner interface{ Scan(...any) error }

func scanRemoteIdentity(row remoteIdentityScanner) (RemoteDeviceIdentityRecord, error) {
	var value RemoteDeviceIdentityRecord
	var lifecycle, createdAt, updatedAt string
	var receiptJSON, receiptDigest []byte
	var quarantine sql.NullString
	err := row.Scan(&value.ID, &value.ServerOrigin, &value.ServerDeviceID,
		&value.SigningPublicKey, &value.ExchangePublicKey, &value.SigningKeyDigest,
		&value.ExchangeKeyDigest, &value.KeyRevision, &value.RecordRevision,
		&lifecycle, &value.PayloadAlgorithm, &value.PayloadNonce,
		&value.PayloadCiphertext, &value.WrapAlgorithm, &value.WrapNonce,
		&value.WrappedDEK, &value.AADDigest, &value.PlaintextDigest,
		&receiptJSON, &receiptDigest, &quarantine, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return RemoteDeviceIdentityRecord{}, domain.NewError(domain.CodeNotFound, "remote identity was not found")
	}
	if err != nil {
		return RemoteDeviceIdentityRecord{}, domain.WrapError(domain.CodeConflict, "remote identity could not be read", err)
	}
	value.Lifecycle = RemoteIdentityLifecycle(lifecycle)
	value.BootstrapReceiptJSON = append([]byte(nil), receiptJSON...)
	value.BootstrapReceiptDigest = append([]byte(nil), receiptDigest...)
	if quarantine.Valid {
		value.QuarantineReason = quarantine.String
	}
	value.CreatedAt, err = parseTime(createdAt)
	if err == nil {
		value.UpdatedAt, err = parseTime(updatedAt)
	}
	if err != nil || !validRemoteIdentityRecord(value) {
		return RemoteDeviceIdentityRecord{}, domain.NewError(domain.CodeVaultCorrupt, "remote identity metadata is invalid")
	}
	if value.QuarantineReason != "" {
		return RemoteDeviceIdentityRecord{}, domain.NewError(domain.CodeQuarantined, "remote identity is quarantined")
	}
	return value, nil
}
