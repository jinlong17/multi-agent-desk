package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/device"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/runtime"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
)

func TestP0ApprovalIPCRequiresLeaseAndReplaysIdempotently(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "device", "device.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Unix(700, 0).UTC()
	deviceID := appTestID(t, "device")
	clientID := appTestID(t, "client")
	accountID := appTestID(t, "account")
	profileID := appTestID(t, "profile")
	credentialID := appTestID(t, "credential")
	workspaceID := appTestID(t, "workspace")
	sessionID := appTestID(t, "session")
	approvalID := appTestID(t, "approval")
	digest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := store.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "p0", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: clientID, Name: "owner", PublicKey: make([]byte, 32), Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityApprovalRespond}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAccount(ctx, domain.Account{ID: accountID, Provider: domain.ProviderCodex, DisplayName: "primary", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRuntimeProfile(ctx, domain.RuntimeProfile{ID: profileID, DeviceID: deviceID, AccountID: accountID, Name: "codex", Provider: domain.ProviderCodex, Settings: []byte(`{}`), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateCredentialInstance(ctx, domain.CredentialInstance{ID: credentialID, DeviceID: deviceID, AccountID: accountID, Provider: domain.ProviderCodex, AuthMethod: domain.AuthMethodInteractive, SecretRef: "vault:codex/test", Status: domain.CredentialHealthy, CredentialRevision: 1, SecretDigest: digest, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateWorkspace(ctx, domain.Workspace{ID: workspaceID, DeviceID: deviceID, Path: "/tmp/p0", Label: "p0", Tags: []string{}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(ctx, domain.Session{ID: sessionID, DeviceID: deviceID, AccountID: accountID, Provider: domain.ProviderCodex, CredentialInstanceID: credentialID, RuntimeProfileID: profileID, WorkspaceID: workspaceID, Status: domain.SessionStarting, StartedAt: now, CapabilitySnapshot: []domain.Capability{domain.CapabilitySessionResume}}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateApproval(ctx, domain.Approval{ID: approvalID, SessionID: sessionID, ProviderApprovalID: "provider-approval-1", Kind: "command", PayloadDigest: digest, Summary: "safe summary", Status: domain.ApprovalPending, IdempotencyKey: "provider-request-1", RequestedAt: now}); err != nil {
		t.Fatal(err)
	}
	lease, err := domain.AcquireControllerLease(nil, sessionID, clientID, now, domain.DefaultLeaseDuration)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveControllerLease(ctx, lease, 0); err != nil {
		t.Fatal(err)
	}
	service := NewSessionService(store, runtime.NewManager(store, "unused"))
	service.Now = func() time.Time { return now.Add(time.Second) }
	body, _ := device.JSONBody(map[string]any{"session_id": sessionID, "approval_id": approvalID, "provider_approval_id": "provider-approval-1", "decision": "approved"})
	auth := device.AuthContext{ClientID: clientID, IdentityRevision: 1, AuthenticatedAt: now, ExpiresAt: now.Add(time.Hour)}
	_, err = service.Handle(ctx, auth, device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "approval-1", Method: "approval.respond", IdempotencyKey: "response-1", LeaseRevision: &lease.Revision, Body: body})
	if domain.CodeOf(err) != domain.CodeProviderUnsupported {
		t.Fatalf("approval without runtime dispatch code=%v err=%v", domain.CodeOf(err), err)
	}
	stored, err := store.Approval(ctx, approvalID)
	if err != nil || stored.Status != domain.ApprovalPending || stored.ResponseState != domain.ApprovalResponseIdle {
		t.Fatalf("stored approval = %+v, err=%v", stored, err)
	}
}

func appTestID(t *testing.T, prefix string) domain.ID {
	t.Helper()
	id, err := domain.NewID(prefix)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
