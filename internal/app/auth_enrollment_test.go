package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/device"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/providers/codex"
	runtimepkg "github.com/jinlong17/multi-agent-desk/internal/runtime"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
	"github.com/jinlong17/multi-agent-desk/internal/vault"
)

func TestAuthBeginCancelUsesPrivateOwnerBoundEnrollment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable fixture uses a POSIX script")
	}
	ctx := context.Background()
	root := t.TempDir()
	store, err := storage.Open(ctx, filepath.Join(root, "device", "device.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Unix(1900, 0).UTC()
	deviceID, clientID := appTestID(t, "device"), appTestID(t, "client")
	otherClientID := appTestID(t, "client")
	accountID, profileID := appTestID(t, "account"), appTestID(t, "profile")
	if err := store.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "auth", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: clientID, Name: "owner", PublicKey: make([]byte, 32), Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityProviderAuth, domain.CapabilityVaultControl}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	otherPublicKey := make([]byte, 32)
	otherPublicKey[0] = 1
	if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: otherClientID, Name: "other-owner", PublicKey: otherPublicKey, Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityProviderAuth}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.CreateAccountWithDefaultProfile(ctx,
		domain.Account{ID: accountID, Provider: domain.ProviderCodex, DisplayName: "auth", Enabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now},
		domain.RuntimeProfile{ID: profileID, DeviceID: deviceID, AccountID: accountID, Name: "auth", Provider: domain.ProviderCodex,
			SelectorAlias: "A", SelectorKey: "a", Settings: []byte(`{}`), Enabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	vaultManager, err := vault.NewPersistentManager(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vaultManager.Initialize(ctx, clientID, "vault-init", []byte("test-password"), now); err != nil {
		t.Fatal(err)
	}
	if err := vaultManager.Unlock([]byte("test-password")); err != nil {
		t.Fatal(err)
	}
	unlockBody, _ := device.JSONBody(map[string]string{"secret": "test-password"})
	if _, err := serviceHandleVaultUnlockForTest(ctx, store, vaultManager, clientID, now, unlockBody); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\necho codex-cli 0.144.2\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MULTIDESK_CODEX_BINARY", binary)
	service := NewSessionService(store, runtimepkg.NewManager(store, "unused"))
	service.Vault = vaultManager
	service.CredentialHomeRoot = filepath.Join(root, "codex-home")
	service.Now = func() time.Time { return now }
	auth := device.AuthContext{ClientID: clientID, IdentityRevision: 1, AuthenticatedAt: now, ExpiresAt: now.Add(time.Hour)}
	otherAuth := device.AuthContext{ClientID: otherClientID, IdentityRevision: 1, AuthenticatedAt: now, ExpiresAt: now.Add(time.Hour)}
	disabledBody, _ := device.JSONBody(map[string]any{"profile_id": profileID, "mode": "device-auth"})
	if _, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "auth-disabled", Method: "auth.begin", IdempotencyKey: "auth-disabled-key", Body: disabledBody}); domain.CodeOf(err) != domain.CodeProviderUnsupported {
		t.Fatalf("disabled auth mode code=%v err=%v", domain.CodeOf(err), err)
	}
	body, _ := device.JSONBody(map[string]any{"profile_id": profileID})
	result, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "auth-begin", Method: "auth.begin", IdempotencyKey: "auth-begin-key", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	view := result.(map[string]any)
	enrollmentID := view["enrollment_id"].(domain.ID)
	staging := view["staging_path"].(string)
	// Bypass the generic RPC response cache to model a daemon crash after the
	// enrollment transaction committed but before that cache was written.
	replayedResult, err := service.dispatch(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "auth-begin-replay", Method: "auth.begin", IdempotencyKey: "auth-begin-key", Body: body})
	if err != nil || replayedResult.(map[string]any)["enrollment_id"].(domain.ID) != enrollmentID {
		t.Fatalf("auth begin store replay=%v err=%v", replayedResult, err)
	}
	if info, err := os.Stat(staging); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("staging mode=%v err=%v", info, err)
	}
	if _, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "auth-begin-conflict", Method: "auth.begin", IdempotencyKey: "auth-begin-conflict-key", Body: body}); domain.CodeOf(err) != domain.CodeConflict {
		t.Fatalf("second active enrollment code=%v err=%v", domain.CodeOf(err), err)
	}
	cancelBody, _ := device.JSONBody(map[string]any{"enrollment_id": enrollmentID})
	if _, err := service.Handle(ctx, otherAuth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "auth-cancel-other", Method: "auth.cancel", IdempotencyKey: "auth-cancel-other-key", Body: cancelBody}); domain.CodeOf(err) != domain.CodePermissionDenied {
		t.Fatalf("non-owner cancel code=%v err=%v", domain.CodeOf(err), err)
	}
	stillActive, err := store.AuthEnrollment(ctx, enrollmentID)
	if err != nil || stillActive.State != storage.EnrollmentBegun {
		t.Fatalf("non-owner changed enrollment=%+v err=%v", stillActive, err)
	}
	if _, err := os.Stat(staging); err != nil {
		t.Fatalf("non-owner removed staging: %v", err)
	}
	if _, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "auth-cancel", Method: "auth.cancel", IdempotencyKey: "auth-cancel-key", Body: cancelBody}); err != nil {
		t.Fatal(err)
	}
	enrollment, err := store.AuthEnrollment(ctx, enrollmentID)
	if err != nil || enrollment.State != storage.EnrollmentCancelled || enrollment.CredentialInstanceID != "" {
		t.Fatalf("enrollment=%+v err=%v", enrollment, err)
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("staging survived cancel: %v", err)
	}
	// Model a crash/removal failure after the terminal database commit.
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "auth.json"), []byte(`{"token":"terminal-residue"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	secondResult, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "auth-begin-2", Method: "auth.begin", IdempotencyKey: "auth-begin-key-2", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	secondView := secondResult.(map[string]any)
	secondEnrollmentID := secondView["enrollment_id"].(domain.ID)
	secondStaging := secondView["staging_path"].(string)
	orphanStaging := filepath.Join(filepath.Dir(secondStaging), string(appTestID(t, "enrollment")))
	if err := os.MkdirAll(orphanStaging, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orphanStaging, "auth.json"), []byte(`{"token":"orphan-test"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.RecoverPendingApprovals(ctx); err != nil {
		t.Fatal(err)
	}
	expired, err := store.AuthEnrollment(ctx, secondEnrollmentID)
	if err != nil || expired.State != storage.EnrollmentExpired || expired.CredentialInstanceID != "" {
		t.Fatalf("expired enrollment=%+v err=%v", expired, err)
	}
	if _, err := os.Stat(secondStaging); !os.IsNotExist(err) {
		t.Fatalf("staging survived recovery: %v", err)
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("terminal staging survived recovery: %v", err)
	}
	if _, err := os.Stat(orphanStaging); !os.IsNotExist(err) {
		t.Fatalf("orphan staging survived recovery: %v", err)
	}

	thirdResult, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "auth-begin-3", Method: "auth.begin", IdempotencyKey: "auth-begin-key-3", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	thirdView := thirdResult.(map[string]any)
	thirdEnrollmentID := thirdView["enrollment_id"].(domain.ID)
	thirdStaging := thirdView["staging_path"].(string)
	initialCredential := []byte(`{"tokens":{"access":"before-validation"}}`)
	finalCredential := []byte(`{"tokens":{"access":"after-validation"}}`)
	if err := os.WriteFile(filepath.Join(thirdStaging, "auth.json"), initialCredential, 0o600); err != nil {
		t.Fatal(err)
	}
	service.EnrollmentValidator = func(_ context.Context, _ codex.BinaryDescriptor, home string) error {
		return os.WriteFile(filepath.Join(home, "auth.json"), finalCredential, 0o600)
	}
	completeBody, _ := device.JSONBody(map[string]any{"enrollment_id": thirdEnrollmentID})
	completeResult, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "auth-complete-3", Method: "auth.complete", IdempotencyKey: "auth-complete-key-3", Body: completeBody})
	if err != nil {
		t.Fatal(err)
	}
	completeView := completeResult.(map[string]any)
	credentialID := completeView["credential_id"].(domain.ID)
	if completeView["state"] != storage.EnrollmentAwaitingConfirmation {
		t.Fatalf("auth complete bypassed confirmation: %+v", completeView)
	}
	if _, err := store.VaultItem(ctx, credentialID); domain.CodeOf(err) != domain.CodeNotFound {
		t.Fatalf("unconfirmed enrollment sealed a Vault item: %v", err)
	}
	if _, err := os.Stat(thirdStaging); err != nil {
		t.Fatalf("unconfirmed staging was removed: %v", err)
	}
	confirmBody, _ := device.JSONBody(map[string]any{"enrollment_id": thirdEnrollmentID, "profile_selector": "@A", "confirmed": true})
	if _, err := service.Handle(ctx, otherAuth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "auth-confirm-other", Method: "auth.confirm", IdempotencyKey: "auth-confirm-other-key", Body: confirmBody}); domain.CodeOf(err) != domain.CodePermissionDenied {
		t.Fatalf("non-owner confirmation code=%v err=%v", domain.CodeOf(err), err)
	}
	confirmedResult, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "auth-confirm-3", Method: "auth.confirm", IdempotencyKey: "auth-confirm-key-3", Body: confirmBody})
	if err != nil {
		t.Fatal(err)
	}
	if confirmedResult.(map[string]any)["state"] != storage.EnrollmentSucceeded {
		t.Fatalf("confirmation did not seal enrollment: %+v", confirmedResult)
	}
	storedCredential, revision, err := vaultManager.ReadCredential(ctx, credentialID)
	if err != nil || revision != 2 || string(storedCredential) != string(finalCredential) {
		t.Fatalf("post-validation credential revision=%d payload=%q err=%v", revision, storedCredential, err)
	}
	for index := range storedCredential {
		storedCredential[index] = 0
	}
	if _, err := os.Stat(thirdStaging); !os.IsNotExist(err) {
		t.Fatalf("successful staging survived completion: %v", err)
	}
	credentialHome := filepath.Join(service.CredentialHomeRoot, string(credentialID))
	if err := os.MkdirAll(credentialHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(credentialHome, "auth.json"), finalCredential, 0o600); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(service.CredentialHomeRoot, string(credentialID)+".writer.lock")
	if err := os.WriteFile(lockPath, []byte(`{"owner":"stale-test"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	logoutBody, _ := device.JSONBody(map[string]any{"credential_id": credentialID})
	workspaceID, sessionID := appTestID(t, "workspace"), appTestID(t, "session")
	if err := store.CreateWorkspace(ctx, domain.Workspace{ID: workspaceID, DeviceID: deviceID, Path: root, Label: "auth", Tags: []string{}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(ctx, domain.Session{ID: sessionID, DeviceID: deviceID, AccountID: accountID, Provider: domain.ProviderCodex, CredentialInstanceID: credentialID, RuntimeProfileID: profileID, WorkspaceID: workspaceID, ProviderSessionID: "auth-active", Status: domain.SessionStarting, StartedAt: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "auth-logout-active", Method: "auth.logout", IdempotencyKey: "auth-logout-active-key", Body: logoutBody}); domain.CodeOf(err) != domain.CodeConflict {
		t.Fatalf("active logout code=%v err=%v", domain.CodeOf(err), err)
	}
	if _, err := os.Stat(credentialHome); err != nil {
		t.Fatalf("active logout removed home: %v", err)
	}
	if _, err := store.VaultItem(ctx, credentialID); err != nil {
		t.Fatalf("active logout removed Vault item: %v", err)
	}
	exitCode := 1
	if _, err := store.TransitionSession(ctx, sessionID, domain.SessionStarting, domain.SessionFailed, now.Add(time.Second), &exitCode, "test_finished"); err != nil {
		t.Fatal(err)
	}
	// Deterministically interleave a new Session start after logout has reserved
	// revocation but before it removes the home. The reservation must reject the
	// start, while an idempotent logout retry consumes it and completes cleanup.
	if err := store.ReserveVaultCredentialRevocation(ctx, credentialID, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	racingSessionID := appTestID(t, "session")
	if err := store.CreateSession(ctx, domain.Session{ID: racingSessionID, DeviceID: deviceID, AccountID: accountID, Provider: domain.ProviderCodex, CredentialInstanceID: credentialID, RuntimeProfileID: profileID, WorkspaceID: workspaceID, ProviderSessionID: "auth-racing", Status: domain.SessionStarting, StartedAt: now.Add(2 * time.Second)}); domain.CodeOf(err) != domain.CodePermissionDenied {
		t.Fatalf("session during logout reservation code=%v err=%v", domain.CodeOf(err), err)
	}
	if _, err := os.Stat(credentialHome); err != nil {
		t.Fatalf("revocation reservation removed home before logout: %v", err)
	}
	if _, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "auth-logout-3", Method: "auth.logout", IdempotencyKey: "auth-logout-key-3", Body: logoutBody}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(credentialHome); !os.IsNotExist(err) {
		t.Fatalf("credential home survived logout: %v", err)
	}
	if _, err := store.VaultItem(ctx, credentialID); domain.CodeOf(err) != domain.CodeNotFound {
		t.Fatalf("Vault item survived logout: %v", err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("credential writer lock survived logout: %v", err)
	}

	service.Now = func() time.Time { return now }
	fourthResult, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "auth-begin-4", Method: "auth.begin", IdempotencyKey: "auth-begin-key-4", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	fourthView := fourthResult.(map[string]any)
	fourthEnrollmentID := fourthView["enrollment_id"].(domain.ID)
	fourthStaging := fourthView["staging_path"].(string)
	service.Now = func() time.Time { return now.Add(11 * time.Minute) }
	expiredCompleteBody, _ := device.JSONBody(map[string]any{"enrollment_id": fourthEnrollmentID})
	if _, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "auth-complete-4", Method: "auth.complete", IdempotencyKey: "auth-complete-key-4", Body: expiredCompleteBody}); domain.CodeOf(err) != domain.CodeDeadlineExceeded {
		t.Fatalf("expired completion code=%v err=%v", domain.CodeOf(err), err)
	}
	fourthEnrollment, err := store.AuthEnrollment(ctx, fourthEnrollmentID)
	if err != nil || fourthEnrollment.State != storage.EnrollmentExpired || fourthEnrollment.CredentialInstanceID != "" {
		t.Fatalf("deadline enrollment=%+v err=%v", fourthEnrollment, err)
	}
	if _, err := os.Stat(fourthStaging); !os.IsNotExist(err) {
		t.Fatalf("expired completion staging survived: %v", err)
	}

	// A caller that abandons auth.begin must not hold the per-profile active
	// slot until the daemon restarts. A later begin sweeps the expired row and
	// removes its staging directory before creating the replacement.
	service.Now = func() time.Time { return now.Add(20 * time.Minute) }
	fifthResult, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "auth-begin-5", Method: "auth.begin", IdempotencyKey: "auth-begin-key-5", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	fifthView := fifthResult.(map[string]any)
	fifthEnrollmentID := fifthView["enrollment_id"].(domain.ID)
	fifthStaging := fifthView["staging_path"].(string)
	service.Now = func() time.Time { return now.Add(31 * time.Minute) }
	sixthResult, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "auth-begin-6", Method: "auth.begin", IdempotencyKey: "auth-begin-key-6", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	fifthEnrollment, err := store.AuthEnrollment(ctx, fifthEnrollmentID)
	if err != nil || fifthEnrollment.State != storage.EnrollmentExpired {
		t.Fatalf("due enrollment=%+v err=%v", fifthEnrollment, err)
	}
	if _, err := os.Stat(fifthStaging); !os.IsNotExist(err) {
		t.Fatalf("due enrollment staging survived: %v", err)
	}
	sixthEnrollmentID := sixthResult.(map[string]any)["enrollment_id"].(domain.ID)
	sixthCancelBody, _ := device.JSONBody(map[string]any{"enrollment_id": sixthEnrollmentID})
	if _, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "auth-cancel-6", Method: "auth.cancel", IdempotencyKey: "auth-cancel-key-6", Body: sixthCancelBody}); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		base := filepath.Join(filepath.Dir(store.Path()), "enrollments")
		if err := os.RemoveAll(base); err != nil {
			t.Fatal(err)
		}
		outside := t.TempDir()
		if err := os.Symlink(outside, base); err != nil {
			t.Fatal(err)
		}
		if _, err := service.createEnrollmentStaging(appTestID(t, "enrollment")); domain.CodeOf(err) != domain.CodeConflict {
			t.Fatalf("symlink enrollment root code=%v err=%v", domain.CodeOf(err), err)
		}
		entries, err := os.ReadDir(outside)
		if err != nil || len(entries) != 0 {
			t.Fatalf("symlink target was mutated: entries=%v err=%v", entries, err)
		}
	}
}

func serviceHandleVaultUnlockForTest(ctx context.Context, store *storage.Store, manager *vault.Manager, clientID domain.ID, now time.Time, body []byte) (any, error) {
	service := NewSessionService(store, runtimepkg.NewManager(store, "unused"))
	service.Vault = manager
	result, err := service.Handle(ctx, device.AuthContext{ClientID: clientID}, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "vault-unlock-secret", Method: "vault.unlock", IdempotencyKey: "vault-unlock-secret-key", Body: body})
	if err != nil {
		return nil, err
	}
	if _, ledgerErr := store.IdempotencyRecord(ctx, clientID, "vault.unlock", "vault-unlock-secret-key"); domain.CodeOf(ledgerErr) != domain.CodeNotFound {
		return nil, ledgerErr
	}
	for _, value := range body {
		if value != 0 {
			return nil, domain.NewError(domain.CodeConflict, "Vault unlock body was not cleared")
		}
	}
	return result, nil
}
