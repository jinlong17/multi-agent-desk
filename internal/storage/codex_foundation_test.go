package storage

import (
	"context"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

func TestCodexFoundationRoundTripsAccountSessionUsageAndApproval(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	now := time.Unix(500, 0).UTC()
	deviceID := storageID("device", storageHexA)
	accountID := storageID("account", storageHexA)
	profileID := storageID("profile", storageHexA)
	credentialID := storageID("credential", storageHexA)
	workspaceID := storageID("workspace", storageHexA)
	sessionID := storageID("session", storageHexA)
	clientID := storageID("client", storageHexA)

	if err := store.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "codex", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: clientID, Name: "owner", PublicKey: bytesOf(1, 32), Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityMetadataRead}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateWorkspace(ctx, domain.Workspace{ID: workspaceID, DeviceID: deviceID, Path: "/tmp/codex", Label: "codex", Tags: []string{}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAccount(ctx, domain.Account{ID: accountID, Provider: domain.ProviderCodex, DisplayName: "Codex primary", ProviderSubjectDigest: storageDigestA, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRuntimeProfile(ctx, domain.RuntimeProfile{ID: profileID, DeviceID: deviceID, AccountID: accountID, Name: "codex-default", Provider: domain.ProviderCodex, Settings: []byte(`{"model":"gpt-5"}`), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateCredentialInstance(ctx, domain.CredentialInstance{ID: credentialID, DeviceID: deviceID, AccountID: accountID, Provider: domain.ProviderCodex, AuthMethod: domain.AuthMethodInteractive, SecretRef: "vault:codex/primary", Status: domain.CredentialHealthy, CredentialRevision: 1, SecretDigest: storageDigestA, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	session := domain.Session{ID: sessionID, DeviceID: deviceID, AccountID: accountID, Provider: domain.ProviderCodex, CredentialInstanceID: credentialID, RuntimeProfileID: profileID, WorkspaceID: workspaceID, ProviderSessionID: "thread-1", Status: domain.SessionStarting, StartedAt: now, CapabilitySnapshot: []domain.Capability{domain.CapabilitySessionResume}}
	if err := store.CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RevokeVaultCredential(ctx, credentialID, now.Add(time.Second)); domain.CodeOf(err) != domain.CodeConflict {
		t.Fatalf("active session did not block credential revocation: %v", err)
	}
	loadedSession, err := store.Session(ctx, sessionID)
	if err != nil || loadedSession.AccountID != accountID || loadedSession.Provider != domain.ProviderCodex {
		t.Fatalf("codex session round trip = %+v, err=%v", loadedSession, err)
	}

	used, limit, percent := 10.0, 100.0, 10.0
	snapshot := domain.UsageSnapshot{ID: storageID("usage", storageHexA), Provider: domain.ProviderCodex, AccountID: accountID, DeviceID: deviceID, Source: domain.UsageSourceOfficial, Confidence: domain.UsageConfidenceHigh, WindowKind: "daily", UsedValue: &used, LimitValue: &limit, UsedPercent: &percent, ObservedAt: now, RawReferenceHash: storageDigestA, SourceVersion: "0.144.2", CapabilityStatus: domain.UsageSupported}
	if err := store.CreateUsageSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	loadedUsage, err := store.UsageSnapshot(ctx, snapshot.ID)
	if err != nil || loadedUsage.UsedPercent == nil || *loadedUsage.UsedPercent != percent {
		t.Fatalf("usage round trip = %+v, err=%v", loadedUsage, err)
	}

	approval := domain.Approval{ID: storageID("approval", storageHexA), SessionID: sessionID, ProviderApprovalID: "approval-1", Kind: "command", PayloadDigest: storageDigestA, Summary: "run approved command", Status: domain.ApprovalPending, IdempotencyKey: "provider-request-1", RequestedAt: now}
	if err := store.CreateApproval(ctx, approval); err != nil {
		t.Fatal(err)
	}
	lease, err := domain.AcquireControllerLease(nil, sessionID, clientID, now, domain.DefaultLeaseDuration)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveControllerLease(ctx, lease, 0); err != nil {
		t.Fatal(err)
	}
	responded, err := store.RespondApproval(ctx, approval.ID, approval.ProviderApprovalID, clientID, "response-1", domain.ApprovalApproved, now.Add(time.Second))
	if err != nil || responded.Status != domain.ApprovalApproved || responded.ResponseState != domain.ApprovalResponseWritten || responded.RequestedDecision != domain.ApprovalDecisionApprove || responded.RespondedByDeviceID != clientID {
		t.Fatalf("approval response = %+v, err=%v", responded, err)
	}
	cancelApproval := domain.Approval{ID: storageID("approval", storageHexB), SessionID: sessionID, ProviderApprovalID: "approval-2", Kind: "file", PayloadDigest: storageDigestA, Summary: "cancel file change", Status: domain.ApprovalPending, IdempotencyKey: "provider-request-2", RequestedAt: now}
	if err := store.CreateApproval(ctx, cancelApproval); err != nil {
		t.Fatal(err)
	}
	claimed, err := store.ClaimApprovalDispatch(ctx, cancelApproval.ID, cancelApproval.ProviderApprovalID, clientID, "response-2", domain.ApprovalDecisionCancel, storageDigestB, now.Add(time.Second))
	if err != nil || claimed.Status != domain.ApprovalPending || claimed.ResponseState != domain.ApprovalResponseDispatching || claimed.RequestedDecision != domain.ApprovalDecisionCancel || claimed.DispatchDigest != storageDigestB {
		t.Fatalf("approval cancel claim = %+v, err=%v", claimed, err)
	}
	cancelled, err := store.CompleteApprovalDispatch(ctx, cancelApproval.ID, storageDigestB, now.Add(2*time.Second))
	if err != nil || cancelled.Status != domain.ApprovalCancelled || cancelled.ResponseState != domain.ApprovalResponseWritten || cancelled.RequestedDecision != domain.ApprovalDecisionCancel {
		t.Fatalf("approval cancel completion = %+v, err=%v", cancelled, err)
	}
	replayed, err := store.ClaimApprovalDispatch(ctx, cancelApproval.ID, cancelApproval.ProviderApprovalID, clientID, "response-2", domain.ApprovalDecisionCancel, storageDigestB, now.Add(3*time.Second))
	if err != nil || replayed.DispatchDigest != cancelled.DispatchDigest || replayed.Status != domain.ApprovalCancelled {
		t.Fatalf("cancel replay=%+v err=%v", replayed, err)
	}
	if _, err := store.ClaimApprovalDispatch(ctx, cancelApproval.ID, cancelApproval.ProviderApprovalID, clientID, "different-key", domain.ApprovalDecisionCancel, storageDigestB, now.Add(3*time.Second)); domain.CodeOf(err) != domain.CodeConflict {
		t.Fatalf("cancel conflict code=%v err=%v", domain.CodeOf(err), err)
	}
	failing := domain.Approval{ID: storageID("approvalfail", storageHexA), SessionID: sessionID, ProviderApprovalID: "approval-fail", Kind: "command", PayloadDigest: storageDigestA, Summary: "ambiguous provider write", Status: domain.ApprovalPending, IdempotencyKey: "provider-request-fail", RequestedAt: now}
	if err := store.CreateApproval(ctx, failing); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimApprovalDispatch(ctx, failing.ID, failing.ProviderApprovalID, clientID, "response-fail", domain.ApprovalDecisionApprove, storageDigestC, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	ambiguous, err := store.FailApprovalDispatch(ctx, failing.ID, storageDigestC, "provider_write_unknown", now.Add(2*time.Second))
	if err != nil || ambiguous.Status != domain.ApprovalExpired || ambiguous.ResponseState != domain.ApprovalResponseAmbiguous || ambiguous.DispatchErrorCode != "provider_write_unknown" {
		t.Fatalf("approval failure=%+v err=%v", ambiguous, err)
	}
	if replayedFailure, err := store.FailApprovalDispatch(ctx, failing.ID, storageDigestC, "provider_write_unknown", now.Add(3*time.Second)); err != nil || replayedFailure.DispatchDigest != ambiguous.DispatchDigest {
		t.Fatalf("approval failure replay=%+v err=%v", replayedFailure, err)
	}
	idle := domain.Approval{ID: storageID("approvalidle", storageHexA), SessionID: sessionID, ProviderApprovalID: "approval-idle", Kind: "command", PayloadDigest: storageDigestA, Summary: "idle on restart", Status: domain.ApprovalPending, IdempotencyKey: "provider-request-idle", RequestedAt: now}
	dispatching := domain.Approval{ID: storageID("approvaldispatch", storageHexA), SessionID: sessionID, ProviderApprovalID: "approval-dispatch", Kind: "command", PayloadDigest: storageDigestA, Summary: "dispatching on restart", Status: domain.ApprovalPending, IdempotencyKey: "provider-request-dispatch", RequestedAt: now}
	if err := store.CreateApproval(ctx, idle); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateApproval(ctx, dispatching); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimApprovalDispatch(ctx, dispatching.ID, dispatching.ProviderApprovalID, clientID, "response-dispatch", domain.ApprovalDecisionDeny, storageDigestD, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := store.ExpirePendingApprovals(ctx, now.Add(4*time.Second)); err != nil {
		t.Fatal(err)
	}
	idleAfter, err := store.Approval(ctx, idle.ID)
	if err != nil || idleAfter.Status != domain.ApprovalExpired || idleAfter.ResponseState != domain.ApprovalResponseAmbiguous || idleAfter.RequestedDecision != domain.ApprovalDecisionCancel || idleAfter.DispatchErrorCode != "daemon_restart_before_dispatch" {
		t.Fatalf("idle restart=%+v err=%v", idleAfter, err)
	}
	dispatchAfter, err := store.Approval(ctx, dispatching.ID)
	if err != nil || dispatchAfter.Status != domain.ApprovalExpired || dispatchAfter.ResponseState != domain.ApprovalResponseAmbiguous || dispatchAfter.RequestedDecision != domain.ApprovalDecisionDeny || dispatchAfter.DispatchDigest != storageDigestD || dispatchAfter.DispatchErrorCode != "daemon_restart" {
		t.Fatalf("dispatch restart=%+v err=%v", dispatchAfter, err)
	}
}

func TestCodexFoundationRejectsUnknownAccountWithoutPartialRows(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	now := time.Unix(600, 0).UTC()
	deviceID := storageID("device", storageHexA)
	profileID := storageID("profile", storageHexA)
	if err := store.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "codex", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	err := store.CreateRuntimeProfile(ctx, domain.RuntimeProfile{ID: profileID, DeviceID: deviceID, AccountID: storageID("account", storageHexA), Name: "orphan", Provider: domain.ProviderCodex, Settings: []byte(`{}`), CreatedAt: now, UpdatedAt: now})
	if domain.CodeOf(err) != domain.CodeNotFound {
		t.Fatalf("unknown account error=%v", err)
	}
	if _, err := store.RuntimeProfile(ctx, profileID); domain.CodeOf(err) != domain.CodeNotFound {
		t.Fatalf("orphan profile was partially written: %v", err)
	}
}

const storageDigestA = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
const storageDigestB = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
const storageDigestC = "1111111111111111111111111111111111111111111111111111111111111111"
const storageDigestD = "2222222222222222222222222222222222222222222222222222222222222222"
