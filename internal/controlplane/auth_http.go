package controlplane

import (
	"context"
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
	state, err := s.store.BootstrapState(request.Context(), s.now())
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
	var body generatedapi.BootstrapOptionsRequestV1
	prepared, err := s.prepareP2Mutation(request, AuthOperationBootstrapOptions, 64<<10, &body, func() error {
		if _, anchorErr := validateBootstrapAnchor(body.Anchor); anchorErr != nil || !jsonSchemaStringLength(body.DisplayName, 1, 128) {
			return fmt.Errorf("invalid_argument: bootstrap anchor is invalid")
		}
		return nil
	})
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer prepared.zero()
	now := s.now()
	var result generatedapi.BootstrapAnchorChallengeV1
	record, replay, err := s.store.WithAuthIdempotency(request.Context(), prepared.Request, now, func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		conn := tx.Conn
		if productErr := tx.BeginProduct(request.Context()); productErr != nil {
			return AuthIdempotencyCommit{}, productErr
		}
		var beginErr error
		result, beginErr = s.bootstrap.BeginTx(request.Context(), conn, prepared.BootstrapToken, body)
		if beginErr != nil {
			return AuthIdempotencyCommit{}, beginErr
		}
		ceremonyID := result.CeremonyId
		if hookErr := tx.OnRollback(func() { s.bootstrap.forgetEphemeral(ceremonyID) }); hookErr != nil {
			return AuthIdempotencyCommit{}, hookErr
		}
		if hookErr := tx.OnCommit(func() { s.bootstrap.armEphemeral(ceremonyID) }); hookErr != nil {
			return AuthIdempotencyCommit{}, hookErr
		}
		encoded, cacheErr := json.Marshal(result)
		if cacheErr != nil {
			return AuthIdempotencyCommit{}, cacheErr
		}
		digest := sha256.Sum256(encoded)
		resourceID := result.CeremonyId
		return AuthIdempotencyCommit{Receipt: authOperationReceipt(operationID, now, &resourceID), CeremonyID: result.CeremonyId, PublicOptionsDigest: digest[:], At: now}, nil
	})
	if err != nil {
		if result.CeremonyId != "" {
			s.bootstrap.forgetEphemeral(result.CeremonyId)
		}
		s.writeSafeError(writer, err)
		return
	}
	if replay {
		if err := s.replayAuthOptions(request.Context(), record, &result); err != nil {
			s.writeSafeError(writer, err)
			return
		}
	}
	writeJSON(writer, http.StatusOK, generatedapi.BootstrapChallengeEnvelopeV1{ApiVersion: generatedapi.BootstrapChallengeEnvelopeV1ApiVersionV1, Data: result, Meta: responseMeta(writer)})
}

func (s *Server) bootstrapVerify(writer http.ResponseWriter, request *http.Request) {
	var body generatedapi.BootstrapVerifyRequestV1
	prepared, err := s.prepareP2Mutation(request, AuthOperationBootstrapVerify, 512<<10, &body, func() error {
		if _, parseErr := transport.ParseUUIDv7(body.CeremonyId); parseErr != nil {
			return fmt.Errorf("invalid_argument: bootstrap ceremony is invalid")
		}
		if credentialErr := validateRegistrationCredential(body.Credential); credentialErr != nil {
			return credentialErr
		}
		signing, signingErr := transport.DecodeBase64URLFixed(body.SigningProof, 64)
		zeroBytes(signing)
		exchange, exchangeErr := transport.DecodeBase64URLFixed(body.ExchangeProof, 32)
		zeroBytes(exchange)
		if signingErr != nil || exchangeErr != nil {
			return fmt.Errorf("invalid_argument: bootstrap proof is invalid")
		}
		return nil
	})
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer prepared.zero()
	now := s.now()
	var result BootstrapCommitResult
	defer func() {
		result.RecoveryCodes.ZeroPlaintext()
		zeroBytes(result.Session.RawToken)
		zeroBytes(result.Session.RawCSRF)
	}()
	record, replay, err := s.store.WithAuthIdempotency(request.Context(), prepared.Request, now, func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		conn := tx.Conn
		if productErr := tx.BeginProduct(request.Context()); productErr != nil {
			return AuthIdempotencyCommit{}, productErr
		}
		if productErr := tx.ConsumeCeremonyOnFailure(body.CeremonyId, ceremonyBootstrapRegistration); productErr != nil {
			return AuthIdempotencyCommit{}, productErr
		}
		if hookErr := tx.OnFinish(func() { s.bootstrap.forgetEphemeral(body.CeremonyId) }); hookErr != nil {
			return AuthIdempotencyCommit{}, hookErr
		}
		var verifyErr error
		result, verifyErr = s.bootstrap.VerifyTx(request.Context(), conn, prepared.BootstrapToken, body)
		if verifyErr != nil {
			return AuthIdempotencyCommit{}, verifyErr
		}
		resourceID := result.Session.ID
		receipt := authOperationReceipt(operationID, now, &resourceID)
		receipt.CookieOutcome = generatedapi.AuthOperationReceiptV1CookieOutcomeIssuedNotReplayable
		receipt.CsrfOutcome = generatedapi.AuthOperationReceiptV1CsrfOutcomeIssuedNotReplayable
		receipt.RecoveryCodesOutcome = generatedapi.AuthOperationReceiptV1RecoveryCodesOutcomeIssuedNotReplayable
		receipt.NextAction = generatedapi.AuthOperationReceiptV1NextActionFreshPasskeyLogin
		return AuthIdempotencyCommit{Receipt: receipt, CookieAction: "secret_issued", At: now}, nil
	})
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if replay {
		s.writeSecretReplay(writer, record)
		return
	}
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
	if challenge, err := s.webauthn.Ceremonies.BootstrapChallenge(request.Context(), id, s.now()); err == nil {
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
	securityCtx, cancelSecurity := detachedAuthSecurityContext(request.Context())
	defer cancelSecurity()
	session, rawToken, err := s.authenticateContext(securityCtx, request)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer zeroBytes(rawToken)
	if err := s.requireAuthenticatedReadHeaders(request); err != nil {
		s.writeSafeError(writer, err)
		return
	}
	csrf := deriveSessionCSRF(rawToken, s.config.PublicOrigin, session.ID, session.CSRFGeneration)
	defer zeroBytes(csrf)
	if !s.store.ValidateCSRFIntegrity(session, csrf) {
		if err := s.revokeSessionIntegrityDurably(request.Context(), session); err != nil {
			s.writeSafeError(writer, err)
			return
		}
		s.writeSafeError(writer, ErrSessionIntegrityInvalid)
		return
	}
	session, err = s.store.TouchBrowserSession(request.Context(), session.ID, session.ActivityRevision, s.now())
	if err != nil {
		s.writeSafeError(writer, fmt.Errorf("unauthenticated"))
		return
	}
	writeJSON(writer, http.StatusOK, generatedapi.CurrentAuthEnvelope{ApiVersion: generatedapi.CurrentAuthEnvelopeApiVersionV1, Data: currentAuthFromStored(session, csrf), Meta: responseMeta(writer)})
}

func (s *Server) passkeyAuthenticationOptions(writer http.ResponseWriter, request *http.Request) {
	prepared, err := s.prepareP2Mutation(request, AuthOperationPasskeyLoginOptions, 16, nil, nil)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer prepared.zero()
	now := s.now()
	var result generatedapi.WebAuthnRequestOptionsV1
	record, replay, err := s.store.WithAuthIdempotency(request.Context(), prepared.Request, now, func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		conn := tx.Conn
		if productErr := tx.BeginProduct(request.Context()); productErr != nil {
			return AuthIdempotencyCommit{}, productErr
		}
		var beginErr error
		result, beginErr = s.auth.BeginLoginTx(request.Context(), conn)
		if beginErr != nil {
			return AuthIdempotencyCommit{}, beginErr
		}
		encoded, cacheErr := json.Marshal(result)
		if cacheErr != nil {
			return AuthIdempotencyCommit{}, cacheErr
		}
		digest := sha256.Sum256(encoded)
		resourceID := result.CeremonyId
		return AuthIdempotencyCommit{Receipt: authOperationReceipt(operationID, now, &resourceID), CeremonyID: result.CeremonyId, PublicOptionsDigest: digest[:], At: now}, nil
	})
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if replay {
		if err := s.replayAuthOptions(request.Context(), record, &result); err != nil {
			s.writeSafeError(writer, err)
			return
		}
	}
	writeJSON(writer, http.StatusOK, generatedapi.WebAuthnRequestOptionsEnvelopeV1{ApiVersion: generatedapi.WebAuthnRequestOptionsEnvelopeV1ApiVersionV1, Data: result, Meta: responseMeta(writer)})
}

func (s *Server) passkeyAuthenticationVerify(writer http.ResponseWriter, request *http.Request) {
	var body generatedapi.WebAuthnAssertionVerifyRequestV1
	prepared, err := s.prepareP2Mutation(request, AuthOperationPasskeyLoginVerify, 384<<10, &body, func() error {
		if _, parseErr := transport.ParseUUIDv7(body.CeremonyId); parseErr != nil {
			return fmt.Errorf("invalid_argument: login ceremony is invalid")
		}
		return validateAssertionCredential(body.Credential)
	})
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer prepared.zero()
	now := s.now()
	var result AuthResult
	defer func() {
		zeroBytes(result.Session.RawToken)
		zeroBytes(result.Session.RawCSRF)
	}()
	record, replay, err := s.store.WithAuthIdempotency(request.Context(), prepared.Request, now, func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		conn := tx.Conn
		if productErr := tx.BeginProduct(request.Context()); productErr != nil {
			return AuthIdempotencyCommit{}, productErr
		}
		if productErr := tx.ConsumeCeremonyOnFailure(body.CeremonyId, ceremonyPasskeyLogin); productErr != nil {
			return AuthIdempotencyCommit{}, productErr
		}
		var verifyErr error
		result, verifyErr = s.auth.VerifyLoginTx(request.Context(), conn, body)
		if verifyErr != nil {
			return AuthIdempotencyCommit{}, verifyErr
		}
		resourceID := result.Session.ID
		receipt := authOperationReceipt(operationID, now, &resourceID)
		receipt.CookieOutcome = generatedapi.AuthOperationReceiptV1CookieOutcomeIssuedNotReplayable
		receipt.CsrfOutcome = generatedapi.AuthOperationReceiptV1CsrfOutcomeIssuedNotReplayable
		receipt.NextAction = generatedapi.AuthOperationReceiptV1NextActionFreshPasskeyLogin
		return AuthIdempotencyCommit{Receipt: receipt, CookieAction: "secret_issued", At: now}, nil
	})
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if replay {
		s.writeSecretReplay(writer, record)
		return
	}
	s.setSessionCookie(writer, result.Session.RawToken, result.Session.ExpiresAt)
	writeJSON(writer, http.StatusOK, generatedapi.CurrentAuthEnvelope{ApiVersion: generatedapi.CurrentAuthEnvelopeApiVersionV1, Data: result.CurrentAuth, Meta: responseMeta(writer)})
}

func (s *Server) passkeyRegistrationOptions(writer http.ResponseWriter, request *http.Request) {
	prepared, err := s.prepareP2Mutation(request, AuthOperationPasskeyRegistrationOptions, 16, nil, nil)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer prepared.zero()
	now := s.now()
	var result generatedapi.WebAuthnCreationOptionsV1
	record, replay, err := s.store.WithAuthIdempotency(request.Context(), prepared.Request, now, func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		conn := tx.Conn
		session, authErr := s.authorizeBrowserWinnerTx(request.Context(), conn, &prepared, false, now)
		if authErr != nil {
			return AuthIdempotencyCommit{}, authErr
		}
		if productErr := tx.BeginProduct(request.Context()); productErr != nil {
			return AuthIdempotencyCommit{}, productErr
		}
		result, authErr = s.auth.BeginRegistrationTx(request.Context(), conn, session)
		if authErr != nil {
			return AuthIdempotencyCommit{}, authErr
		}
		encoded, cacheErr := json.Marshal(result)
		if cacheErr != nil {
			return AuthIdempotencyCommit{}, cacheErr
		}
		digest := sha256.Sum256(encoded)
		resourceID := result.CeremonyId
		return AuthIdempotencyCommit{Receipt: authOperationReceipt(operationID, now, &resourceID), CeremonyID: result.CeremonyId, PublicOptionsDigest: digest[:], At: now}, nil
	})
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if replay {
		if err := s.validateBrowserBeginReplay(request.Context(), record, &prepared); err != nil {
			s.writeSafeError(writer, err)
			return
		}
		if err := s.replayAuthOptions(request.Context(), record, &result); err != nil {
			s.writeSafeError(writer, err)
			return
		}
	}
	writeJSON(writer, http.StatusOK, generatedapi.WebAuthnCreationOptionsEnvelopeV1{ApiVersion: generatedapi.WebAuthnCreationOptionsEnvelopeV1ApiVersionV1, Data: result, Meta: responseMeta(writer)})
}

func (s *Server) passkeyRegistrationVerify(writer http.ResponseWriter, request *http.Request) {
	var body generatedapi.WebAuthnRegistrationVerifyRequestV1
	prepared, err := s.prepareP2Mutation(request, AuthOperationPasskeyRegistrationVerify, 512<<10, &body, func() error {
		if _, parseErr := transport.ParseUUIDv7(body.CeremonyId); parseErr != nil {
			return fmt.Errorf("invalid_argument: registration ceremony is invalid")
		}
		return validateRegistrationCredential(body.Credential)
	})
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer prepared.zero()
	now := s.now()
	var result AuthResult
	defer func() {
		zeroBytes(result.Session.RawToken)
		zeroBytes(result.Session.RawCSRF)
	}()
	record, replay, err := s.store.WithAuthIdempotency(request.Context(), prepared.Request, now, func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		conn := tx.Conn
		session, authErr := s.authorizeBrowserWinnerTx(request.Context(), conn, &prepared, false, now)
		if authErr != nil {
			return AuthIdempotencyCommit{}, authErr
		}
		if productErr := tx.BeginProduct(request.Context()); productErr != nil {
			return AuthIdempotencyCommit{}, productErr
		}
		if productErr := tx.ConsumeCeremonyOnFailure(body.CeremonyId, ceremonyPasskeyRegistration); productErr != nil {
			return AuthIdempotencyCommit{}, productErr
		}
		result, authErr = s.auth.VerifyRegistrationTx(request.Context(), conn, session, body)
		if authErr != nil {
			return AuthIdempotencyCommit{}, authErr
		}
		resourceID := result.Session.ID
		receipt := authOperationReceipt(operationID, now, &resourceID)
		receipt.CookieOutcome = generatedapi.AuthOperationReceiptV1CookieOutcomeIssuedNotReplayable
		receipt.CsrfOutcome = generatedapi.AuthOperationReceiptV1CsrfOutcomeIssuedNotReplayable
		receipt.NextAction = generatedapi.AuthOperationReceiptV1NextActionFreshPasskeyLogin
		return AuthIdempotencyCommit{Receipt: receipt, CookieAction: "secret_issued", At: now}, nil
	})
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if replay {
		if err := s.validateBrowserReplayCSRF(request.Context(), &prepared); err != nil {
			s.writeSafeError(writer, err)
			return
		}
		s.writeSecretReplay(writer, record)
		return
	}
	s.setSessionCookie(writer, result.Session.RawToken, result.Session.ExpiresAt)
	writeJSON(writer, http.StatusOK, generatedapi.CurrentAuthEnvelope{ApiVersion: generatedapi.CurrentAuthEnvelopeApiVersionV1, Data: result.CurrentAuth, Meta: responseMeta(writer)})
}

func (s *Server) uvOptions(writer http.ResponseWriter, request *http.Request) {
	prepared, err := s.prepareP2Mutation(request, AuthOperationUVOptions, 16, nil, nil)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer prepared.zero()
	now := s.now()
	var result generatedapi.WebAuthnRequestOptionsV1
	record, replay, err := s.store.WithAuthIdempotency(request.Context(), prepared.Request, now, func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		conn := tx.Conn
		session, authErr := s.authorizeBrowserWinnerTx(request.Context(), conn, &prepared, true, now)
		if authErr != nil {
			return AuthIdempotencyCommit{}, authErr
		}
		if productErr := tx.BeginProduct(request.Context()); productErr != nil {
			return AuthIdempotencyCommit{}, productErr
		}
		result, authErr = s.auth.BeginUVTx(request.Context(), conn, session)
		if authErr != nil {
			return AuthIdempotencyCommit{}, authErr
		}
		encoded, cacheErr := json.Marshal(result)
		if cacheErr != nil {
			return AuthIdempotencyCommit{}, cacheErr
		}
		digest := sha256.Sum256(encoded)
		resourceID := result.CeremonyId
		return AuthIdempotencyCommit{Receipt: authOperationReceipt(operationID, now, &resourceID), CeremonyID: result.CeremonyId, PublicOptionsDigest: digest[:], At: now}, nil
	})
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if replay {
		if err := s.validateBrowserBeginReplay(request.Context(), record, &prepared); err != nil {
			s.writeSafeError(writer, err)
			return
		}
		if err := s.replayAuthOptions(request.Context(), record, &result); err != nil {
			s.writeSafeError(writer, err)
			return
		}
	}
	writeJSON(writer, http.StatusOK, generatedapi.WebAuthnRequestOptionsEnvelopeV1{ApiVersion: generatedapi.WebAuthnRequestOptionsEnvelopeV1ApiVersionV1, Data: result, Meta: responseMeta(writer)})
}

func (s *Server) uvVerify(writer http.ResponseWriter, request *http.Request) {
	var body generatedapi.WebAuthnAssertionVerifyRequestV1
	prepared, err := s.prepareP2Mutation(request, AuthOperationUVVerify, 384<<10, &body, func() error {
		if _, parseErr := transport.ParseUUIDv7(body.CeremonyId); parseErr != nil {
			return fmt.Errorf("invalid_argument: UV ceremony is invalid")
		}
		return validateAssertionCredential(body.Credential)
	})
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer prepared.zero()
	now := s.now()
	var result AuthResult
	defer func() {
		zeroBytes(result.Session.RawToken)
		zeroBytes(result.Session.RawCSRF)
	}()
	record, replay, err := s.store.WithAuthIdempotency(request.Context(), prepared.Request, now, func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		conn := tx.Conn
		session, authErr := s.authorizeBrowserWinnerTx(request.Context(), conn, &prepared, true, now)
		if authErr != nil {
			return AuthIdempotencyCommit{}, authErr
		}
		if productErr := tx.BeginProduct(request.Context()); productErr != nil {
			return AuthIdempotencyCommit{}, productErr
		}
		if productErr := tx.ConsumeCeremonyOnFailure(body.CeremonyId, ceremonyRecentUV); productErr != nil {
			return AuthIdempotencyCommit{}, productErr
		}
		result, authErr = s.auth.VerifyUVTx(request.Context(), conn, session, body)
		if authErr != nil {
			return AuthIdempotencyCommit{}, authErr
		}
		resourceID := result.Session.ID
		receipt := authOperationReceipt(operationID, now, &resourceID)
		receipt.CookieOutcome = generatedapi.AuthOperationReceiptV1CookieOutcomeIssuedNotReplayable
		receipt.CsrfOutcome = generatedapi.AuthOperationReceiptV1CsrfOutcomeIssuedNotReplayable
		receipt.NextAction = generatedapi.AuthOperationReceiptV1NextActionFreshPasskeyLogin
		return AuthIdempotencyCommit{Receipt: receipt, CookieAction: "secret_issued", At: now}, nil
	})
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if replay {
		if err := s.validateBrowserReplayCSRF(request.Context(), &prepared); err != nil {
			s.writeSafeError(writer, err)
			return
		}
		s.writeSecretReplay(writer, record)
		return
	}
	s.setSessionCookie(writer, result.Session.RawToken, result.Session.ExpiresAt)
	writeJSON(writer, http.StatusOK, generatedapi.CurrentAuthEnvelope{ApiVersion: generatedapi.CurrentAuthEnvelopeApiVersionV1, Data: result.CurrentAuth, Meta: responseMeta(writer)})
}

func (s *Server) recoveryVerify(writer http.ResponseWriter, request *http.Request) {
	var body generatedapi.RecoveryVerifyRequestV1
	prepared, err := s.prepareP2Mutation(request, AuthOperationRecoveryVerify, 4096, &body, func() error {
		if _, parseErr := ParseRecoveryCode(body.Code); parseErr != nil {
			return fmt.Errorf("invalid_argument: recovery code schema is invalid")
		}
		return nil
	})
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer prepared.zero()
	now := s.now()
	var result AuthResult
	defer func() {
		zeroBytes(result.Session.RawToken)
		zeroBytes(result.Session.RawCSRF)
	}()
	record, replay, err := s.store.WithAuthIdempotency(request.Context(), prepared.Request, now, func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		conn := tx.Conn
		if productErr := tx.BeginProduct(request.Context()); productErr != nil {
			return AuthIdempotencyCommit{}, productErr
		}
		var verifyErr error
		result, verifyErr = s.auth.VerifyRecoveryTx(request.Context(), conn, s.recoveryLimiter, s.requestSource(request), body.Code)
		if verifyErr != nil {
			return AuthIdempotencyCommit{}, verifyErr
		}
		resourceID := result.Session.ID
		receipt := authOperationReceipt(operationID, now, &resourceID)
		receipt.CookieOutcome = generatedapi.AuthOperationReceiptV1CookieOutcomeIssuedNotReplayable
		receipt.CsrfOutcome = generatedapi.AuthOperationReceiptV1CsrfOutcomeIssuedNotReplayable
		receipt.NextAction = generatedapi.AuthOperationReceiptV1NextActionUseAnotherRecoveryCode
		return AuthIdempotencyCommit{Receipt: receipt, CookieAction: "secret_issued", At: now}, nil
	})
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if replay {
		s.writeSecretReplay(writer, record)
		return
	}
	s.setSessionCookie(writer, result.Session.RawToken, result.Session.ExpiresAt)
	writeJSON(writer, http.StatusOK, generatedapi.CurrentAuthEnvelope{ApiVersion: generatedapi.CurrentAuthEnvelopeApiVersionV1, Data: result.CurrentAuth, Meta: responseMeta(writer)})
}

func (s *Server) recoveryRotate(writer http.ResponseWriter, request *http.Request) {
	prepared, err := s.prepareP2Mutation(request, AuthOperationRecoveryCodesRotate, 16, nil, nil)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer prepared.zero()
	now := s.now()
	var result RecoveryCodeSet
	defer func() { result.ZeroPlaintext() }()
	record, replay, err := s.store.WithAuthIdempotency(request.Context(), prepared.Request, now, func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		conn := tx.Conn
		session, authErr := s.authorizeBrowserWinnerTx(request.Context(), conn, &prepared, true, now)
		if authErr != nil {
			return AuthIdempotencyCommit{}, authErr
		}
		if productErr := tx.BeginProduct(request.Context()); productErr != nil {
			return AuthIdempotencyCommit{}, productErr
		}
		result, authErr = s.auth.RotateRecoveryCodesTx(request.Context(), conn, session)
		if authErr != nil {
			return AuthIdempotencyCommit{}, authErr
		}
		resourceID := result.BatchID
		receipt := authOperationReceipt(operationID, now, &resourceID)
		receipt.RecoveryCodesOutcome = generatedapi.AuthOperationReceiptV1RecoveryCodesOutcomeIssuedNotReplayable
		receipt.NextAction = generatedapi.AuthOperationReceiptV1NextActionRotateRecoveryCodes
		return AuthIdempotencyCommit{Receipt: receipt, CookieAction: "none", At: now}, nil
	})
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if replay {
		if err := s.validateBrowserReplayCSRF(request.Context(), &prepared); err != nil {
			s.writeSafeError(writer, err)
			return
		}
		s.writeSecretReplay(writer, record)
		return
	}
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
	session, err = s.store.TouchBrowserSession(request.Context(), session.ID, session.ActivityRevision, s.now())
	if err != nil {
		s.writeSafeError(writer, fmt.Errorf("unauthenticated"))
		return
	}
	values, err := s.store.ListPasskeys(request.Context(), session.UserID)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	result := make([]generatedapi.PasskeyV1, 0, len(values))
	for _, value := range values {
		var transportNames []generatedapi.PasskeyV1Transports
		_ = json.Unmarshal(value.TransportsJSON, &transportNames)
		result = append(result, generatedapi.PasskeyV1{Id: value.ID, Name: value.Name, CreatedAt: value.CreatedAt, LastUsedAt: value.LastUsedAt, Transports: transportNames, CredentialRevision: int(value.CredentialRevision), Current: value.ID == session.AuthenticationPasskeyID})
	}
	writeJSON(writer, http.StatusOK, generatedapi.PasskeyListEnvelopeV1{ApiVersion: generatedapi.PasskeyListEnvelopeV1ApiVersionV1, Data: generatedapi.PasskeyListResultV1{Passkeys: result}, Meta: responseMeta(writer)})
}

func (s *Server) passkeyDelete(writer http.ResponseWriter, request *http.Request) {
	prepared, err := s.prepareP2Mutation(request, AuthOperationPasskeyDelete, 16, nil, nil)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer prepared.zero()
	now := s.now()
	passkeyID := request.PathValue("passkeyId")
	var data generatedapi.PasskeyDeleteResultV1
	record, replay, err := s.store.WithAuthIdempotency(request.Context(), prepared.Request, now, func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		conn := tx.Conn
		session, authErr := s.authorizeBrowserWinnerTx(request.Context(), conn, &prepared, true, now)
		if authErr != nil {
			return AuthIdempotencyCommit{}, authErr
		}
		if productErr := tx.BeginProduct(request.Context()); productErr != nil {
			return AuthIdempotencyCommit{}, productErr
		}
		if session.RecentUVAt == nil || now.Sub(*session.RecentUVAt) < 0 || now.Sub(*session.RecentUVAt) > 5*time.Minute {
			return AuthIdempotencyCommit{}, fmt.Errorf("recent user verification is required")
		}
		result, deleteErr := deletePasskeyCASTx(request.Context(), conn, session.UserID, passkeyID, session.ID, prepared.Revision, now)
		if deleteErr != nil {
			return AuthIdempotencyCommit{}, deleteErr
		}
		data = generatedapi.PasskeyDeleteResultV1{PasskeyId: passkeyID, RevokedSessionCount: result.RevokedSessionCount, CurrentSessionRevoked: result.CurrentSessionRevoked}
		receipt := authOperationReceipt(operationID, now, &passkeyID)
		cookieAction := "none"
		if result.CurrentSessionRevoked {
			receipt.CookieOutcome = generatedapi.AuthOperationReceiptV1CookieOutcomeCleared
			cookieAction = "clear"
		}
		return AuthIdempotencyCommit{Receipt: receipt, PublicResult: data, CookieAction: cookieAction, At: now}, nil
	})
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if replay {
		if err := s.validateBrowserReplayCSRF(request.Context(), &prepared); err != nil {
			s.writeSafeError(writer, err)
			return
		}
		if err := decodeAuthPublicResult(record, &data); err != nil {
			s.writeSafeError(writer, err)
			return
		}
	}
	if data.CurrentSessionRevoked {
		s.clearSessionCookie(writer)
	}
	writeJSON(writer, http.StatusOK, generatedapi.PasskeyDeleteEnvelopeV1{ApiVersion: generatedapi.PasskeyDeleteEnvelopeV1ApiVersionV1, Data: data, Meta: responseMeta(writer)})
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
	current, err = s.store.TouchBrowserSession(request.Context(), current.ID, current.ActivityRevision, s.now())
	if err != nil {
		s.writeSafeError(writer, fmt.Errorf("unauthenticated"))
		return
	}
	values, err := s.store.ListBrowserSessions(request.Context(), current.UserID, s.now())
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	result := make([]generatedapi.BrowserSessionV1, 0, len(values))
	for _, value := range values {
		result = append(result, generatedapi.BrowserSessionV1{Id: value.ID, AuthenticationMethod: generatedapi.BrowserSessionV1AuthenticationMethod(value.AuthenticationMethod), CreatedAt: value.CreatedAt, LastSeenAt: value.LastSeenAt, ExpiresAt: value.ExpiresAt, IdleExpiresAt: value.IdleExpiresAt, Revision: int(value.Revision), ActivityRevision: int(value.ActivityRevision), Current: value.ID == current.ID})
	}
	writeJSON(writer, http.StatusOK, generatedapi.BrowserSessionListEnvelopeV1{ApiVersion: generatedapi.BrowserSessionListEnvelopeV1ApiVersionV1, Data: generatedapi.BrowserSessionListResultV1{Sessions: result}, Meta: responseMeta(writer)})
}

func (s *Server) browserSessionDelete(writer http.ResponseWriter, request *http.Request) {
	prepared, err := s.prepareP2Mutation(request, AuthOperationSessionDelete, 16, nil, nil)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer prepared.zero()
	id := request.PathValue("sessionId")
	now := s.now()
	var current BrowserSession
	var data generatedapi.BrowserSessionRevokeResultV1
	record, replay, err := s.store.WithAuthIdempotency(request.Context(), prepared.Request, now, func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		conn := tx.Conn
		var authErr error
		current, authErr = s.authorizeBrowserWinnerTx(request.Context(), conn, &prepared, true, now)
		if authErr != nil {
			return AuthIdempotencyCommit{}, authErr
		}
		if productErr := tx.BeginProduct(request.Context()); productErr != nil {
			return AuthIdempotencyCommit{}, productErr
		}
		revoked, revokeErr := revokeBrowserSession(request.Context(), conn, id, prepared.Revision, now)
		if revokeErr != nil {
			return AuthIdempotencyCommit{}, revokeErr
		}
		currentRevoked := id == current.ID
		data = generatedapi.BrowserSessionRevokeResultV1{SessionId: revoked.SessionID, RevokedAt: revoked.RevokedAt, Revision: int(revoked.Revision), CurrentSessionRevoked: currentRevoked}
		receipt := authOperationReceipt(operationID, now, &id)
		cookieAction := "none"
		if currentRevoked {
			receipt.CookieOutcome = generatedapi.AuthOperationReceiptV1CookieOutcomeCleared
			cookieAction = "clear"
		}
		return AuthIdempotencyCommit{Receipt: receipt, PublicResult: data, CookieAction: cookieAction, At: now}, nil
	})
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if replay {
		if err := s.validateBrowserReplayCSRF(request.Context(), &prepared); err != nil {
			s.writeSafeError(writer, err)
			return
		}
		if err = decodeAuthPublicResult(record, &data); err != nil {
			s.writeSafeError(writer, err)
			return
		}
	}
	if data.CurrentSessionRevoked {
		s.clearSessionCookie(writer)
	}
	writeJSON(writer, http.StatusOK, generatedapi.BrowserSessionRevokeEnvelopeV1{ApiVersion: generatedapi.BrowserSessionRevokeEnvelopeV1ApiVersionV1, Data: data, Meta: responseMeta(writer)})
}

func (s *Server) authLogout(writer http.ResponseWriter, request *http.Request) {
	prepared, err := s.prepareP2Mutation(request, AuthOperationLogout, 16, nil, nil)
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	defer prepared.zero()
	now := s.now()
	var current BrowserSession
	var data generatedapi.OperationStatusData
	record, replay, err := s.store.WithAuthIdempotency(request.Context(), prepared.Request, now, func(tx *AuthIdempotencyTx, operationID string) (AuthIdempotencyCommit, error) {
		conn := tx.Conn
		var authErr error
		current, authErr = s.authorizeBrowserWinnerTx(request.Context(), conn, &prepared, false, now)
		if authErr != nil {
			return AuthIdempotencyCommit{}, authErr
		}
		if productErr := tx.BeginProduct(request.Context()); productErr != nil {
			return AuthIdempotencyCommit{}, productErr
		}
		if _, authErr = revokeBrowserSession(request.Context(), conn, current.ID, current.Revision, now); authErr != nil {
			return AuthIdempotencyCommit{}, authErr
		}
		resourceID := current.ID
		data = generatedapi.OperationStatusData{Status: generatedapi.OperationStatusDataStatusRevoked, Id: &resourceID}
		receipt := authOperationReceipt(operationID, now, &resourceID)
		receipt.CookieOutcome = generatedapi.AuthOperationReceiptV1CookieOutcomeCleared
		return AuthIdempotencyCommit{Receipt: receipt, PublicResult: data, CookieAction: "clear", At: now}, nil
	})
	if err != nil {
		s.writeSafeError(writer, err)
		return
	}
	if replay {
		if err := s.validateBrowserReplayCSRF(request.Context(), &prepared); err != nil {
			s.writeSafeError(writer, err)
			return
		}
		if err := decodeAuthPublicResult(record, &data); err != nil || data.Id == nil {
			s.writeSafeError(writer, fmt.Errorf("stored logout receipt is invalid"))
			return
		}
	}
	s.clearSessionCookie(writer)
	writeJSON(writer, http.StatusOK, generatedapi.OperationStatusEnvelope{ApiVersion: generatedapi.OperationStatusEnvelopeApiVersionV1, Data: data, Meta: responseMeta(writer)})
}

func (s *Server) authenticate(request *http.Request) (BrowserSession, []byte, error) {
	return s.authenticateContext(request.Context(), request)
}

func (s *Server) authenticateContext(ctx context.Context, request *http.Request) (BrowserSession, []byte, error) {
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
	session, err := s.store.BrowserSessionByToken(ctx, raw, s.now())
	if err != nil {
		zeroBytes(raw)
		return BrowserSession{}, nil, fmt.Errorf("unauthenticated")
	}
	return session, raw, nil
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

func bootstrapAuthorization(request *http.Request) (string, error) {
	values := request.Header.Values("Authorization")
	if len(values) != 1 || !strings.HasPrefix(values[0], "Bootstrap ") || strings.Count(values[0], " ") != 1 {
		return "", fmt.Errorf("bootstrap token is invalid")
	}
	return strings.TrimPrefix(values[0], "Bootstrap "), nil
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
	details := any([]any{})
	var sessionConflict *SessionRevisionConflictError
	switch {
	case errors.Is(err, ErrPasskeyCounterRegressed):
		code, status, message = "passkey_counter_regressed", http.StatusUnauthorized, "Passkey verification was rejected"
	case errors.Is(err, ErrLastPasskeyRequired):
		code, status, message = "last_passkey_required", http.StatusConflict, "the last Passkey cannot be deleted"
	case errors.Is(err, ErrRevisionChanged):
		code, status, message = "conflict", http.StatusPreconditionFailed, "resource revision changed"
	case errors.Is(err, ErrSessionIntegrityInvalid):
		code, status, message = "session_integrity_invalid", http.StatusUnauthorized, "browser session integrity verification failed"
	case errors.As(err, &sessionConflict):
		code, status, message = "session_revision_conflict", http.StatusPreconditionFailed, "browser session revision changed"
		details = generatedapi.SessionRevisionConflictDetailsV1{SessionId: sessionConflict.SessionID, ExpectedRevision: int(sessionConflict.ExpectedRevision), CurrentRevision: int(sessionConflict.CurrentRevision)}
	case errors.Is(err, ErrAuthIdempotencyKeyReused):
		code, status, message = "idempotency_key_reused", http.StatusConflict, "Idempotency-Key was already used for a different request"
	case errors.Is(err, ErrAuthIdempotencyInProgress):
		code, status, message = "idempotency_in_progress", http.StatusConflict, "the matching operation is still in progress"
		writer.Header().Set("Retry-After", "1")
	case errors.Is(err, ErrCeremonyRestartRequired):
		code, status, message = "ceremony_restart_required", http.StatusConflict, "the ceremony must be restarted with a fresh Idempotency-Key"
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
	safeErrorDetails(writer, status, code, message, writer.Header().Get("X-Request-ID"), details)
}
