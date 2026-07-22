package controlplane

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/jinlong17/multi-agent-desk/internal/app"
	generatedapi "github.com/jinlong17/multi-agent-desk/internal/controlplane/api/generated"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
	"github.com/jinlong17/multi-agent-desk/internal/vault"
)

func remoteBootstrapFixture(t *testing.T, now time.Time) (*storage.Store, *vault.Manager, *app.RemoteBootstrapService, generatedapi.BootstrapAnchorDescriptorV1) {
	t.Helper()
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "device", "device.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	deviceID, err := domain.NewID("device")
	if err != nil {
		t.Fatal(err)
	}
	clientID, err := domain.NewID("client")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "anchor", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: clientID, Name: "owner", PublicKey: bytes.Repeat([]byte{1}, 32), Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityVaultControl}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	manager, err := vault.NewPersistentManager(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	password := []byte("bootstrap-integration-password")
	if _, err := manager.Initialize(ctx, clientID, "bootstrap-integration", password, now); err != nil {
		t.Fatal(err)
	}
	if err := manager.Unlock(password); err != nil {
		t.Fatal(err)
	}
	service := &app.RemoteBootstrapService{Store: store, Vault: manager, Now: func() time.Time { return now }, ClientVersion: "0.1.0-test", Platform: "darwin", Architecture: "arm64"}
	descriptor, err := service.Prepare(ctx, app.BootstrapPrepareInput{ServerOrigin: "https://control.example.test", Name: "Test Daemon"})
	if err != nil {
		t.Fatal(err)
	}
	return store, manager, service, descriptor
}

func validRegistrationCredential(t *testing.T, options generatedapi.WebAuthnCreationOptionsV1, serverOrigin string) (generatedapi.WebAuthnRegistrationCredentialV1, *ecdsa.PrivateKey) {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	credentialPublicKey, err := webauthncbor.Marshal(webauthncose.EC2PublicKeyData{
		PublicKeyData: webauthncose.PublicKeyData{KeyType: int64(webauthncose.EllipticKey), Algorithm: int64(webauthncose.AlgES256)},
		Curve:         int64(webauthncose.P256),
		XCoord:        privateKey.PublicKey.X.FillBytes(make([]byte, 32)),
		YCoord:        privateKey.PublicKey.Y.FillBytes(make([]byte, 32)),
	})
	if err != nil {
		t.Fatal(err)
	}
	credentialID := make([]byte, 32)
	if _, err := rand.Read(credentialID); err != nil {
		t.Fatal(err)
	}
	rpIDHash := sha256.Sum256([]byte(options.PublicKey.Rp.Id))
	authenticatorData := make([]byte, 0, 37+16+2+len(credentialID)+len(credentialPublicKey))
	authenticatorData = append(authenticatorData, rpIDHash[:]...)
	authenticatorData = append(authenticatorData, byte(0x45)) // UP + UV + attested credential data.
	counter := make([]byte, 4)
	binary.BigEndian.PutUint32(counter, 0)
	authenticatorData = append(authenticatorData, counter...)
	authenticatorData = append(authenticatorData, make([]byte, 16)...)
	credentialLength := make([]byte, 2)
	binary.BigEndian.PutUint16(credentialLength, uint16(len(credentialID)))
	authenticatorData = append(authenticatorData, credentialLength...)
	authenticatorData = append(authenticatorData, credentialID...)
	authenticatorData = append(authenticatorData, credentialPublicKey...)
	attestationObject, err := webauthncbor.Marshal(map[string]any{
		"fmt": "none", "authData": authenticatorData, "attStmt": map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientData, err := json.Marshal(map[string]any{
		"type": "webauthn.create", "challenge": options.PublicKey.Challenge,
		"origin": serverOrigin, "crossOrigin": false,
	})
	if err != nil {
		t.Fatal(err)
	}
	encodedID := base64.RawURLEncoding.EncodeToString(credentialID)
	return generatedapi.WebAuthnRegistrationCredentialV1{
		Id: encodedID, RawId: encodedID, Type: generatedapi.WebAuthnRegistrationCredentialV1TypePublicKey,
		ClientExtensionResults: generatedapi.WebAuthnExtensionResultsV1{},
		Response: generatedapi.WebAuthnRegistrationResponseV1{
			ClientDataJSON:    base64.RawURLEncoding.EncodeToString(clientData),
			AttestationObject: base64.RawURLEncoding.EncodeToString(attestationObject),
		},
	}, privateKey
}

func validAssertionCredential(t *testing.T, options generatedapi.WebAuthnRequestOptionsV1, serverOrigin, credentialID string, privateKey *ecdsa.PrivateKey, counter uint32) generatedapi.WebAuthnAssertionCredentialV1 {
	t.Helper()
	clientData, err := json.Marshal(map[string]any{
		"type": "webauthn.get", "challenge": options.PublicKey.Challenge,
		"origin": serverOrigin, "crossOrigin": false,
	})
	if err != nil {
		t.Fatal(err)
	}
	rpIDHash := sha256.Sum256([]byte(options.PublicKey.RpId))
	authenticatorData := make([]byte, 37)
	copy(authenticatorData, rpIDHash[:])
	authenticatorData[32] = 0x05 // UP + UV.
	binary.BigEndian.PutUint32(authenticatorData[33:], counter)
	clientHash := sha256.Sum256(clientData)
	signed := make([]byte, 0, len(authenticatorData)+len(clientHash))
	signed = append(signed, authenticatorData...)
	signed = append(signed, clientHash[:]...)
	signedHash := sha256.Sum256(signed)
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, signedHash[:])
	if err != nil {
		t.Fatal(err)
	}
	return generatedapi.WebAuthnAssertionCredentialV1{
		Id: credentialID, RawId: credentialID, Type: generatedapi.WebAuthnAssertionCredentialV1TypePublicKey,
		ClientExtensionResults: generatedapi.WebAuthnExtensionResultsV1{},
		Response: generatedapi.WebAuthnAssertionResponseV1{
			ClientDataJSON:    base64.RawURLEncoding.EncodeToString(clientData),
			AuthenticatorData: base64.RawURLEncoding.EncodeToString(authenticatorData),
			Signature:         base64.RawURLEncoding.EncodeToString(signature),
		},
	}
}

func TestDaemonAnchoredBootstrapRestartFreshStartCommitAndActivate(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1_900_000_000, 123_000_000).UTC()
	localStore, _, remote, descriptor := remoteBootstrapFixture(t, now)
	serverStore := openTestStore(t, filepath.Join(privateTestDirectory(t), "server.sqlite"))
	config := Config{Listen: "127.0.0.1:0", PublicOrigin: descriptor.ServerOrigin, RPID: "example.test", shutdownTimeout: time.Second}
	token, created, err := serverStore.EnsureBootstrapToken(ctx, now)
	if err != nil || !created {
		t.Fatalf("bootstrap token created=%v err=%v", created, err)
	}

	firstWebAuthn, err := NewWebAuthnService(config, serverStore)
	if err != nil {
		t.Fatal(err)
	}
	firstWebAuthn.Now = func() time.Time { return now }
	firstWebAuthn.Ceremonies.Now = firstWebAuthn.now
	firstBootstrap := &BootstrapService{Config: config, Store: serverStore, WebAuthn: firstWebAuthn, Now: func() time.Time { return now }}
	staleChallenge, err := firstBootstrap.Begin(ctx, token, generatedapi.BootstrapOptionsRequestV1{DisplayName: "Owner", Anchor: descriptor.Anchor})
	if err != nil {
		t.Fatal(err)
	}
	var storedPayload []byte
	if err := serverStore.db.QueryRow(`SELECT payload_json FROM webauthn_ceremonies WHERE id=?`, staleChallenge.CeremonyId).Scan(&storedPayload); err != nil {
		t.Fatal(err)
	}
	firstBootstrap.ephemeralMu.Lock()
	privateCopy := append([]byte(nil), firstBootstrap.ephemeral[staleChallenge.CeremonyId].private...)
	firstBootstrap.ephemeralMu.Unlock()
	defer zeroBytes(privateCopy)
	if bytes.Contains(storedPayload, []byte("ServerEphemeralPrivate")) || bytes.Contains(storedPayload, []byte(base64.StdEncoding.EncodeToString(privateCopy))) {
		t.Fatal("server ephemeral X25519 private key entered durable ceremony state")
	}

	restarted, err := NewServer(config, serverStore)
	if err != nil {
		t.Fatal(err)
	}
	firstBootstrap.forgetEphemeral(staleChallenge.CeremonyId)
	if _, err := restarted.webauthn.Ceremonies.Load(ctx, staleChallenge.CeremonyId, ceremonyBootstrapRegistration, now); err == nil {
		t.Fatal("restart-local bootstrap ceremony survived server reconstruction")
	}
	restarted.webauthn.Now = func() time.Time { return now }
	restarted.webauthn.Ceremonies.Now = restarted.webauthn.now
	restarted.bootstrap.Now = func() time.Time { return now }
	challenge, err := restarted.bootstrap.Begin(ctx, token, generatedapi.BootstrapOptionsRequestV1{DisplayName: "Owner", Anchor: descriptor.Anchor})
	if err != nil {
		t.Fatal(err)
	}
	proof, err := remote.Prove(ctx, challenge, challenge)
	if err != nil {
		t.Fatal(err)
	}
	initialCredential, initialPrivateKey := validRegistrationCredential(t, challenge.PasskeyCreationOptions, challenge.ServerOrigin)
	verifyRequest := generatedapi.BootstrapVerifyRequestV1{
		CeremonyId:    challenge.CeremonyId,
		Credential:    initialCredential,
		SigningProof:  proof.SigningProof,
		ExchangeProof: proof.ExchangeProof,
	}
	type verifyOutcome struct {
		result BootstrapCommitResult
		err    error
	}
	start := make(chan struct{})
	outcomes := make(chan verifyOutcome, 2)
	for range 2 {
		go func() {
			<-start
			value, verifyErr := restarted.bootstrap.Verify(ctx, token, verifyRequest)
			outcomes <- verifyOutcome{result: value, err: verifyErr}
		}()
	}
	close(start)
	var result BootstrapCommitResult
	succeeded, failed := 0, 0
	for range 2 {
		outcome := <-outcomes
		if outcome.err != nil {
			failed++
			continue
		}
		succeeded++
		result = outcome.result
	}
	if succeeded != 1 || failed != 1 {
		t.Fatalf("parallel bootstrap finishes succeeded=%d failed=%d", succeeded, failed)
	}
	defer result.RecoveryCodes.ZeroPlaintext()
	defer zeroBytes(result.Session.RawToken)
	defer zeroBytes(result.Session.RawCSRF)
	if len(result.RecoveryCodes.Plaintext) != 10 || result.CurrentAuth.AuthenticationMethod != generatedapi.CurrentAuthAuthenticationMethodPasskey || result.Receipt.AnchorDeviceId != descriptor.Anchor.DeviceId {
		t.Fatalf("bootstrap result drifted: auth_method_match=%t anchor_match=%t recovery_count=%d", result.CurrentAuth.AuthenticationMethod == generatedapi.CurrentAuthAuthenticationMethodPasskey, result.Receipt.AnchorDeviceId == descriptor.Anchor.DeviceId, len(result.RecoveryCodes.Plaintext))
	}
	if _, err := serverStore.BrowserSessionByToken(ctx, result.Session.RawToken, now); err != nil {
		t.Fatalf("bootstrap session was not committed: %v", err)
	}
	state, err := serverStore.BootstrapState(ctx, now)
	if err != nil || !state.Initialized {
		t.Fatalf("bootstrap state invalid: initialized=%t err_present=%t", state.Initialized, err != nil)
	}
	for table, want := range map[string]int{"users": 1, "passkeys": 1, "anchor_devices": 1, "recovery_batches": 1, "recovery_codes": 10, "bootstrap_receipts": 1} {
		var count int
		if err := serverStore.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&count); err != nil || count != want {
			t.Fatalf("table %s count=%d want=%d err=%v", table, count, want, err)
		}
	}
	if _, err := restarted.bootstrap.Verify(ctx, token, generatedapi.BootstrapVerifyRequestV1{CeremonyId: challenge.CeremonyId}); err == nil {
		t.Fatal("completed bootstrap was replayed")
	}

	activated, err := remote.Activate(ctx, result.Receipt, result.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	if activated.CeremonyId != challenge.CeremonyId {
		t.Fatalf("activated receipt ceremony mismatch")
	}
	if _, err := remote.Activate(ctx, result.Receipt, result.Receipt); err != nil {
		t.Fatalf("exact activation replay was not idempotent: %v", err)
	}
	active, err := localStore.ActiveRemoteDeviceIdentityForOrigin(ctx, descriptor.ServerOrigin)
	if err != nil || active.ServerDeviceID != descriptor.Anchor.DeviceId || active.Lifecycle != storage.RemoteIdentityActive {
		t.Fatalf("active remote identity invalid: err_present=%t device_match=%t lifecycle_match=%t", err != nil, active.ServerDeviceID == descriptor.Anchor.DeviceId, active.Lifecycle == storage.RemoteIdentityActive)
	}
	mapping, err := localStore.ControlPlaneMapping(ctx, "device", active.ID)
	if err != nil || mapping.ServerID != descriptor.Anchor.DeviceId {
		t.Fatalf("active mapping invalid: err_present=%t server_id_match=%t", err != nil, mapping.ServerID == descriptor.Anchor.DeviceId)
	}

	restarted.auth.Now = func() time.Time { return now.Add(time.Second) }
	restarted.webauthn.Now = restarted.auth.Now
	loginOptions, err := restarted.auth.BeginLogin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	loginResult, err := restarted.auth.VerifyLogin(ctx, generatedapi.WebAuthnAssertionVerifyRequestV1{
		CeremonyId: loginOptions.CeremonyId,
		Credential: validAssertionCredential(t, loginOptions, config.PublicOrigin, initialCredential.Id, initialPrivateKey, 1),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer zeroBytes(loginResult.Session.RawToken)
	defer zeroBytes(loginResult.Session.RawCSRF)
	storedLogin, err := serverStore.BrowserSessionByToken(ctx, loginResult.Session.RawToken, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}

	restarted.auth.Now = func() time.Time { return now.Add(2 * time.Second) }
	restarted.webauthn.Now = restarted.auth.Now
	uvOptions, err := restarted.auth.BeginUV(ctx, storedLogin)
	if err != nil {
		t.Fatal(err)
	}
	uvResult, err := restarted.auth.VerifyUV(ctx, storedLogin, generatedapi.WebAuthnAssertionVerifyRequestV1{
		CeremonyId: uvOptions.CeremonyId,
		Credential: validAssertionCredential(t, uvOptions, config.PublicOrigin, initialCredential.Id, initialPrivateKey, 2),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer zeroBytes(uvResult.Session.RawToken)
	defer zeroBytes(uvResult.Session.RawCSRF)
	if _, err := serverStore.BrowserSessionByToken(ctx, loginResult.Session.RawToken, now.Add(2*time.Second)); err == nil {
		t.Fatal("UV rotation left the prior login session active")
	}
	storedUV, err := serverStore.BrowserSessionByToken(ctx, uvResult.Session.RawToken, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	restarted.auth.Now = func() time.Time { return now.Add(3 * time.Second) }
	rotated, err := restarted.auth.RotateRecoveryCodes(ctx, storedUV)
	if err != nil {
		t.Fatal(err)
	}
	defer rotated.ZeroPlaintext()
	// Replay is owned by the HTTP AuthIdempotencyOperation gate in v0.9;
	// direct service calls deliberately have no parallel marker table.

	restarted.auth.Now = func() time.Time { return now.Add(4 * time.Second) }
	limiter := &RecoveryLimiter{Now: restarted.auth.Now}
	recoveryResult, err := restarted.auth.VerifyRecovery(ctx, limiter, "192.0.2.10", rotated.Plaintext[0])
	if err != nil {
		t.Fatal(err)
	}
	defer zeroBytes(recoveryResult.Session.RawToken)
	defer zeroBytes(recoveryResult.Session.RawCSRF)
	storedRecovery, err := serverStore.BrowserSessionByToken(ctx, recoveryResult.Session.RawToken, now.Add(4*time.Second))
	if err != nil || recoveryResult.CurrentAuth.AuthenticationMethod != generatedapi.CurrentAuthAuthenticationMethodRecovery || len(recoveryResult.CurrentAuth.Capabilities) != 1 {
		t.Fatalf("recovery session read failed: err_present=%t auth_method_match=%t capability_count=%d session_id_present=%t", err != nil, recoveryResult.CurrentAuth.AuthenticationMethod == generatedapi.CurrentAuthAuthenticationMethodRecovery, len(recoveryResult.CurrentAuth.Capabilities), storedRecovery.ID != "")
	}

	restarted.auth.Now = func() time.Time { return now.Add(5 * time.Second) }
	restarted.webauthn.Now = restarted.auth.Now
	replacementOptions, err := restarted.auth.BeginRegistration(ctx, storedRecovery)
	if err != nil {
		t.Fatal(err)
	}
	replacementCredential, _ := validRegistrationCredential(t, replacementOptions, config.PublicOrigin)
	replacementResult, err := restarted.auth.VerifyRegistration(ctx, storedRecovery, generatedapi.WebAuthnRegistrationVerifyRequestV1{
		CeremonyId: replacementOptions.CeremonyId, Credential: replacementCredential,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer zeroBytes(replacementResult.Session.RawToken)
	defer zeroBytes(replacementResult.Session.RawCSRF)
	for label, raw := range map[string][]byte{
		"bootstrap": result.Session.RawToken, "uv": uvResult.Session.RawToken, "recovery": recoveryResult.Session.RawToken,
	} {
		if _, err := serverStore.BrowserSessionByToken(ctx, raw, now.Add(5*time.Second)); err == nil {
			t.Fatalf("replacement registration left %s session active", label)
		}
	}
	if _, err := serverStore.BrowserSessionByToken(ctx, replacementResult.Session.RawToken, now.Add(5*time.Second)); err != nil {
		t.Fatalf("replacement normal session is not active: %v", err)
	}
	storedInitial, err := serverStore.PasskeyByCredentialID(ctx, mustDecodeBase64URL(t, initialCredential.Id))
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := serverStore.DeletePasskeyCAS(ctx, result.CurrentAuth.UserId, storedInitial.ID, replacementResult.Session.ID, storedInitial.CredentialRevision, now.Add(6*time.Second))
	if err != nil || deleted.CurrentSessionRevoked || deleted.RevokedSessionCount != 0 {
		t.Fatalf("old Passkey delete invalid: err_present=%t current_revoked=%t revoked_count=%d", err != nil, deleted.CurrentSessionRevoked, deleted.RevokedSessionCount)
	}
	remaining, err := serverStore.ListPasskeys(ctx, result.CurrentAuth.UserId)
	if err != nil || len(remaining) != 1 || remaining[0].Credential.ID == nil || base64.RawURLEncoding.EncodeToString(remaining[0].Credential.ID) != replacementCredential.Id {
		t.Fatalf("remaining Passkeys invalid: err_present=%t count=%d credential_present=%t replacement_match=%t", err != nil, len(remaining), len(remaining) == 1 && remaining[0].Credential.ID != nil, len(remaining) == 1 && remaining[0].Credential.ID != nil && base64.RawURLEncoding.EncodeToString(remaining[0].Credential.ID) == replacementCredential.Id)
	}
}

func mustDecodeBase64URL(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestRemoteBootstrapStrictTransfersAndRefetchMismatch(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	_, _, remote, descriptor := remoteBootstrapFixture(t, now)
	encoded, err := app.EncodePublicBootstrapTransfer(descriptor, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.DecodeBootstrapDescriptor(append(encoded, []byte(` {}`)...)); err == nil {
		t.Fatal("descriptor decoder accepted trailing JSON")
	}
	challengeID, err := transport.NewUUIDv7()
	if err != nil {
		t.Fatal(err)
	}
	storageDigest, err := transport.BootstrapKeyEnvelopeAssertionJCSV1(
		1, 1, descriptor.Anchor.KeyEnvelopeAssertion.RecordRevision,
		descriptor.Anchor.KeyEnvelopeAssertion.SealedAt, "pending",
	)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(storageDigest)
	challenge := generatedapi.BootstrapAnchorChallengeV1{
		Version: generatedapi.BootstrapAnchorChallengeV1VersionN1, CeremonyId: challengeID,
		ServerOrigin: descriptor.ServerOrigin, Anchor: descriptor.Anchor,
		Challenge:                        base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32)),
		ServerEphemeralExchangePublicKey: base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32)),
		StorageAssertionDigest:           base64.RawURLEncoding.EncodeToString(digest[:]), ExpiresAt: now.Add(time.Minute),
	}
	mismatch := challenge
	mismatch.Challenge = base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{4}, 32))
	if _, err := remote.Prove(context.Background(), challenge, mismatch); domain.CodeOf(err) != domain.CodeIdentityConfirmationMismatch {
		t.Fatalf("refetch mismatch code=%v err=%v", domain.CodeOf(err), err)
	}
}
