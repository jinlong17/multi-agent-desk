package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/device"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
	runtimepkg "github.com/jinlong17/multi-agent-desk/internal/runtime"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
	"github.com/jinlong17/multi-agent-desk/internal/vault"
)

func TestSelectorPreviewAndConfirmedReservationAreTheOnlyCodexStartPath(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := storage.Open(ctx, filepath.Join(root, "device", "device.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Unix(8_000, 0).UTC()
	deviceID, clientID := appTestID(t, "device"), appTestID(t, "client")
	accountID, profileID := appTestID(t, "account"), appTestID(t, "profile")
	credentialID, workspaceID, enrollmentID := appTestID(t, "credential"), appTestID(t, "workspace"), appTestID(t, "enrollment")
	if err := store.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "selector",
		SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: clientID, Name: "owner", PublicKey: make([]byte, 32),
		Revision: 1, Status: domain.ClientIdentityActive,
		Caps:      []domain.Capability{domain.CapabilityMetadataRead, domain.CapabilitySessionStart, domain.CapabilityProviderAuth, domain.CapabilityVaultControl},
		CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.CreateAccountWithDefaultProfile(ctx,
		domain.Account{ID: accountID, Provider: domain.ProviderCodex, DisplayName: "Work", Enabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now},
		domain.RuntimeProfile{ID: profileID, DeviceID: deviceID, AccountID: accountID, Name: "Work Linux", Provider: domain.ProviderCodex,
			SelectorAlias: "A", SelectorKey: "a", Settings: []byte(`{}`), Enabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateWorkspace(ctx, domain.Workspace{ID: workspaceID, DeviceID: deviceID, Path: root,
		Label: "workspace", Tags: []string{}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	vaultManager, err := vault.NewPersistentManager(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vaultManager.Initialize(ctx, clientID, "selector-vault", []byte("test-password"), now); err != nil {
		t.Fatal(err)
	}
	if err := vaultManager.Unlock([]byte("test-password")); err != nil {
		t.Fatal(err)
	}
	credential := domain.CredentialInstance{ID: credentialID, DeviceID: deviceID, AccountID: accountID,
		Provider: domain.ProviderCodex, AuthMethod: domain.AuthMethodInteractive, SecretRef: "vault:" + string(credentialID),
		Status: domain.CredentialUnknown, CredentialRevision: 1, SecretDigest: strings.Repeat("0", 64), CreatedAt: now, UpdatedAt: now}
	enrollment := storage.AuthEnrollment{ID: enrollmentID, ClientDeviceID: clientID, RuntimeProfileID: profileID,
		CredentialInstanceID: credentialID, BinaryFingerprint: strings.Repeat("a", 64), StagingPath: filepath.Join(root, string(enrollmentID)),
		State: storage.EnrollmentBegun, IdempotencyDigest: strings.Repeat("b", 64), ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now}
	if err := store.BeginAuthEnrollment(ctx, enrollment, &credential); err != nil {
		t.Fatal(err)
	}
	completionDigest, aliasDigest := strings.Repeat("c", 64), strings.Repeat("d", 64)
	if _, err := store.ClaimAuthEnrollment(ctx, enrollmentID, clientID, completionDigest, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AwaitAuthEnrollmentConfirmation(ctx, enrollmentID, clientID, accountID, profileID, credentialID,
		1, 1, 1, aliasDigest, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmAuthEnrollmentAttestation(ctx, enrollmentID, clientID, aliasDigest, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := vaultManager.SealEnrollmentCredential(ctx, enrollmentID, clientID, completionDigest,
		vault.CredentialMetadata{CredentialInstanceID: credentialID, AccountID: accountID, DeviceID: deviceID,
			Provider: domain.ProviderCodex, ExpectedRevision: 1, CreatedAt: now, UpdatedAt: now.Add(4 * time.Second)},
		[]byte(`{"tokens":{"access":"selector-test-only"}}`)); err != nil {
		t.Fatal(err)
	}

	service := NewSessionService(store, runtimepkg.NewManager(store, "unused"))
	service.Vault = vaultManager
	service.Now = func() time.Time { return now.Add(5 * time.Second) }
	preflight := SelectorPreflight{ProviderVersion: "0.144.2", BinaryFingerprint: strings.Repeat("e", 64),
		SchemaFingerprint: strings.Repeat("f", 64), CapabilityDigest: strings.Repeat("1", 64),
		Capabilities: []domain.Capability{domain.CapabilityProviderUsageRead, domain.CapabilitySessionControl}}
	service.SelectorPreflight = func(context.Context) (SelectorPreflight, error) { return preflight, nil }
	auth := device.AuthContext{ClientID: clientID, IdentityRevision: 1, AuthenticatedAt: now, ExpiresAt: now.Add(time.Hour)}
	refreshWithoutSelector, _ := device.JSONBody(map[string]any{})
	if _, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor,
		RequestID: "usage-refresh-missing-selector", Method: "usage.refresh", Body: refreshWithoutSelector}); domain.CodeOf(err) != domain.CodeInvalidArgument {
		t.Fatalf("missing-selector refresh code=%v err=%v", domain.CodeOf(err), err)
	}
	refreshBody, _ := device.JSONBody(map[string]any{"profile": "@A"})
	if _, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor,
		RequestID: "usage-refresh-no-runtime", Method: "usage.refresh", Body: refreshBody}); domain.CodeOf(err) != domain.CodeUsageUnavailable {
		t.Fatalf("inactive-runtime refresh code=%v err=%v", domain.CodeOf(err), err)
	}
	previewBody, _ := device.JSONBody(map[string]any{"provider": "codex", "profile_selector": "@A", "workspace_id": workspaceID})
	result, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor,
		RequestID: "selector-preview", Method: "sessions.preview", Body: previewBody})
	if err != nil {
		t.Fatal(err)
	}
	preview := result.(map[string]any)
	confirmation := map[string]any{"confirmed": true, "account_id": preview["account_id"],
		"account_revision": preview["account_revision"], "runtime_profile_id": preview["runtime_profile_id"],
		"profile_revision": preview["profile_revision"], "credential_instance_id": preview["credential_instance_id"],
		"credential_revision": preview["credential_revision"], "device_id": preview["device_id"],
		"workspace_id": preview["workspace_id"], "usage_snapshot_id": preview["usage_snapshot_id"],
		"provider_version": preview["provider_version"]}
	startBody, _ := device.JSONBody(map[string]any{"provider": "codex", "profile_selector": "@A",
		"workspace_id": workspaceID, "preview_id": preview["preview_id"], "confirmation": confirmation})
	started, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor,
		RequestID: "selector-start", Method: "session.start", IdempotencyKey: "selector-start-key", Body: startBody})
	if err != nil {
		t.Fatal(err)
	}
	if started.(map[string]any)["status"] != domain.SessionFailed {
		t.Fatalf("P1 reservation receipt was not terminal: %+v", started)
	}
	rawBody, _ := device.JSONBody(map[string]any{"device_id": deviceID,
		"credential_instance_id": credentialID, "runtime_profile_id": profileID, "workspace_id": workspaceID})
	if _, err := service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor,
		RequestID: "raw-start", Method: "session.start", IdempotencyKey: "raw-start-key", Body: rawBody}); domain.CodeOf(err) != domain.CodeIdentityConfirmationRequired {
		t.Fatalf("raw start code=%v err=%v", domain.CodeOf(err), err)
	}
	sessions, err := store.ListSessions(ctx)
	if err != nil || len(sessions) != 1 || sessions[0].Status != domain.SessionFailed {
		t.Fatalf("unexpected sessions after raw bypass attempt: %+v err=%v", sessions, err)
	}
}
