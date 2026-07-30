package controlplane

import (
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	generatedapi "github.com/jinlong17/multi-agent-desk/internal/controlplane/api/generated"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

func TestWebAuthnExactOptionsAndOneShotCeremony(t *testing.T) {
	store := openTestStore(t, filepath.Join(privateTestDirectory(t), "server.sqlite"))
	service, err := NewWebAuthnService(Config{PublicOrigin: "https://control.example.test", RPID: "example.test"}, store)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	service.Now = func() time.Time { return now }
	user := StoredUser{ID: "018f47a0-7b1c-7cc2-8000-000000000001", Handle: []byte("0123456789abcdef0123456789abcdef"), DisplayName: "Owner"}
	options, ceremony, err := service.BeginRegistration(t.Context(), ceremonyBootstrapRegistration, user, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if options.CeremonyId != ceremony.ID || options.PublicKey.Timeout != 60000 || options.PublicKey.AuthenticatorSelection.UserVerification != generatedapi.WebAuthnAuthenticatorSelectionV1UserVerificationRequired || options.PublicKey.Attestation != generatedapi.WebAuthnCreationPublicKeyV1AttestationNone || len(options.PublicKey.Extensions) != 0 || len(options.PublicKey.PubKeyCredParams) != 3 {
		t.Fatalf("creation options drifted: %+v", options)
	}
	first, _ := options.PublicKey.PubKeyCredParams[0].AsWebAuthnCredentialParameterV10()
	second, _ := options.PublicKey.PubKeyCredParams[1].AsWebAuthnCredentialParameterV11()
	third, _ := options.PublicKey.PubKeyCredParams[2].AsWebAuthnCredentialParameterV12()
	if first.Alg != -7 || second.Alg != -8 || third.Alg != -257 {
		t.Fatalf("credential algorithms drifted: %d %d %d", first.Alg, second.Alg, third.Alg)
	}
	if err := service.Ceremonies.put(t.Context(), ceremony); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewWebAuthnService(Config{PublicOrigin: "https://control.example.test", RPID: "example.test"}, store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Ceremonies.Load(t.Context(), ceremony.ID, ceremonyBootstrapRegistration, now); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Ceremonies.Consume(t.Context(), ceremony.ID, ceremonyBootstrapRegistration); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Ceremonies.Load(t.Context(), ceremony.ID, ceremonyBootstrapRegistration, now); err == nil {
		t.Fatal("ceremony replay was accepted")
	}

	credentialID := []byte("credential-id")
	user.Credentials = []webauthn.Credential{{ID: credentialID}}
	assertion, err := service.BeginAssertion(t.Context(), ceremonyPasskeyLogin, user, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if assertion.PublicKey.Timeout != 60000 || assertion.PublicKey.RpId != "example.test" || len(assertion.PublicKey.AllowCredentials) != 1 || assertion.PublicKey.AllowCredentials[0].Id != base64.RawURLEncoding.EncodeToString(credentialID) || len(assertion.PublicKey.Extensions) != 0 {
		t.Fatalf("assertion options drifted: %+v", assertion)
	}
}

func TestWebAuthnCeremonyCapacityIsBoundedAndExpiredRowsAreReclaimed(t *testing.T) {
	store := openTestStore(t, filepath.Join(privateTestDirectory(t), "server.sqlite"))
	now := time.Now().UTC()
	tx, err := store.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	firstID := ""
	for range maxWebAuthnCeremonies {
		id, err := transport.NewUUIDv7()
		if err != nil {
			t.Fatal(err)
		}
		if firstID == "" {
			firstID = id
		}
		if _, err := tx.Exec(`INSERT INTO webauthn_ceremonies(id,kind,payload_json,expires_at,created_at) VALUES(?,?,?,?,?)`, id, string(ceremonyPasskeyLogin), []byte("{}"), formatServerTime(now.Add(time.Minute)), formatServerTime(now)); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	service, err := NewWebAuthnService(Config{PublicOrigin: "https://control.example.test", RPID: "example.test"}, store)
	if err != nil {
		t.Fatal(err)
	}
	service.Now = func() time.Time { return now }
	service.Ceremonies.Now = service.now
	id, _ := transport.NewUUIDv7()
	value := &webAuthnCeremony{ID: id, Kind: ceremonyPasskeyLogin, ExpiresAt: now.Add(time.Minute)}
	if err := service.Ceremonies.put(t.Context(), value); err == nil || !strings.Contains(err.Error(), "capacity") {
		t.Fatalf("full ceremony store err=%v", err)
	}
	if _, err := store.db.Exec(`UPDATE webauthn_ceremonies SET expires_at=? WHERE id=?`, formatServerTime(now.Add(-time.Second)), firstID); err != nil {
		t.Fatal(err)
	}
	if err := service.Ceremonies.put(t.Context(), value); err != nil {
		t.Fatalf("expired ceremony row was not reclaimed: %v", err)
	}
}

func TestWebAuthnCredentialBoundaryRejectsExtensionsAndNonCanonicalBase64(t *testing.T) {
	registration := generatedapi.WebAuthnRegistrationCredentialV1{
		Id: "YQ", RawId: "YQ", Type: generatedapi.WebAuthnRegistrationCredentialV1TypePublicKey,
		ClientExtensionResults: generatedapi.WebAuthnExtensionResultsV1{},
		Response:               generatedapi.WebAuthnRegistrationResponseV1{ClientDataJSON: "YQ", AttestationObject: "YQ"},
	}
	if err := validateRegistrationCredential(registration); err != nil {
		t.Fatal(err)
	}
	registration.Id = "YQ=="
	if err := validateRegistrationCredential(registration); err == nil {
		t.Fatal("padded Base64 was accepted")
	}
	registration.Id = "YQ"
	registration.ClientExtensionResults["prf"] = true
	if err := validateRegistrationCredential(registration); err == nil {
		t.Fatal("unknown WebAuthn extension was accepted")
	}
	registration.ClientExtensionResults = nil
	if err := validateRegistrationCredential(registration); err == nil {
		t.Fatal("null WebAuthn extensions were accepted")
	}
}
