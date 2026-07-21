package controlplane

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
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
	if limiter == nil || !limiter.Allow(source) {
		return AuthResult{}, ErrRecoveryInvalidOrRateLimited
	}
	canonical, err := ParseRecoveryCode(code)
	malformed := err != nil
	if malformed {
		canonical = formatRecoveryCode(make([]byte, recoveryCodeEntropySize))
	}
	candidates, err := s.Store.RecoveryCandidates(ctx)
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
	user, err := s.Store.SoleUser(ctx)
	if err != nil {
		return AuthResult{}, ErrRecoveryInvalidOrRateLimited
	}
	now := s.now()
	session, err := NewBrowserSessionCreate(user.ID, "recovery", "", now)
	if err != nil {
		return AuthResult{}, err
	}
	session.RawCSRF = deriveSessionCSRF(session.RawToken, session.ID)
	if _, err := s.Store.ConsumeRecoveryCode(ctx, codeID, session, now); err != nil {
		zeroBytes(session.RawToken)
		zeroBytes(session.RawCSRF)
		return AuthResult{}, err
	}
	return AuthResult{Session: session, CurrentAuth: currentAuthDTO(session, session.RawCSRF)}, nil
}

func (s *AuthService) RotateRecoveryCodes(ctx context.Context, session BrowserSession, scopeDigest [sha256.Size]byte) (RecoveryCodeSet, error) {
	now := s.now()
	if session.AuthenticationMethod != "passkey" || session.RecentUVAt == nil || now.Sub(*session.RecentUVAt) < 0 || now.Sub(*session.RecentUVAt) > 5*time.Minute {
		return RecoveryCodeSet{}, fmt.Errorf("recent user verification is required")
	}
	set, err := GenerateRecoveryCodeSet(now)
	if err != nil {
		return RecoveryCodeSet{}, err
	}
	if err := s.Store.RotateRecoveryCodes(ctx, RecoveryRotationInput{UserID: session.UserID, ScopeDigest: scopeDigest, BatchID: set.BatchID, Codes: set.Hashes, At: now}); err != nil {
		set.ZeroPlaintext()
		return RecoveryCodeSet{}, err
	}
	return set, nil
}

func (s *AuthService) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *AuthService) BeginLogin(ctx context.Context) (generatedapi.WebAuthnRequestOptionsV1, error) {
	user, err := s.Store.SoleUser(ctx)
	if err != nil {
		return generatedapi.WebAuthnRequestOptionsV1{}, fmt.Errorf("Passkey authentication is unavailable")
	}
	return s.WebAuthn.BeginAssertion(ctx, ceremonyPasskeyLogin, user, "")
}

func (s *AuthService) VerifyLogin(ctx context.Context, request generatedapi.WebAuthnAssertionVerifyRequestV1) (AuthResult, error) {
	now := s.now()
	ceremony, err := s.WebAuthn.Ceremonies.Load(ctx, request.CeremonyId, ceremonyPasskeyLogin, now)
	if err != nil {
		return AuthResult{}, err
	}
	committed := false
	defer func() {
		if !committed {
			s.WebAuthn.Ceremonies.consumeFailure(request.CeremonyId, ceremonyPasskeyLogin)
		}
	}()
	credential, observed, err := s.WebAuthn.FinishAssertion(ceremony, request.Credential)
	if err != nil {
		return AuthResult{}, err
	}
	passkey, err := s.Store.PasskeyByCredentialID(ctx, credential.ID)
	if err != nil || !passkey.Active {
		return AuthResult{}, fmt.Errorf("Passkey authentication failed")
	}
	session, err := NewBrowserSessionCreate(passkey.UserID, "passkey", passkey.ID, now)
	if err != nil {
		return AuthResult{}, err
	}
	session.RawCSRF = deriveSessionCSRF(session.RawToken, session.ID)
	if _, err := s.Store.CommitPasskeyAssertion(ctx, PasskeyAssertionCommit{CeremonyID: ceremony.ID, PasskeyID: passkey.ID, ExpectedCredentialRevision: passkey.CredentialRevision, ObservedSignCount: observed, Credential: *credential, NewSession: session, At: now}); err != nil {
		zeroBytes(session.RawToken)
		zeroBytes(session.RawCSRF)
		return AuthResult{}, err
	}
	committed = true
	return AuthResult{Session: session, CurrentAuth: currentAuthDTO(session, session.RawCSRF)}, nil
}

func (s *AuthService) BeginRegistration(ctx context.Context, session BrowserSession) (generatedapi.WebAuthnCreationOptionsV1, error) {
	user, err := s.Store.SoleUser(ctx)
	if err != nil || user.ID != session.UserID {
		return generatedapi.WebAuthnCreationOptionsV1{}, fmt.Errorf("Passkey registration is unavailable")
	}
	options, ceremony, err := s.WebAuthn.BeginRegistration(ctx, ceremonyPasskeyRegistration, user, session.ID)
	if err != nil {
		return generatedapi.WebAuthnCreationOptionsV1{}, err
	}
	if err := s.WebAuthn.Ceremonies.put(ctx, ceremony); err != nil {
		return generatedapi.WebAuthnCreationOptionsV1{}, err
	}
	return options, nil
}

func (s *AuthService) VerifyRegistration(ctx context.Context, authenticated BrowserSession, request generatedapi.WebAuthnRegistrationVerifyRequestV1) (AuthResult, error) {
	now := s.now()
	ceremony, err := s.WebAuthn.Ceremonies.Load(ctx, request.CeremonyId, ceremonyPasskeyRegistration, now)
	if err != nil || ceremony.BrowserSessionID != authenticated.ID || ceremony.User.ID != authenticated.UserID {
		return AuthResult{}, fmt.Errorf("WebAuthn registration ceremony is invalid")
	}
	committed := false
	defer func() {
		if !committed {
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
	replacement, err := NewBrowserSessionCreate(authenticated.UserID, "passkey", passkeyForSession, now)
	if err != nil {
		return AuthResult{}, err
	}
	replacement.RawCSRF = deriveSessionCSRF(replacement.RawToken, replacement.ID)
	transports, _ := json.Marshal(credential.Transport)
	passkey := StoredPasskey{ID: passkeyID, UserID: authenticated.UserID, Credential: *credential, Name: "Passkey", TransportsJSON: transports, SignCount: credential.Authenticator.SignCount, CredentialRevision: 1, Active: true, CreatedAt: now, UpdatedAt: now}
	if err := s.Store.CommitPasskeyRegistration(ctx, PasskeyRegistrationCommit{CeremonyID: ceremony.ID, UserID: authenticated.UserID, CurrentSessionID: authenticated.ID, ExpectedSessionRevision: authenticated.Revision, Passkey: passkey, ReplacementSession: replacement, RevokeAllOtherSessions: revokeAll, At: now}); err != nil {
		zeroBytes(replacement.RawToken)
		zeroBytes(replacement.RawCSRF)
		return AuthResult{}, err
	}
	committed = true
	return AuthResult{Session: replacement, CurrentAuth: currentAuthDTO(replacement, replacement.RawCSRF)}, nil
}

func (s *AuthService) BeginUV(ctx context.Context, session BrowserSession) (generatedapi.WebAuthnRequestOptionsV1, error) {
	if session.AuthenticationMethod != "passkey" {
		return generatedapi.WebAuthnRequestOptionsV1{}, fmt.Errorf("recent user verification requires a normal Passkey session")
	}
	user, err := s.Store.SoleUser(ctx)
	if err != nil || user.ID != session.UserID {
		return generatedapi.WebAuthnRequestOptionsV1{}, fmt.Errorf("recent user verification is unavailable")
	}
	return s.WebAuthn.BeginAssertion(ctx, ceremonyRecentUV, user, session.ID)
}

func (s *AuthService) VerifyUV(ctx context.Context, authenticated BrowserSession, request generatedapi.WebAuthnAssertionVerifyRequestV1) (AuthResult, error) {
	now := s.now()
	ceremony, err := s.WebAuthn.Ceremonies.Load(ctx, request.CeremonyId, ceremonyRecentUV, now)
	if err != nil || ceremony.BrowserSessionID != authenticated.ID {
		return AuthResult{}, fmt.Errorf("recent user-verification ceremony is invalid")
	}
	committed := false
	defer func() {
		if !committed {
			s.WebAuthn.Ceremonies.consumeFailure(request.CeremonyId, ceremonyRecentUV)
		}
	}()
	credential, observed, err := s.WebAuthn.FinishAssertion(ceremony, request.Credential)
	if err != nil {
		return AuthResult{}, err
	}
	passkey, err := s.Store.PasskeyByCredentialID(ctx, credential.ID)
	if err != nil || passkey.UserID != authenticated.UserID {
		return AuthResult{}, fmt.Errorf("recent user verification failed")
	}
	replacement, err := NewBrowserSessionCreate(authenticated.UserID, "passkey", passkey.ID, now)
	if err != nil {
		return AuthResult{}, err
	}
	replacement.RawCSRF = deriveSessionCSRF(replacement.RawToken, replacement.ID)
	if _, err := s.Store.CommitPasskeyAssertion(ctx, PasskeyAssertionCommit{CeremonyID: ceremony.ID, PasskeyID: passkey.ID, ExpectedCredentialRevision: passkey.CredentialRevision, ObservedSignCount: observed, Credential: *credential, NewSession: replacement, RevokeSessionID: authenticated.ID, At: now}); err != nil {
		zeroBytes(replacement.RawToken)
		zeroBytes(replacement.RawCSRF)
		return AuthResult{}, err
	}
	committed = true
	return AuthResult{Session: replacement, CurrentAuth: currentAuthDTO(replacement, replacement.RawCSRF)}, nil
}

func deriveSessionCSRF(rawSessionToken []byte, sessionID string) []byte {
	framed, _ := transport.Frame([]byte("multidesk-browser-csrf-v1"), []byte(sessionID))
	mac := hmac.New(sha256.New, rawSessionToken)
	_, _ = mac.Write(framed)
	return mac.Sum(nil)
}
