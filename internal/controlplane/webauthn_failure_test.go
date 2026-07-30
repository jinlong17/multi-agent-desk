package controlplane

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/go-webauthn/webauthn/webauthn"
	generatedapi "github.com/jinlong17/multi-agent-desk/internal/controlplane/api/generated"
)

func testCredentialWithPublicKey(t *testing.T, credential webauthn.Credential) webauthn.Credential {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	x := privateKey.PublicKey.X.FillBytes(make([]byte, 32))
	y := privateKey.PublicKey.Y.FillBytes(make([]byte, 32))
	credential.PublicKey, err = webauthncbor.Marshal(webauthncose.EC2PublicKeyData{
		PublicKeyData: webauthncose.PublicKeyData{KeyType: int64(webauthncose.EllipticKey), Algorithm: int64(webauthncose.AlgES256)},
		Curve:         int64(webauthncose.P256), XCoord: x, YCoord: y,
	})
	if err != nil {
		t.Fatal(err)
	}
	return credential
}

func assertionFailureCredential(t *testing.T, ceremony *webAuthnCeremony, credentialID []byte, origin string) generatedapi.WebAuthnAssertionCredentialV1 {
	t.Helper()
	clientData, err := json.Marshal(map[string]any{"type": "webauthn.get", "challenge": ceremony.Session.Challenge, "origin": origin, "crossOrigin": false})
	if err != nil {
		t.Fatal(err)
	}
	authenticatorData := make([]byte, 37)
	rpDigest := sha256.Sum256([]byte("example.test"))
	copy(authenticatorData, rpDigest[:])
	authenticatorData[32] = 0x05 // user present + user verified
	binary.BigEndian.PutUint32(authenticatorData[33:], 1)
	id := base64.RawURLEncoding.EncodeToString(credentialID)
	return generatedapi.WebAuthnAssertionCredentialV1{
		Id: id, RawId: id, Type: generatedapi.WebAuthnAssertionCredentialV1TypePublicKey,
		ClientExtensionResults: generatedapi.WebAuthnExtensionResultsV1{},
		Response: generatedapi.WebAuthnAssertionResponseV1{
			ClientDataJSON:    base64.RawURLEncoding.EncodeToString(clientData),
			AuthenticatorData: base64.RawURLEncoding.EncodeToString(authenticatorData),
			Signature:         base64.RawURLEncoding.EncodeToString([]byte{0x30, 0x00}),
		},
	}
}

func assertCeremonyConsumed(t *testing.T, ceremonies *CeremonyStore, id string, kind ceremonyKind, now time.Time) {
	t.Helper()
	if _, err := ceremonies.Load(t.Context(), id, kind, now); err == nil || !strings.Contains(err.Error(), "replayed") {
		t.Fatalf("ceremony %s/%s remained replayable: %v", kind, id, err)
	}
}

func TestWebAuthnFailuresConsumeBootstrapLoginRegistrationAndUV(t *testing.T) {
	now := time.Now().UTC()
	config := Config{PublicOrigin: "https://control.example.test", RPID: "example.test"}

	t.Run("bootstrap-parse", func(t *testing.T) {
		store := openTestStore(t, filepath.Join(privateTestDirectory(t), "server.sqlite"))
		token, _, err := store.EnsureBootstrapToken(t.Context(), now)
		if err != nil {
			t.Fatal(err)
		}
		tokenDigest, err := store.ValidateBootstrapToken(t.Context(), token, now)
		if err != nil {
			t.Fatal(err)
		}
		service, err := NewWebAuthnService(config, store)
		if err != nil {
			t.Fatal(err)
		}
		user := StoredUser{ID: "018f47a0-7b1c-7cc2-8000-000000000001", Handle: []byte("0123456789abcdef0123456789abcdef"), DisplayName: "Owner"}
		_, ceremony, err := service.BeginRegistration(t.Context(), ceremonyBootstrapRegistration, user, "", 0)
		if err != nil {
			t.Fatal(err)
		}
		ceremony.TokenDigest = tokenDigest
		ceremony.BootstrapChallenge = &generatedapi.BootstrapAnchorChallengeV1{}
		if err := service.Ceremonies.put(t.Context(), ceremony); err != nil {
			t.Fatal(err)
		}
		bootstrap := &BootstrapService{Config: config, Store: store, WebAuthn: service, Now: func() time.Time { return now }}
		if _, err := bootstrap.Verify(t.Context(), token, generatedapi.BootstrapVerifyRequestV1{CeremonyId: ceremony.ID}); err == nil {
			t.Fatal("invalid bootstrap credential was accepted")
		}
		assertCeremonyConsumed(t, service.Ceremonies, ceremony.ID, ceremonyBootstrapRegistration, now)
	})

	store := openTestStore(t, filepath.Join(privateTestDirectory(t), "server.sqlite"))
	user, passkey, rawSession := seedPasskeySession(t, store, 0)
	passkey.Credential = testCredentialWithPublicKey(t, passkey.Credential)
	credentialJSON, _ := json.Marshal(passkey.Credential)
	if _, err := store.db.Exec(`UPDATE passkeys SET credential_json=? WHERE id=?`, credentialJSON, passkey.ID); err != nil {
		t.Fatal(err)
	}
	user.Credentials = []webauthn.Credential{passkey.Credential}
	service, err := NewWebAuthnService(config, store)
	if err != nil {
		t.Fatal(err)
	}
	service.Now = func() time.Time { return now }
	service.Ceremonies.Now = service.now
	auth := &AuthService{Config: config, Store: store, WebAuthn: service, Now: func() time.Time { return now }}
	authenticated, err := store.BrowserSessionByToken(t.Context(), rawSession.RawToken, now)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name   string
		origin string
	}{
		{name: "login-origin", origin: "https://evil.example.test"},
		{name: "login-signature", origin: config.PublicOrigin},
	} {
		t.Run(test.name, func(t *testing.T) {
			options, err := auth.BeginLogin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			ceremony, err := service.Ceremonies.Load(t.Context(), options.CeremonyId, ceremonyPasskeyLogin, now)
			if err != nil {
				t.Fatal(err)
			}
			credential := assertionFailureCredential(t, ceremony, passkey.Credential.ID, test.origin)
			if _, err := auth.VerifyLogin(t.Context(), generatedapi.WebAuthnAssertionVerifyRequestV1{CeremonyId: ceremony.ID, Credential: credential}); err == nil {
				t.Fatal("invalid assertion was accepted")
			}
			assertCeremonyConsumed(t, service.Ceremonies, ceremony.ID, ceremonyPasskeyLogin, now)
		})
	}

	t.Run("registration-parse", func(t *testing.T) {
		options, err := auth.BeginRegistration(t.Context(), authenticated)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := auth.VerifyRegistration(t.Context(), authenticated, generatedapi.WebAuthnRegistrationVerifyRequestV1{CeremonyId: options.CeremonyId}); err == nil {
			t.Fatal("invalid registration was accepted")
		}
		assertCeremonyConsumed(t, service.Ceremonies, options.CeremonyId, ceremonyPasskeyRegistration, now)
	})

	t.Run("uv-parse", func(t *testing.T) {
		options, err := auth.BeginUV(t.Context(), authenticated)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := auth.VerifyUV(t.Context(), authenticated, generatedapi.WebAuthnAssertionVerifyRequestV1{CeremonyId: options.CeremonyId}); err == nil {
			t.Fatal("invalid UV assertion was accepted")
		}
		assertCeremonyConsumed(t, service.Ceremonies, options.CeremonyId, ceremonyRecentUV, now)
	})

}
