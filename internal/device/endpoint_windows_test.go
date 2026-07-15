//go:build windows

package device

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

func TestNamedPipeAuthenticatedDaemon(t *testing.T) {
	root := filepath.Join(t.TempDir(), "device")
	if err := createPrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	clientPublic, clientPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	daemonPublic, daemonPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clientID, _ := domain.NewID("client")
	daemonID, _ := domain.NewID("device")
	clientRecord := domain.ClientIdentity{ID: clientID, Name: "windows-test", PublicKey: clientPublic, Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityMetadataRead}}
	lookup := windowsClientLookup{value: clientRecord}
	listener, err := Listen(root)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	authenticator, err := NewServerAuthenticator(DaemonIdentity{DeviceID: daemonID, PrivateKey: daemonPrivate}, make([]byte, 32), lookup)
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{Listener: listener, Authenticator: authenticator, Authorizer: func(_ context.Context, auth AuthContext, method string) error {
		if method != "daemon.status" || !auth.Has(domain.CapabilityMetadataRead) {
			return domain.NewError(domain.CodePermissionDenied, "capability is not granted")
		}
		return nil
	}, Handler: HandlerFunc(func(_ context.Context, _ AuthContext, _ Request) (any, error) {
		return map[string]string{"status": "ok"}, nil
	})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(ctx) }()
	connection, err := Dial(root, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	accepted, err := (ClientAuthenticator{Identity: ClientIdentity{ClientID: clientID, PrivateKey: clientPrivate, DaemonID: daemonID, DaemonPublicKey: daemonPublic}, RequestedCapabilities: []domain.Capability{domain.CapabilityMetadataRead}}).Handshake(ctx, connection)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.IdentityRevision != 1 {
		t.Fatalf("unexpected revision: %d", accepted.IdentityRevision)
	}
	if err := writeFrame(connection, Request{ProtocolMajor: ProtocolMajor, RequestID: "windows-status", Method: "daemon.status"}); err != nil {
		t.Fatal(err)
	}
	responseBytes, err := readFrame(connection)
	if err != nil {
		t.Fatal(err)
	}
	var response Response
	if err := decodeStrict(responseBytes, &response); err != nil {
		t.Fatal(err)
	}
	if !response.OK {
		t.Fatalf("request rejected: %+v", response.Error)
	}
	_ = listener.Close()
	cancel()
	select {
	case <-serveErr:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop")
	}
}

type windowsClientLookup struct{ value domain.ClientIdentity }

func (l windowsClientLookup) ClientIdentity(_ context.Context, id domain.ID) (domain.ClientIdentity, error) {
	if id != l.value.ID {
		return domain.ClientIdentity{}, domain.NewError(domain.CodeNotFound, "client not found")
	}
	return l.value, nil
}
