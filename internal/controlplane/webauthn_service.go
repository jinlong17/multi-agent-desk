package controlplane

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/go-webauthn/webauthn/webauthn"
	generatedapi "github.com/jinlong17/multi-agent-desk/internal/controlplane/api/generated"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

const webAuthnCeremonyLifetime = 5 * time.Minute
const maxWebAuthnCeremonies = 1024

type ceremonyKind string

const (
	ceremonyBootstrapRegistration ceremonyKind = "bootstrap_registration"
	ceremonyPasskeyLogin          ceremonyKind = "passkey_login"
	ceremonyPasskeyRegistration   ceremonyKind = "passkey_registration"
	ceremonyRecentUV              ceremonyKind = "recent_uv"
)

type webAuthnCeremony struct {
	ID                 string
	Kind               ceremonyKind
	User               StoredUser
	Session            webauthn.SessionData
	BrowserSessionID   string
	TokenDigest        [32]byte
	ExpiresAt          time.Time
	BootstrapChallenge *generatedapi.BootstrapAnchorChallengeV1
}

type WebAuthnService struct {
	Library    *webauthn.WebAuthn
	Ceremonies *CeremonyStore
	RPID       string
	RPName     string
	Now        func() time.Time
}

func NewWebAuthnService(config Config, store *Store) (*WebAuthnService, error) {
	if store == nil {
		return nil, fmt.Errorf("WebAuthn store is required")
	}
	requireResident := false
	library, err := webauthn.New(&webauthn.Config{
		RPID:          config.RPID,
		RPDisplayName: "MultiAgentDesk",
		RPOrigins:     []string{config.PublicOrigin},
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:        protocol.ResidentKeyRequirementPreferred,
			RequireResidentKey: &requireResident,
			UserVerification:   protocol.VerificationRequired,
		},
		AttestationPreference: protocol.PreferNoAttestation,
		Timeouts: webauthn.TimeoutsConfig{
			Registration: webauthn.TimeoutConfig{Enforce: true, Timeout: time.Minute, TimeoutUVD: time.Minute},
			Login:        webauthn.TimeoutConfig{Enforce: true, Timeout: time.Minute, TimeoutUVD: time.Minute},
		},
	})
	if err != nil {
		return nil, err
	}
	service := &WebAuthnService{Library: library, Ceremonies: &CeremonyStore{Store: store}, RPID: config.RPID, RPName: "MultiAgentDesk", Now: time.Now}
	service.Ceremonies.Now = service.now
	return service, nil
}

func (s *WebAuthnService) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *WebAuthnService) BeginRegistration(ctx context.Context, kind ceremonyKind, user StoredUser, browserSessionID string) (generatedapi.WebAuthnCreationOptionsV1, *webAuthnCeremony, error) {
	if ctx == nil || s == nil || s.Library == nil || s.Ceremonies == nil || (kind != ceremonyBootstrapRegistration && kind != ceremonyPasskeyRegistration) {
		return generatedapi.WebAuthnCreationOptionsV1{}, nil, fmt.Errorf("registration service is invalid")
	}
	fixedParameters := []protocol.CredentialParameter{
		{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgES256},
		{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgEdDSA},
		{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgRS256},
	}
	creation, session, err := s.Library.BeginRegistration(user,
		webauthn.WithCredentialParameters(fixedParameters),
		webauthn.WithExclusions(webauthn.Credentials(user.WebAuthnCredentials()).CredentialDescriptors()),
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
	)
	if err != nil {
		return generatedapi.WebAuthnCreationOptionsV1{}, nil, err
	}
	now := s.now()
	session.Expires = now.Add(webAuthnCeremonyLifetime)
	id, err := transport.NewUUIDv7()
	if err != nil {
		return generatedapi.WebAuthnCreationOptionsV1{}, nil, err
	}
	options, err := creationOptionsDTO(id, creation.Response)
	if err != nil {
		return generatedapi.WebAuthnCreationOptionsV1{}, nil, err
	}
	ceremony := &webAuthnCeremony{ID: id, Kind: kind, User: user, Session: *session, BrowserSessionID: browserSessionID, ExpiresAt: now.Add(webAuthnCeremonyLifetime)}
	return options, ceremony, nil
}

func (s *WebAuthnService) BeginAssertion(ctx context.Context, kind ceremonyKind, user StoredUser, browserSessionID string) (generatedapi.WebAuthnRequestOptionsV1, error) {
	if ctx == nil || s == nil || s.Library == nil || s.Ceremonies == nil || (kind != ceremonyPasskeyLogin && kind != ceremonyRecentUV) {
		return generatedapi.WebAuthnRequestOptionsV1{}, fmt.Errorf("assertion service is invalid")
	}
	assertion, session, err := s.Library.BeginLogin(user, webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return generatedapi.WebAuthnRequestOptionsV1{}, err
	}
	now := s.now()
	session.Expires = now.Add(webAuthnCeremonyLifetime)
	id, err := transport.NewUUIDv7()
	if err != nil {
		return generatedapi.WebAuthnRequestOptionsV1{}, err
	}
	options, err := requestOptionsDTO(id, assertion.Response)
	if err != nil {
		return generatedapi.WebAuthnRequestOptionsV1{}, err
	}
	if err := s.Ceremonies.put(ctx, &webAuthnCeremony{ID: id, Kind: kind, User: user, Session: *session, BrowserSessionID: browserSessionID, ExpiresAt: now.Add(webAuthnCeremonyLifetime)}); err != nil {
		return generatedapi.WebAuthnRequestOptionsV1{}, err
	}
	return options, nil
}

func (s *WebAuthnService) FinishRegistration(ceremony *webAuthnCeremony, credential generatedapi.WebAuthnRegistrationCredentialV1) (*webauthn.Credential, error) {
	if ceremony == nil {
		return nil, fmt.Errorf("registration ceremony is missing")
	}
	if err := validateRegistrationCredential(credential); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(credential)
	if err != nil || len(encoded) > 384<<10 {
		return nil, fmt.Errorf("registration credential is invalid")
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(encoded)
	if err != nil {
		return nil, fmt.Errorf("registration credential could not be parsed")
	}
	return s.Library.CreateCredential(ceremony.User, ceremony.Session, parsed)
}

func (s *WebAuthnService) FinishAssertion(ceremony *webAuthnCeremony, credential generatedapi.WebAuthnAssertionCredentialV1) (*webauthn.Credential, uint32, error) {
	if ceremony == nil {
		return nil, 0, fmt.Errorf("assertion ceremony is missing")
	}
	if err := validateAssertionCredential(credential); err != nil {
		return nil, 0, err
	}
	encoded, err := json.Marshal(credential)
	if err != nil || len(encoded) > 256<<10 {
		return nil, 0, fmt.Errorf("assertion credential is invalid")
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(encoded)
	if err != nil {
		return nil, 0, fmt.Errorf("assertion credential could not be parsed")
	}
	observed := parsed.Response.AuthenticatorData.Counter
	result, err := s.Library.ValidateLogin(ceremony.User, ceremony.Session, parsed)
	if err != nil {
		return nil, 0, fmt.Errorf("WebAuthn assertion verification failed")
	}
	return result, observed, nil
}

func creationOptionsDTO(id string, value protocol.PublicKeyCredentialCreationOptions) (generatedapi.WebAuthnCreationOptionsV1, error) {
	userID, err := protocolUserIDBytes(value.User.ID)
	if err != nil || len(value.Challenge) != 32 || len(userID) < 1 || len(userID) > 64 {
		return generatedapi.WebAuthnCreationOptionsV1{}, fmt.Errorf("generated registration options are invalid")
	}
	parameters := make([]generatedapi.WebAuthnCredentialParameterV1, 3)
	if err := parameters[0].FromWebAuthnCredentialParameterV10(generatedapi.WebAuthnCredentialParameterV10{Alg: generatedapi.Minus7, Type: generatedapi.WebAuthnCredentialParameterV10TypePublicKey}); err != nil {
		return generatedapi.WebAuthnCreationOptionsV1{}, err
	}
	if err := parameters[1].FromWebAuthnCredentialParameterV11(generatedapi.WebAuthnCredentialParameterV11{Alg: generatedapi.Minus8, Type: generatedapi.WebAuthnCredentialParameterV11TypePublicKey}); err != nil {
		return generatedapi.WebAuthnCreationOptionsV1{}, err
	}
	if err := parameters[2].FromWebAuthnCredentialParameterV12(generatedapi.WebAuthnCredentialParameterV12{Alg: generatedapi.Minus257, Type: generatedapi.WebAuthnCredentialParameterV12TypePublicKey}); err != nil {
		return generatedapi.WebAuthnCreationOptionsV1{}, err
	}
	excluded := credentialDescriptorsDTO(value.CredentialExcludeList)
	return generatedapi.WebAuthnCreationOptionsV1{CeremonyId: id, PublicKey: generatedapi.WebAuthnCreationPublicKeyV1{
		Challenge:        base64.RawURLEncoding.EncodeToString(value.Challenge),
		Rp:               generatedapi.WebAuthnRelyingPartyV1{Id: value.RelyingParty.ID, Name: value.RelyingParty.Name},
		User:             generatedapi.WebAuthnUserV1{Id: base64.RawURLEncoding.EncodeToString(userID), Name: value.User.Name, DisplayName: value.User.DisplayName},
		PubKeyCredParams: parameters, Timeout: generatedapi.WebAuthnCreationPublicKeyV1TimeoutN60000,
		ExcludeCredentials: excluded,
		AuthenticatorSelection: generatedapi.WebAuthnAuthenticatorSelectionV1{
			ResidentKey: generatedapi.Preferred, RequireResidentKey: generatedapi.WebAuthnAuthenticatorSelectionV1RequireResidentKeyFalse,
			UserVerification: generatedapi.WebAuthnAuthenticatorSelectionV1UserVerificationRequired,
		},
		Attestation: generatedapi.WebAuthnCreationPublicKeyV1AttestationNone, Extensions: generatedapi.WebAuthnExtensionResultsV1{},
	}}, nil
}

func requestOptionsDTO(id string, value protocol.PublicKeyCredentialRequestOptions) (generatedapi.WebAuthnRequestOptionsV1, error) {
	if len(value.Challenge) != 32 || len(value.AllowedCredentials) < 1 || len(value.AllowedCredentials) > 64 {
		return generatedapi.WebAuthnRequestOptionsV1{}, fmt.Errorf("generated assertion options are invalid")
	}
	return generatedapi.WebAuthnRequestOptionsV1{CeremonyId: id, PublicKey: generatedapi.WebAuthnRequestPublicKeyV1{
		Challenge: base64.RawURLEncoding.EncodeToString(value.Challenge), Timeout: generatedapi.WebAuthnRequestPublicKeyV1TimeoutN60000,
		RpId: value.RelyingPartyID, AllowCredentials: credentialDescriptorsDTO(value.AllowedCredentials),
		UserVerification: generatedapi.WebAuthnRequestPublicKeyV1UserVerificationRequired, Extensions: generatedapi.WebAuthnExtensionResultsV1{},
	}}, nil
}

func credentialDescriptorsDTO(values []protocol.CredentialDescriptor) []generatedapi.WebAuthnCredentialDescriptorV1 {
	result := make([]generatedapi.WebAuthnCredentialDescriptorV1, 0, len(values))
	for _, value := range values {
		transports := make([]generatedapi.WebAuthnCredentialDescriptorV1Transports, 0, len(value.Transport))
		for _, transportValue := range value.Transport {
			transports = append(transports, generatedapi.WebAuthnCredentialDescriptorV1Transports(transportValue))
		}
		var pointer *[]generatedapi.WebAuthnCredentialDescriptorV1Transports
		if len(transports) != 0 {
			pointer = &transports
		}
		result = append(result, generatedapi.WebAuthnCredentialDescriptorV1{Type: generatedapi.WebAuthnCredentialDescriptorV1TypePublicKey, Id: base64.RawURLEncoding.EncodeToString(value.CredentialID), Transports: pointer})
	}
	return result
}

func validateRegistrationCredential(value generatedapi.WebAuthnRegistrationCredentialV1) error {
	if value.Type != generatedapi.WebAuthnRegistrationCredentialV1TypePublicKey || value.ClientExtensionResults == nil || len(value.ClientExtensionResults) != 0 {
		return fmt.Errorf("registration credential type or extensions are invalid")
	}
	if value.AuthenticatorAttachment != nil && !value.AuthenticatorAttachment.Valid() {
		return fmt.Errorf("registration authenticator attachment is invalid")
	}
	id, err := decodeCanonicalBase64(value.Id, 1, 1024)
	if err != nil {
		return err
	}
	rawID, err := decodeCanonicalBase64(value.RawId, 1, 1024)
	if err != nil || !bytes.Equal(id, rawID) {
		return fmt.Errorf("registration credential IDs do not match")
	}
	if _, err := decodeCanonicalBase64(value.Response.ClientDataJSON, 1, 65536); err != nil {
		return err
	}
	if _, err := decodeCanonicalBase64(value.Response.AttestationObject, 1, 262144); err != nil {
		return err
	}
	if value.Response.Transports != nil {
		if len(*value.Response.Transports) > 7 || hasDuplicateRegistrationTransports(*value.Response.Transports) {
			return fmt.Errorf("registration transports are invalid")
		}
		for _, item := range *value.Response.Transports {
			if !item.Valid() {
				return fmt.Errorf("registration transport is invalid")
			}
		}
	}
	return nil
}

func validateAssertionCredential(value generatedapi.WebAuthnAssertionCredentialV1) error {
	if value.Type != generatedapi.WebAuthnAssertionCredentialV1TypePublicKey || value.ClientExtensionResults == nil || len(value.ClientExtensionResults) != 0 {
		return fmt.Errorf("assertion credential type or extensions are invalid")
	}
	if value.AuthenticatorAttachment != nil && !value.AuthenticatorAttachment.Valid() {
		return fmt.Errorf("assertion authenticator attachment is invalid")
	}
	id, err := decodeCanonicalBase64(value.Id, 1, 1024)
	if err != nil {
		return err
	}
	rawID, err := decodeCanonicalBase64(value.RawId, 1, 1024)
	if err != nil || !bytes.Equal(id, rawID) {
		return fmt.Errorf("assertion credential IDs do not match")
	}
	for encoded, bounds := range map[string][2]int{
		value.Response.ClientDataJSON:    {1, 65536},
		value.Response.AuthenticatorData: {37, 65536},
		value.Response.Signature:         {1, 65536},
	} {
		if _, err := decodeCanonicalBase64(encoded, bounds[0], bounds[1]); err != nil {
			return err
		}
	}
	if value.Response.UserHandle != nil {
		if _, err := decodeCanonicalBase64(*value.Response.UserHandle, 1, 1024); err != nil {
			return err
		}
	}
	return nil
}

func decodeCanonicalBase64(value string, minimum, maximum int) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) < minimum || len(decoded) > maximum || base64.RawURLEncoding.EncodeToString(decoded) != value {
		return nil, fmt.Errorf("WebAuthn binary field is invalid")
	}
	return decoded, nil
}

func hasDuplicateRegistrationTransports(values []generatedapi.WebAuthnRegistrationResponseV1Transports) bool {
	seen := make(map[generatedapi.WebAuthnRegistrationResponseV1Transports]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}

func randomUserHandle() ([]byte, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return nil, err
	}
	return value, nil
}

func protocolUserIDBytes(value any) ([]byte, error) {
	switch typed := value.(type) {
	case protocol.URLEncodedBase64:
		return append([]byte(nil), typed...), nil
	case []byte:
		return append([]byte(nil), typed...), nil
	default:
		return nil, fmt.Errorf("generated WebAuthn user ID has an unexpected type")
	}
}
