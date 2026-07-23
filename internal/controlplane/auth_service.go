package controlplane

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	generatedapi "github.com/jinlong17/multi-agent-desk/internal/controlplane/api/generated"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

type AuthService struct {
	Config   Config
	Store    *Store
	WebAuthn *WebAuthnService
	Now      func() time.Time
}

type AuthResult struct {
	Session     BrowserSessionCreate
	CurrentAuth generatedapi.CurrentAuth
}

func (s *AuthService) VerifyRecovery(ctx context.Context, limiter *RecoveryLimiter, source, code string) (AuthResult, error) {
	return s.verifyRecovery(ctx, nil, limiter, source, code)
}

func (s *AuthService) VerifyRecoveryTx(ctx context.Context, conn *sql.Conn, limiter *RecoveryLimiter, source, code string) (AuthResult, error) {
	if conn == nil {
		return AuthResult{}, fmt.Errorf("recovery transaction is required")
	}
	return s.verifyRecovery(ctx, conn, limiter, source, code)
}

func (s *AuthService) verifyRecovery(ctx context.Context, conn *sql.Conn, limiter *RecoveryLimiter, source, code string) (AuthResult, error) {
	if limiter == nil || !limiter.Allow(source) {
		return AuthResult{}, ErrRecoveryInvalidOrRateLimited
	}
	canonical, err := ParseRecoveryCode(code)
	malformed := err != nil
	if malformed {
		canonical = formatRecoveryCode(make([]byte, recoveryCodeEntropySize))
	}
	var candidates []RecoveryCandidate
	if conn == nil {
		candidates, err = s.Store.RecoveryCandidates(ctx)
	} else {
		candidates, err = recoveryCandidates(ctx, conn)
	}
	if err != nil {
		return AuthResult{}, ErrRecoveryInvalidOrRateLimited
	}
	if len(candidates) == 0 {
		candidates = []RecoveryCandidate{{ID: "none", Salt: make([]byte, recoverySaltSize), Hash: make([]byte, recoveryHashSize), Status: "active"}}
	}
	codeID, err := MatchRecoveryCode(ctx, canonical, candidates)
	if malformed {
		return AuthResult{}, ErrRecoveryInvalidOrRateLimited
	}
	if err != nil {
		return AuthResult{}, err
	}
	var user StoredUser
	if conn == nil {
		user, err = s.Store.SoleUser(ctx)
	} else {
		user, err = soleUser(ctx, conn)
	}
	if err != nil {
		return AuthResult{}, ErrRecoveryInvalidOrRateLimited
	}
	now := s.now()
	session, err := NewBrowserSessionCreate(user.ID, "recovery", "", s.Config.PublicOrigin, now)
	if err != nil {
		return AuthResult{}, err
	}
	var consumeErr error
	if conn == nil {
		_, consumeErr = s.Store.ConsumeRecoveryCode(ctx, codeID, session, now)
	} else {
		_, consumeErr = consumeRecoveryCodeTx(ctx, conn, codeID, session, now)
	}
	if consumeErr != nil {
		zeroBytes(session.RawToken)
		zeroBytes(session.RawCSRF)
		return AuthResult{}, consumeErr
	}
	return AuthResult{Session: session, CurrentAuth: currentAuthDTO(session, session.RawCSRF)}, nil
}

func (s *AuthService) RotateRecoveryCodes(ctx context.Context, session BrowserSession) (RecoveryCodeSet, error) {
	return s.rotateRecoveryCodes(ctx, nil, session)
}

func (s *AuthService) RotateRecoveryCodesTx(ctx context.Context, conn *sql.Conn, session BrowserSession) (RecoveryCodeSet, error) {
	if conn == nil {
		return RecoveryCodeSet{}, fmt.Errorf("recovery rotation transaction is required")
	}
	return s.rotateRecoveryCodes(ctx, conn, session)
}

func (s *AuthService) rotateRecoveryCodes(ctx context.Context, conn *sql.Conn, session BrowserSession) (RecoveryCodeSet, error) {
	now := s.now()
	if session.AuthenticationMethod != "passkey" || session.RecentUVAt == nil || now.Sub(*session.RecentUVAt) < 0 || now.Sub(*session.RecentUVAt) > 5*time.Minute {
		return RecoveryCodeSet{}, fmt.Errorf("recent user verification is required")
	}
	set, err := GenerateRecoveryCodeSet(now)
	if err != nil {
		return RecoveryCodeSet{}, err
	}
	input := RecoveryRotationInput{UserID: session.UserID, BatchID: set.BatchID, Codes: set.Hashes, At: now}
	var rotateErr error
	if conn == nil {
		rotateErr = s.Store.RotateRecoveryCodes(ctx, input)
	} else {
		rotateErr = rotateRecoveryCodesTx(ctx, conn, input)
	}
	if rotateErr != nil {
		set.ZeroPlaintext()
		return RecoveryCodeSet{}, rotateErr
	}
	return set, nil
}

func (s *AuthService) now() time.Time {
	if s.Now != nil {
		return normalizeServerTime(s.Now())
	}
	return normalizeServerTime(time.Now())
}

func (s *AuthService) BeginLogin(ctx context.Context) (generatedapi.WebAuthnRequestOptionsV1, error) {
	return s.beginLogin(ctx, nil)
}

func (s *AuthService) BeginLoginTx(ctx context.Context, conn *sql.Conn) (generatedapi.WebAuthnRequestOptionsV1, error) {
	if conn == nil {
		return generatedapi.WebAuthnRequestOptionsV1{}, fmt.Errorf("login transaction is required")
	}
	return s.beginLogin(ctx, conn)
}

func (s *AuthService) beginLogin(ctx context.Context, conn *sql.Conn) (generatedapi.WebAuthnRequestOptionsV1, error) {
	var user StoredUser
	var err error
	if conn == nil {
		user, err = s.Store.SoleUser(ctx)
	} else {
		user, err = soleUser(ctx, conn)
	}
	if err != nil {
		return generatedapi.WebAuthnRequestOptionsV1{}, fmt.Errorf("Passkey authentication is unavailable")
	}
	if conn == nil {
		return s.WebAuthn.BeginAssertion(ctx, ceremonyPasskeyLogin, user, "", 0)
	}
	return s.WebAuthn.BeginAssertionTx(ctx, conn, ceremonyPasskeyLogin, user, "", 0)
}

func (s *AuthService) VerifyLogin(ctx context.Context, request generatedapi.WebAuthnAssertionVerifyRequestV1) (AuthResult, error) {
	return s.verifyLogin(ctx, nil, request)
}

func (s *AuthService) VerifyLoginTx(ctx context.Context, conn *sql.Conn, request generatedapi.WebAuthnAssertionVerifyRequestV1) (AuthResult, error) {
	if conn == nil {
		return AuthResult{}, fmt.Errorf("login transaction is required")
	}
	return s.verifyLogin(ctx, conn, request)
}

func (s *AuthService) verifyLogin(ctx context.Context, conn *sql.Conn, request generatedapi.WebAuthnAssertionVerifyRequestV1) (AuthResult, error) {
	now := s.now()
	var ceremony *webAuthnCeremony
	var err error
	if conn == nil {
		ceremony, err = s.WebAuthn.Ceremonies.Load(ctx, request.CeremonyId, ceremonyPasskeyLogin, now)
	} else {
		ceremony, err = s.WebAuthn.Ceremonies.loadTx(ctx, conn, request.CeremonyId, ceremonyPasskeyLogin, now)
	}
	if err != nil {
		return AuthResult{}, err
	}
	committed := false
	defer func() {
		if conn == nil && !committed {
			s.WebAuthn.Ceremonies.consumeFailure(request.CeremonyId, ceremonyPasskeyLogin)
		}
	}()
	credential, observed, err := s.WebAuthn.FinishAssertion(ceremony, request.Credential)
	if err != nil {
		return AuthResult{}, err
	}
	var passkey StoredPasskey
	if conn == nil {
		passkey, err = s.Store.PasskeyByCredentialID(ctx, credential.ID)
	} else {
		passkey, err = passkeyByCredentialID(ctx, conn, credential.ID)
	}
	if err != nil || !passkey.Active {
		return AuthResult{}, fmt.Errorf("Passkey authentication failed")
	}
	session, err := NewBrowserSessionCreate(passkey.UserID, "passkey", passkey.ID, s.Config.PublicOrigin, now)
	if err != nil {
		return AuthResult{}, err
	}
	commit := PasskeyAssertionCommit{CeremonyID: ceremony.ID, PasskeyID: passkey.ID, ExpectedCredentialRevision: passkey.CredentialRevision, ObservedSignCount: observed, Credential: *credential, NewSession: session, At: now}
	var commitErr error
	if conn == nil {
		_, commitErr = s.Store.CommitPasskeyAssertion(ctx, commit)
	} else {
		_, commitErr = commitPasskeyAssertionTx(ctx, conn, commit)
	}
	if commitErr != nil {
		zeroBytes(session.RawToken)
		zeroBytes(session.RawCSRF)
		return AuthResult{}, commitErr
	}
	committed = true
	return AuthResult{Session: session, CurrentAuth: currentAuthDTO(session, session.RawCSRF)}, nil
}

func (s *AuthService) BeginRegistration(ctx context.Context, session BrowserSession) (generatedapi.WebAuthnCreationOptionsV1, error) {
	return s.beginRegistration(ctx, nil, session)
}

func (s *AuthService) BeginRegistrationTx(ctx context.Context, conn *sql.Conn, session BrowserSession) (generatedapi.WebAuthnCreationOptionsV1, error) {
	if conn == nil {
		return generatedapi.WebAuthnCreationOptionsV1{}, fmt.Errorf("registration transaction is required")
	}
	return s.beginRegistration(ctx, conn, session)
}

func (s *AuthService) beginRegistration(ctx context.Context, conn *sql.Conn, session BrowserSession) (generatedapi.WebAuthnCreationOptionsV1, error) {
	var user StoredUser
	var err error
	if conn == nil {
		user, err = s.Store.SoleUser(ctx)
	} else {
		user, err = soleUser(ctx, conn)
	}
	if err != nil || user.ID != session.UserID {
		return generatedapi.WebAuthnCreationOptionsV1{}, fmt.Errorf("Passkey registration is unavailable")
	}
	options, ceremony, err := s.WebAuthn.BeginRegistration(ctx, ceremonyPasskeyRegistration, user, session.ID, session.CSRFGeneration)
	if err != nil {
		return generatedapi.WebAuthnCreationOptionsV1{}, err
	}
	ceremony.CreationOptions = &options
	if conn == nil {
		err = s.WebAuthn.Ceremonies.put(ctx, ceremony)
	} else {
		err = s.WebAuthn.Ceremonies.putTx(ctx, conn, ceremony)
	}
	if err != nil {
		return generatedapi.WebAuthnCreationOptionsV1{}, err
	}
	return options, nil
}

func (s *AuthService) VerifyRegistration(ctx context.Context, authenticated BrowserSession, request generatedapi.WebAuthnRegistrationVerifyRequestV1) (AuthResult, error) {
	return s.verifyRegistration(ctx, nil, authenticated, request)
}

func (s *AuthService) VerifyRegistrationTx(ctx context.Context, conn *sql.Conn, authenticated BrowserSession, request generatedapi.WebAuthnRegistrationVerifyRequestV1) (AuthResult, error) {
	if conn == nil {
		return AuthResult{}, fmt.Errorf("registration transaction is required")
	}
	return s.verifyRegistration(ctx, conn, authenticated, request)
}

func (s *AuthService) verifyRegistration(ctx context.Context, conn *sql.Conn, authenticated BrowserSession, request generatedapi.WebAuthnRegistrationVerifyRequestV1) (AuthResult, error) {
	now := s.now()
	var ceremony *webAuthnCeremony
	var err error
	if conn == nil {
		ceremony, err = s.WebAuthn.Ceremonies.Load(ctx, request.CeremonyId, ceremonyPasskeyRegistration, now)
	} else {
		ceremony, err = s.WebAuthn.Ceremonies.loadTx(ctx, conn, request.CeremonyId, ceremonyPasskeyRegistration, now)
	}
	if err != nil || ceremony.BrowserSessionID != authenticated.ID || ceremony.BrowserSessionCSRFGeneration != authenticated.CSRFGeneration || ceremony.User.ID != authenticated.UserID {
		return AuthResult{}, fmt.Errorf("WebAuthn registration ceremony is invalid")
	}
	committed := false
	defer func() {
		if conn == nil && !committed {
			s.WebAuthn.Ceremonies.consumeFailure(request.CeremonyId, ceremonyPasskeyRegistration)
		}
	}()
	credential, err := s.WebAuthn.FinishRegistration(ceremony, request.Credential)
	if err != nil {
		return AuthResult{}, err
	}
	passkeyID, err := transport.NewUUIDv7()
	if err != nil {
		return AuthResult{}, err
	}
	passkeyForSession := authenticated.AuthenticationPasskeyID
	revokeAll := authenticated.AuthenticationMethod == "recovery"
	if revokeAll {
		passkeyForSession = passkeyID
	}
	replacement, err := NewBrowserSessionCreate(authenticated.UserID, "passkey", passkeyForSession, s.Config.PublicOrigin, now)
	if err != nil {
		return AuthResult{}, err
	}
	transports, _ := json.Marshal(credential.Transport)
	passkey := StoredPasskey{ID: passkeyID, UserID: authenticated.UserID, Credential: *credential, Name: "Passkey", TransportsJSON: transports, SignCount: credential.Authenticator.SignCount, CredentialRevision: 1, Active: true, CreatedAt: now, UpdatedAt: now}
	commit := PasskeyRegistrationCommit{CeremonyID: ceremony.ID, UserID: authenticated.UserID, CurrentSessionID: authenticated.ID, ExpectedSessionRevision: authenticated.Revision, Passkey: passkey, ReplacementSession: replacement, RevokeAllOtherSessions: revokeAll, At: now}
	var commitErr error
	if conn == nil {
		commitErr = s.Store.CommitPasskeyRegistration(ctx, commit)
	} else {
		credentialJSON, validateErr := validatePasskeyRegistrationCommit(commit)
		if validateErr != nil {
			commitErr = validateErr
		} else {
			commitErr = commitPasskeyRegistrationTx(ctx, conn, commit, credentialJSON)
		}
	}
	if commitErr != nil {
		zeroBytes(replacement.RawToken)
		zeroBytes(replacement.RawCSRF)
		return AuthResult{}, commitErr
	}
	committed = true
	return AuthResult{Session: replacement, CurrentAuth: currentAuthDTO(replacement, replacement.RawCSRF)}, nil
}

func (s *AuthService) BeginUV(ctx context.Context, session BrowserSession) (generatedapi.WebAuthnRequestOptionsV1, error) {
	return s.beginUV(ctx, nil, session)
}

func (s *AuthService) BeginUVTx(ctx context.Context, conn *sql.Conn, session BrowserSession) (generatedapi.WebAuthnRequestOptionsV1, error) {
	if conn == nil {
		return generatedapi.WebAuthnRequestOptionsV1{}, fmt.Errorf("UV transaction is required")
	}
	return s.beginUV(ctx, conn, session)
}

func (s *AuthService) beginUV(ctx context.Context, conn *sql.Conn, session BrowserSession) (generatedapi.WebAuthnRequestOptionsV1, error) {
	if session.AuthenticationMethod != "passkey" {
		return generatedapi.WebAuthnRequestOptionsV1{}, fmt.Errorf("recent user verification requires a normal Passkey session")
	}
	var user StoredUser
	var err error
	if conn == nil {
		user, err = s.Store.SoleUser(ctx)
	} else {
		user, err = soleUser(ctx, conn)
	}
	if err != nil || user.ID != session.UserID {
		return generatedapi.WebAuthnRequestOptionsV1{}, fmt.Errorf("recent user verification is unavailable")
	}
	if conn == nil {
		return s.WebAuthn.BeginAssertion(ctx, ceremonyRecentUV, user, session.ID, session.CSRFGeneration)
	}
	return s.WebAuthn.BeginAssertionTx(ctx, conn, ceremonyRecentUV, user, session.ID, session.CSRFGeneration)
}

func (s *AuthService) VerifyUV(ctx context.Context, authenticated BrowserSession, request generatedapi.WebAuthnAssertionVerifyRequestV1) (AuthResult, error) {
	return s.verifyUV(ctx, nil, authenticated, request)
}

func (s *AuthService) VerifyUVTx(ctx context.Context, conn *sql.Conn, authenticated BrowserSession, request generatedapi.WebAuthnAssertionVerifyRequestV1) (AuthResult, error) {
	if conn == nil {
		return AuthResult{}, fmt.Errorf("UV transaction is required")
	}
	return s.verifyUV(ctx, conn, authenticated, request)
}

func (s *AuthService) verifyUV(ctx context.Context, conn *sql.Conn, authenticated BrowserSession, request generatedapi.WebAuthnAssertionVerifyRequestV1) (AuthResult, error) {
	now := s.now()
	var ceremony *webAuthnCeremony
	var err error
	if conn == nil {
		ceremony, err = s.WebAuthn.Ceremonies.Load(ctx, request.CeremonyId, ceremonyRecentUV, now)
	} else {
		ceremony, err = s.WebAuthn.Ceremonies.loadTx(ctx, conn, request.CeremonyId, ceremonyRecentUV, now)
	}
	if err != nil || ceremony.BrowserSessionID != authenticated.ID || ceremony.BrowserSessionCSRFGeneration != authenticated.CSRFGeneration {
		return AuthResult{}, fmt.Errorf("recent user-verification ceremony is invalid")
	}
	committed := false
	defer func() {
		if conn == nil && !committed {
			s.WebAuthn.Ceremonies.consumeFailure(request.CeremonyId, ceremonyRecentUV)
		}
	}()
	credential, observed, err := s.WebAuthn.FinishAssertion(ceremony, request.Credential)
	if err != nil {
		return AuthResult{}, err
	}
	var passkey StoredPasskey
	if conn == nil {
		passkey, err = s.Store.PasskeyByCredentialID(ctx, credential.ID)
	} else {
		passkey, err = passkeyByCredentialID(ctx, conn, credential.ID)
	}
	if err != nil || passkey.UserID != authenticated.UserID {
		return AuthResult{}, fmt.Errorf("recent user verification failed")
	}
	replacement, err := NewBrowserSessionCreate(authenticated.UserID, "passkey", passkey.ID, s.Config.PublicOrigin, now)
	if err != nil {
		return AuthResult{}, err
	}
	commit := PasskeyAssertionCommit{CeremonyID: ceremony.ID, PasskeyID: passkey.ID, ExpectedCredentialRevision: passkey.CredentialRevision, ObservedSignCount: observed, Credential: *credential, NewSession: replacement, RevokeSessionID: authenticated.ID, At: now}
	var commitErr error
	if conn == nil {
		_, commitErr = s.Store.CommitPasskeyAssertion(ctx, commit)
	} else {
		_, commitErr = commitPasskeyAssertionTx(ctx, conn, commit)
	}
	if commitErr != nil {
		zeroBytes(replacement.RawToken)
		zeroBytes(replacement.RawCSRF)
		return AuthResult{}, commitErr
	}
	committed = true
	return AuthResult{Session: replacement, CurrentAuth: currentAuthDTO(replacement, replacement.RawCSRF)}, nil
}

func deriveSessionCSRF(rawSessionToken []byte, serverOrigin, sessionID string, generation int64) []byte {
	framed, _ := transport.Frame([]byte("multidesk-browser-csrf-v1"), []byte("1"), []byte(serverOrigin), []byte(sessionID), []byte(strconv.FormatInt(generation, 10)))
	mac := hmac.New(sha256.New, rawSessionToken)
	_, _ = mac.Write(framed)
	return mac.Sum(nil)
}
