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
	ErrSessionIntegrityInvalid = errors.New("session_integrity_invalid")
)

type SessionRevisionConflictError struct {
	SessionID        string
	ExpectedRevision int64
	CurrentRevision  int64
}

func (e *SessionRevisionConflictError) Error() string { return "session_revision_conflict" }

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
	if err := validatePasskeyAssertionCommit(input); err != nil {
		return BrowserSession{}, err
	}
	var securityErr error
	result, err := withImmediateConn(ctx, s.db, "passkey assertion", func(conn *sql.Conn) (BrowserSession, error) {
		value, txErr := commitPasskeyAssertionTx(ctx, conn, input)
		if errors.Is(txErr, ErrPasskeyCounterRegressed) {
			securityErr = txErr
			return value, nil
		}
		return value, txErr
	})
	if err != nil {
		return BrowserSession{}, err
	}
	return result, securityErr
}

func validatePasskeyAssertionCommit(input PasskeyAssertionCommit) error {
	if _, err := transport.ParseUUIDv7(input.CeremonyID); err != nil {
		return fmt.Errorf("passkey assertion ceremony is invalid")
	}
	if _, err := transport.ParseUUIDv7(input.PasskeyID); err != nil || input.ExpectedCredentialRevision < 1 || input.At.IsZero() || len(input.Credential.ID) == 0 {
		return fmt.Errorf("passkey assertion commit is invalid")
	}
	if !input.RecentUVOnly {
		if err := validateBrowserSessionCreate(input.NewSession, input.At); err != nil {
			return err
		}
	}
	return nil
}

func commitPasskeyAssertionTx(ctx context.Context, conn *sql.Conn, input PasskeyAssertionCommit) (BrowserSession, error) {
	if conn == nil {
		return BrowserSession{}, fmt.Errorf("passkey assertion transaction is required")
	}
	if err := validatePasskeyAssertionCommit(input); err != nil {
		return BrowserSession{}, err
	}
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

type rowsQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type readQueryer interface {
	rowQueryer
	rowsQueryer
}

const browserSessionSelect = `SELECT id,user_id,token_digest,csrf_digest,csrf_generation,authentication_method,authentication_passkey_id,authenticated_at,last_seen_at,recent_uv_at,expires_at,idle_expires_at,revoked_at,revision,activity_revision,created_at,updated_at FROM browser_sessions`

func browserSessionByID(ctx context.Context, queryer rowQueryer, id string, now time.Time) (BrowserSession, error) {
	return scanBrowserSession(queryer.QueryRowContext(ctx, browserSessionSelect+` WHERE id=?`, id), now)
}

func browserSessionByToken(ctx context.Context, queryer rowQueryer, digest []byte, now time.Time) (BrowserSession, error) {
	return scanBrowserSession(queryer.QueryRowContext(ctx, browserSessionSelect+` WHERE token_digest=?`, digest), now)
}

func scanBrowserSession(row interface{ Scan(...any) error }, now time.Time) (BrowserSession, error) {
	value, err := scanStoredBrowserSession(row)
	if err != nil {
		return BrowserSession{}, err
	}
	if value.RevokedAt != nil || !value.ExpiresAt.After(now.UTC()) || !value.IdleExpiresAt.After(now.UTC()) {
		return BrowserSession{}, fmt.Errorf("browser session is expired")
	}
	return value, nil
}

func scanStoredBrowserSession(row interface{ Scan(...any) error }) (BrowserSession, error) {
	var value BrowserSession
	var recentUV, revoked, passkey sql.NullString
	var authenticated, lastSeen, expires, idle, created, updated string
	if err := row.Scan(&value.ID, &value.UserID, &value.TokenDigest, &value.CSRFDigest, &value.CSRFGeneration, &value.AuthenticationMethod, &passkey, &authenticated, &lastSeen, &recentUV, &expires, &idle, &revoked, &value.Revision, &value.ActivityRevision, &created, &updated); err != nil {
		return BrowserSession{}, fmt.Errorf("browser session is invalid")
	}
	value.AuthenticationPasskeyID = passkey.String
	var err error
	for target, source := range map[*time.Time]string{&value.AuthenticatedAt: authenticated, &value.LastSeenAt: lastSeen, &value.ExpiresAt: expires, &value.IdleExpiresAt: idle, &value.CreatedAt: created, &value.UpdatedAt: updated} {
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
	return value, nil
}

func storedBrowserSessionByToken(ctx context.Context, queryer rowQueryer, digest []byte) (BrowserSession, error) {
	return scanStoredBrowserSession(queryer.QueryRowContext(ctx, browserSessionSelect+` WHERE token_digest=?`, digest))
}

func (s *Store) TouchBrowserSession(ctx context.Context, id string, expectedActivityRevision int64, now time.Time) (BrowserSession, error) {
	return touchBrowserSession(ctx, s.db, id, expectedActivityRevision, now)
}

type execQueryer interface {
	dbExecer
	rowQueryer
}

func touchBrowserSession(ctx context.Context, database execQueryer, id string, expectedActivityRevision int64, now time.Time) (BrowserSession, error) {
	if _, err := transport.ParseUUIDv7(id); err != nil || expectedActivityRevision < 1 || now.IsZero() {
		return BrowserSession{}, fmt.Errorf("browser session touch is invalid")
	}
	now = normalizeServerTime(now)
	nextIdle := now.Add(browserIdleLifetime)
	threshold := now.Add(-5 * time.Minute)
	result, err := database.ExecContext(ctx, `UPDATE browser_sessions SET last_seen_at=?,idle_expires_at=min(expires_at,?),activity_revision=activity_revision+1,updated_at=? WHERE id=? AND activity_revision=? AND last_seen_at<=? AND revoked_at IS NULL AND expires_at>? AND idle_expires_at>?`, formatServerTime(now), formatServerTime(nextIdle), formatServerTime(now), id, expectedActivityRevision, formatServerTime(threshold), formatServerTime(now), formatServerTime(now))
	if err != nil {
		return BrowserSession{}, fmt.Errorf("touch browser session: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed > 1 {
		return BrowserSession{}, fmt.Errorf("touch browser session affected multiple rows")
	}
	// Both a coalesced no-op and a CAS loser reload the authoritative row. A
	// concurrent revoke/expiry therefore fails closed; a concurrent touch winner
	// leaves the caller on the same state revision and permits the request.
	return browserSessionByID(ctx, database, id, now)
}

type BrowserSessionRevoke struct {
	SessionID string
	RevokedAt time.Time
	Revision  int64
}

func (s *Store) RevokeBrowserSession(ctx context.Context, id string, expectedRevision int64, now time.Time) (BrowserSessionRevoke, error) {
	return revokeBrowserSession(ctx, s.db, id, expectedRevision, now)
}

func revokeBrowserSession(ctx context.Context, database execQueryer, id string, expectedRevision int64, now time.Time) (BrowserSessionRevoke, error) {
	result, err := database.ExecContext(ctx, `UPDATE browser_sessions SET revoked_at=?,revision=revision+1,updated_at=? WHERE id=? AND revision=? AND revoked_at IS NULL`, formatServerTime(now), formatServerTime(now), id, expectedRevision)
	if err != nil {
		return BrowserSessionRevoke{}, fmt.Errorf("revoke browser session: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		var currentRevision int64
		if scanErr := database.QueryRowContext(ctx, `SELECT revision FROM browser_sessions WHERE id=?`, id).Scan(&currentRevision); scanErr != nil {
			return BrowserSessionRevoke{}, ErrRevisionChanged
		}
		return BrowserSessionRevoke{}, &SessionRevisionConflictError{SessionID: id, ExpectedRevision: expectedRevision, CurrentRevision: currentRevision}
	}
	return BrowserSessionRevoke{SessionID: id, RevokedAt: normalizeServerTime(now), Revision: expectedRevision + 1}, nil
}

func (s *Store) RevokeSessionIntegrityFailure(ctx context.Context, id string, expectedRevision int64, now time.Time) error {
	return revokeSessionIntegrityFailure(ctx, s.db, id, expectedRevision, now)
}

func revokeSessionIntegrityFailure(ctx context.Context, database execQueryer, id string, expectedRevision int64, now time.Time) error {
	if _, err := transport.ParseUUIDv7(id); err != nil || expectedRevision < 1 || now.IsZero() {
		return fmt.Errorf("browser session integrity revocation is invalid")
	}
	result, err := database.ExecContext(ctx, `UPDATE browser_sessions SET revoked_at=?,revision=revision+1,updated_at=? WHERE id=? AND revision=? AND revoked_at IS NULL`, formatServerTime(now), formatServerTime(now), id, expectedRevision)
	if err != nil {
		return fmt.Errorf("revoke corrupt browser session: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 1 {
		return nil
	}
	// A concurrent state transition cannot restore authority. Reload without
	// the live-session filter and require that the row is already revoked or no
	// longer valid; either way the caller still returns integrity-invalid.
	var revoked sql.NullString
	var revision int64
	var expires, idle string
	if err := database.QueryRowContext(ctx, `SELECT revoked_at,revision,expires_at,idle_expires_at FROM browser_sessions WHERE id=?`, id).Scan(&revoked, &revision, &expires, &idle); err != nil {
		return fmt.Errorf("revoke corrupt browser session: %w", err)
	}
	expiresAt, expiresErr := parseServerTime(expires)
	idleAt, idleErr := parseServerTime(idle)
	if revoked.Valid || expiresErr == nil && idleErr == nil && (!expiresAt.After(now.UTC()) || !idleAt.After(now.UTC())) {
		return nil
	}
	return fmt.Errorf("revoke corrupt browser session after revision %d", revision)
}

func (s *Store) ListBrowserSessions(ctx context.Context, userID string, now time.Time) ([]BrowserSession, error) {
	rows, err := s.db.QueryContext(ctx, browserSessionSelect+` WHERE user_id=? AND revoked_at IS NULL AND expires_at>? AND idle_expires_at>? ORDER BY created_at DESC,id`, userID, formatServerTime(now), formatServerTime(now))
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
	return withImmediateConn(ctx, s.db, "passkey delete", func(conn *sql.Conn) (PasskeyDeleteResult, error) {
		return deletePasskeyCASTx(ctx, conn, userID, passkeyID, currentSessionID, expectedRevision, now)
	})
}

func deletePasskeyCASTx(ctx context.Context, conn *sql.Conn, userID, passkeyID, currentSessionID string, expectedRevision int64, now time.Time) (PasskeyDeleteResult, error) {
	if conn == nil {
		return PasskeyDeleteResult{}, fmt.Errorf("passkey delete transaction is required")
	}
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
