package controlplane

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	generatedapi "github.com/jinlong17/multi-agent-desk/internal/controlplane/api/generated"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

const browserSessionCookieName = "__Host-mad_session"

func (s *Server) mountP2(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/bootstrap/status", s.bootstrapStatus)
	mux.HandleFunc("POST /v1/bootstrap/options", s.bootstrapOptions)
	mux.HandleFunc("POST /v1/bootstrap/verify", s.bootstrapVerify)
	mux.HandleFunc("GET /v1/bootstrap/ceremonies/{ceremonyId}", s.bootstrapCeremony)
	mux.HandleFunc("GET /v1/auth/current", s.authCurrent)
	mux.HandleFunc("POST /v1/auth/logout", s.authLogout)
	mux.HandleFunc("POST /v1/auth/passkeys/options", s.passkeyAuthenticationOptions)
	mux.HandleFunc("POST /v1/auth/passkeys/verify", s.passkeyAuthenticationVerify)
	mux.HandleFunc("POST /v1/auth/passkeys/registration/options", s.passkeyRegistrationOptions)
	mux.HandleFunc("POST /v1/auth/passkeys/registration/verify", s.passkeyRegistrationVerify)
	mux.HandleFunc("GET /v1/auth/passkeys", s.passkeyList)
	mux.HandleFunc("DELETE /v1/auth/passkeys/{passkeyId}", s.passkeyDelete)
	mux.HandleFunc("POST /v1/auth/uv/options", s.uvOptions)
	mux.HandleFunc("POST /v1/auth/uv/verify", s.uvVerify)
	mux.HandleFunc("POST /v1/auth/recovery/verify", s.recoveryVerify)
	mux.HandleFunc("POST /v1/auth/recovery-codes/rotate", s.recoveryRotate)
	mux.HandleFunc("GET /v1/auth/sessions", s.browserSessionList)
	mux.HandleFunc("DELETE /v1/auth/sessions/{sessionId}", s.browserSessionDelete)
}

func (s *Server) bootstrapStatus(writer http.ResponseWriter, request *http.Request) {
	state, err := s.store.BootstrapState(request.Context(), time.Now().UTC())
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	value := generatedapi.BootstrapStatusV1{State: generatedapi.Uninitialized}
	if state.Initialized {
		value.State = generatedapi.Initialized
	} else if state.InProgress {
		value.State = generatedapi.InProgress
		value.ExpiresAt = state.ExpiresAt
	}
	writeJSON(writer, http.StatusOK, generatedapi.BootstrapStatusEnvelopeV1{ApiVersion: generatedapi.BootstrapStatusEnvelopeV1ApiVersionV1, Data: value, Meta: responseMeta(writer)})
}

func (s *Server) bootstrapOptions(writer http.ResponseWriter, request *http.Request) {
	if err := s.requirePreAuthMutation(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if _, err := requireIdempotencyKey(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	token, err := bootstrapAuthorization(request)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	var body generatedapi.BootstrapOptionsRequestV1
	if err := transport.DecodeStrictJSON(request.Body, 64<<10, &body); err != nil {
		s.writeSafeError(writer, fmt.Errorf("invalid_argument: bootstrap request JSON is invalid"))
		return
	}
	result, err := s.bootstrap.Begin(request.Context(), token, body)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, generatedapi.BootstrapChallengeEnvelopeV1{ApiVersion: generatedapi.BootstrapChallengeEnvelopeV1ApiVersionV1, Data: result, Meta: responseMeta(writer)})
}

func (s *Server) bootstrapVerify(writer http.ResponseWriter, request *http.Request) {
	if err := s.requirePreAuthMutation(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if _, err := requireIdempotencyKey(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	token, err := bootstrapAuthorization(request)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	var body generatedapi.BootstrapVerifyRequestV1
	if err := transport.DecodeStrictJSON(request.Body, 384<<10, &body); err != nil {
		s.writeSafeError(writer, fmt.Errorf("invalid_argument: bootstrap verification JSON is invalid"))
		return
	}
	result, err := s.bootstrap.Verify(request.Context(), token, body)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer result.RecoveryCodes.ZeroPlaintext()
	defer zeroBytes(result.Session.RawToken)
	defer zeroBytes(result.Session.RawCSRF)
	s.setSessionCookie(writer, result.Session.RawToken, result.Session.ExpiresAt)
	data := generatedapi.BootstrapResultV1{CurrentAuth: result.CurrentAuth, Receipt: result.Receipt, RecoveryCodes: generatedapi.RecoveryCodesResultV1{BatchId: result.RecoveryCodes.BatchID, Codes: result.RecoveryCodes.Plaintext, GeneratedAt: result.RecoveryCodes.GeneratedAt}}
	writeJSON(writer, http.StatusOK, generatedapi.BootstrapResultEnvelopeV1{ApiVersion: generatedapi.BootstrapResultEnvelopeV1ApiVersionV1, Data: data, Meta: responseMeta(writer)})
}

func (s *Server) bootstrapCeremony(writer http.ResponseWriter, request *http.Request) {
	if !s.ceremonyLimiter.Allow(s.requestSource(request)) {
		s.writeSafeError(writer, fmt.Errorf("rate_limited"))
		return
	}
	id := request.PathValue("ceremonyId")
	if _, err := transport.ParseUUIDv7(id); err != nil {
		s.writeSafeError(writer, fmt.Errorf("invalid_argument: ceremony ID is invalid"))
		return
	}
	if challenge, err := s.webauthn.Ceremonies.BootstrapChallenge(request.Context(), id, time.Now().UTC()); err == nil {
		var data generatedapi.BootstrapCeremonyV1
		_ = data.FromBootstrapAnchorChallengeV1(challenge)
		writeJSON(writer, http.StatusOK, generatedapi.BootstrapCeremonyEnvelopeV1{ApiVersion: generatedapi.BootstrapCeremonyEnvelopeV1ApiVersionV1, Data: data, Meta: responseMeta(writer)})
		return
	}
	receiptJSON, err := s.store.BootstrapReceipt(request.Context(), id)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	var receipt generatedapi.BootstrapCommitReceiptV1
	if err := transport.DecodeStrictJSON(strings.NewReader(string(receiptJSON)), 4096, &receipt); err != nil {
		s.writeSafeError(writer, fmt.Errorf("stored bootstrap receipt is invalid"))
		return
	}
	var data generatedapi.BootstrapCeremonyV1
	_ = data.FromBootstrapCommitReceiptV1(receipt)
	writeJSON(writer, http.StatusOK, generatedapi.BootstrapCeremonyEnvelopeV1{ApiVersion: generatedapi.BootstrapCeremonyEnvelopeV1ApiVersionV1, Data: data, Meta: responseMeta(writer)})
}

func (s *Server) authCurrent(writer http.ResponseWriter, request *http.Request) {
	session, rawToken, err := s.authenticate(request)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer zeroBytes(rawToken)
	if err := s.requireAuthenticatedReadHeaders(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	csrf := deriveSessionCSRF(rawToken, session.ID)
	defer zeroBytes(csrf)
	if !s.store.ValidateCSRF(session, csrf) {
		s.writeSafeError(writer, fmt.Errorf("browser session CSRF state is invalid"))
		return
	}
	writeJSON(writer, http.StatusOK, generatedapi.CurrentAuthEnvelope{ApiVersion: generatedapi.CurrentAuthEnvelopeApiVersionV1, Data: currentAuthFromStored(session, csrf), Meta: responseMeta(writer)})
}

func (s *Server) passkeyAuthenticationOptions(writer http.ResponseWriter, request *http.Request) {
	if err := s.requirePreAuthMutation(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if _, err := requireIdempotencyKey(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if err := decodeEmptyJSON(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	result, err := s.auth.BeginLogin(request.Context())
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, generatedapi.WebAuthnRequestOptionsEnvelopeV1{ApiVersion: generatedapi.WebAuthnRequestOptionsEnvelopeV1ApiVersionV1, Data: result, Meta: responseMeta(writer)})
}

func (s *Server) passkeyAuthenticationVerify(writer http.ResponseWriter, request *http.Request) {
	if err := s.requirePreAuthMutation(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if _, err := requireIdempotencyKey(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	var body generatedapi.WebAuthnAssertionVerifyRequestV1
	if err := transport.DecodeStrictJSON(request.Body, 256<<10, &body); err != nil {
		s.writeSafeError(writer, fmt.Errorf("invalid_argument: assertion JSON is invalid"))
		return
	}
	result, err := s.auth.VerifyLogin(request.Context(), body)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer zeroBytes(result.Session.RawToken)
	defer zeroBytes(result.Session.RawCSRF)
	s.setSessionCookie(writer, result.Session.RawToken, result.Session.ExpiresAt)
	writeJSON(writer, http.StatusOK, generatedapi.CurrentAuthEnvelope{ApiVersion: generatedapi.CurrentAuthEnvelopeApiVersionV1, Data: result.CurrentAuth, Meta: responseMeta(writer)})
}

func (s *Server) passkeyRegistrationOptions(writer http.ResponseWriter, request *http.Request) {
	session, _, err := s.requireAuthenticatedMutation(request)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if err := decodeEmptyJSON(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	result, err := s.auth.BeginRegistration(request.Context(), session)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, generatedapi.WebAuthnCreationOptionsEnvelopeV1{ApiVersion: generatedapi.WebAuthnCreationOptionsEnvelopeV1ApiVersionV1, Data: result, Meta: responseMeta(writer)})
}

func (s *Server) passkeyRegistrationVerify(writer http.ResponseWriter, request *http.Request) {
	session, _, err := s.requireAuthenticatedMutation(request)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	var body generatedapi.WebAuthnRegistrationVerifyRequestV1
	if err := transport.DecodeStrictJSON(request.Body, 384<<10, &body); err != nil {
		s.writeSafeError(writer, fmt.Errorf("invalid_argument: registration JSON is invalid"))
		return
	}
	result, err := s.auth.VerifyRegistration(request.Context(), session, body)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer zeroBytes(result.Session.RawToken)
	defer zeroBytes(result.Session.RawCSRF)
	s.setSessionCookie(writer, result.Session.RawToken, result.Session.ExpiresAt)
	writeJSON(writer, http.StatusOK, generatedapi.CurrentAuthEnvelope{ApiVersion: generatedapi.CurrentAuthEnvelopeApiVersionV1, Data: result.CurrentAuth, Meta: responseMeta(writer)})
}

func (s *Server) uvOptions(writer http.ResponseWriter, request *http.Request) {
	session, _, err := s.requireAuthenticatedMutation(request)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if err := requireNormalBrowserSession(session); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if err := decodeEmptyJSON(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	result, err := s.auth.BeginUV(request.Context(), session)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, generatedapi.WebAuthnRequestOptionsEnvelopeV1{ApiVersion: generatedapi.WebAuthnRequestOptionsEnvelopeV1ApiVersionV1, Data: result, Meta: responseMeta(writer)})
}

func (s *Server) uvVerify(writer http.ResponseWriter, request *http.Request) {
	session, _, err := s.requireAuthenticatedMutation(request)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if err := requireNormalBrowserSession(session); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	var body generatedapi.WebAuthnAssertionVerifyRequestV1
	if err := transport.DecodeStrictJSON(request.Body, 256<<10, &body); err != nil {
		s.writeSafeError(writer, fmt.Errorf("invalid_argument: assertion JSON is invalid"))
		return
	}
	result, err := s.auth.VerifyUV(request.Context(), session, body)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer zeroBytes(result.Session.RawToken)
	defer zeroBytes(result.Session.RawCSRF)
	s.setSessionCookie(writer, result.Session.RawToken, result.Session.ExpiresAt)
	writeJSON(writer, http.StatusOK, generatedapi.CurrentAuthEnvelope{ApiVersion: generatedapi.CurrentAuthEnvelopeApiVersionV1, Data: result.CurrentAuth, Meta: responseMeta(writer)})
}

func (s *Server) recoveryVerify(writer http.ResponseWriter, request *http.Request) {
	if err := s.requirePreAuthMutation(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if _, err := requireIdempotencyKey(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	var body generatedapi.RecoveryVerifyRequestV1
	if err := transport.DecodeStrictJSON(request.Body, 4096, &body); err != nil {
		s.writeSafeError(writer, ErrRecoveryInvalidOrRateLimited)
		return
	}
	result, err := s.auth.VerifyRecovery(request.Context(), s.recoveryLimiter, s.requestSource(request), body.Code)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer zeroBytes(result.Session.RawToken)
	defer zeroBytes(result.Session.RawCSRF)
	s.setSessionCookie(writer, result.Session.RawToken, result.Session.ExpiresAt)
	writeJSON(writer, http.StatusOK, generatedapi.CurrentAuthEnvelope{ApiVersion: generatedapi.CurrentAuthEnvelopeApiVersionV1, Data: result.CurrentAuth, Meta: responseMeta(writer)})
}

func (s *Server) recoveryRotate(writer http.ResponseWriter, request *http.Request) {
	session, idempotencyKey, err := s.requireAuthenticatedMutation(request)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if err := requireNormalBrowserSession(session); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if err := decodeEmptyJSON(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	framed, _ := transport.Frame([]byte("multidesk-one-time-operation-v1"), []byte(session.UserID), []byte("recovery_rotate"), []byte(idempotencyKey))
	scope := sha256.Sum256(framed)
	result, err := s.auth.RotateRecoveryCodes(request.Context(), session, scope)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer result.ZeroPlaintext()
	writeJSON(writer, http.StatusOK, generatedapi.RecoveryCodesEnvelopeV1{ApiVersion: generatedapi.RecoveryCodesEnvelopeV1ApiVersionV1, Data: generatedapi.RecoveryCodesResultV1{BatchId: result.BatchID, Codes: result.Plaintext, GeneratedAt: result.GeneratedAt}, Meta: responseMeta(writer)})
}

func (s *Server) passkeyList(writer http.ResponseWriter, request *http.Request) {
	session, rawToken, err := s.authenticate(request)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer zeroBytes(rawToken)
	if err := s.requireAuthenticatedReadHeaders(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if err := requireNormalBrowserSession(session); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	values, err := s.store.ListPasskeys(request.Context(), session.UserID)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	result := make([]generatedapi.PasskeyV1, 0, len(values))
	maxRevision := 1
	for _, value := range values {
		var transportNames []generatedapi.PasskeyV1Transports
		_ = json.Unmarshal(value.TransportsJSON, &transportNames)
		result = append(result, generatedapi.PasskeyV1{Id: value.ID, Name: value.Name, CreatedAt: value.CreatedAt, LastUsedAt: value.LastUsedAt, Transports: transportNames, CredentialRevision: int(value.CredentialRevision), Current: value.ID == session.AuthenticationPasskeyID})
		if int(value.CredentialRevision) > maxRevision {
			maxRevision = int(value.CredentialRevision)
		}
	}
	writeJSON(writer, http.StatusOK, generatedapi.PasskeyListEnvelopeV1{ApiVersion: generatedapi.PasskeyListEnvelopeV1ApiVersionV1, Data: generatedapi.PasskeyListResultV1{Passkeys: result, Revision: maxRevision}, Meta: responseMeta(writer)})
}

func (s *Server) passkeyDelete(writer http.ResponseWriter, request *http.Request) {
	session, _, err := s.requireAuthenticatedMutation(request)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if err := requireNormalBrowserSession(session); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if err := decodeEmptyJSON(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	now := time.Now().UTC()
	if session.AuthenticationMethod != "passkey" || session.RecentUVAt == nil || now.Sub(*session.RecentUVAt) < 0 || now.Sub(*session.RecentUVAt) > 5*time.Minute {
		s.writeSafeError(writer, fmt.Errorf("recent user verification is required"))
		return
	}
	revision, err := parseIfMatch(request)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	passkeyID := request.PathValue("passkeyId")
	if _, err := transport.ParseUUIDv7(passkeyID); err != nil {
		s.writeSafeError(writer, fmt.Errorf("invalid_argument: Passkey ID is invalid"))
		return
	}
	result, err := s.store.DeletePasskeyCAS(request.Context(), session.UserID, passkeyID, session.ID, revision, now)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if result.CurrentSessionRevoked {
		s.clearSessionCookie(writer)
	}
	writeJSON(writer, http.StatusOK, generatedapi.PasskeyDeleteEnvelopeV1{ApiVersion: generatedapi.PasskeyDeleteEnvelopeV1ApiVersionV1, Data: generatedapi.PasskeyDeleteResultV1{PasskeyId: passkeyID, RevokedSessionCount: result.RevokedSessionCount, CurrentSessionRevoked: result.CurrentSessionRevoked}, Meta: responseMeta(writer)})
}

func (s *Server) browserSessionList(writer http.ResponseWriter, request *http.Request) {
	current, rawToken, err := s.authenticate(request)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer zeroBytes(rawToken)
	if err := s.requireAuthenticatedReadHeaders(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if err := requireNormalBrowserSession(current); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	values, err := s.store.ListBrowserSessions(request.Context(), current.UserID, time.Now().UTC())
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	result := make([]generatedapi.BrowserSessionV1, 0, len(values))
	maxRevision := 1
	for _, value := range values {
		result = append(result, generatedapi.BrowserSessionV1{Id: value.ID, AuthenticationMethod: generatedapi.BrowserSessionV1AuthenticationMethod(value.AuthenticationMethod), CreatedAt: value.CreatedAt, LastSeenAt: value.UpdatedAt, ExpiresAt: value.ExpiresAt, Current: value.ID == current.ID})
		if int(value.Revision) > maxRevision {
			maxRevision = int(value.Revision)
		}
	}
	writeJSON(writer, http.StatusOK, generatedapi.BrowserSessionListEnvelopeV1{ApiVersion: generatedapi.BrowserSessionListEnvelopeV1ApiVersionV1, Data: generatedapi.BrowserSessionListResultV1{Sessions: result, Revision: maxRevision}, Meta: responseMeta(writer)})
}

func (s *Server) browserSessionDelete(writer http.ResponseWriter, request *http.Request) {
	current, _, err := s.requireAuthenticatedMutation(request)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if err := requireNormalBrowserSession(current); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if err := decodeEmptyJSON(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	revision, err := parseIfMatch(request)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	id := request.PathValue("sessionId")
	if _, err := transport.ParseUUIDv7(id); err != nil {
		s.writeSafeError(writer, fmt.Errorf("invalid_argument: session ID is invalid"))
		return
	}
	if _, err := s.store.RevokeBrowserSession(request.Context(), id, revision, time.Now().UTC()); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if id == current.ID {
		s.clearSessionCookie(writer)
	}
	writeJSON(writer, http.StatusOK, generatedapi.OperationStatusEnvelope{ApiVersion: generatedapi.OperationStatusEnvelopeApiVersionV1, Data: generatedapi.OperationStatusData{Status: generatedapi.OperationStatusDataStatusRevoked, Id: &id}, Meta: responseMeta(writer)})
}

func (s *Server) authLogout(writer http.ResponseWriter, request *http.Request) {
	current, _, err := s.requireAuthenticatedMutation(request)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if err := decodeEmptyJSON(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if _, err := s.store.RevokeBrowserSession(request.Context(), current.ID, current.Revision, time.Now().UTC()); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	s.clearSessionCookie(writer)
	writeJSON(writer, http.StatusOK, generatedapi.OperationStatusEnvelope{ApiVersion: generatedapi.OperationStatusEnvelopeApiVersionV1, Data: generatedapi.OperationStatusData{Status: generatedapi.OperationStatusDataStatusRevoked, Id: &current.ID}, Meta: responseMeta(writer)})
}

func (s *Server) authenticate(request *http.Request) (BrowserSession, []byte, error) {
	if len(request.Header.Values("Cookie")) != 1 {
		return BrowserSession{}, nil, fmt.Errorf("unauthenticated")
	}
	matching := 0
	for _, candidate := range request.Cookies() {
		if candidate.Name == browserSessionCookieName {
			matching++
		}
	}
	if matching != 1 {
		return BrowserSession{}, nil, fmt.Errorf("unauthenticated")
	}
	cookie, err := request.Cookie(browserSessionCookieName)
	if err != nil || cookie.Value == "" {
		return BrowserSession{}, nil, fmt.Errorf("unauthenticated")
	}
	raw, err := transport.DecodeBase64URLFixed(cookie.Value, 32)
	if err != nil {
		return BrowserSession{}, nil, fmt.Errorf("unauthenticated")
	}
	session, err := s.store.BrowserSessionByToken(request.Context(), raw, time.Now().UTC())
	if err != nil {
		zeroBytes(raw)
		return BrowserSession{}, nil, fmt.Errorf("unauthenticated")
	}
	return session, raw, nil
}

func (s *Server) requireAuthenticatedMutation(request *http.Request) (BrowserSession, string, error) {
	if err := s.requireBrowserMutationHeaders(request); err != nil {
		return BrowserSession{}, "", err
	}
	key, err := requireIdempotencyKey(request)
	if err != nil {
		return BrowserSession{}, "", err
	}
	session, rawToken, err := s.authenticate(request)
	zeroBytes(rawToken)
	if err != nil {
		return BrowserSession{}, "", err
	}
	csrfValues := request.Header.Values("X-CSRF-Token")
	if len(csrfValues) != 1 {
		return BrowserSession{}, "", fmt.Errorf("csrf_invalid")
	}
	rawCSRF, err := transport.DecodeBase64URLFixed(csrfValues[0], 32)
	if err != nil || !s.store.ValidateCSRF(session, rawCSRF) {
		zeroBytes(rawCSRF)
		return BrowserSession{}, "", fmt.Errorf("csrf_invalid")
	}
	zeroBytes(rawCSRF)
	return session, key, nil
}

func requireNormalBrowserSession(session BrowserSession) error {
	if session.AuthenticationMethod != "passkey" {
		return fmt.Errorf("permission_denied: recovery sessions may only register a replacement Passkey")
	}
	return nil
}

func (s *Server) requirePreAuthMutation(request *http.Request) error {
	if err := s.requireBrowserMutationHeaders(request); err != nil {
		return err
	}
	if !s.preAuthLimiter.Allow(s.requestSource(request)) {
		return fmt.Errorf("rate_limited")
	}
	return nil
}

func (s *Server) requireBrowserMutationHeaders(request *http.Request) error {
	origin, ok := exactHeader(request, "Origin")
	if !ok || origin != s.config.PublicOrigin {
		return fmt.Errorf("origin_or_fetch_metadata_invalid")
	}
	site, siteOK := exactHeader(request, "Sec-Fetch-Site")
	mode, modeOK := exactHeader(request, "Sec-Fetch-Mode")
	destination, destinationOK := exactHeader(request, "Sec-Fetch-Dest")
	if !siteOK || !modeOK || !destinationOK || site != "same-origin" || (mode != "cors" && mode != "same-origin") || destination != "empty" {
		return fmt.Errorf("origin_or_fetch_metadata_invalid")
	}
	contentType, ok := exactHeader(request, "Content-Type")
	if !ok || contentType != "application/json" {
		return fmt.Errorf("content_type_invalid")
	}
	return nil
}

func (s *Server) requireAuthenticatedReadHeaders(request *http.Request) error {
	site, siteOK := exactHeader(request, "Sec-Fetch-Site")
	mode, modeOK := exactHeader(request, "Sec-Fetch-Mode")
	destination, destinationOK := exactHeader(request, "Sec-Fetch-Dest")
	if !siteOK || !modeOK || !destinationOK || site != "same-origin" || (mode != "cors" && mode != "same-origin") || destination != "empty" {
		return fmt.Errorf("origin_or_fetch_metadata_invalid")
	}
	if values := request.Header.Values("Origin"); len(values) > 1 || (len(values) == 1 && values[0] != s.config.PublicOrigin) {
		return fmt.Errorf("origin_or_fetch_metadata_invalid")
	}
	return nil
}

func exactHeader(request *http.Request, name string) (string, bool) {
	values := request.Header.Values(name)
	returnValue := ""
	if len(values) == 1 {
		returnValue = values[0]
	}
	return returnValue, len(values) == 1
}

func decodeEmptyJSON(request *http.Request) error {
	var value struct{}
	if err := transport.DecodeStrictJSON(request.Body, 16, &value); err != nil {
		return fmt.Errorf("invalid_argument: request body must be an empty JSON object")
	}
	return nil
}

func requireIdempotencyKey(request *http.Request) (string, error) {
	values := request.Header.Values("Idempotency-Key")
	if len(values) == 0 {
		return "", fmt.Errorf("idempotency_key_required")
	}
	if len(values) != 1 || len(values[0]) < 16 || len(values[0]) > 128 {
		return "", fmt.Errorf("idempotency_key_invalid")
	}
	for _, value := range []byte(values[0]) {
		if value < 0x21 || value > 0x7e {
			return "", fmt.Errorf("idempotency_key_invalid")
		}
	}
	return values[0], nil
}

func bootstrapAuthorization(request *http.Request) (string, error) {
	values := request.Header.Values("Authorization")
	if len(values) != 1 || !strings.HasPrefix(values[0], "Bootstrap ") || strings.Count(values[0], " ") != 1 {
		return "", fmt.Errorf("bootstrap token is invalid")
	}
	return strings.TrimPrefix(values[0], "Bootstrap "), nil
}

func parseIfMatch(request *http.Request) (int64, error) {
	values := request.Header.Values("If-Match")
	if len(values) == 0 {
		return 0, fmt.Errorf("if_match_required")
	}
	if len(values) != 1 {
		return 0, fmt.Errorf("invalid_argument: revision precondition is invalid")
	}
	parsed, err := transport.ParseIfMatch(values[0])
	if err != nil {
		return 0, fmt.Errorf("invalid_argument: revision precondition is invalid")
	}
	return int64(parsed), nil
}

func (s *Server) setSessionCookie(writer http.ResponseWriter, raw []byte, expires time.Time) {
	http.SetCookie(writer, &http.Cookie{Name: browserSessionCookieName, Value: base64.RawURLEncoding.EncodeToString(raw), Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, Expires: expires.UTC()})
}

func (s *Server) clearSessionCookie(writer http.ResponseWriter) {
	http.SetCookie(writer, &http.Cookie{Name: browserSessionCookieName, Value: "", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0).UTC()})
}

func responseMeta(writer http.ResponseWriter) generatedapi.ResponseMeta {
	return generatedapi.ResponseMeta{RequestId: writer.Header().Get("X-Request-ID"), NextCursor: nil}
}

func (s *Server) requestSource(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil || net.ParseIP(host) == nil {
		return "unknown"
	}
	remote := net.ParseIP(host)
	trusted := false
	for _, value := range s.config.TrustedProxyCIDRs {
		_, network, parseErr := net.ParseCIDR(value)
		if parseErr == nil && network.Contains(remote) {
			trusted = true
			break
		}
	}
	if trusted {
		forwarded := request.Header.Values("X-Forwarded-For")
		if len(forwarded) != 1 || strings.Contains(forwarded[0], ",") || net.ParseIP(forwarded[0]) == nil {
			return "unknown"
		}
		return net.ParseIP(forwarded[0]).String()
	}
	return remote.String()
}

func (s *Server) writeSafeError(writer http.ResponseWriter, err error) {
	code := "invalid_argument"
	status := http.StatusBadRequest
	message := "request was rejected"
	switch {
	case errors.Is(err, ErrPasskeyCounterRegressed):
		code, status, message = "passkey_counter_regressed", http.StatusUnauthorized, "Passkey verification was rejected"
	case errors.Is(err, ErrLastPasskeyRequired):
		code, status, message = "last_passkey_required", http.StatusConflict, "the last Passkey cannot be deleted"
	case errors.Is(err, ErrRevisionChanged):
		code, status, message = "conflict", http.StatusPreconditionFailed, "resource revision changed"
	case errors.Is(err, ErrRecoveryInvalidOrRateLimited), errors.Is(err, ErrRecoveryConsumed):
		code, status, message = "recovery_invalid_or_rate_limited", http.StatusUnauthorized, "recovery verification failed"
	case errors.Is(err, ErrOneTimeResultUnavailable):
		code, status, message = "one_time_result_unavailable", http.StatusConflict, "one-time result is unavailable"
	case strings.Contains(err.Error(), "unauthenticated"):
		code, status, message = "unauthenticated", http.StatusUnauthorized, "authentication is required"
	case strings.Contains(err.Error(), "csrf"):
		code, status, message = "csrf_invalid", http.StatusForbidden, "CSRF verification failed"
	case strings.Contains(err.Error(), "permission_denied"):
		code, status, message = "permission_denied", http.StatusForbidden, "the recovery session is restricted"
	case strings.Contains(err.Error(), "origin_or_fetch"):
		code, status, message = "origin_mismatch", http.StatusForbidden, "same-origin request metadata is required"
	case strings.Contains(err.Error(), "content_type"):
		code, status, message = "invalid_argument", http.StatusUnsupportedMediaType, "application/json is required"
	case strings.Contains(err.Error(), "idempotency_key_required"):
		code, status, message = "idempotency_key_required", http.StatusBadRequest, "Idempotency-Key is required"
	case strings.Contains(err.Error(), "idempotency"):
		code, status, message = "invalid_argument", http.StatusBadRequest, "a valid Idempotency-Key is required"
	case strings.Contains(err.Error(), "if_match_required"):
		code, status, message = "if_match_required", http.StatusPreconditionRequired, "If-Match is required"
	case strings.Contains(err.Error(), "recent user verification"):
		code, status, message = "recent_uv_required", http.StatusForbidden, "recent user verification is required"
	case strings.Contains(err.Error(), "rate_limited"):
		code, status, message = "rate_limited", http.StatusTooManyRequests, "request rate limit exceeded"
	case strings.Contains(err.Error(), "bootstrap token"):
		code, status, message = "bootstrap_unavailable", http.StatusUnauthorized, "bootstrap authorization failed"
	case strings.Contains(err.Error(), "already initialized"):
		code, status, message = "bootstrap_replayed", http.StatusConflict, "bootstrap is already complete"
	case strings.Contains(err.Error(), "challenge_expired"):
		code, status, message = "webauthn_challenge_expired", http.StatusConflict, "WebAuthn ceremony expired"
	case strings.Contains(err.Error(), "challenge_replayed"):
		code, status, message = "webauthn_challenge_replayed", http.StatusConflict, "WebAuthn ceremony is unavailable"
	case strings.Contains(err.Error(), "not found"):
		code, status, message = "not_found", http.StatusNotFound, "resource was not found"
	case strings.Contains(err.Error(), "WebAuthn") || strings.Contains(err.Error(), "webauthn"):
		code, status, message = "webauthn_verification_failed", http.StatusUnauthorized, "WebAuthn verification failed"
	}
	safeError(writer, status, code, message, writer.Header().Get("X-Request-ID"))
}
