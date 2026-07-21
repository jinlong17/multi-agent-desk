package controlplane

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"sync"
	"time"

	generatedapi "github.com/jinlong17/multi-agent-desk/internal/controlplane/api/generated"
	identitycrypto "github.com/jinlong17/multi-agent-desk/internal/crypto"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
	"golang.org/x/crypto/curve25519"
)

type BootstrapService struct {
	Config   Config
	Store    *Store
	WebAuthn *WebAuthnService
	Now      func() time.Time

	ephemeralMu sync.Mutex
	ephemeral   map[string]*bootstrapEphemeral
}

type bootstrapEphemeral struct {
	private []byte
	claimed bool
}

type BootstrapCommitResult struct {
	Receipt       generatedapi.BootstrapCommitReceiptV1
	RecoveryCodes RecoveryCodeSet
	Session       BrowserSessionCreate
	CurrentAuth   generatedapi.CurrentAuth
}

func (s *BootstrapService) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *BootstrapService) Begin(ctx context.Context, token string, request generatedapi.BootstrapOptionsRequestV1) (generatedapi.BootstrapAnchorChallengeV1, error) {
	if s == nil || s.Store == nil || s.WebAuthn == nil {
		return generatedapi.BootstrapAnchorChallengeV1{}, fmt.Errorf("bootstrap service is unavailable")
	}
	now := s.now()
	tokenDigest, err := s.Store.ValidateBootstrapToken(ctx, token, now)
	if err != nil {
		return generatedapi.BootstrapAnchorChallengeV1{}, err
	}
	anchorMaterial, err := validateBootstrapAnchor(request.Anchor)
	if err != nil || request.DisplayName == "" || len(request.DisplayName) > 128 {
		return generatedapi.BootstrapAnchorChallengeV1{}, fmt.Errorf("bootstrap anchor is invalid")
	}
	userID, err := transport.NewUUIDv7()
	if err != nil {
		return generatedapi.BootstrapAnchorChallengeV1{}, err
	}
	userHandle, err := randomUserHandle()
	if err != nil {
		return generatedapi.BootstrapAnchorChallengeV1{}, err
	}
	user := StoredUser{ID: userID, Handle: userHandle, DisplayName: request.DisplayName, Revision: 1, CreatedAt: now, UpdatedAt: now}
	options, ceremony, err := s.WebAuthn.BeginRegistration(ctx, ceremonyBootstrapRegistration, user, "")
	if err != nil {
		return generatedapi.BootstrapAnchorChallengeV1{}, err
	}
	challenge := make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		return generatedapi.BootstrapAnchorChallengeV1{}, err
	}
	ephemeralPrivate := make([]byte, 32)
	if _, err := rand.Read(ephemeralPrivate); err != nil {
		zeroBytes(challenge)
		return generatedapi.BootstrapAnchorChallengeV1{}, err
	}
	ephemeralPublic, err := curve25519.X25519(ephemeralPrivate, curve25519.Basepoint)
	if err != nil {
		zeroBytes(challenge)
		zeroBytes(ephemeralPrivate)
		return generatedapi.BootstrapAnchorChallengeV1{}, err
	}
	result := generatedapi.BootstrapAnchorChallengeV1{
		Version: generatedapi.BootstrapAnchorChallengeV1VersionN1, CeremonyId: ceremony.ID,
		ServerOrigin: s.Config.PublicOrigin, Anchor: request.Anchor, PasskeyCreationOptions: options,
		Challenge: base64.RawURLEncoding.EncodeToString(challenge), ServerEphemeralExchangePublicKey: base64.RawURLEncoding.EncodeToString(ephemeralPublic),
		StorageAssertionDigest: base64.RawURLEncoding.EncodeToString(anchorMaterial.storageAssertionDigest), ExpiresAt: ceremony.ExpiresAt,
	}
	zeroBytes(challenge)
	ceremony.TokenDigest = tokenDigest
	ceremony.BootstrapChallenge = &result
	if err := s.WebAuthn.Ceremonies.put(ctx, ceremony); err != nil {
		zeroBytes(ephemeralPrivate)
		return generatedapi.BootstrapAnchorChallengeV1{}, err
	}
	s.rememberEphemeral(ceremony.ID, ephemeralPrivate)
	zeroBytes(ephemeralPrivate)
	return result, nil
}

func (s *BootstrapService) Verify(ctx context.Context, token string, request generatedapi.BootstrapVerifyRequestV1) (BootstrapCommitResult, error) {
	now := s.now()
	tokenDigest, err := s.Store.ValidateBootstrapToken(ctx, token, now)
	if err != nil {
		return BootstrapCommitResult{}, err
	}
	ceremony, err := s.WebAuthn.Ceremonies.Load(ctx, request.CeremonyId, ceremonyBootstrapRegistration, now)
	if err != nil {
		return BootstrapCommitResult{}, err
	}
	if ceremony.BootstrapChallenge == nil || subtle.ConstantTimeCompare(tokenDigest[:], ceremony.TokenDigest[:]) != 1 {
		s.forgetEphemeral(request.CeremonyId)
		s.WebAuthn.Ceremonies.consumeFailure(request.CeremonyId, ceremonyBootstrapRegistration)
		return BootstrapCommitResult{}, fmt.Errorf("bootstrap challenge is unavailable")
	}
	ephemeralPrivate, found := s.claimEphemeral(request.CeremonyId)
	if !found {
		s.WebAuthn.Ceremonies.consumeFailure(request.CeremonyId, ceremonyBootstrapRegistration)
		return BootstrapCommitResult{}, fmt.Errorf("webauthn_challenge_replayed")
	}
	if ephemeralPrivate == nil {
		return BootstrapCommitResult{}, fmt.Errorf("webauthn_challenge_replayed")
	}
	defer zeroBytes(ephemeralPrivate)
	defer s.forgetEphemeral(request.CeremonyId)
	committed := false
	defer func() {
		if !committed {
			s.WebAuthn.Ceremonies.consumeFailure(request.CeremonyId, ceremonyBootstrapRegistration)
		}
	}()
	credential, err := s.WebAuthn.FinishRegistration(ceremony, request.Credential)
	if err != nil {
		return BootstrapCommitResult{}, err
	}
	anchorMaterial, err := validateBootstrapAnchor(ceremony.BootstrapChallenge.Anchor)
	if err != nil {
		return BootstrapCommitResult{}, err
	}
	signingProof, err := transport.DecodeBase64URLFixed(request.SigningProof, 64)
	if err != nil {
		return BootstrapCommitResult{}, fmt.Errorf("bootstrap signing proof is invalid")
	}
	exchangeProof, err := transport.DecodeBase64URLFixed(request.ExchangeProof, 32)
	if err != nil {
		return BootstrapCommitResult{}, fmt.Errorf("bootstrap exchange proof is invalid")
	}
	challenge, _ := transport.DecodeBase64URLFixed(ceremony.BootstrapChallenge.Challenge, 32)
	ephemeralPublic, _ := transport.DecodeBase64URLFixed(ceremony.BootstrapChallenge.ServerEphemeralExchangePublicKey, 32)
	proofContext := identitycrypto.DevicePoPContextV1{
		Purpose: "bootstrap", CeremonyID: ceremony.ID, DeviceID: ceremony.BootstrapChallenge.Anchor.DeviceId,
		SigningPublicKey: anchorMaterial.signingPublicKey, ExchangePublicKey: anchorMaterial.exchangePublicKey,
		StorageMode: string(ceremony.BootstrapChallenge.Anchor.StorageMode), StorageAssertionDigest: anchorMaterial.storageAssertionDigest,
		ServerEphemeralExchangePublicKey: ephemeralPublic, Challenge: challenge, ExpiresAt: ceremony.ExpiresAt,
	}
	if err := identitycrypto.VerifyDevicePoPV1(ephemeralPrivate, signingProof, exchangeProof, proofContext); err != nil {
		return BootstrapCommitResult{}, fmt.Errorf("bootstrap Device proof verification failed")
	}
	passkeyID, err := transport.NewUUIDv7()
	if err != nil {
		return BootstrapCommitResult{}, err
	}
	recovery, err := GenerateRecoveryCodeSet(now)
	if err != nil {
		return BootstrapCommitResult{}, err
	}
	session, err := NewBrowserSessionCreate(ceremony.User.ID, "passkey", passkeyID, now)
	if err != nil {
		recovery.ZeroPlaintext()
		return BootstrapCommitResult{}, err
	}
	session.RawCSRF = deriveSessionCSRF(session.RawToken, session.ID)
	signingProofDigest := sha256.Sum256(signingProof)
	exchangeProofDigest := sha256.Sum256(exchangeProof)
	receipt := generatedapi.BootstrapCommitReceiptV1{
		Version: generatedapi.BootstrapCommitReceiptV1VersionN1, Type: generatedapi.BootstrapCommitReceipt,
		CeremonyId: ceremony.ID, UserId: ceremony.User.ID, AnchorDeviceId: ceremony.BootstrapChallenge.Anchor.DeviceId,
		ServerOrigin: s.Config.PublicOrigin, SigningKeyDigest: ceremony.BootstrapChallenge.Anchor.SigningKeyDigest,
		ExchangeKeyDigest: ceremony.BootstrapChallenge.Anchor.ExchangeKeyDigest, StorageMode: generatedapi.BootstrapCommitReceiptV1StorageModePortableVaultV1,
		StorageAssertionDigest: ceremony.BootstrapChallenge.StorageAssertionDigest,
		SigningProofDigest:     base64.RawURLEncoding.EncodeToString(signingProofDigest[:]), ExchangeProofDigest: base64.RawURLEncoding.EncodeToString(exchangeProofDigest[:]), ActivatedAt: now,
	}
	receiptJSON, err := json.Marshal(receipt)
	if err != nil || len(receiptJSON) > 4096 {
		recovery.ZeroPlaintext()
		return BootstrapCommitResult{}, fmt.Errorf("bootstrap receipt is invalid")
	}
	receiptDigest := sha256.Sum256(receiptJSON)
	credential.Authenticator.CloneWarning = false
	transportsJSON, _ := json.Marshal(credential.Transport)
	assertion := ceremony.BootstrapChallenge.Anchor.KeyEnvelopeAssertion
	assertionJSON, err := transport.BootstrapKeyEnvelopeAssertionJCSV1(int(assertion.FormatVersion), int(assertion.KeyRevision), assertion.RecordRevision, assertion.SealedAt, string(assertion.Status))
	if err != nil {
		recovery.ZeroPlaintext()
		zeroBytes(session.RawToken)
		zeroBytes(session.RawCSRF)
		return BootstrapCommitResult{}, err
	}
	capabilitiesJSON, _ := json.Marshal(ceremony.BootstrapChallenge.Anchor.Capabilities)
	commit := BootstrapCommitInput{
		TokenDigest: ceremony.TokenDigest, CeremonyID: ceremony.ID, User: ceremony.User,
		Passkey:        StoredPasskey{ID: passkeyID, UserID: ceremony.User.ID, Credential: *credential, Name: "Initial Passkey", TransportsJSON: transportsJSON, SignCount: credential.Authenticator.SignCount, CredentialRevision: 1, Active: true, CreatedAt: now, UpdatedAt: now},
		AnchorDeviceID: ceremony.BootstrapChallenge.Anchor.DeviceId, AnchorName: ceremony.BootstrapChallenge.Anchor.Name,
		AnchorPlatform: string(ceremony.BootstrapChallenge.Anchor.Platform), AnchorArchitecture: ceremony.BootstrapChallenge.Anchor.Architecture, AnchorClientVersion: ceremony.BootstrapChallenge.Anchor.ClientVersion,
		SigningPublicKey: anchorMaterial.signingPublicKey, ExchangePublicKey: anchorMaterial.exchangePublicKey,
		SigningKeyDigest: anchorMaterial.signingKeyDigest, ExchangeKeyDigest: anchorMaterial.exchangeKeyDigest, PinDigest: anchorMaterial.pinDigest,
		StorageAssertionJSON: assertionJSON, StorageAssertionDigest: anchorMaterial.storageAssertionDigest, CapabilitiesJSON: capabilitiesJSON,
		RecoveryBatchID: recovery.BatchID, RecoveryCodes: recovery.Hashes, BrowserSession: session,
		ReceiptJSON: receiptJSON, ReceiptDigest: receiptDigest[:], At: now,
	}
	if err := s.Store.CommitBootstrap(ctx, commit); err != nil {
		recovery.ZeroPlaintext()
		zeroBytes(session.RawToken)
		zeroBytes(session.RawCSRF)
		return BootstrapCommitResult{}, err
	}
	committed = true
	current := currentAuthDTO(session, session.RawCSRF)
	return BootstrapCommitResult{Receipt: receipt, RecoveryCodes: recovery, Session: session, CurrentAuth: current}, nil
}

func (s *BootstrapService) rememberEphemeral(ceremonyID string, private []byte) {
	s.ephemeralMu.Lock()
	defer s.ephemeralMu.Unlock()
	if s.ephemeral == nil {
		s.ephemeral = make(map[string]*bootstrapEphemeral)
	}
	if previous := s.ephemeral[ceremonyID]; previous != nil {
		zeroBytes(previous.private)
	}
	s.ephemeral[ceremonyID] = &bootstrapEphemeral{private: append([]byte(nil), private...)}
}

// claimEphemeral returns (nil, false) after a restart/missing ceremony and
// (nil, true) to a concurrent loser. Only the unique claimant may consume the
// durable ceremony while verification is in flight.
func (s *BootstrapService) claimEphemeral(ceremonyID string) ([]byte, bool) {
	s.ephemeralMu.Lock()
	defer s.ephemeralMu.Unlock()
	value := s.ephemeral[ceremonyID]
	if value == nil {
		return nil, false
	}
	if value.claimed {
		return nil, true
	}
	value.claimed = true
	return append([]byte(nil), value.private...), true
}

func (s *BootstrapService) forgetEphemeral(ceremonyID string) {
	s.ephemeralMu.Lock()
	defer s.ephemeralMu.Unlock()
	if value := s.ephemeral[ceremonyID]; value != nil {
		zeroBytes(value.private)
		delete(s.ephemeral, ceremonyID)
	}
}

type bootstrapAnchorMaterial struct {
	signingPublicKey       []byte
	exchangePublicKey      []byte
	signingKeyDigest       []byte
	exchangeKeyDigest      []byte
	pinDigest              []byte
	storageAssertionDigest []byte
}

func validateBootstrapAnchor(anchor generatedapi.BootstrapAnchorV1) (bootstrapAnchorMaterial, error) {
	var result bootstrapAnchorMaterial
	if anchor.Kind != generatedapi.BootstrapAnchorV1KindDaemon || anchor.StorageMode != generatedapi.BootstrapAnchorV1StorageModePortableVaultV1 || !anchor.Platform.Valid() || anchor.Name == "" || len(anchor.Name) > 128 || anchor.Architecture == "" || len(anchor.Architecture) > 32 || anchor.ClientVersion == "" || len(anchor.ClientVersion) > 64 {
		return result, fmt.Errorf("bootstrap anchor metadata is invalid")
	}
	if _, err := transport.ParseUUIDv7(anchor.DeviceId); err != nil {
		return result, fmt.Errorf("bootstrap anchor Device ID is invalid")
	}
	var err error
	if result.signingPublicKey, err = transport.DecodeBase64URLFixed(anchor.SigningPublicKey, 32); err != nil {
		return result, err
	}
	if result.exchangePublicKey, err = transport.DecodeBase64URLFixed(anchor.ExchangePublicKey, 32); err != nil {
		return result, err
	}
	if result.signingKeyDigest, err = transport.DecodeBase64URLFixed(anchor.SigningKeyDigest, 32); err != nil {
		return result, err
	}
	if result.exchangeKeyDigest, err = transport.DecodeBase64URLFixed(anchor.ExchangeKeyDigest, 32); err != nil {
		return result, err
	}
	if result.pinDigest, err = transport.DecodeBase64URLFixed(anchor.PinDigest, 32); err != nil {
		return result, err
	}
	signingDigest := sha256.Sum256(result.signingPublicKey)
	exchangeDigest := sha256.Sum256(result.exchangePublicKey)
	pinDigest, err := identitycrypto.DevicePinDigestV1(anchor.DeviceId, result.signingPublicKey, result.exchangePublicKey)
	if err != nil || subtle.ConstantTimeCompare(signingDigest[:], result.signingKeyDigest) != 1 || subtle.ConstantTimeCompare(exchangeDigest[:], result.exchangeKeyDigest) != 1 || subtle.ConstantTimeCompare(pinDigest[:], result.pinDigest) != 1 {
		return result, fmt.Errorf("bootstrap anchor key digest is invalid")
	}
	assertion := anchor.KeyEnvelopeAssertion
	if assertion.FormatVersion != generatedapi.BootstrapKeyEnvelopeAssertionV1FormatVersionN1 || assertion.KeyRevision != generatedapi.BootstrapKeyEnvelopeAssertionV1KeyRevisionN1 || assertion.RecordRevision < 1 || assertion.Status != generatedapi.BootstrapKeyEnvelopeAssertionV1StatusPending || assertion.SealedAt.IsZero() || assertion.SealedAt.Location() != time.UTC {
		return result, fmt.Errorf("bootstrap key-envelope assertion is invalid")
	}
	assertionJSON, err := transport.BootstrapKeyEnvelopeAssertionJCSV1(int(assertion.FormatVersion), int(assertion.KeyRevision), assertion.RecordRevision, assertion.SealedAt, string(assertion.Status))
	if err != nil || len(assertionJSON) > 4096 {
		return result, fmt.Errorf("bootstrap key-envelope assertion is invalid")
	}
	assertionDigest := sha256.Sum256(assertionJSON)
	result.storageAssertionDigest = assertionDigest[:]
	if len(anchor.Capabilities) == 0 || len(anchor.Capabilities) > 32 {
		return result, fmt.Errorf("bootstrap capabilities are invalid")
	}
	capabilities := append(generatedapi.DeviceCapabilityListV1(nil), anchor.Capabilities...)
	if !sort.SliceIsSorted(capabilities, func(i, j int) bool { return capabilities[i] < capabilities[j] }) {
		return result, fmt.Errorf("bootstrap capabilities are not canonical")
	}
	if slices.ContainsFunc(capabilities, func(value generatedapi.DeviceCapabilityV1) bool { return !value.Valid() }) {
		return result, fmt.Errorf("bootstrap capability is invalid")
	}
	for index := 1; index < len(capabilities); index++ {
		if capabilities[index] == capabilities[index-1] {
			return result, fmt.Errorf("bootstrap capability is duplicated")
		}
	}
	return result, nil
}

func currentAuthDTO(session BrowserSessionCreate, rawCSRF []byte) generatedapi.CurrentAuth {
	capabilities := normalAuthCapabilities()
	if session.AuthenticationMethod == "recovery" {
		capabilities = []generatedapi.AuthCapabilityV1{generatedapi.AuthCapabilityV1MadV1PasskeyManage}
	}
	return generatedapi.CurrentAuth{
		UserId: session.UserID, BrowserSessionId: session.ID, AuthenticationMethod: generatedapi.CurrentAuthAuthenticationMethod(session.AuthenticationMethod),
		AuthenticatedAt: session.AuthenticatedAt, RecentUvAt: session.RecentUVAt, ExpiresAt: session.ExpiresAt, IdleExpiresAt: session.IdleExpiresAt,
		CsrfToken: base64.RawURLEncoding.EncodeToString(rawCSRF), Capabilities: capabilities,
	}
}

func currentAuthFromStored(session BrowserSession, rawCSRF []byte) generatedapi.CurrentAuth {
	return currentAuthDTO(BrowserSessionCreate{ID: session.ID, UserID: session.UserID, AuthenticationMethod: session.AuthenticationMethod, AuthenticationPasskeyID: session.AuthenticationPasskeyID, AuthenticatedAt: session.AuthenticatedAt, RecentUVAt: session.RecentUVAt, ExpiresAt: session.ExpiresAt, IdleExpiresAt: session.IdleExpiresAt}, rawCSRF)
}

func normalAuthCapabilities() []generatedapi.AuthCapabilityV1 {
	return []generatedapi.AuthCapabilityV1{
		generatedapi.AuthCapabilityV1MadV1DeviceEnrollApprove, generatedapi.AuthCapabilityV1MadV1DeviceEnrollRequest,
		generatedapi.AuthCapabilityV1MadV1DeviceRevoke, generatedapi.AuthCapabilityV1MadV1MetadataRead,
		generatedapi.AuthCapabilityV1MadV1PasskeyManage, generatedapi.AuthCapabilityV1MadV1ProfileWrite,
		generatedapi.AuthCapabilityV1MadV1SessionCommandCreate, generatedapi.AuthCapabilityV1MadV1SessionRevoke,
	}
}
