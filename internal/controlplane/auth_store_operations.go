package controlplane

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

var (
	ErrPasskeyCounterRegressed = errors.New("passkey_counter_regressed")
	ErrLastPasskeyRequired     = errors.New("last_passkey_required")
	ErrRevisionChanged         = errors.New("revision_changed")
)

type PasskeyAssertionCommit struct {
	CeremonyID                 string
	PasskeyID                  string
	ExpectedCredentialRevision int64
	ObservedSignCount          uint32
	Credential                 webauthn.Credential
	NewSession                 BrowserSessionCreate
	RevokeSessionID            string
	RecentUVOnly               bool
	At                         time.Time
}

// CommitPasskeyAssertion applies the authenticator counter CAS and the
// resulting browser authority in one BEGIN IMMEDIATE transaction. A non-zero
// counter regression commits only revocation of sessions derived from the
// suspect credential and returns ErrPasskeyCounterRegressed.
func (s *Store) CommitPasskeyAssertion(ctx context.Context, input PasskeyAssertionCommit) (BrowserSession, error) {
	if _, err := transport.ParseUUIDv7(input.CeremonyID); err != nil {
		return BrowserSession{}, fmt.Errorf("passkey assertion ceremony is invalid")
	}
	if _, err := transport.ParseUUIDv7(input.PasskeyID); err != nil || input.ExpectedCredentialRevision < 1 || input.At.IsZero() || len(input.Credential.ID) == 0 {
		return BrowserSession{}, fmt.Errorf("passkey assertion commit is invalid")
	}
	if !input.RecentUVOnly {
		if err := validateBrowserSessionCreate(input.NewSession, input.At); err != nil {
			return BrowserSession{}, err
		}
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return BrowserSession{}, fmt.Errorf("acquire passkey assertion connection: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return BrowserSession{}, fmt.Errorf("begin passkey assertion: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	kind := ceremonyPasskeyLogin
	if input.RevokeSessionID != "" {
		kind = ceremonyRecentUV
	}
	if err := consumeWebAuthnCeremony(ctx, conn, input.CeremonyID, kind); err != nil {
		return BrowserSession{}, err
	}
	var storedRevision int64
	var storedCount uint32
	var active int
	if err := conn.QueryRowContext(ctx, "SELECT credential_revision,sign_count,active FROM passkeys WHERE id=?", input.PasskeyID).Scan(&storedRevision, &storedCount, &active); err != nil || active != 1 {
		return BrowserSession{}, fmt.Errorf("passkey is unavailable")
	}
	// BEGIN IMMEDIATE gives this writer the latest committed counter. A caller
	// that loaded an older revision must re-evaluate its observed counter here,
	// rather than fail a blind CAS and accidentally skip clone detection.
	if storedCount != 0 && input.ObservedSignCount <= storedCount {
		if _, err := conn.ExecContext(ctx, `UPDATE browser_sessions SET revoked_at=?,revision=revision+1,updated_at=? WHERE authentication_passkey_id=? AND revoked_at IS NULL`, formatServerTime(input.At), formatServerTime(input.At), input.PasskeyID); err != nil {
			return BrowserSession{}, fmt.Errorf("revoke cloned-passkey sessions: %w", err)
		}
		if err := insertAuthAudit(ctx, conn, "browser", "passkey.authenticate", "denied", "passkey_counter_regressed", input.PasskeyID, "", input.At); err != nil {
			return BrowserSession{}, err
		}
		if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
			return BrowserSession{}, fmt.Errorf("commit cloned-passkey revocation: %w", err)
		}
		committed = true
		return BrowserSession{}, ErrPasskeyCounterRegressed
	}
	input.Credential.Authenticator.SignCount = input.ObservedSignCount
	input.Credential.Authenticator.CloneWarning = false
	credentialJSON, err := json.Marshal(input.Credential)
	if err != nil || len(credentialJSON) > 1<<20 {
		return BrowserSession{}, fmt.Errorf("updated passkey is invalid")
	}
	result, err := conn.ExecContext(ctx, `UPDATE passkeys SET credential_json=?,sign_count=?,credential_revision=credential_revision+1,last_used_at=?,updated_at=? WHERE id=? AND credential_revision=? AND active=1`, credentialJSON, input.ObservedSignCount, formatServerTime(input.At), formatServerTime(input.At), input.PasskeyID, storedRevision)
	if err != nil {
		return BrowserSession{}, fmt.Errorf("update passkey assertion counter: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return BrowserSession{}, ErrRevisionChanged
	}
	if input.RecentUVOnly {
		if _, err := conn.ExecContext(ctx, `UPDATE browser_sessions SET recent_uv_at=?,revision=revision+1,updated_at=? WHERE id=? AND revoked_at IS NULL AND expires_at>? AND idle_expires_at>?`, formatServerTime(input.At), formatServerTime(input.At), input.RevokeSessionID, formatServerTime(input.At), formatServerTime(input.At)); err != nil {
			return BrowserSession{}, fmt.Errorf("record recent user verification: %w", err)
		}
	} else {
		if input.RevokeSessionID != "" {
			if _, err := conn.ExecContext(ctx, `UPDATE browser_sessions SET revoked_at=?,revision=revision+1,updated_at=? WHERE id=? AND revoked_at IS NULL`, formatServerTime(input.At), formatServerTime(input.At), input.RevokeSessionID); err != nil {
				return BrowserSession{}, fmt.Errorf("rotate prior browser session: %w", err)
			}
		}
		if err := insertBrowserSession(ctx, conn, input.NewSession, input.At); err != nil {
			return BrowserSession{}, err
		}
	}
	if err := insertAuthAudit(ctx, conn, "browser", "passkey.authenticate", "allowed", "passkey_verified", input.PasskeyID, input.NewSession.UserID, input.At); err != nil {
		return BrowserSession{}, err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return BrowserSession{}, fmt.Errorf("commit passkey assertion: %w", err)
	}
	committed = true
	if input.RecentUVOnly {
		return browserSessionByID(ctx, conn, input.RevokeSessionID, input.At)
	}
	digest := sha256.Sum256(input.NewSession.RawToken)
	return browserSessionByToken(ctx, conn, digest[:], input.At)
}

func (s *Store) CreateBrowserSession(ctx context.Context, value BrowserSessionCreate, now time.Time) (BrowserSession, error) {
	if err := insertBrowserSession(ctx, s.db, value, now); err != nil {
		return BrowserSession{}, err
	}
	return s.BrowserSessionByToken(ctx, value.RawToken, now)
}

func (s *Store) BrowserSessionByID(ctx context.Context, id string, now time.Time) (BrowserSession, error) {
	if _, err := transport.ParseUUIDv7(id); err != nil {
		return BrowserSession{}, fmt.Errorf("browser session is invalid")
	}
	return browserSessionByID(ctx, s.db, id, now)
}

type rowQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const browserSessionSelect = `SELECT id,user_id,token_digest,csrf_digest,authentication_method,authentication_passkey_id,authenticated_at,recent_uv_at,expires_at,idle_expires_at,revoked_at,revision,created_at,updated_at FROM browser_sessions`

func browserSessionByID(ctx context.Context, queryer rowQueryer, id string, now time.Time) (BrowserSession, error) {
	return scanBrowserSession(queryer.QueryRowContext(ctx, browserSessionSelect+` WHERE id=?`, id), now)
}

func browserSessionByToken(ctx context.Context, queryer rowQueryer, digest []byte, now time.Time) (BrowserSession, error) {
	return scanBrowserSession(queryer.QueryRowContext(ctx, browserSessionSelect+` WHERE token_digest=?`, digest), now)
}

func scanBrowserSession(row interface{ Scan(...any) error }, now time.Time) (BrowserSession, error) {
	var value BrowserSession
	var recentUV, revoked, passkey sql.NullString
	var authenticated, expires, idle, created, updated string
	if err := row.Scan(&value.ID, &value.UserID, &value.TokenDigest, &value.CSRFDigest, &value.AuthenticationMethod, &passkey, &authenticated, &recentUV, &expires, &idle, &revoked, &value.Revision, &created, &updated); err != nil {
		return BrowserSession{}, fmt.Errorf("browser session is invalid")
	}
	value.AuthenticationPasskeyID = passkey.String
	var err error
	for target, source := range map[*time.Time]string{&value.AuthenticatedAt: authenticated, &value.ExpiresAt: expires, &value.IdleExpiresAt: idle, &value.CreatedAt: created, &value.UpdatedAt: updated} {
		if *target, err = parseServerTime(source); err != nil {
			return BrowserSession{}, fmt.Errorf("browser session is corrupt")
		}
	}
	if recentUV.Valid {
		parsed, err := parseServerTime(recentUV.String)
		if err != nil {
			return BrowserSession{}, fmt.Errorf("browser session is corrupt")
		}
		value.RecentUVAt = &parsed
	}
	if revoked.Valid {
		parsed, err := parseServerTime(revoked.String)
		if err != nil {
			return BrowserSession{}, fmt.Errorf("browser session is corrupt")
		}
		value.RevokedAt = &parsed
	}
	if value.RevokedAt != nil || !value.ExpiresAt.After(now.UTC()) || !value.IdleExpiresAt.After(now.UTC()) {
		return BrowserSession{}, fmt.Errorf("browser session is expired")
	}
	return value, nil
}

func (s *Store) TouchBrowserSession(ctx context.Context, id string, expectedRevision int64, now time.Time) (int64, error) {
	nextIdle := now.UTC().Add(browserIdleLifetime)
	result, err := s.db.ExecContext(ctx, `UPDATE browser_sessions SET idle_expires_at=min(expires_at,?),revision=revision+1,updated_at=? WHERE id=? AND revision=? AND revoked_at IS NULL AND expires_at>? AND idle_expires_at>?`, formatServerTime(nextIdle), formatServerTime(now), id, expectedRevision, formatServerTime(now), formatServerTime(now))
	if err != nil {
		return 0, fmt.Errorf("touch browser session: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return 0, ErrRevisionChanged
	}
	return expectedRevision + 1, nil
}

func (s *Store) RevokeBrowserSession(ctx context.Context, id string, expectedRevision int64, now time.Time) (bool, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE browser_sessions SET revoked_at=?,revision=revision+1,updated_at=? WHERE id=? AND revision=? AND revoked_at IS NULL`, formatServerTime(now), formatServerTime(now), id, expectedRevision)
	if err != nil {
		return false, fmt.Errorf("revoke browser session: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return false, ErrRevisionChanged
	}
	return true, nil
}

func (s *Store) ListBrowserSessions(ctx context.Context, userID string, now time.Time) ([]BrowserSession, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,token_digest,csrf_digest,authentication_method,authentication_passkey_id,authenticated_at,recent_uv_at,expires_at,idle_expires_at,revoked_at,revision,created_at,updated_at FROM browser_sessions WHERE user_id=? AND revoked_at IS NULL AND expires_at>? AND idle_expires_at>? ORDER BY created_at DESC,id`, userID, formatServerTime(now), formatServerTime(now))
	if err != nil {
		return nil, fmt.Errorf("list browser sessions: %w", err)
	}
	defer rows.Close()
	var result []BrowserSession
	for rows.Next() {
		value, err := scanBrowserSession(rows, now)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *Store) ListPasskeys(ctx context.Context, userID string) ([]StoredPasskey, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,credential_json,name,transports_json,sign_count,credential_revision,active,created_at,last_used_at,updated_at FROM passkeys WHERE user_id=? AND active=1 ORDER BY created_at,id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list passkeys: %w", err)
	}
	defer rows.Close()
	var result []StoredPasskey
	for rows.Next() {
		value, err := scanStoredPasskey(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

type PasskeyDeleteResult struct {
	RevokedSessionCount   int
	CurrentSessionRevoked bool
}

func (s *Store) DeletePasskeyCAS(ctx context.Context, userID, passkeyID, currentSessionID string, expectedRevision int64, now time.Time) (PasskeyDeleteResult, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return PasskeyDeleteResult{}, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return PasskeyDeleteResult{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	var activeCount int
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM passkeys WHERE user_id=? AND active=1", userID).Scan(&activeCount); err != nil {
		return PasskeyDeleteResult{}, err
	}
	if activeCount <= 1 {
		return PasskeyDeleteResult{}, ErrLastPasskeyRequired
	}
	result, err := conn.ExecContext(ctx, `UPDATE passkeys SET active=0,credential_revision=credential_revision+1,updated_at=? WHERE id=? AND user_id=? AND credential_revision=? AND active=1`, formatServerTime(now), passkeyID, userID, expectedRevision)
	if err != nil {
		return PasskeyDeleteResult{}, err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return PasskeyDeleteResult{}, ErrRevisionChanged
	}
	var current int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM browser_sessions WHERE id=? AND authentication_passkey_id=? AND revoked_at IS NULL`, currentSessionID, passkeyID).Scan(&current); err != nil {
		return PasskeyDeleteResult{}, err
	}
	revokedResult, err := conn.ExecContext(ctx, `UPDATE browser_sessions SET revoked_at=?,revision=revision+1,updated_at=? WHERE authentication_passkey_id=? AND revoked_at IS NULL`, formatServerTime(now), formatServerTime(now), passkeyID)
	if err != nil {
		return PasskeyDeleteResult{}, err
	}
	revoked, _ := revokedResult.RowsAffected()
	if err := insertAuthAudit(ctx, conn, "browser", "passkey.delete", "allowed", "passkey_deleted", passkeyID, userID, now); err != nil {
		return PasskeyDeleteResult{}, err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return PasskeyDeleteResult{}, err
	}
	committed = true
	return PasskeyDeleteResult{RevokedSessionCount: int(revoked), CurrentSessionRevoked: current == 1}, nil
}

func insertAuthAudit(ctx context.Context, execer dbExecer, actorClass, action, decision, reasonCode, targetID, userID string, at time.Time) error {
	id, err := transport.NewUUIDv7()
	if err != nil {
		return err
	}
	if _, err := execer.ExecContext(ctx, `INSERT INTO auth_audit_events(id,user_id,actor_class,action,decision,reason_code,target_id,created_at) VALUES(?,?,?,?,?,?,?,?)`, id, nullString(userID), actorClass, action, decision, reasonCode, nullString(targetID), formatServerTime(at)); err != nil {
		return fmt.Errorf("store authentication audit: %w", err)
	}
	return nil
}

func sessionTokenDigest(raw []byte) []byte {
	digest := sha256.Sum256(raw)
	return digest[:]
}
