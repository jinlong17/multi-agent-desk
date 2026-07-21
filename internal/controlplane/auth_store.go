package controlplane

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

const (
	bootstrapTokenLifetime  = 10 * time.Minute
	browserAbsoluteLifetime = 12 * time.Hour
	browserIdleLifetime     = 30 * time.Minute
	recoverySessionLifetime = 15 * time.Minute
)

type BootstrapState struct {
	Initialized bool
	InProgress  bool
	ExpiresAt   *time.Time
}

type StoredUser struct {
	ID          string
	Handle      []byte
	DisplayName string
	Revision    int64
	Credentials []webauthn.Credential
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (u StoredUser) WebAuthnID() []byte          { return append([]byte(nil), u.Handle...) }
func (u StoredUser) WebAuthnName() string        { return u.DisplayName }
func (u StoredUser) WebAuthnDisplayName() string { return u.DisplayName }
func (u StoredUser) WebAuthnCredentials() []webauthn.Credential {
	return append([]webauthn.Credential(nil), u.Credentials...)
}

type StoredPasskey struct {
	ID                 string
	UserID             string
	Credential         webauthn.Credential
	Name               string
	TransportsJSON     []byte
	SignCount          uint32
	CredentialRevision int64
	Active             bool
	CreatedAt          time.Time
	LastUsedAt         *time.Time
	UpdatedAt          time.Time
}

type RecoveryCodeHash struct {
	ID      string
	Ordinal int
	Salt    []byte
	Hash    []byte
}

type BootstrapCommitInput struct {
	TokenDigest            [sha256.Size]byte
	CeremonyID             string
	User                   StoredUser
	Passkey                StoredPasskey
	AnchorDeviceID         string
	AnchorName             string
	AnchorPlatform         string
	AnchorArchitecture     string
	AnchorClientVersion    string
	SigningPublicKey       []byte
	ExchangePublicKey      []byte
	SigningKeyDigest       []byte
	ExchangeKeyDigest      []byte
	PinDigest              []byte
	StorageAssertionJSON   []byte
	StorageAssertionDigest []byte
	CapabilitiesJSON       []byte
	RecoveryBatchID        string
	RecoveryCodes          []RecoveryCodeHash
	BrowserSession         BrowserSessionCreate
	ReceiptJSON            []byte
	ReceiptDigest          []byte
	At                     time.Time
}

type BrowserSessionCreate struct {
	ID                      string
	UserID                  string
	RawToken                []byte
	RawCSRF                 []byte
	AuthenticationMethod    string
	AuthenticationPasskeyID string
	AuthenticatedAt         time.Time
	RecentUVAt              *time.Time
	ExpiresAt               time.Time
	IdleExpiresAt           time.Time
}

type BrowserSession struct {
	ID                      string
	UserID                  string
	TokenDigest             []byte
	CSRFDigest              []byte
	AuthenticationMethod    string
	AuthenticationPasskeyID string
	AuthenticatedAt         time.Time
	RecentUVAt              *time.Time
	ExpiresAt               time.Time
	IdleExpiresAt           time.Time
	RevokedAt               *time.Time
	Revision                int64
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

func (s *Store) EnsureBootstrapToken(ctx context.Context, now time.Time) (string, bool, error) {
	material, err := randomFixed(32)
	if err != nil {
		return "", false, err
	}
	defer zeroBytes(material)
	digest := sha256.Sum256(material)
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return "", false, fmt.Errorf("acquire bootstrap token connection: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return "", false, fmt.Errorf("begin bootstrap token transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	var users, states int
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&users); err != nil {
		return "", false, fmt.Errorf("read bootstrap users: %w", err)
	}
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM bootstrap_state").Scan(&states); err != nil {
		return "", false, fmt.Errorf("read bootstrap state: %w", err)
	}
	if users != 0 || states != 0 {
		if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
			return "", false, fmt.Errorf("commit bootstrap state read: %w", err)
		}
		committed = true
		return "", false, nil
	}
	expires := now.UTC().Add(bootstrapTokenLifetime)
	if _, err := conn.ExecContext(ctx, `INSERT INTO bootstrap_state(singleton,token_digest,token_expires_at,revision,created_at,updated_at) VALUES(1,?,?,1,?,?)`, digest[:], formatServerTime(expires), formatServerTime(now), formatServerTime(now)); err != nil {
		return "", false, fmt.Errorf("store bootstrap token digest: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return "", false, fmt.Errorf("commit bootstrap token: %w", err)
	}
	committed = true
	return base64.RawURLEncoding.EncodeToString(material), true, nil
}

func (s *Store) RotateBootstrapToken(ctx context.Context, now time.Time) (string, error) {
	material, err := randomFixed(32)
	if err != nil {
		return "", err
	}
	defer zeroBytes(material)
	digest := sha256.Sum256(material)
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return "", fmt.Errorf("acquire bootstrap rotation connection: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN EXCLUSIVE"); err != nil {
		return "", fmt.Errorf("bootstrap rotation requires the server to be stopped: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	var users, anchors int
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&users); err != nil {
		return "", err
	}
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM anchor_devices WHERE lifecycle='active'").Scan(&anchors); err != nil {
		return "", err
	}
	if users != 0 || anchors != 0 {
		return "", fmt.Errorf("bootstrap token cannot be rotated after initialization")
	}
	if _, err := conn.ExecContext(ctx, `DELETE FROM webauthn_ceremonies`); err != nil {
		return "", fmt.Errorf("expire incomplete WebAuthn ceremonies: %w", err)
	}
	expires := now.UTC().Add(bootstrapTokenLifetime)
	if _, err := conn.ExecContext(ctx, `INSERT INTO bootstrap_state(singleton,token_digest,token_expires_at,revision,created_at,updated_at)
		VALUES(1,?,?,1,?,?) ON CONFLICT(singleton) DO UPDATE SET token_digest=excluded.token_digest,
		token_expires_at=excluded.token_expires_at,revision=bootstrap_state.revision+1,updated_at=excluded.updated_at`,
		digest[:], formatServerTime(expires), formatServerTime(now), formatServerTime(now)); err != nil {
		return "", fmt.Errorf("rotate bootstrap token digest: %w", err)
	}
	auditID, err := transport.NewUUIDv7()
	if err != nil {
		return "", err
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO pre_user_audit_events(id,action,decision,error_code,request_id,created_at) VALUES(?,'bootstrap.token.rotate','allowed',NULL,NULL,?)`, auditID, formatServerTime(now)); err != nil {
		return "", fmt.Errorf("store bootstrap rotation audit: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return "", fmt.Errorf("commit bootstrap token rotation: %w", err)
	}
	committed = true
	return base64.RawURLEncoding.EncodeToString(material), nil
}

func (s *Store) BootstrapState(ctx context.Context, now time.Time) (BootstrapState, error) {
	var users int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&users); err != nil {
		return BootstrapState{}, fmt.Errorf("read bootstrap user count: %w", err)
	}
	if users != 0 {
		return BootstrapState{Initialized: true}, nil
	}
	var digest []byte
	var expiresText sql.NullString
	err := s.db.QueryRowContext(ctx, "SELECT token_digest,token_expires_at FROM bootstrap_state WHERE singleton=1").Scan(&digest, &expiresText)
	if errors.Is(err, sql.ErrNoRows) {
		return BootstrapState{}, nil
	}
	if err != nil {
		return BootstrapState{}, fmt.Errorf("read bootstrap state: %w", err)
	}
	if len(digest) != 32 || !expiresText.Valid {
		return BootstrapState{}, fmt.Errorf("bootstrap state is corrupt")
	}
	expires, err := parseServerTime(expiresText.String)
	if err != nil {
		return BootstrapState{}, fmt.Errorf("bootstrap expiry is corrupt")
	}
	return BootstrapState{InProgress: expires.After(now.UTC()), ExpiresAt: &expires}, nil
}

func (s *Store) ValidateBootstrapToken(ctx context.Context, plaintext string, now time.Time) ([sha256.Size]byte, error) {
	var result [sha256.Size]byte
	decoded, err := base64.RawURLEncoding.DecodeString(plaintext)
	if err != nil || len(decoded) != 32 || base64.RawURLEncoding.EncodeToString(decoded) != plaintext {
		return result, fmt.Errorf("bootstrap token is invalid")
	}
	defer zeroBytes(decoded)
	digest := sha256.Sum256(decoded)
	var stored []byte
	var expiresText string
	if err := s.db.QueryRowContext(ctx, "SELECT token_digest,token_expires_at FROM bootstrap_state WHERE singleton=1").Scan(&stored, &expiresText); err != nil {
		return result, fmt.Errorf("bootstrap token is unavailable")
	}
	expires, err := parseServerTime(expiresText)
	if err != nil || !expires.After(now.UTC()) || len(stored) != 32 || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return result, fmt.Errorf("bootstrap token is invalid")
	}
	return digest, nil
}

func (s *Store) CommitBootstrap(ctx context.Context, input BootstrapCommitInput) error {
	if err := validateBootstrapCommitInput(input); err != nil {
		return err
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire bootstrap commit connection: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin bootstrap commit: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	var userCount int
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&userCount); err != nil || userCount != 0 {
		return fmt.Errorf("server is already initialized")
	}
	var storedDigest []byte
	var tokenExpiry string
	if err := conn.QueryRowContext(ctx, "SELECT token_digest,token_expires_at FROM bootstrap_state WHERE singleton=1").Scan(&storedDigest, &tokenExpiry); err != nil {
		return fmt.Errorf("bootstrap token is unavailable")
	}
	expires, err := parseServerTime(tokenExpiry)
	if err != nil || !expires.After(input.At) || subtle.ConstantTimeCompare(storedDigest, input.TokenDigest[:]) != 1 {
		return fmt.Errorf("bootstrap token is expired or changed")
	}
	if err := consumeWebAuthnCeremony(ctx, conn, input.CeremonyID, ceremonyBootstrapRegistration); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO users(id,singleton,user_handle,display_name,revision,created_at,updated_at) VALUES(?,1,?,?,1,?,?)`, input.User.ID, input.User.Handle, input.User.DisplayName, formatServerTime(input.At), formatServerTime(input.At)); err != nil {
		return fmt.Errorf("create bootstrap user: %w", err)
	}
	credentialJSON, err := json.Marshal(input.Passkey.Credential)
	if err != nil {
		return fmt.Errorf("encode bootstrap passkey: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO passkeys(id,user_id,credential_id,credential_json,name,transports_json,sign_count,credential_revision,active,created_at,last_used_at,updated_at)
		VALUES(?,?,?,?,?,?,?,1,1,?,NULL,?)`, input.Passkey.ID, input.User.ID, input.Passkey.Credential.ID, credentialJSON, input.Passkey.Name, string(input.Passkey.TransportsJSON), input.Passkey.SignCount, formatServerTime(input.At), formatServerTime(input.At)); err != nil {
		return fmt.Errorf("create bootstrap passkey: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO anchor_devices(id,kind,name,platform,architecture,client_version,signing_public_key,exchange_public_key,signing_key_digest,exchange_key_digest,pin_digest,storage_mode,storage_assertion_json,storage_assertion_digest,capabilities_json,lifecycle,key_revision,revision,created_at,updated_at)
		VALUES(?,'daemon',?,?,?,?,?,?,?,?,?,'portable_vault_v1',?,?,?,'active',1,1,?,?)`,
		input.AnchorDeviceID, input.AnchorName, input.AnchorPlatform, input.AnchorArchitecture, input.AnchorClientVersion,
		input.SigningPublicKey, input.ExchangePublicKey, input.SigningKeyDigest, input.ExchangeKeyDigest, input.PinDigest,
		string(input.StorageAssertionJSON), input.StorageAssertionDigest, string(input.CapabilitiesJSON), formatServerTime(input.At), formatServerTime(input.At)); err != nil {
		return fmt.Errorf("create bootstrap anchor: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO recovery_batches(id,user_id,status,created_at,invalidated_at) VALUES(?,?,'active',?,NULL)`, input.RecoveryBatchID, input.User.ID, formatServerTime(input.At)); err != nil {
		return fmt.Errorf("create recovery batch: %w", err)
	}
	for _, code := range input.RecoveryCodes {
		if _, err := conn.ExecContext(ctx, `INSERT INTO recovery_codes(id,batch_id,user_id,ordinal,salt,code_hash,status,created_at,consumed_at) VALUES(?,?,?,?,?,?,'active',?,NULL)`, code.ID, input.RecoveryBatchID, input.User.ID, code.Ordinal, code.Salt, code.Hash, formatServerTime(input.At)); err != nil {
			return fmt.Errorf("create recovery code hash: %w", err)
		}
	}
	if err := insertBrowserSession(ctx, conn, input.BrowserSession, input.At); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO bootstrap_receipts(ceremony_id,user_id,anchor_device_id,receipt_json,receipt_digest,created_at) VALUES(?,?,?,?,?,?)`, input.CeremonyID, input.User.ID, input.AnchorDeviceID, string(input.ReceiptJSON), input.ReceiptDigest, formatServerTime(input.At)); err != nil {
		return fmt.Errorf("store bootstrap receipt: %w", err)
	}
	auditID, err := transport.NewUUIDv7()
	if err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO auth_audit_events(id,user_id,actor_class,action,decision,reason_code,target_id,created_at) VALUES(?,?,'pre_user','bootstrap.commit','allowed','bootstrap_committed',?,?)`, auditID, input.User.ID, input.AnchorDeviceID, formatServerTime(input.At)); err != nil {
		return fmt.Errorf("store bootstrap audit: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `UPDATE bootstrap_state SET token_digest=NULL,token_expires_at=NULL,revision=revision+1,updated_at=? WHERE singleton=1`, formatServerTime(input.At)); err != nil {
		return fmt.Errorf("consume bootstrap token: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("commit bootstrap transaction: %w", err)
	}
	committed = true
	return nil
}

func validateBootstrapCommitInput(input BootstrapCommitInput) error {
	if _, err := transport.ParseUUIDv7(input.CeremonyID); err != nil {
		return fmt.Errorf("bootstrap ceremony is invalid")
	}
	for _, id := range []string{input.User.ID, input.Passkey.ID, input.AnchorDeviceID, input.RecoveryBatchID, input.BrowserSession.ID} {
		if _, err := transport.ParseUUIDv7(id); err != nil {
			return fmt.Errorf("bootstrap identity is invalid")
		}
	}
	if len(input.User.Handle) < 1 || len(input.User.Handle) > 64 || input.User.DisplayName == "" || len(input.User.DisplayName) > 128 || len(input.Passkey.Credential.ID) < 1 || len(input.Passkey.Credential.ID) > 1024 ||
		len(input.SigningPublicKey) != 32 || len(input.ExchangePublicKey) != 32 || len(input.SigningKeyDigest) != 32 || len(input.ExchangeKeyDigest) != 32 || len(input.PinDigest) != 32 || len(input.StorageAssertionDigest) != 32 ||
		len(input.StorageAssertionJSON) < 2 || len(input.StorageAssertionJSON) > 4096 || len(input.CapabilitiesJSON) < 2 || len(input.CapabilitiesJSON) > 4096 || len(input.RecoveryCodes) != 10 || len(input.ReceiptJSON) < 2 || len(input.ReceiptJSON) > 4096 || len(input.ReceiptDigest) != 32 || input.At.IsZero() {
		return fmt.Errorf("bootstrap commit input is invalid")
	}
	seen := make(map[int]struct{}, 10)
	for _, code := range input.RecoveryCodes {
		if _, err := transport.ParseUUIDv7(code.ID); err != nil || code.Ordinal < 1 || code.Ordinal > 10 || len(code.Salt) != 16 || len(code.Hash) != 32 {
			return fmt.Errorf("bootstrap recovery code hash is invalid")
		}
		if _, duplicate := seen[code.Ordinal]; duplicate {
			return fmt.Errorf("bootstrap recovery ordinal is duplicated")
		}
		seen[code.Ordinal] = struct{}{}
	}
	return validateBrowserSessionCreate(input.BrowserSession, input.At)
}

func (s *Store) BootstrapReceipt(ctx context.Context, ceremonyID string) ([]byte, error) {
	var receipt []byte
	if err := s.db.QueryRowContext(ctx, "SELECT receipt_json FROM bootstrap_receipts WHERE ceremony_id=?", ceremonyID).Scan(&receipt); errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("bootstrap receipt was not found")
	} else if err != nil {
		return nil, fmt.Errorf("read bootstrap receipt: %w", err)
	}
	return append([]byte(nil), receipt...), nil
}

func (s *Store) SoleUser(ctx context.Context) (StoredUser, error) {
	var user StoredUser
	var created, updated string
	if err := s.db.QueryRowContext(ctx, `SELECT id,user_handle,display_name,revision,created_at,updated_at FROM users WHERE singleton=1`).Scan(&user.ID, &user.Handle, &user.DisplayName, &user.Revision, &created, &updated); err != nil {
		return StoredUser{}, fmt.Errorf("read user: %w", err)
	}
	var err error
	if user.CreatedAt, err = parseServerTime(created); err != nil {
		return StoredUser{}, err
	}
	if user.UpdatedAt, err = parseServerTime(updated); err != nil {
		return StoredUser{}, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT credential_json FROM passkeys WHERE user_id=? AND active=1 ORDER BY created_at`, user.ID)
	if err != nil {
		return StoredUser{}, fmt.Errorf("list user passkeys: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var encoded []byte
		var credential webauthn.Credential
		if err := rows.Scan(&encoded); err != nil || json.Unmarshal(encoded, &credential) != nil {
			return StoredUser{}, fmt.Errorf("stored passkey is invalid")
		}
		user.Credentials = append(user.Credentials, credential)
	}
	if err := rows.Err(); err != nil {
		return StoredUser{}, fmt.Errorf("list user passkeys: %w", err)
	}
	return user, nil
}

func (s *Store) PasskeyByCredentialID(ctx context.Context, credentialID []byte) (StoredPasskey, error) {
	return scanStoredPasskey(s.db.QueryRowContext(ctx, `SELECT id,user_id,credential_json,name,transports_json,sign_count,credential_revision,active,created_at,last_used_at,updated_at FROM passkeys WHERE credential_id=?`, credentialID))
}

func scanStoredPasskey(row interface{ Scan(...any) error }) (StoredPasskey, error) {
	var value StoredPasskey
	var credentialJSON, transportsJSON []byte
	var active int
	var created, updated string
	var lastUsed sql.NullString
	if err := row.Scan(&value.ID, &value.UserID, &credentialJSON, &value.Name, &transportsJSON, &value.SignCount, &value.CredentialRevision, &active, &created, &lastUsed, &updated); err != nil {
		return StoredPasskey{}, err
	}
	if err := json.Unmarshal(credentialJSON, &value.Credential); err != nil {
		return StoredPasskey{}, fmt.Errorf("stored passkey credential is invalid")
	}
	value.TransportsJSON = append([]byte(nil), transportsJSON...)
	value.Active = active == 1
	var err error
	if value.CreatedAt, err = parseServerTime(created); err != nil {
		return StoredPasskey{}, err
	}
	if value.UpdatedAt, err = parseServerTime(updated); err != nil {
		return StoredPasskey{}, err
	}
	if lastUsed.Valid {
		parsed, err := parseServerTime(lastUsed.String)
		if err != nil {
			return StoredPasskey{}, err
		}
		value.LastUsedAt = &parsed
	}
	return value, nil
}

func NewBrowserSessionCreate(userID, method, passkeyID string, now time.Time) (BrowserSessionCreate, error) {
	id, err := transport.NewUUIDv7()
	if err != nil {
		return BrowserSessionCreate{}, err
	}
	token, err := randomFixed(32)
	if err != nil {
		return BrowserSessionCreate{}, err
	}
	csrf, err := randomFixed(32)
	if err != nil {
		zeroBytes(token)
		return BrowserSessionCreate{}, err
	}
	expires := now.UTC().Add(browserAbsoluteLifetime)
	idle := now.UTC().Add(browserIdleLifetime)
	if method == "recovery" {
		expires = now.UTC().Add(recoverySessionLifetime)
		idle = expires
		passkeyID = ""
	}
	recent := now.UTC()
	var recentPtr *time.Time
	if method == "passkey" {
		recentPtr = &recent
	}
	return BrowserSessionCreate{ID: id, UserID: userID, RawToken: token, RawCSRF: csrf, AuthenticationMethod: method, AuthenticationPasskeyID: passkeyID, AuthenticatedAt: now.UTC(), RecentUVAt: recentPtr, ExpiresAt: expires, IdleExpiresAt: idle}, nil
}

func validateBrowserSessionCreate(value BrowserSessionCreate, now time.Time) error {
	if _, err := transport.ParseUUIDv7(value.ID); err != nil {
		return fmt.Errorf("browser session ID is invalid")
	}
	if _, err := transport.ParseUUIDv7(value.UserID); err != nil {
		return fmt.Errorf("browser session user is invalid")
	}
	if len(value.RawToken) != 32 || len(value.RawCSRF) != 32 || value.AuthenticatedAt.IsZero() || value.ExpiresAt.IsZero() || value.IdleExpiresAt.IsZero() || value.ExpiresAt.Before(value.IdleExpiresAt) || value.AuthenticatedAt.After(value.IdleExpiresAt) {
		return fmt.Errorf("browser session timing or material is invalid")
	}
	if value.AuthenticationMethod == "passkey" {
		if _, err := transport.ParseUUIDv7(value.AuthenticationPasskeyID); err != nil || value.RecentUVAt == nil {
			return fmt.Errorf("passkey browser session is invalid")
		}
	} else if value.AuthenticationMethod != "recovery" || value.AuthenticationPasskeyID != "" || value.RecentUVAt != nil {
		return fmt.Errorf("recovery browser session is invalid")
	}
	return nil
}

type dbExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertBrowserSession(ctx context.Context, execer dbExecer, value BrowserSessionCreate, now time.Time) error {
	if err := validateBrowserSessionCreate(value, now); err != nil {
		return err
	}
	tokenDigest := sha256.Sum256(value.RawToken)
	csrfDigest := sha256.Sum256(value.RawCSRF)
	if _, err := execer.ExecContext(ctx, `INSERT INTO browser_sessions(id,user_id,token_digest,csrf_digest,authentication_method,authentication_passkey_id,authenticated_at,recent_uv_at,expires_at,idle_expires_at,revoked_at,revision,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,NULL,1,?,?)`, value.ID, value.UserID, tokenDigest[:], csrfDigest[:], value.AuthenticationMethod, nullString(value.AuthenticationPasskeyID), formatServerTime(value.AuthenticatedAt), nullTime(value.RecentUVAt), formatServerTime(value.ExpiresAt), formatServerTime(value.IdleExpiresAt), formatServerTime(now), formatServerTime(now)); err != nil {
		return fmt.Errorf("create browser session: %w", err)
	}
	return nil
}

func (s *Store) BrowserSessionByToken(ctx context.Context, rawToken []byte, now time.Time) (BrowserSession, error) {
	if len(rawToken) != 32 {
		return BrowserSession{}, fmt.Errorf("browser session is invalid")
	}
	digest := sha256.Sum256(rawToken)
	return browserSessionByToken(ctx, s.db, digest[:], now)
}

func (s *Store) RotateSessionCSRF(ctx context.Context, sessionID string, expectedRevision int64, rawCSRF []byte, now time.Time) (int64, error) {
	if len(rawCSRF) != 32 {
		return 0, fmt.Errorf("CSRF value is invalid")
	}
	digest := sha256.Sum256(rawCSRF)
	result, err := s.db.ExecContext(ctx, `UPDATE browser_sessions SET csrf_digest=?,revision=revision+1,updated_at=? WHERE id=? AND revision=? AND revoked_at IS NULL AND expires_at>? AND idle_expires_at>?`, digest[:], formatServerTime(now), sessionID, expectedRevision, formatServerTime(now), formatServerTime(now))
	if err != nil {
		return 0, fmt.Errorf("rotate session CSRF: %w", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return 0, fmt.Errorf("browser session changed")
	}
	return expectedRevision + 1, nil
}

func (s *Store) ValidateCSRF(session BrowserSession, raw []byte) bool {
	if len(raw) != 32 || len(session.CSRFDigest) != 32 {
		return false
	}
	digest := sha256.Sum256(raw)
	return subtle.ConstantTimeCompare(digest[:], session.CSRFDigest) == 1
}

func randomFixed(size int) ([]byte, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return nil, fmt.Errorf("secure randomness failed: %w", err)
	}
	return value, nil
}

func zeroBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

func formatServerTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }
func parseServerTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.Location() != time.UTC {
		return time.Time{}, fmt.Errorf("stored UTC time is invalid")
	}
	return parsed, nil
}
func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func nullTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatServerTime(*value)
}
