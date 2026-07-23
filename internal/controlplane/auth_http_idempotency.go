package controlplane

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"unicode/utf8"

	generatedapi "github.com/jinlong17/multi-agent-desk/internal/controlplane/api/generated"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

type preparedP2Mutation struct {
	Request        AuthIdempotencyRequest
	CanonicalBody  []byte
	BootstrapToken string
	RawSession     []byte
	RawCSRF        []byte
	Revision       int64
}

func (p *preparedP2Mutation) zero() {
	if p == nil {
		return
	}
	zeroBytes(p.RawSession)
	zeroBytes(p.RawCSRF)
	zeroBytes(p.CanonicalBody)
	p.RawSession = nil
	p.RawCSRF = nil
	p.CanonicalBody = nil
	p.BootstrapToken = ""
}

// prepareP2Mutation performs only bounded transport/schema checks and derives
// replay identity. It deliberately does not require live browser/bootstrap
// authority, touch a session, consume a ceremony, audit, or mutate product
// state; those checks run only for a new global-key winner on the same conn.
func (s *Server) prepareP2Mutation(request *http.Request, operation AuthIdempotencyOperation, maxBody int64, destination any, validate func() error) (prepared preparedP2Mutation, err error) {
	defer func() {
		if err != nil {
			prepared.zero()
		}
	}()
	contract, ok := authOperationContracts[operation]
	if !ok || request.Method != contract.Method || request.URL.ForceQuery || request.URL.RawQuery != "" || request.URL.RawPath != "" || request.URL.EscapedPath() != request.URL.Path {
		return prepared, fmt.Errorf("invalid_argument: auth operation route is invalid")
	}
	if contract.Actor == AuthActorBrowserSession {
		if err := s.requireBrowserMutationHeaders(request); err != nil {
			return prepared, err
		}
	} else if err := s.requirePreAuthMutation(request); err != nil {
		return prepared, err
	}

	keyValues := request.Header.Values("Idempotency-Key")
	if len(keyValues) == 0 {
		return prepared, fmt.Errorf("idempotency_key_required")
	}
	if len(keyValues) != 1 {
		return prepared, fmt.Errorf("idempotency_key_invalid")
	}
	normalizedKey, err := transport.NormalizeIdempotencyKeyV1(keyValues[0])
	if err != nil {
		return prepared, fmt.Errorf("idempotency_key_invalid")
	}
	keyDigest, err := transport.AuthIdempotencyKeyDigestV1(normalizedKey)
	if err != nil {
		return prepared, fmt.Errorf("idempotency_key_invalid")
	}
	if maxBody < 2 || maxBody > 1<<20 {
		return prepared, fmt.Errorf("invalid_argument: request body limit is invalid")
	}
	contents, err := io.ReadAll(io.LimitReader(request.Body, maxBody+1))
	if err != nil || int64(len(contents)) > maxBody {
		zeroBytes(contents)
		return prepared, fmt.Errorf("invalid_argument: request JSON exceeds its limit")
	}
	defer zeroBytes(contents)
	if destination == nil {
		prepared.CanonicalBody, err = transport.CanonicalEmptyJSONObjectV1(contents)
	} else {
		prepared.CanonicalBody, err = transport.CanonicalStrictJSONV1(contents, maxBody, destination)
	}
	if err != nil {
		return prepared, fmt.Errorf("invalid_argument: request JSON is invalid")
	}
	if validate != nil {
		if err := validate(); err != nil {
			return prepared, err
		}
	}
	if err := validateP2CanonicalRequestSchema(operation, prepared.CanonicalBody); err != nil {
		return prepared, err
	}
	bodyDigest, err := transport.AuthIdempotencyBodyDigestV1(prepared.CanonicalBody)
	if err != nil {
		return prepared, err
	}

	canonicalPath := request.URL.Path
	canonicalIfMatch := ""
	if contract.IfMatch {
		ifMatchValues := request.Header.Values("If-Match")
		if len(ifMatchValues) == 0 {
			return prepared, fmt.Errorf("if_match_required")
		}
		if len(ifMatchValues) != 1 {
			return prepared, fmt.Errorf("invalid_argument: revision precondition is invalid")
		}
		revision, parseErr := transport.ParseIfMatch(ifMatchValues[0])
		if parseErr != nil {
			return prepared, fmt.Errorf("invalid_argument: revision precondition is invalid")
		}
		prepared.Revision = int64(revision)
		canonicalIfMatch = ifMatchValues[0]
	} else if len(request.Header.Values("If-Match")) != 0 {
		return prepared, fmt.Errorf("invalid_argument: If-Match is forbidden")
	}

	var actorIdentity [sha256.Size]byte
	switch contract.Actor {
	case AuthActorBootstrapToken:
		prepared.BootstrapToken, err = bootstrapAuthorization(request)
		if err != nil {
			return prepared, err
		}
		rawToken, decodeErr := transport.DecodeBase64URLFixed(prepared.BootstrapToken, 32)
		if decodeErr != nil {
			return prepared, fmt.Errorf("bootstrap token is invalid")
		}
		actorIdentity = sha256.Sum256(rawToken)
		zeroBytes(rawToken)
	case AuthActorPreauthBrowser:
		framed, frameErr := transport.Frame([]byte("multidesk-preauth-browser-actor-v1"), []byte("1"), []byte(s.config.PublicOrigin))
		if frameErr != nil {
			return prepared, frameErr
		}
		actorIdentity = sha256.Sum256(framed)
	case AuthActorBrowserSession:
		prepared.RawSession, err = rawBrowserSessionToken(request)
		if err != nil {
			return prepared, err
		}
		actorIdentity = sha256.Sum256(prepared.RawSession)
		csrfValues := request.Header.Values("X-CSRF-Token")
		if len(csrfValues) != 1 {
			prepared.zero()
			return prepared, fmt.Errorf("csrf_invalid")
		}
		prepared.RawCSRF, err = transport.DecodeBase64URLFixed(csrfValues[0], 32)
		if err != nil {
			prepared.zero()
			return prepared, fmt.Errorf("csrf_invalid")
		}
	default:
		return prepared, fmt.Errorf("invalid_argument: auth operation actor is invalid")
	}
	requestIdentity, err := transport.AuthIdempotencyRequestIdentityDigestV1(
		s.config.PublicOrigin, string(contract.Actor), actorIdentity[:], string(operation), request.Method,
		canonicalPath, bodyDigest, canonicalIfMatch,
	)
	if err != nil {
		prepared.zero()
		return prepared, fmt.Errorf("invalid_argument: auth request identity is invalid")
	}
	prepared.Request = AuthIdempotencyRequest{
		KeyDigest: keyDigest, RequestIdentity: requestIdentity, ActorClass: contract.Actor,
		ActorIdentity: actorIdentity, Method: request.Method, CanonicalPath: canonicalPath,
		BodyDigest: bodyDigest, CanonicalIfMatch: canonicalIfMatch, Operation: operation,
		ServerBootEpoch: s.serverBootEpoch,
	}
	if err := validateAuthIdempotencyRequest(prepared.Request); err != nil {
		prepared.zero()
		return prepared, fmt.Errorf("invalid_argument: auth operation route is invalid")
	}
	return prepared, nil
}

func jsonSchemaStringLength(value string, minimum, maximum int) bool {
	if minimum < 0 || maximum < minimum || !utf8.ValidString(value) {
		return false
	}
	length := utf8.RuneCountInString(value)
	return length >= minimum && length <= maximum
}

// validateP2CanonicalRequestSchema closes constraints that encoding/json's
// generated DTOs cannot represent: explicit null versus omitted optional
// members, and the exact raw date-time primitive used by the bootstrap anchor.
// It runs after strict decode but before the global idempotency lookup.
func validateP2CanonicalRequestSchema(operation AuthIdempotencyOperation, canonical []byte) error {
	rejectNull := func(values ...json.RawMessage) error {
		for _, value := range values {
			if len(value) != 0 && bytes.Equal(value, []byte("null")) {
				return fmt.Errorf("invalid_argument: request JSON contains an explicit null")
			}
		}
		return nil
	}
	switch operation {
	case AuthOperationBootstrapOptions:
		var raw struct {
			Anchor struct {
				KeyEnvelopeAssertion struct {
					SealedAt string `json:"sealedAt"`
				} `json:"keyEnvelopeAssertion"`
			} `json:"anchor"`
		}
		if err := json.Unmarshal(canonical, &raw); err != nil {
			return fmt.Errorf("invalid_argument: bootstrap anchor JSON is invalid")
		}
		if _, err := transport.ParseUTCDateTime(raw.Anchor.KeyEnvelopeAssertion.SealedAt); err != nil {
			return fmt.Errorf("invalid_argument: bootstrap sealedAt is invalid")
		}
	case AuthOperationBootstrapVerify, AuthOperationPasskeyRegistrationVerify:
		var raw struct {
			Credential struct {
				AuthenticatorAttachment json.RawMessage `json:"authenticatorAttachment"`
				Response                struct {
					Transports json.RawMessage `json:"transports"`
				} `json:"response"`
			} `json:"credential"`
		}
		if err := json.Unmarshal(canonical, &raw); err != nil {
			return fmt.Errorf("invalid_argument: registration credential JSON is invalid")
		}
		if err := rejectNull(raw.Credential.AuthenticatorAttachment, raw.Credential.Response.Transports); err != nil {
			return err
		}
	case AuthOperationPasskeyLoginVerify, AuthOperationUVVerify:
		var raw struct {
			Credential struct {
				AuthenticatorAttachment json.RawMessage `json:"authenticatorAttachment"`
				Response                struct {
					UserHandle json.RawMessage `json:"userHandle"`
				} `json:"response"`
			} `json:"credential"`
		}
		if err := json.Unmarshal(canonical, &raw); err != nil {
			return fmt.Errorf("invalid_argument: assertion credential JSON is invalid")
		}
		if err := rejectNull(raw.Credential.AuthenticatorAttachment, raw.Credential.Response.UserHandle); err != nil {
			return err
		}
	}
	return nil
}

func rawBrowserSessionToken(request *http.Request) ([]byte, error) {
	if len(request.Header.Values("Cookie")) != 1 {
		return nil, fmt.Errorf("unauthenticated")
	}
	matching := 0
	for _, candidate := range request.Cookies() {
		if candidate.Name == browserSessionCookieName {
			matching++
		}
	}
	if matching != 1 {
		return nil, fmt.Errorf("unauthenticated")
	}
	cookie, err := request.Cookie(browserSessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil, fmt.Errorf("unauthenticated")
	}
	raw, err := transport.DecodeBase64URLFixed(cookie.Value, 32)
	if err != nil {
		return nil, fmt.Errorf("unauthenticated")
	}
	return raw, nil
}

func (s *Server) authorizeBrowserWinnerTx(ctx context.Context, conn *sql.Conn, prepared *preparedP2Mutation, requireNormal bool, now time.Time) (BrowserSession, error) {
	if conn == nil || prepared == nil || len(prepared.RawSession) != 32 || len(prepared.RawCSRF) != 32 {
		return BrowserSession{}, fmt.Errorf("unauthenticated")
	}
	session, err := browserSessionByToken(ctx, conn, prepared.Request.ActorIdentity[:], now)
	if err != nil {
		return BrowserSession{}, fmt.Errorf("unauthenticated")
	}
	derived := deriveSessionCSRF(prepared.RawSession, s.config.PublicOrigin, session.ID, session.CSRFGeneration)
	defer zeroBytes(derived)
	storedValid := s.store.ValidateCSRFIntegrity(session, derived)
	submittedValid := hmac.Equal(prepared.RawCSRF, derived)
	if !storedValid || !submittedValid {
		if revokeErr := revokeSessionIntegrityFailure(ctx, conn, session.ID, session.Revision, now); revokeErr != nil {
			return BrowserSession{}, revokeErr
		}
		return BrowserSession{}, commitAuthSecurityFailure(ErrSessionIntegrityInvalid)
	}
	session, err = touchBrowserSession(ctx, conn, session.ID, session.ActivityRevision, now)
	if err != nil {
		return BrowserSession{}, err
	}
	if requireNormal {
		if err := requireNormalBrowserSession(session); err != nil {
			return BrowserSession{}, commitAuthPrefixFailure(err)
		}
	}
	return session, nil
}

func (s *Server) validateBrowserReplayCSRF(ctx context.Context, prepared *preparedP2Mutation) error {
	_, err := s.browserReplaySession(ctx, prepared)
	return err
}

func (s *Server) browserReplaySession(ctx context.Context, prepared *preparedP2Mutation) (BrowserSession, error) {
	if prepared == nil || len(prepared.RawSession) != 32 || len(prepared.RawCSRF) != 32 {
		return BrowserSession{}, fmt.Errorf("unauthenticated")
	}
	securityCtx, cancelSecurity := detachedAuthSecurityContext(ctx)
	defer cancelSecurity()
	session, err := storedBrowserSessionByToken(securityCtx, s.store.db, prepared.Request.ActorIdentity[:])
	if err != nil {
		return BrowserSession{}, fmt.Errorf("unauthenticated")
	}
	derived := deriveSessionCSRF(prepared.RawSession, s.config.PublicOrigin, session.ID, session.CSRFGeneration)
	defer zeroBytes(derived)
	storedValid := s.store.ValidateCSRFIntegrity(session, derived)
	submittedValid := hmac.Equal(prepared.RawCSRF, derived)
	if !storedValid || !submittedValid {
		if revokeErr := s.revokeSessionIntegrityDurably(ctx, session); revokeErr != nil {
			return BrowserSession{}, revokeErr
		}
		return BrowserSession{}, ErrSessionIntegrityInvalid
	}
	return session, nil
}

const authSecurityFinalizeTimeout = 2 * time.Second

func detachedAuthSecurityContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(context.WithoutCancel(ctx), authSecurityFinalizeTimeout)
}

// revokeSessionIntegrityDurably ensures a discovered integrity failure reaches
// its state-revision CAS before an HTTP cancellation can release the handler.
// Callers must propagate an error: reporting integrity_invalid without the
// durable security transition would overstate the resulting authority state.
func (s *Server) revokeSessionIntegrityDurably(ctx context.Context, session BrowserSession) error {
	securityCtx, cancelSecurity := detachedAuthSecurityContext(ctx)
	defer cancelSecurity()
	return s.store.RevokeSessionIntegrityFailure(securityCtx, session.ID, session.Revision, s.now())
}

func (s *Server) validateBrowserBeginReplay(ctx context.Context, record AuthIdempotencyRecord, prepared *preparedP2Mutation) error {
	session, err := s.browserReplaySession(ctx, prepared)
	if err != nil {
		return err
	}
	now := s.now()
	if session.RevokedAt != nil || !session.ExpiresAt.After(now) || !session.IdleExpiresAt.After(now) {
		return ErrCeremonyRestartRequired
	}
	kind := ceremonyPasskeyRegistration
	if record.Operation == AuthOperationUVOptions {
		kind = ceremonyRecentUV
	} else if record.Operation != AuthOperationPasskeyRegistrationOptions {
		return fmt.Errorf("stored browser begin operation is invalid")
	}
	ceremony, err := s.webauthn.Ceremonies.Load(ctx, record.CeremonyID, kind, now)
	if err != nil || ceremony.BrowserSessionID != session.ID || ceremony.BrowserSessionCSRFGeneration != session.CSRFGeneration {
		return ErrCeremonyRestartRequired
	}
	return nil
}

func authOperationReceipt(operationID string, at time.Time, resourceID *string) AuthOperationReceiptV1 {
	at = normalizeServerTime(at)
	return AuthOperationReceiptV1{
		OperationId: operationID, State: generatedapi.Committed, CommittedAt: at, ResourceId: resourceID,
		CookieOutcome:        generatedapi.AuthOperationReceiptV1CookieOutcomeNone,
		CsrfOutcome:          generatedapi.AuthOperationReceiptV1CsrfOutcomeNone,
		RecoveryCodesOutcome: generatedapi.AuthOperationReceiptV1RecoveryCodesOutcomeNone,
		NextAction:           generatedapi.AuthOperationReceiptV1NextActionNone,
	}
}

func decodeAuthOperationReceipt(record AuthIdempotencyRecord) (AuthOperationReceiptV1, error) {
	var receipt AuthOperationReceiptV1
	if len(record.PublicReceiptJSON) == 0 {
		return AuthOperationReceiptV1{}, fmt.Errorf("stored auth operation receipt is invalid")
	}
	if json.Unmarshal(record.PublicReceiptJSON, &receipt) != nil || receipt.OperationId == "" {
		var envelope struct {
			Receipt AuthOperationReceiptV1 `json:"receipt"`
		}
		if json.Unmarshal(record.PublicReceiptJSON, &envelope) != nil {
			return AuthOperationReceiptV1{}, fmt.Errorf("stored auth operation receipt is invalid")
		}
		receipt = envelope.Receipt
	}
	if receipt.OperationId != record.OperationID || receipt.Operation != record.Operation || !receipt.State.Valid() {
		return AuthOperationReceiptV1{}, fmt.Errorf("stored auth operation receipt is invalid")
	}
	return receipt, nil
}

func decodeAuthPublicResult(record AuthIdempotencyRecord, destination any) error {
	var envelope struct {
		Receipt AuthOperationReceiptV1 `json:"receipt"`
		Result  json.RawMessage        `json:"result"`
	}
	if destination == nil || json.Unmarshal(record.PublicReceiptJSON, &envelope) != nil || len(envelope.Result) == 0 {
		return fmt.Errorf("stored public auth result is invalid")
	}
	if envelope.Receipt.OperationId != record.OperationID || envelope.Receipt.Operation != record.Operation || !envelope.Receipt.State.Valid() {
		return fmt.Errorf("stored public auth receipt is invalid")
	}
	if err := json.Unmarshal(envelope.Result, destination); err != nil {
		return fmt.Errorf("stored public auth result is invalid")
	}
	return nil
}

func (s *Server) writeSecretReplay(writer http.ResponseWriter, record AuthIdempotencyRecord) {
	receipt, err := decodeAuthOperationReceipt(record)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	safeErrorDetails(writer, http.StatusConflict, "one_time_result_unavailable", "one-time result is unavailable", writer.Header().Get("X-Request-ID"), generatedapi.AuthReceiptErrorDetailsV1{Receipt: receipt})
}

func (s *Server) replayAuthOptions(ctx context.Context, record AuthIdempotencyRecord, destination any) error {
	var kind ceremonyKind
	switch record.Operation {
	case AuthOperationBootstrapOptions, AuthOperationPasskeyRegistrationOptions:
		kind = ceremonyPasskeyRegistration
		if record.Operation == AuthOperationBootstrapOptions {
			kind = ceremonyBootstrapRegistration
		}
	case AuthOperationPasskeyLoginOptions:
		kind = ceremonyPasskeyLogin
	case AuthOperationUVOptions:
		kind = ceremonyRecentUV
	default:
		return fmt.Errorf("stored auth options operation is invalid")
	}
	ceremony, err := s.webauthn.Ceremonies.Load(ctx, record.CeremonyID, kind, s.now())
	if err != nil {
		return ErrCeremonyRestartRequired
	}
	var value any
	switch record.Operation {
	case AuthOperationBootstrapOptions:
		if !s.bootstrap.hasEphemeral(record.CeremonyID) {
			return ErrCeremonyRestartRequired
		}
		value = ceremony.BootstrapChallenge
	case AuthOperationPasskeyRegistrationOptions:
		value = ceremony.CreationOptions
	case AuthOperationPasskeyLoginOptions, AuthOperationUVOptions:
		value = ceremony.RequestOptions
	}
	if value == nil || len(record.PublicOptionsDigest) != sha256.Size {
		return ErrCeremonyRestartRequired
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("stored auth options are invalid")
	}
	digest := sha256.Sum256(encoded)
	if !hmac.Equal(digest[:], record.PublicOptionsDigest) {
		return fmt.Errorf("stored auth options digest is invalid")
	}
	if err := json.Unmarshal(encoded, destination); err != nil {
		return fmt.Errorf("stored auth options are invalid")
	}
	return nil
}
