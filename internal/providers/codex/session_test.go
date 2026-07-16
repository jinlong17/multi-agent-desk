package codex

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

func sessionIDs(t *testing.T) (domain.ID, domain.ID, domain.ID, domain.ID, domain.ID) {
	t.Helper()
	ids := make([]domain.ID, 5)
	for i, prefix := range []string{"session", "account", "credential", "profile", "workspace"} {
		ids[i], _ = domain.NewID(prefix)
	}
	return ids[0], ids[1], ids[2], ids[3], ids[4]
}

func TestProviderSessionHandshakeAndApprovalNotification(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	row := CompatibilityRows()[2]
	serverDone := make(chan error, 1)
	go func() {
		reader := NewFrameReader(serverConn)
		frame, err := reader.Read()
		if err != nil {
			serverDone <- err
			return
		}
		var initialize RPCRequest
		if err := DecodeObject(frame, &initialize); err != nil || initialize.Method != MethodInitialize {
			serverDone <- domain.NewError(domain.CodeProviderProtocolError, "initialize not received")
			return
		}
		result := map[string]any{"userAgent": "Codex/" + row.Version, "codexHome": "/tmp/codex", "platformFamily": "unix", "platformOs": "macos"}
		if err := WriteFrame(serverConn, RPCResponse{JSONRPC: "2.0", ID: json.RawMessage("1"), Result: mustJSON(result)}); err != nil {
			serverDone <- err
			return
		}
		if _, err := reader.Read(); err != nil {
			serverDone <- err
			return
		}
		params := json.RawMessage(`{"approvalId":"approval-1","command":null,"commandActions":null,"cwd":null,"environmentId":null,"itemId":"item-1","networkApprovalContext":null,"proposedExecpolicyAmendment":null,"proposedNetworkPolicyAmendments":null,"reason":null,"startedAtMs":1,"threadId":"thread-1","turnId":"turn-1"}`)
		if err := WriteFrame(serverConn, RPCRequest{JSONRPC: "2.0", ID: 77, Method: MethodApprovalCommand, Params: params}); err != nil {
			serverDone <- err
			return
		}
		response, err := reader.Read()
		if err != nil {
			serverDone <- err
			return
		}
		var approvalResponse RPCResponse
		if err := DecodeObject(response, &approvalResponse); err != nil || string(approvalResponse.ID) != "77" || string(approvalResponse.Result) != `{"decision":"accept"}` {
			serverDone <- domain.NewError(domain.CodeProviderProtocolError, "approval response did not match")
			return
		}
		serverDone <- nil
	}()
	sessionID, accountID, credentialID, profileID, workspaceID := sessionIDs(t)
	caps, err := CapabilitiesFor(BinaryDescriptor{Provider: ProviderName, Path: "/opt/codex", Version: row.Version, SchemaFingerprint: row.SchemaFingerprint})
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewProviderSession(NewClient(clientConn, clientConn), SessionConfig{SessionID: sessionID, AccountID: accountID, CredentialInstanceID: credentialID, RuntimeProfileID: profileID, WorkspaceID: workspaceID, ProviderVersion: row.Version, Capabilities: caps})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	event, err := session.ReadEvent(context.Background())
	if err != nil || event.ProviderApprovalID != "approval-1" || event.PayloadDigest == "" {
		t.Fatalf("event=%+v err=%v", event, err)
	}
	if err := session.RespondApproval(context.Background(), "approval-1", "approved"); err != nil {
		t.Fatal(err)
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
}

func TestProviderSessionStopsAndResumesFailClosed(t *testing.T) {
	sessionID, accountID, credentialID, profileID, workspaceID := sessionIDs(t)
	caps, err := CapabilitiesFor(BinaryDescriptor{Provider: ProviderName, Path: "/opt/codex", Version: CompatibilityRows()[0].Version, SchemaFingerprint: CompatibilityRows()[0].SchemaFingerprint})
	if err != nil {
		t.Fatal(err)
	}
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	session, err := NewProviderSession(NewClient(client, client), SessionConfig{SessionID: sessionID, AccountID: accountID, CredentialInstanceID: credentialID, RuntimeProfileID: profileID, WorkspaceID: workspaceID, ProviderVersion: CompatibilityRows()[0].Version, Capabilities: caps})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Resume(context.Background()); domain.CodeOf(err) != domain.CodeProviderResumeUnsupported {
		t.Fatalf("resume err=%v", err)
	}
	// A session that has not completed initialize cannot send a Provider stop.
	if err := session.Stop(context.Background()); domain.CodeOf(err) != domain.CodeProviderUnsupported {
		t.Fatalf("stop err=%v", err)
	}
}

func mustJSON(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}
