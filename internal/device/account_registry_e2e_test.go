package device_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/app"
	device "github.com/jinlong17/multi-agent-desk/internal/device"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
	runtimepkg "github.com/jinlong17/multi-agent-desk/internal/runtime"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
)

func TestAuthenticatedAccountRegistryAndRealProviderFailClosed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	root := filepath.Join(t.TempDir(), "device-root")
	if _, err := device.Bootstrap(ctx, root, "account-e2e", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(ctx, device.DeviceDatabasePath(root))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	listener, err := device.Listen(root)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	daemon, err := device.LoadDaemonIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	authenticator, err := device.NewServerAuthenticator(daemon, mustEndpointInstance(t), store)
	if err != nil {
		t.Fatal(err)
	}
	manager := runtimepkg.NewManager(store, os.Args[0])
	defer manager.Close()
	service := app.NewSessionService(store, manager)
	server := &device.Server{Listener: listener, Authenticator: authenticator,
		Authorizer: (app.Authorizer{Clients: store}).Authorize, Handler: service}
	go func() { _ = server.Serve(ctx) }()

	owner := LoadOwnerForE2E(t, root)
	connection, err := device.Dial(root, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	auth, err := (device.ClientAuthenticator{Identity: owner, RequestedCapabilities: []domain.Capability{
		domain.CapabilityMetadataRead, domain.CapabilityClientAdmin, domain.CapabilitySessionStart,
	}}).Handshake(ctx, connection)
	if err != nil {
		t.Fatal(err)
	}
	client := &device.Client{Connection: connection, Auth: auth}
	body, _ := device.JSONBody(map[string]any{"provider": "codex", "name": "Personal", "alias": "A"})
	created := mustCall(t, client, device.Request{ProtocolMajor: device.ProtocolMajor,
		RequestID: "account-create-1", Method: "accounts.create", IdempotencyKey: "account-create-key", Body: body})
	var result struct {
		Account struct {
			ID       domain.ID `json:"id"`
			Revision int64     `json:"revision"`
		} `json:"account"`
		Profile struct {
			ID           domain.ID `json:"id"`
			Selector     string    `json:"selector"`
			AuthStatus   string    `json:"auth_status"`
			Availability string    `json:"availability"`
		} `json:"profile"`
	}
	decodeResult(t, created, &result)
	if result.Account.ID == "" || result.Account.Revision != 1 || result.Profile.Selector != "@A" ||
		result.Profile.AuthStatus != "login_required" || result.Profile.Availability != "unknown" {
		t.Fatalf("unexpected create result: %+v", result)
	}

	replayed := mustCall(t, client, device.Request{ProtocolMajor: device.ProtocolMajor,
		RequestID: "account-create-2", Method: "accounts.create", IdempotencyKey: "account-create-key", Body: body})
	var replayResult map[string]any
	if err := json.Unmarshal(replayed.Result, &replayResult); err != nil {
		t.Fatal(err)
	}
	var originalResult map[string]any
	if err := json.Unmarshal(created.Result, &originalResult); err != nil {
		t.Fatal(err)
	}
	if originalResult["account"] == nil || replayResult["account"] == nil {
		t.Fatal("idempotent create did not replay the account result")
	}

	listBody, _ := device.JSONBody(map[string]any{"limit": 3})
	mustCall(t, client, device.Request{ProtocolMajor: device.ProtocolMajor,
		RequestID: "account-list-1", Method: "accounts.list", Body: listBody})
	changedBody, _ := device.JSONBody(map[string]any{"profile_selector": "@A", "workspace_path": "/tmp/project",
		"confirmation": map[string]any{"account_id": result.Account.ID,
			"credential_instance_id": "", "runtime_profile_id": domain.ID("profile_00000000000000000000000000000000"), "usage_snapshot_id": ""}})
	changedResponse, changedErr := client.Call(ctx, device.Request{ProtocolMajor: device.ProtocolMajor,
		RequestID: "changed-provider-start", Method: "sessions.start", IdempotencyKey: "changed-provider-start-key", Body: changedBody})
	if changedErr == nil || changedResponse.OK || domain.CodeOf(changedErr) != domain.CodeIdentityConfirmationRequired {
		t.Fatalf("start without daemon preview did not fail closed: response=%+v err=%v", changedResponse, changedErr)
	}

	startBody, _ := device.JSONBody(map[string]any{"profile_selector": "@A", "workspace_path": "/tmp/project"})
	response, callErr := client.Call(ctx, device.Request{ProtocolMajor: device.ProtocolMajor,
		RequestID: "real-provider-start", Method: "sessions.start", IdempotencyKey: "real-provider-start-key", Body: startBody})
	if callErr == nil || response.OK || domain.CodeOf(callErr) != domain.CodeIdentityConfirmationRequired {
		t.Fatalf("raw selector start did not fail closed: response=%+v err=%v", response, callErr)
	}
	sessions, err := store.ListSessions(ctx)
	if err != nil || len(sessions) != 0 {
		t.Fatalf("real provider failure created sessions: %+v, %v", sessions, err)
	}
}
