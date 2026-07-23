package controlplane

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	generatedapi "github.com/jinlong17/multi-agent-desk/internal/controlplane/api/generated"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

const maxWebAuthnCeremonyPayload = 1 << 20

type CeremonyStore struct {
	Store *Store
	Now   func() time.Time
}

func (s *CeremonyStore) put(ctx context.Context, value *webAuthnCeremony) error {
	now, payload, err := s.validatePut(value)
	if err != nil {
		return err
	}
	_, err = withImmediateConn(ctx, s.Store.db, "WebAuthn ceremony", func(conn *sql.Conn) (struct{}, error) {
		return struct{}{}, putWebAuthnCeremonyTx(ctx, conn, value, payload, now)
	})
	return err
}

func (s *CeremonyStore) validatePut(value *webAuthnCeremony) (time.Time, []byte, error) {
	if s == nil || s.Store == nil || value == nil || value.ID == "" || value.ExpiresAt.IsZero() || !validCeremonyKind(value.Kind) {
		return time.Time{}, nil, fmt.Errorf("WebAuthn ceremony is invalid")
	}
	if _, err := transport.ParseUUIDv7(value.ID); err != nil {
		return time.Time{}, nil, fmt.Errorf("WebAuthn ceremony ID is invalid")
	}
	payload, err := json.Marshal(value)
	if err != nil || len(payload) < 2 || len(payload) > maxWebAuthnCeremonyPayload {
		return time.Time{}, nil, fmt.Errorf("WebAuthn ceremony payload is invalid")
	}
	now := normalizeServerTime(time.Now())
	if s.Now != nil {
		now = normalizeServerTime(s.Now())
	}
	return now, payload, nil
}

func (s *CeremonyStore) putTx(ctx context.Context, conn *sql.Conn, value *webAuthnCeremony) error {
	now, payload, err := s.validatePut(value)
	if err != nil {
		return err
	}
	return putWebAuthnCeremonyTx(ctx, conn, value, payload, now)
}

func putWebAuthnCeremonyTx(ctx context.Context, conn *sql.Conn, value *webAuthnCeremony, payload []byte, now time.Time) error {
	if conn == nil {
		return fmt.Errorf("WebAuthn ceremony transaction is required")
	}
	if _, err := conn.ExecContext(ctx, `DELETE FROM webauthn_ceremonies WHERE expires_at<=?`, formatServerTime(now)); err != nil {
		return err
	}
	var count int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM webauthn_ceremonies`).Scan(&count); err != nil {
		return err
	}
	if count >= maxWebAuthnCeremonies {
		return fmt.Errorf("WebAuthn ceremony capacity is exhausted")
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO webauthn_ceremonies(id,kind,payload_json,expires_at,created_at) VALUES(?,?,?,?,?)`, value.ID, string(value.Kind), payload, formatServerTime(value.ExpiresAt), formatServerTime(now)); err != nil {
		return fmt.Errorf("store WebAuthn ceremony: %w", err)
	}
	return nil
}

func (s *CeremonyStore) Load(ctx context.Context, id string, kind ceremonyKind, now time.Time) (*webAuthnCeremony, error) {
	if s == nil || s.Store == nil {
		return nil, fmt.Errorf("webauthn_challenge_replayed")
	}
	value, err := s.loadWith(ctx, s.Store.db, id, kind, now)
	if err != nil && (strings.Contains(err.Error(), "expired") || strings.Contains(err.Error(), "replayed")) {
		_ = s.Consume(context.Background(), id, kind)
	}
	return value, err
}

func (s *CeremonyStore) loadTx(ctx context.Context, conn *sql.Conn, id string, kind ceremonyKind, now time.Time) (*webAuthnCeremony, error) {
	if conn == nil {
		return nil, fmt.Errorf("webauthn_challenge_replayed")
	}
	return s.loadWith(ctx, conn, id, kind, now)
}

func (s *CeremonyStore) loadWith(ctx context.Context, queryer rowQueryer, id string, kind ceremonyKind, now time.Time) (*webAuthnCeremony, error) {
	if s == nil || s.Store == nil || !validCeremonyKind(kind) {
		return nil, fmt.Errorf("webauthn_challenge_replayed")
	}
	if _, err := transport.ParseUUIDv7(id); err != nil {
		return nil, fmt.Errorf("webauthn_challenge_replayed")
	}
	var storedKind, expiresText string
	var payload []byte
	err := queryer.QueryRowContext(ctx, `SELECT kind,payload_json,expires_at FROM webauthn_ceremonies WHERE id=?`, id).Scan(&storedKind, &payload, &expiresText)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("webauthn_challenge_replayed")
	}
	if err != nil {
		return nil, fmt.Errorf("read WebAuthn ceremony: %w", err)
	}
	if storedKind != string(kind) {
		return nil, fmt.Errorf("webauthn_challenge_replayed")
	}
	expires, err := parseServerTime(expiresText)
	if err != nil {
		return nil, fmt.Errorf("webauthn_challenge_replayed")
	}
	if !expires.After(now.UTC()) {
		return nil, fmt.Errorf("webauthn_challenge_expired")
	}
	var value webAuthnCeremony
	if err := json.Unmarshal(payload, &value); err != nil || value.ID != id || value.Kind != kind || !value.ExpiresAt.Equal(expires) {
		return nil, fmt.Errorf("webauthn_challenge_replayed")
	}
	return &value, nil
}

func (s *CeremonyStore) Consume(ctx context.Context, id string, kind ceremonyKind) error {
	if s == nil || s.Store == nil {
		return fmt.Errorf("webauthn_challenge_replayed")
	}
	result, err := s.Store.db.ExecContext(ctx, `DELETE FROM webauthn_ceremonies WHERE id=? AND kind=?`, id, string(kind))
	if err != nil {
		return err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return fmt.Errorf("webauthn_challenge_replayed")
	}
	return nil
}

// InvalidateAll is called once during server construction. Ceremony rows are
// restart-local because Bootstrap PoP private material is deliberately held in
// process memory only; no stored WebAuthn SessionData may outlive that boundary.
func (s *CeremonyStore) InvalidateAll(ctx context.Context) error {
	if s == nil || s.Store == nil {
		return fmt.Errorf("WebAuthn ceremony store is unavailable")
	}
	if _, err := s.Store.db.ExecContext(ctx, `DELETE FROM webauthn_ceremonies`); err != nil {
		return fmt.Errorf("invalidate restart-local WebAuthn ceremonies: %w", err)
	}
	return nil
}

func (s *CeremonyStore) consumeFailure(id string, kind ceremonyKind) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Consume(ctx, id, kind)
}

func (s *CeremonyStore) BootstrapChallenge(ctx context.Context, id string, now time.Time) (generatedapi.BootstrapAnchorChallengeV1, error) {
	value, err := s.Load(ctx, id, ceremonyBootstrapRegistration, now)
	if err != nil || value.BootstrapChallenge == nil {
		return generatedapi.BootstrapAnchorChallengeV1{}, fmt.Errorf("bootstrap ceremony was not found")
	}
	return *value.BootstrapChallenge, nil
}

func consumeWebAuthnCeremony(ctx context.Context, execer dbExecer, id string, kind ceremonyKind) error {
	result, err := execer.ExecContext(ctx, `DELETE FROM webauthn_ceremonies WHERE id=? AND kind=?`, id, string(kind))
	if err != nil {
		return fmt.Errorf("consume WebAuthn ceremony: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return fmt.Errorf("webauthn_challenge_replayed")
	}
	return nil
}

// consumeWebAuthnCeremonyIfPresent is used only while finalizing a failed
// outer transaction. The product savepoint may already have deleted the row,
// so both zero and one affected row are valid typed outcomes.
func consumeWebAuthnCeremonyIfPresent(ctx context.Context, execer dbExecer, id string, kind ceremonyKind) error {
	if _, err := transport.ParseUUIDv7(id); err != nil || !validCeremonyKind(kind) {
		return fmt.Errorf("WebAuthn ceremony binding is invalid")
	}
	result, err := execer.ExecContext(ctx, `DELETE FROM webauthn_ceremonies WHERE id=? AND kind=?`, id, string(kind))
	if err != nil {
		return fmt.Errorf("consume WebAuthn ceremony if present: %w", err)
	}
	if changed, rowsErr := result.RowsAffected(); rowsErr != nil || changed < 0 || changed > 1 {
		return fmt.Errorf("consume WebAuthn ceremony if present: invalid row count")
	}
	return nil
}

func validCeremonyKind(kind ceremonyKind) bool {
	switch kind {
	case ceremonyBootstrapRegistration, ceremonyPasskeyLogin, ceremonyPasskeyRegistration, ceremonyRecentUV:
		return true
	default:
		return false
	}
}
