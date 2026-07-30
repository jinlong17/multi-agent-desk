package controlplane

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	generatedapi "github.com/jinlong17/multi-agent-desk/internal/controlplane/api/generated"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

type AuthIdempotencyOperation = generatedapi.AuthIdempotencyOperationV1

const (
	AuthOperationBootstrapOptions           = generatedapi.BootstrapOptions
	AuthOperationBootstrapVerify            = generatedapi.BootstrapVerify
	AuthOperationPasskeyLoginOptions        = generatedapi.PasskeyLoginOptions
	AuthOperationPasskeyLoginVerify         = generatedapi.PasskeyLoginVerify
	AuthOperationPasskeyRegistrationOptions = generatedapi.PasskeyRegistrationOptions
	AuthOperationPasskeyRegistrationVerify  = generatedapi.PasskeyRegistrationVerify
	AuthOperationPasskeyDelete              = generatedapi.PasskeyDelete
	AuthOperationUVOptions                  = generatedapi.UvOptions
	AuthOperationUVVerify                   = generatedapi.UvVerify
	AuthOperationRecoveryVerify             = generatedapi.RecoveryVerify
	AuthOperationRecoveryCodesRotate        = generatedapi.RecoveryCodesRotate
	AuthOperationLogout                     = generatedapi.Logout
	AuthOperationSessionDelete              = generatedapi.SessionDelete
)

var authIdempotencyOperations = [...]AuthIdempotencyOperation{
	AuthOperationBootstrapOptions,
	AuthOperationBootstrapVerify,
	AuthOperationPasskeyLoginOptions,
	AuthOperationPasskeyLoginVerify,
	AuthOperationPasskeyRegistrationOptions,
	AuthOperationPasskeyRegistrationVerify,
	AuthOperationPasskeyDelete,
	AuthOperationUVOptions,
	AuthOperationUVVerify,
	AuthOperationRecoveryVerify,
	AuthOperationRecoveryCodesRotate,
	AuthOperationLogout,
	AuthOperationSessionDelete,
}

type AuthIdempotencyActorClass string

const (
	AuthActorBootstrapToken AuthIdempotencyActorClass = "bootstrap_token"
	AuthActorPreauthBrowser AuthIdempotencyActorClass = "preauth_browser"
	AuthActorBrowserSession AuthIdempotencyActorClass = "browser_session"
)

type authOperationContract struct {
	Actor       AuthIdempotencyActorClass
	Method      string
	Path        string
	DynamicPath string
	IfMatch     bool
	Begin       bool
	Secret      bool
}

var authOperationContracts = map[AuthIdempotencyOperation]authOperationContract{
	AuthOperationBootstrapOptions:           {Actor: AuthActorBootstrapToken, Method: http.MethodPost, Path: "/v1/bootstrap/options", Begin: true},
	AuthOperationBootstrapVerify:            {Actor: AuthActorBootstrapToken, Method: http.MethodPost, Path: "/v1/bootstrap/verify", Secret: true},
	AuthOperationPasskeyLoginOptions:        {Actor: AuthActorPreauthBrowser, Method: http.MethodPost, Path: "/v1/auth/passkeys/options", Begin: true},
	AuthOperationPasskeyLoginVerify:         {Actor: AuthActorPreauthBrowser, Method: http.MethodPost, Path: "/v1/auth/passkeys/verify", Secret: true},
	AuthOperationPasskeyRegistrationOptions: {Actor: AuthActorBrowserSession, Method: http.MethodPost, Path: "/v1/auth/passkeys/registration/options", Begin: true},
	AuthOperationPasskeyRegistrationVerify:  {Actor: AuthActorBrowserSession, Method: http.MethodPost, Path: "/v1/auth/passkeys/registration/verify", Secret: true},
	AuthOperationPasskeyDelete:              {Actor: AuthActorBrowserSession, Method: http.MethodDelete, DynamicPath: "/v1/auth/passkeys/", IfMatch: true},
	AuthOperationUVOptions:                  {Actor: AuthActorBrowserSession, Method: http.MethodPost, Path: "/v1/auth/uv/options", Begin: true},
	AuthOperationUVVerify:                   {Actor: AuthActorBrowserSession, Method: http.MethodPost, Path: "/v1/auth/uv/verify", Secret: true},
	AuthOperationRecoveryVerify:             {Actor: AuthActorPreauthBrowser, Method: http.MethodPost, Path: "/v1/auth/recovery/verify", Secret: true},
	AuthOperationRecoveryCodesRotate:        {Actor: AuthActorBrowserSession, Method: http.MethodPost, Path: "/v1/auth/recovery-codes/rotate", Secret: true},
	AuthOperationLogout:                     {Actor: AuthActorBrowserSession, Method: http.MethodPost, Path: "/v1/auth/logout"},
	AuthOperationSessionDelete:              {Actor: AuthActorBrowserSession, Method: http.MethodDelete, DynamicPath: "/v1/auth/sessions/", IfMatch: true},
}

type AuthIdempotencyRequest struct {
	KeyDigest           [sha256.Size]byte
	RequestIdentity     [sha256.Size]byte
	ActorClass          AuthIdempotencyActorClass
	ActorIdentity       [sha256.Size]byte
	Method              string
	CanonicalPath       string
	BodyDigest          [sha256.Size]byte
	CanonicalIfMatch    string
	Operation           AuthIdempotencyOperation
	ServerBootEpoch     string
	ProposedOperationID string
}

type AuthIdempotencyRecord struct {
	AuthIdempotencyRequest
	OperationID         string
	State               string
	PublicReceiptJSON   []byte
	CookieAction        string
	CeremonyID          string
	PublicOptionsDigest []byte
	CreatedAt           time.Time
	CommittedAt         *time.Time
	ExpiresAt           time.Time
}

type AuthOperationReceiptV1 = generatedapi.AuthOperationReceiptV1

type AuthIdempotencyCommit struct {
	KeyDigest           [sha256.Size]byte
	OperationID         string
	Receipt             AuthOperationReceiptV1
	PublicResult        any
	CookieAction        string
	CeremonyID          string
	PublicOptionsDigest []byte
	At                  time.Time
}

var (
	ErrAuthIdempotencyKeyReused  = errors.New("idempotency_key_reused")
	ErrAuthIdempotencyInProgress = errors.New("idempotency_in_progress")
	ErrCeremonyRestartRequired   = errors.New("ceremony_restart_required")
)

type authSecurityCommitError struct{ cause error }

func (e *authSecurityCommitError) Error() string { return e.cause.Error() }
func (e *authSecurityCommitError) Unwrap() error { return e.cause }

// commitAuthSecurityFailure is reserved for a fail-closed security transition
// (for example ceremony consume, clone revocation, or integrity revocation)
// that must survive while the provisional idempotency winner row is removed.
func commitAuthSecurityFailure(err error) error {
	if err == nil {
		return nil
	}
	return &authSecurityCommitError{cause: err}
}

func authOperationActorClass(operation AuthIdempotencyOperation) (AuthIdempotencyActorClass, bool) {
	contract, ok := authOperationContracts[operation]
	return contract.Actor, ok
}

func validateAuthIdempotencyRequest(value AuthIdempotencyRequest) error {
	contract, ok := authOperationContracts[value.Operation]
	if !ok || value.ActorClass != contract.Actor || value.Method != contract.Method || len(value.ActorIdentity) != sha256.Size || len(value.ServerBootEpoch) != 36 {
		return fmt.Errorf("auth idempotency operation contract is invalid")
	}
	if _, err := transport.ParseUUIDv7(value.ServerBootEpoch); err != nil {
		return fmt.Errorf("auth idempotency boot epoch is invalid")
	}
	if value.ProposedOperationID != "" {
		if _, err := transport.ParseUUIDv7(value.ProposedOperationID); err != nil {
			return fmt.Errorf("auth idempotency operation ID is invalid")
		}
	}
	if contract.Path != "" {
		if value.CanonicalPath != contract.Path {
			return fmt.Errorf("auth idempotency canonical path is invalid")
		}
	} else {
		if !strings.HasPrefix(value.CanonicalPath, contract.DynamicPath) {
			return fmt.Errorf("auth idempotency canonical path is invalid")
		}
		id := strings.TrimPrefix(value.CanonicalPath, contract.DynamicPath)
		parsed, err := transport.ParseUUIDv7(id)
		if err != nil || parsed.String() != id || id != strings.ToLower(id) {
			return fmt.Errorf("auth idempotency concrete path ID is invalid")
		}
	}
	if contract.IfMatch {
		if !validCanonicalIfMatch(value.CanonicalIfMatch) {
			return fmt.Errorf("auth idempotency If-Match is invalid")
		}
	} else if value.CanonicalIfMatch != "" {
		return fmt.Errorf("auth idempotency If-Match is forbidden")
	}
	return nil
}

func validCanonicalIfMatch(value string) bool {
	if !strings.HasPrefix(value, `"rev-`) || !strings.HasSuffix(value, `"`) {
		return false
	}
	digits := strings.TrimSuffix(strings.TrimPrefix(value, `"rev-`), `"`)
	if digits == "" || digits[0] == '0' {
		return false
	}
	for _, value := range []byte(digits) {
		if value < '0' || value > '9' {
			return false
		}
	}
	return true
}

const authProductSavepoint = "mad_auth_product"

type AuthIdempotencyTx struct {
	Conn                *sql.Conn
	productStarted      bool
	failureCeremonyID   string
	failureCeremonyKind ceremonyKind
	rollbackHooks       []func()
	commitHooks         []func()
	finishHooks         []func()
	rollbackHooksRun    bool
	commitHooksRun      bool
	finishHooksRun      bool
}

func (t *AuthIdempotencyTx) BeginProduct(ctx context.Context) error {
	if t == nil || t.Conn == nil || t.productStarted {
		return fmt.Errorf("auth product transaction is invalid")
	}
	if _, err := t.Conn.ExecContext(ctx, "SAVEPOINT "+authProductSavepoint); err != nil {
		return fmt.Errorf("begin auth product savepoint: %w", err)
	}
	t.productStarted = true
	return nil
}

func (t *AuthIdempotencyTx) ConsumeCeremonyOnFailure(ceremonyID string, kind ceremonyKind) error {
	if t == nil || !t.productStarted || !validCeremonyKind(kind) {
		return fmt.Errorf("auth failure ceremony binding is invalid")
	}
	if _, err := transport.ParseUUIDv7(ceremonyID); err != nil {
		return fmt.Errorf("auth failure ceremony binding is invalid")
	}
	t.failureCeremonyID = ceremonyID
	t.failureCeremonyKind = kind
	return nil
}

// OnRollback binds process-local state to the same outcome as the product
// savepoint. The hook runs after a product rollback, or after the outer
// transaction aborts before it can be finalized.
func (t *AuthIdempotencyTx) OnRollback(hook func()) error {
	if t == nil || !t.productStarted || hook == nil || t.rollbackHooksRun {
		return fmt.Errorf("auth rollback hook is invalid")
	}
	t.rollbackHooks = append(t.rollbackHooks, hook)
	return nil
}

// OnCommit registers process-local state activation that must happen only
// after the outer SQLite COMMIT has made the corresponding product state and
// idempotency receipt durable.
func (t *AuthIdempotencyTx) OnCommit(hook func()) error {
	if t == nil || !t.productStarted || hook == nil || t.commitHooksRun {
		return fmt.Errorf("auth commit hook is invalid")
	}
	t.commitHooks = append(t.commitHooks, hook)
	return nil
}

// OnFinish registers process-local one-shot cleanup that must happen after
// either the success or failure path has finished using the action result.
func (t *AuthIdempotencyTx) OnFinish(hook func()) error {
	if t == nil || hook == nil || t.finishHooksRun {
		return fmt.Errorf("auth finish hook is invalid")
	}
	t.finishHooks = append(t.finishHooks, hook)
	return nil
}

func (t *AuthIdempotencyTx) runRollbackHooks() {
	if t == nil || t.rollbackHooksRun {
		return
	}
	t.rollbackHooksRun = true
	for index := len(t.rollbackHooks) - 1; index >= 0; index-- {
		t.rollbackHooks[index]()
	}
	t.rollbackHooks = nil
}

func (t *AuthIdempotencyTx) runCommitHooks() {
	if t == nil || t.commitHooksRun {
		return
	}
	t.commitHooksRun = true
	for _, hook := range t.commitHooks {
		hook()
	}
	t.commitHooks = nil
}

func (t *AuthIdempotencyTx) runFinishHooks() {
	if t == nil || t.finishHooksRun {
		return
	}
	t.finishHooksRun = true
	for index := len(t.finishHooks) - 1; index >= 0; index-- {
		t.finishHooks[index]()
	}
	t.finishHooks = nil
}

func (t *AuthIdempotencyTx) rollbackProduct(ctx context.Context) error {
	if t == nil || t.Conn == nil || !t.productStarted {
		return nil
	}
	if _, err := t.Conn.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+authProductSavepoint); err != nil {
		return fmt.Errorf("rollback auth product savepoint: %w", err)
	}
	if _, err := t.Conn.ExecContext(ctx, "RELEASE SAVEPOINT "+authProductSavepoint); err != nil {
		return fmt.Errorf("release auth product savepoint: %w", err)
	}
	t.productStarted = false
	t.runRollbackHooks()
	return nil
}

type authPrefixCommitError struct{ cause error }

func (e *authPrefixCommitError) Error() string { return e.cause.Error() }
func (e *authPrefixCommitError) Unwrap() error { return e.cause }

func commitAuthPrefixFailure(err error) error {
	if err == nil {
		return nil
	}
	return &authPrefixCommitError{cause: err}
}

type AuthIdempotencyAction func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error)

// WithAuthIdempotency is the sole P2 auth mutation transaction boundary. The
// global-key gate, caller's product/security transition, public receipt, and
// optional ceremony binding share one BEGIN IMMEDIATE transaction. A crash or
// callback failure therefore leaves neither a durable in-progress marker nor a
// partially committed product effect.
func (s *Store) WithAuthIdempotency(ctx context.Context, input AuthIdempotencyRequest, now time.Time, action AuthIdempotencyAction) (AuthIdempotencyRecord, bool, error) {
	if err := validateAuthIdempotencyRequest(input); err != nil || now.IsZero() {
		return AuthIdempotencyRecord{}, false, fmt.Errorf("invalid auth idempotency begin: %w", err)
	}
	if action == nil {
		return AuthIdempotencyRecord{}, false, fmt.Errorf("auth idempotency action is required")
	}
	if input.ProposedOperationID == "" {
		var err error
		input.ProposedOperationID, err = transport.NewUUIDv7()
		if err != nil {
			return AuthIdempotencyRecord{}, false, err
		}
	}
	now = normalizeServerTime(now)
	conn, err := s.db.Conn(ctx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return AuthIdempotencyRecord{}, false, ErrAuthIdempotencyInProgress
		}
		return AuthIdempotencyRecord{}, false, fmt.Errorf("acquire auth idempotency connection: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "locked") || strings.Contains(strings.ToLower(err.Error()), "busy") {
			return AuthIdempotencyRecord{}, false, ErrAuthIdempotencyInProgress
		}
		return AuthIdempotencyRecord{}, false, fmt.Errorf("begin auth idempotency: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	if _, err := conn.ExecContext(ctx, "DELETE FROM auth_idempotency_operations WHERE expires_at<=?", formatServerTime(now)); err != nil {
		return AuthIdempotencyRecord{}, false, fmt.Errorf("clean auth idempotency rows: %w", err)
	}
	record, err := authIdempotencyByKey(ctx, conn, input.KeyDigest)
	if err == nil {
		if !sameAuthIdempotencyIdentity(record, input) {
			return AuthIdempotencyRecord{}, false, ErrAuthIdempotencyKeyReused
		}
		if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
			return AuthIdempotencyRecord{}, false, err
		}
		committed = true
		if record.State == "in_progress" {
			return record, true, ErrAuthIdempotencyInProgress
		}
		if contract := authOperationContracts[record.Operation]; contract.Begin && record.ServerBootEpoch != input.ServerBootEpoch {
			return record, true, ErrCeremonyRestartRequired
		}
		return record, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return AuthIdempotencyRecord{}, false, err
	}
	expires := now.Add(24 * time.Hour)
	if _, err := conn.ExecContext(ctx, `INSERT INTO auth_idempotency_operations(key_digest,request_identity_digest,actor_class,actor_identity_digest,method,canonical_path,body_digest,canonical_if_match,operation_id,operation,state,server_boot_epoch,public_receipt_json,cookie_action,ceremony_id,public_options_digest,created_at,committed_at,expires_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,'in_progress',?,NULL,'none',NULL,NULL,?,NULL,?)`, input.KeyDigest[:], input.RequestIdentity[:], input.ActorClass, input.ActorIdentity[:], input.Method, input.CanonicalPath, input.BodyDigest[:], input.CanonicalIfMatch, input.ProposedOperationID, input.Operation, input.ServerBootEpoch, formatServerTime(now), formatServerTime(expires)); err != nil {
		return AuthIdempotencyRecord{}, false, fmt.Errorf("insert auth idempotency row: %w", err)
	}
	tx := &AuthIdempotencyTx{Conn: conn}
	defer tx.runFinishHooks()
	defer func() {
		if !committed {
			tx.runRollbackHooks()
		}
	}()
	commit, err := action(tx, input.ProposedOperationID)
	finalizeCtx, cancelFinalize := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancelFinalize()
	finishFailure := func(cause error, preserveProduct bool) (AuthIdempotencyRecord, bool, error) {
		if !preserveProduct {
			if err := tx.rollbackProduct(finalizeCtx); err != nil {
				return AuthIdempotencyRecord{}, false, err
			}
			if tx.failureCeremonyID != "" {
				if err := consumeWebAuthnCeremonyIfPresent(finalizeCtx, conn, tx.failureCeremonyID, tx.failureCeremonyKind); err != nil {
					return AuthIdempotencyRecord{}, false, fmt.Errorf("consume failed auth ceremony: %w", err)
				}
			}
		}
		result, deleteErr := conn.ExecContext(finalizeCtx, `DELETE FROM auth_idempotency_operations WHERE key_digest=? AND operation_id=? AND state='in_progress'`, input.KeyDigest[:], input.ProposedOperationID)
		if deleteErr != nil {
			return AuthIdempotencyRecord{}, false, fmt.Errorf("remove failed auth idempotency winner: %w", deleteErr)
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return AuthIdempotencyRecord{}, false, ErrAuthIdempotencyKeyReused
		}
		if _, commitErr := conn.ExecContext(finalizeCtx, "COMMIT"); commitErr != nil {
			return AuthIdempotencyRecord{}, false, fmt.Errorf("commit auth failure outcome: %w", commitErr)
		}
		committed = true
		return AuthIdempotencyRecord{}, false, cause
	}
	if err != nil {
		var securityFailure *authSecurityCommitError
		if errors.As(err, &securityFailure) {
			if errors.Is(securityFailure.cause, ErrSessionIntegrityInvalid) || errors.Is(securityFailure.cause, ErrPasskeyCounterRegressed) {
				return finishFailure(securityFailure.cause, true)
			}
			err = securityFailure.cause
		}
		if errors.Is(err, ErrPasskeyCounterRegressed) {
			return finishFailure(err, true)
		}
		var prefixFailure *authPrefixCommitError
		if tx.productStarted || errors.As(err, &prefixFailure) {
			if prefixFailure != nil {
				err = prefixFailure.cause
			}
			return finishFailure(err, false)
		}
		return AuthIdempotencyRecord{}, false, err
	}
	commit.KeyDigest = input.KeyDigest
	commit.OperationID = input.ProposedOperationID
	commit.Receipt.OperationId = input.ProposedOperationID
	commit.Receipt.Operation = input.Operation
	if commit.At.IsZero() {
		commit.At = now
	}
	if commit.CookieAction == "" {
		commit.CookieAction = "none"
	}
	if commit.Receipt.CommittedAt.IsZero() {
		commit.Receipt.CommittedAt = commit.At
	}
	contract := authOperationContracts[input.Operation]
	if contract.Begin != (commit.CeremonyID != "") {
		err := fmt.Errorf("auth idempotency ceremony class is invalid")
		if tx.productStarted {
			return finishFailure(err, false)
		}
		return AuthIdempotencyRecord{}, false, err
	}
	if err := validateAuthIdempotencyCommit(commit); err != nil {
		if tx.productStarted {
			return finishFailure(err, false)
		}
		return AuthIdempotencyRecord{}, false, err
	}
	var storedReceipt any = commit.Receipt
	if commit.PublicResult != nil {
		storedReceipt = struct {
			Receipt AuthOperationReceiptV1 `json:"receipt"`
			Result  any                    `json:"result"`
		}{Receipt: commit.Receipt, Result: commit.PublicResult}
	}
	receiptJSON, err := json.Marshal(storedReceipt)
	if err != nil || len(receiptJSON) > 16<<10 {
		err := fmt.Errorf("auth operation receipt is invalid")
		if tx.productStarted {
			return finishFailure(err, false)
		}
		return AuthIdempotencyRecord{}, false, err
	}
	result, err := conn.ExecContext(finalizeCtx, `UPDATE auth_idempotency_operations SET state='committed',public_receipt_json=?,cookie_action=?,ceremony_id=?,public_options_digest=?,committed_at=? WHERE key_digest=? AND operation_id=? AND operation=? AND state='in_progress'`, string(receiptJSON), commit.CookieAction, nullString(commit.CeremonyID), nullBytes(commit.PublicOptionsDigest), formatServerTime(commit.At), input.KeyDigest[:], input.ProposedOperationID, input.Operation)
	if err != nil {
		err = fmt.Errorf("commit auth idempotency row: %w", err)
		if tx.productStarted {
			return finishFailure(err, false)
		}
		return AuthIdempotencyRecord{}, false, err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		if tx.productStarted {
			return finishFailure(ErrAuthIdempotencyKeyReused, false)
		}
		return AuthIdempotencyRecord{}, false, ErrAuthIdempotencyKeyReused
	}
	if _, err := conn.ExecContext(finalizeCtx, "COMMIT"); err != nil {
		cause := fmt.Errorf("commit auth idempotency transaction: %w", err)
		if tx.productStarted {
			return finishFailure(cause, false)
		}
		return AuthIdempotencyRecord{}, false, cause
	}
	committed = true
	tx.runCommitHooks()
	operationID := input.ProposedOperationID
	input.ProposedOperationID = ""
	return AuthIdempotencyRecord{AuthIdempotencyRequest: input, OperationID: operationID, State: "committed", PublicReceiptJSON: receiptJSON, CookieAction: commit.CookieAction, CeremonyID: commit.CeremonyID, PublicOptionsDigest: append([]byte(nil), commit.PublicOptionsDigest...), CreatedAt: now, CommittedAt: &commit.At, ExpiresAt: expires}, false, nil
}

func validateAuthIdempotencyCommit(input AuthIdempotencyCommit) error {
	if _, err := transport.ParseUUIDv7(input.OperationID); err != nil || input.Receipt.OperationId != input.OperationID || !input.Receipt.State.Valid() || input.Receipt.CommittedAt.IsZero() || input.At.IsZero() || !input.Receipt.Operation.Valid() {
		return fmt.Errorf("auth idempotency commit is invalid")
	}
	if input.CookieAction != "none" && input.CookieAction != "clear" && input.CookieAction != "secret_issued" {
		return fmt.Errorf("auth idempotency cookie action is invalid")
	}
	if !input.Receipt.CookieOutcome.Valid() || !input.Receipt.CsrfOutcome.Valid() || !input.Receipt.RecoveryCodesOutcome.Valid() || !input.Receipt.NextAction.Valid() {
		return fmt.Errorf("auth idempotency receipt outcome is invalid")
	}
	contract, ok := authOperationContracts[input.Receipt.Operation]
	if !ok || contract.Secret && input.PublicResult != nil {
		return fmt.Errorf("auth idempotency public result class is invalid")
	}
	if input.Receipt.ResourceId != nil {
		if _, err := transport.ParseUUIDv7(*input.Receipt.ResourceId); err != nil {
			return fmt.Errorf("auth idempotency receipt resource is invalid")
		}
	}
	if (input.CeremonyID == "") != (len(input.PublicOptionsDigest) == 0) || (len(input.PublicOptionsDigest) != 0 && len(input.PublicOptionsDigest) != sha256.Size) {
		return fmt.Errorf("auth idempotency ceremony binding is invalid")
	}
	if input.CeremonyID != "" {
		if _, err := transport.ParseUUIDv7(input.CeremonyID); err != nil {
			return fmt.Errorf("auth idempotency ceremony ID is invalid")
		}
	}
	return nil
}

func authIdempotencyByKey(ctx context.Context, queryer rowQueryer, key [sha256.Size]byte) (AuthIdempotencyRecord, error) {
	var value AuthIdempotencyRecord
	var requestIdentity, actorIdentity, bodyDigest, optionsDigest []byte
	var receipt, ceremony, committed sql.NullString
	var created, expires string
	err := queryer.QueryRowContext(ctx, `SELECT request_identity_digest,actor_class,actor_identity_digest,method,canonical_path,body_digest,canonical_if_match,operation_id,operation,state,server_boot_epoch,public_receipt_json,cookie_action,ceremony_id,public_options_digest,created_at,committed_at,expires_at FROM auth_idempotency_operations WHERE key_digest=?`, key[:]).Scan(
		&requestIdentity, &value.ActorClass, &actorIdentity, &value.Method, &value.CanonicalPath, &bodyDigest, &value.CanonicalIfMatch, &value.OperationID, &value.Operation, &value.State, &value.ServerBootEpoch, &receipt, &value.CookieAction, &ceremony, &optionsDigest, &created, &committed, &expires,
	)
	if err != nil {
		return AuthIdempotencyRecord{}, err
	}
	if len(requestIdentity) != sha256.Size || len(actorIdentity) != sha256.Size || len(bodyDigest) != sha256.Size || (len(optionsDigest) != 0 && len(optionsDigest) != sha256.Size) {
		return AuthIdempotencyRecord{}, fmt.Errorf("auth idempotency row is corrupt")
	}
	value.KeyDigest = key
	copy(value.RequestIdentity[:], requestIdentity)
	copy(value.ActorIdentity[:], actorIdentity)
	copy(value.BodyDigest[:], bodyDigest)
	value.PublicReceiptJSON = []byte(receipt.String)
	value.CeremonyID = ceremony.String
	value.PublicOptionsDigest = append([]byte(nil), optionsDigest...)
	var errTime error
	if value.CreatedAt, errTime = parseServerTime(created); errTime != nil {
		return AuthIdempotencyRecord{}, errTime
	}
	if value.ExpiresAt, errTime = parseServerTime(expires); errTime != nil {
		return AuthIdempotencyRecord{}, errTime
	}
	if committed.Valid {
		parsed, err := parseServerTime(committed.String)
		if err != nil {
			return AuthIdempotencyRecord{}, err
		}
		value.CommittedAt = &parsed
	}
	return value, nil
}

func sameAuthIdempotencyIdentity(stored AuthIdempotencyRecord, input AuthIdempotencyRequest) bool {
	return subtle.ConstantTimeCompare(stored.RequestIdentity[:], input.RequestIdentity[:]) == 1 &&
		subtle.ConstantTimeCompare(stored.ActorIdentity[:], input.ActorIdentity[:]) == 1 &&
		subtle.ConstantTimeCompare(stored.BodyDigest[:], input.BodyDigest[:]) == 1 &&
		stored.ActorClass == input.ActorClass && stored.Method == input.Method && stored.CanonicalPath == input.CanonicalPath && stored.CanonicalIfMatch == input.CanonicalIfMatch && stored.Operation == input.Operation
}

func nullBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}
