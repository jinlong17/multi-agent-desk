package vault

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
)

func TestPersistentVaultFirstUseRoundTripAndCAS(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "device", "device.db")
	store, err := storage.Open(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Unix(1700, 0).UTC()
	deviceID, clientID := vaultID(t, "device"), vaultID(t, "client")
	if err := store.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "vault-v1", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: clientID, Name: "owner", PublicKey: make([]byte, 32), Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityVaultControl}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	manager, err := NewPersistentManager(ctx, store)
	if err != nil || manager.Status() != StateUninitialized {
		t.Fatalf("initial state=%s err=%v", manager.Status(), err)
	}
	password := []byte("correct horse battery staple")
	if state, err := manager.Initialize(ctx, clientID, "init-key-1", password, now); err != nil || state != StateLocked {
		t.Fatalf("initialize state=%s err=%v", state, err)
	}
	if state, err := manager.Initialize(ctx, clientID, "init-key-1", []byte("different retry input"), now); err != nil || state != StateLocked {
		t.Fatalf("idempotent init state=%s err=%v", state, err)
	}
	if _, err := manager.Initialize(ctx, clientID, "init-key-2", password, now); domain.CodeOf(err) != domain.CodeVaultAlreadyInitialized {
		t.Fatalf("competing init code=%v err=%v", domain.CodeOf(err), err)
	}
	if err := manager.Unlock([]byte("wrong")); domain.CodeOf(err) != domain.CodeVaultUnlockFailed {
		t.Fatalf("wrong password code=%v err=%v", domain.CodeOf(err), err)
	}
	if err := manager.Unlock(password); err != nil {
		t.Fatal(err)
	}

	accountID, profileID, credentialID := vaultID(t, "account"), vaultID(t, "profile"), vaultID(t, "credential")
	if err := store.CreateAccount(ctx, domain.Account{ID: accountID, Provider: domain.ProviderCodex, DisplayName: "vault", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRuntimeProfile(ctx, domain.RuntimeProfile{ID: profileID, DeviceID: deviceID, AccountID: accountID, Name: "vault", Provider: domain.ProviderCodex, Settings: []byte(`{}`), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	initialDigest := "0000000000000000000000000000000000000000000000000000000000000000"
	if err := store.CreateCredentialInstance(ctx, domain.CredentialInstance{ID: credentialID, DeviceID: deviceID, AccountID: accountID, Provider: domain.ProviderCodex, AuthMethod: domain.AuthMethodInteractive, SecretRef: "vault:" + string(credentialID), Status: domain.CredentialUnknown, CredentialRevision: 1, SecretDigest: initialDigest, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"tokens":{"access":"test-only","refresh":"test-only"}}`)
	revision, err := manager.SealCredential(ctx, CredentialMetadata{CredentialInstanceID: credentialID, AccountID: accountID, DeviceID: deviceID, Provider: domain.ProviderCodex, ExpectedRevision: 1, CreatedAt: now, UpdatedAt: now.Add(time.Second)}, payload)
	if err != nil || revision != 2 {
		t.Fatalf("seal revision=%d err=%v", revision, err)
	}
	itemRevision2, err := store.VaultItem(ctx, credentialID)
	if err != nil {
		t.Fatal(err)
	}
	plain, readRevision, err := manager.ReadCredential(ctx, credentialID)
	if err != nil || readRevision != 2 || string(plain) != string(payload) {
		t.Fatalf("read revision=%d payload=%q err=%v", readRevision, plain, err)
	}
	zero(plain)
	if _, err := manager.SealCredential(ctx, CredentialMetadata{CredentialInstanceID: credentialID, AccountID: accountID, DeviceID: deviceID, Provider: domain.ProviderCodex, ExpectedRevision: 1, CreatedAt: now, UpdatedAt: now.Add(time.Second)}, payload); domain.CodeOf(err) != domain.CodeCredentialRevisionConflict {
		t.Fatalf("stale CAS code=%v err=%v", domain.CodeOf(err), err)
	}
	refreshedPayload := []byte(`{"tokens":{"access":"refreshed-test-only","refresh":"refreshed-test-only"}}`)
	commitRevision, err := manager.SealCredential(ctx, CredentialMetadata{CredentialInstanceID: credentialID, AccountID: accountID, DeviceID: deviceID, Provider: domain.ProviderCodex, ExpectedRevision: revision, CreatedAt: now, UpdatedAt: now.Add(2 * time.Second)}, refreshedPayload)
	if err != nil {
		t.Fatal(err)
	}
	itemRevision3, err := store.VaultItem(ctx, credentialID)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(itemRevision2.PayloadNonce, itemRevision3.PayloadNonce) || bytes.Equal(itemRevision2.WrapNonce, itemRevision3.WrapNonce) || bytes.Equal(itemRevision2.WrappedDEK, itemRevision3.WrappedDEK) {
		t.Fatal("Vault revision reused a payload nonce, wrap nonce, or DEK envelope")
	}
	failureEnrollmentID := vaultID(t, "enrollment")
	failureEnrollment := storage.AuthEnrollment{ID: failureEnrollmentID, ClientDeviceID: clientID, RuntimeProfileID: profileID,
		CredentialInstanceID: credentialID, BinaryFingerprint: strings.Repeat("d", 64), StagingPath: filepath.Join(t.TempDir(), string(failureEnrollmentID)),
		State: storage.EnrollmentBegun, IdempotencyDigest: strings.Repeat("e", 64), ExpiresAt: now.Add(time.Minute), CreatedAt: now, UpdatedAt: now}
	if err := store.BeginAuthEnrollment(ctx, failureEnrollment, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.FinishAuthEnrollment(ctx, failureEnrollmentID, clientID, storage.EnrollmentFailed, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	unchanged, unchangedRevision, err := manager.ReadCredential(ctx, credentialID)
	if err != nil || unchangedRevision != commitRevision || string(unchanged) != string(refreshedPayload) {
		t.Fatalf("failed re-enrollment changed prior credential revision=%d payload=%q err=%v", unchangedRevision, unchanged, err)
	}
	zero(unchanged)

	// Explicit enrollment confirmation precedes the transaction that commits
	// encrypted bytes, credential revision, profile binding, and the durable
	// succeeded marker. The same completion digest remains replay-safe.
	enrollmentID, enrollmentCredentialID := vaultID(t, "enrollment"), vaultID(t, "credential")
	enrollmentCredential := domain.CredentialInstance{ID: enrollmentCredentialID, DeviceID: deviceID, AccountID: accountID,
		Provider: domain.ProviderCodex, AuthMethod: domain.AuthMethodInteractive, SecretRef: "vault:" + string(enrollmentCredentialID),
		Status: domain.CredentialUnknown, CredentialRevision: 1, SecretDigest: initialDigest, CreatedAt: now, UpdatedAt: now}
	enrollment := storage.AuthEnrollment{ID: enrollmentID, ClientDeviceID: clientID, RuntimeProfileID: profileID,
		CredentialInstanceID: enrollmentCredentialID, BinaryFingerprint: strings.Repeat("a", 64), StagingPath: filepath.Join(t.TempDir(), string(enrollmentID)),
		State: storage.EnrollmentBegun, IdempotencyDigest: strings.Repeat("b", 64), ExpiresAt: now.Add(time.Minute), CreatedAt: now, UpdatedAt: now}
	if err := store.BeginAuthEnrollment(ctx, enrollment, &enrollmentCredential); err != nil {
		t.Fatal(err)
	}
	completionDigest := strings.Repeat("c", 64)
	if _, err := store.ClaimAuthEnrollment(ctx, enrollmentID, clientID, completionDigest, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	aliasDigest := strings.Repeat("d", 64)
	if _, err := store.AwaitAuthEnrollmentConfirmation(ctx, enrollmentID, clientID, accountID, profileID,
		enrollmentCredentialID, 1, 1, 1, aliasDigest, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmAuthEnrollmentAttestation(ctx, enrollmentID, clientID, aliasDigest, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	enrollmentRevision, err := manager.SealEnrollmentCredential(ctx, enrollmentID, clientID, completionDigest, CredentialMetadata{
		CredentialInstanceID: enrollmentCredentialID, AccountID: accountID, DeviceID: deviceID, Provider: domain.ProviderCodex,
		ExpectedRevision: 1, CreatedAt: now, UpdatedAt: now.Add(3 * time.Second)}, payload)
	if err != nil || enrollmentRevision != 2 {
		t.Fatalf("enrollment seal revision=%d err=%v", enrollmentRevision, err)
	}
	replayedEnrollment, err := store.ClaimAuthEnrollment(ctx, enrollmentID, clientID, completionDigest, now.Add(4*time.Second))
	if err != nil || replayedEnrollment.State != storage.EnrollmentSucceeded || replayedEnrollment.CompletionIdempotencyDigest != completionDigest {
		t.Fatalf("enrollment replay=%+v err=%v", replayedEnrollment, err)
	}
	if err := store.ReserveVaultCredentialRevocation(ctx, enrollmentCredentialID, now.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.SealCredential(ctx, CredentialMetadata{CredentialInstanceID: enrollmentCredentialID,
		AccountID: accountID, DeviceID: deviceID, Provider: domain.ProviderCodex, ExpectedRevision: 2,
		CreatedAt: now, UpdatedAt: now.Add(6 * time.Second)}, payload); domain.CodeOf(err) != domain.CodeCredentialRevisionConflict {
		t.Fatalf("seal during revocation code=%v err=%v", domain.CodeOf(err), err)
	}
	raw, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `PRAGMA ignore_check_constraints=ON`); err != nil {
		_ = raw.Close()
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `UPDATE vault_items SET envelope_version=2 WHERE credential_instance_id=?`, credentialID); err != nil {
		_ = raw.Close()
		t.Fatal(err)
	}
	if _, _, err := manager.ReadCredential(ctx, credentialID); domain.CodeOf(err) != domain.CodeVaultCorrupt {
		_ = raw.Close()
		t.Fatalf("unsupported envelope version code=%v err=%v", domain.CodeOf(err), err)
	}
	if _, err := raw.ExecContext(ctx, `UPDATE vault_items SET envelope_version=1, payload_ciphertext=zeroblob(length(payload_ciphertext)) WHERE credential_instance_id=?`, credentialID); err != nil {
		_ = raw.Close()
		t.Fatal(err)
	}
	_ = raw.Close()
	if plain, _, err := manager.ReadCredential(ctx, credentialID); domain.CodeOf(err) != domain.CodeVaultCorrupt || len(plain) != 0 {
		t.Fatalf("tampered payload result=%q code=%v err=%v", plain, domain.CodeOf(err), err)
	}

	revoked, err := store.RevokeVaultCredential(ctx, credentialID, now.Add(5*time.Second))
	if err != nil || revoked.Status != domain.CredentialRevoked || revoked.SecretDigest != initialDigest {
		t.Fatalf("revoked credential=%+v err=%v", revoked, err)
	}
	if _, err := store.VaultItem(ctx, credentialID); domain.CodeOf(err) != domain.CodeNotFound {
		t.Fatalf("revoked vault item still exists: %v", err)
	}
	if err := manager.Lock(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewPersistentManager(ctx, store)
	if err != nil || reopened.Status() != StateLocked {
		t.Fatalf("restart state=%s err=%v", reopened.Status(), err)
	}
}

func TestVaultInitializationReplayRejectsCorruptSingleton(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "device", "device.db")
	store, err := storage.Open(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Unix(1850, 0).UTC()
	deviceID, clientID := vaultID(t, "device"), vaultID(t, "client")
	if err := store.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "vault-corrupt", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: clientID, Name: "owner", PublicKey: make([]byte, 32), Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityVaultControl}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	manager, err := NewPersistentManager(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	password := []byte("corruption-test-password")
	if _, err := manager.Initialize(ctx, clientID, "corrupt-replay-key", password, now); err != nil {
		t.Fatal(err)
	}
	config, err := store.VaultConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if _, err := raw.ExecContext(ctx, `PRAGMA ignore_check_constraints=ON`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `UPDATE vault_config SET init_request_digest=? WHERE singleton_id=1`, strings.Repeat("z", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Initialize(ctx, clientID, "corrupt-replay-key", password, now); domain.CodeOf(err) != domain.CodeVaultCorrupt {
		t.Fatalf("corrupt replay code=%v err=%v", domain.CodeOf(err), err)
	}
	if _, err := raw.ExecContext(ctx, `UPDATE vault_config SET init_request_digest=?, argon_time=99 WHERE singleton_id=1`, config.InitRequestDigest); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Initialize(ctx, clientID, "corrupt-replay-key", password, now); domain.CodeOf(err) != domain.CodeVaultCorrupt {
		t.Fatalf("hostile KDF replay code=%v err=%v", domain.CodeOf(err), err)
	}
}

func TestPersistentVaultConcurrentInitializersAcrossStoresHaveOneWinner(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "device", "device.db")
	firstStore, err := storage.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer firstStore.Close()
	now := time.Unix(1800, 0).UTC()
	deviceID, clientID := vaultID(t, "device"), vaultID(t, "client")
	if err := firstStore.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "vault-race", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := firstStore.CreateClientIdentity(ctx, domain.ClientIdentity{ID: clientID, Name: "owner", PublicKey: make([]byte, 32), Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityVaultControl}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	secondStore, err := storage.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer secondStore.Close()
	firstManager, err := NewPersistentManager(ctx, firstStore)
	if err != nil {
		t.Fatal(err)
	}
	secondManager, err := NewPersistentManager(ctx, secondStore)
	if err != nil {
		t.Fatal(err)
	}

	errorsByWriter := make([]error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		_, errorsByWriter[0] = firstManager.Initialize(ctx, clientID, "race-key-one", []byte("first-password"), now)
	}()
	go func() {
		defer wait.Done()
		_, errorsByWriter[1] = secondManager.Initialize(ctx, clientID, "race-key-two", []byte("second-password"), now)
	}()
	wait.Wait()
	winners, losers := 0, 0
	for _, initializeErr := range errorsByWriter {
		switch domain.CodeOf(initializeErr) {
		case "":
			winners++
		case domain.CodeVaultAlreadyInitialized:
			losers++
		default:
			t.Fatalf("unexpected race result: %v", initializeErr)
		}
	}
	if winners != 1 || losers != 1 {
		t.Fatalf("race winners=%d losers=%d errors=%v", winners, losers, errorsByWriter)
	}
	reopened, err := NewPersistentManager(ctx, secondStore)
	if err != nil || reopened.Status() != StateLocked {
		t.Fatalf("committed race state=%s err=%v", reopened.Status(), err)
	}
}

func TestPersistentVaultInitializationDependencyAndTransactionBoundaries(t *testing.T) {
	t.Run("fake credential does not block initialization", func(t *testing.T) {
		ctx := context.Background()
		store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "device", "device.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		now := time.Unix(1810, 0).UTC()
		deviceID, clientID, credentialID := vaultID(t, "device"), vaultID(t, "client"), vaultID(t, "credential")
		if err := store.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "vault-fake", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: clientID, Name: "owner", PublicKey: make([]byte, 32), Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityVaultControl}, CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		if err := store.CreateCredentialInstance(ctx, domain.CredentialInstance{ID: credentialID, DeviceID: deviceID, Provider: "fake", AuthMethod: "fake", SecretRef: "fake:test", Status: domain.CredentialHealthy, CredentialRevision: 1, SecretDigest: strings.Repeat("a", 64), CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		manager, err := NewPersistentManager(ctx, store)
		if err != nil {
			t.Fatal(err)
		}
		if state, err := manager.Initialize(ctx, clientID, "fake-state-init", []byte("password"), now); err != nil || state != StateLocked {
			t.Fatalf("fake dependency initialization state=%s err=%v", state, err)
		}
	})

	t.Run("codex credential blocks initialization", func(t *testing.T) {
		ctx := context.Background()
		store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "device", "device.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		now := time.Unix(1820, 0).UTC()
		deviceID, clientID := vaultID(t, "device"), vaultID(t, "client")
		accountID, credentialID := vaultID(t, "account"), vaultID(t, "credential")
		if err := store.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "vault-codex", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: clientID, Name: "owner", PublicKey: make([]byte, 32), Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityVaultControl}, CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		if err := store.CreateAccount(ctx, domain.Account{ID: accountID, Provider: domain.ProviderCodex, DisplayName: "vault-codex", CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		if err := store.CreateCredentialInstance(ctx, domain.CredentialInstance{ID: credentialID, DeviceID: deviceID, AccountID: accountID, Provider: domain.ProviderCodex, AuthMethod: domain.AuthMethodInteractive, SecretRef: "vault:" + string(credentialID), Status: domain.CredentialUnknown, CredentialRevision: 1, SecretDigest: strings.Repeat("0", 64), CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		manager, err := NewPersistentManager(ctx, store)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := manager.Initialize(ctx, clientID, "codex-state-init", []byte("password"), now); domain.CodeOf(err) != domain.CodeConflict {
			t.Fatalf("Codex dependency initialization code=%v err=%v", domain.CodeOf(err), err)
		}
		if _, err := store.VaultConfig(ctx); domain.CodeOf(err) != domain.CodeNotFound {
			t.Fatalf("blocked initialization wrote config: %v", err)
		}
	})

	t.Run("cancel before commit leaves no singleton and committed state restarts locked", func(t *testing.T) {
		ctx := context.Background()
		store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "device", "device.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		now := time.Unix(1830, 0).UTC()
		deviceID, clientID := vaultID(t, "device"), vaultID(t, "client")
		if err := store.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "vault-boundary", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: clientID, Name: "owner", PublicKey: make([]byte, 32), Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityVaultControl}, CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		manager, err := NewPersistentManager(ctx, store)
		if err != nil {
			t.Fatal(err)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := manager.Initialize(cancelled, clientID, "cancelled-init", []byte("password"), now); err == nil {
			t.Fatal("cancelled initialization unexpectedly succeeded")
		}
		if _, err := store.VaultConfig(ctx); domain.CodeOf(err) != domain.CodeNotFound {
			t.Fatalf("cancelled initialization wrote config: %v", err)
		}
		if _, err := manager.Initialize(ctx, clientID, "committed-init", []byte("password"), now); err != nil {
			t.Fatal(err)
		}
		restarted, err := NewPersistentManager(ctx, store)
		if err != nil || restarted.Status() != StateLocked {
			t.Fatalf("post-commit restart state=%s err=%v", restarted.Status(), err)
		}
	})
}

func TestPersistentVaultRejectsDuplicateAndNonObjectJSON(t *testing.T) {
	for _, payload := range [][]byte{[]byte(`[]`), []byte(`{"a":1,"a":2}`), make([]byte, maxPayloadSize+1)} {
		if err := validateJSONObject(payload); err == nil && len(payload) <= maxPayloadSize {
			t.Fatalf("accepted invalid JSON %q", payload)
		}
	}
}

func TestVaultV1FrozenArgonGCMAndAADVectors(t *testing.T) {
	salt := make([]byte, 16)
	for index := range salt {
		salt[index] = byte(index)
	}
	key := deriveKEK([]byte("password"), salt, 3, 65536, 4)
	defer zero(key)
	if got := hex.EncodeToString(key); got != "bd4ace85c295fb38d844e16771a9e43925282ac982c8656be55a2ef6cf20fd58" {
		t.Fatalf("Argon2id vector=%s", got)
	}
	nonce := make([]byte, 12)
	for index := range nonce {
		nonce[index] = byte(index)
	}
	gcm, err := newGCM(key)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(gcm.Seal(nil, nonce, keyCheckPlaintext, keyCheckAAD)); got != "1c7524f614cbdad72c42f71a9d5d3342c85209117b8cd1bec7f6096c275153e85d9b0926ad2f46df77230c3b334ec68a61" {
		t.Fatalf("AES-GCM key-check vector=%s", got)
	}
	metadata := CredentialMetadata{
		DeviceID:             "device_0123456789abcdef0123456789abcdef",
		Provider:             domain.ProviderCodex,
		CredentialInstanceID: "credential_0123456789abcdef0123456789abcdef",
		AccountID:            "account_0123456789abcdef0123456789abcdef",
	}
	wantAAD := "0000000131000000276465766963655f303132333435363738396162636465663031323334353637383961626364656600000005636f6465780000002b63726564656e7469616c5f3031323334353637383961626364656630313233343536373839616263646566000000286163636f756e745f30313233343536373839616263646566303132333435363738396162636465660000000132"
	if got := hex.EncodeToString(credentialAAD(metadata, 2)); got != wantAAD {
		t.Fatalf("canonical AAD=%s", got)
	}
	baseline := credentialAAD(metadata, 2)
	mutations := []CredentialMetadata{metadata, metadata, metadata, metadata}
	mutations[0].DeviceID = "device_ffeeddccbbaa99887766554433221100"
	mutations[1].Provider = "fake"
	mutations[2].CredentialInstanceID = "credential_ffeeddccbbaa99887766554433221100"
	mutations[3].AccountID = "account_ffeeddccbbaa99887766554433221100"
	for index, mutation := range mutations {
		if bytes.Equal(baseline, credentialAAD(mutation, 2)) {
			t.Fatalf("AAD mutation %d did not change encoding", index)
		}
	}
	if bytes.Equal(baseline, credentialAAD(metadata, 3)) {
		t.Fatal("AAD revision mutation did not change encoding")
	}
}

func TestCurrentKEKIsAtomicWithLock(t *testing.T) {
	manager := &Manager{state: StateUnlocked, kek: make([]byte, 32)}
	for index := range manager.kek {
		manager.kek[index] = byte(index + 1)
	}
	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(started)
		_ = manager.Lock()
		close(done)
	}()
	<-started
	key, err := manager.currentKEK()
	if err == nil && len(key) != 32 {
		t.Fatalf("atomic key snapshot length=%d", len(key))
	}
	if err != nil && domain.CodeOf(err) != domain.CodeVaultLocked {
		t.Fatalf("unexpected lock race error: %v", err)
	}
	zero(key)
	<-done
	if _, err := manager.currentKEK(); domain.CodeOf(err) != domain.CodeVaultLocked {
		t.Fatalf("post-lock key access=%v", err)
	}
}
