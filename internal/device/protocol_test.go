package device

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"net"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
)

type memoryClients struct {
	clients map[domain.ID]domain.ClientIdentity
}

func (m memoryClients) ClientIdentity(_ context.Context, id domain.ID) (domain.ClientIdentity, error) {
	client, ok := m.clients[id]
	if !ok {
		return domain.ClientIdentity{}, domain.NewError(domain.CodeNotFound, "client not found")
	}
	return client, nil
}

func TestFrameRejectsDuplicateKeysAndOversize(t *testing.T) {
	if err := validateJSON([]byte(`{"a":1,"a":2}`)); err == nil {
		t.Fatal("duplicate JSON key accepted")
	}
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], MaxFrameBytes+1)
	if _, err := readFrame(bytesReader(length[:])); domain.CodeOf(err) != domain.CodeFrameTooLarge {
		t.Fatalf("oversize frame code = %v", domain.CodeOf(err))
	}
}

type bytesReader []byte

func (r bytesReader) Read(p []byte) (int, error) {
	if len(r) == 0 {
		return 0, errors.New("eof")
	}
	n := copy(p, r)
	return n, nil
}

func TestAuthenticatedHandshakeBindsBothKeysAndCapabilities(t *testing.T) {
	clientPublic, clientPrivate, _ := ed25519.GenerateKey(rand.Reader)
	daemonPublic, daemonPrivate, _ := ed25519.GenerateKey(rand.Reader)
	clientID, _ := domain.NewID("client")
	daemonID, _ := domain.NewID("device")
	clientRecord := domain.ClientIdentity{ID: clientID, Name: "test", PublicKey: clientPublic, Revision: 4, Status: domain.ClientIdentityActive,
		Caps: []domain.Capability{domain.CapabilityMetadataRead, domain.CapabilitySessionObserve}}
	server, err := NewServerAuthenticator(DaemonIdentity{DeviceID: daemonID, PrivateKey: daemonPrivate}, make([]byte, 32), memoryClients{clients: map[domain.ID]domain.ClientIdentity{clientID: clientRecord}})
	if err != nil {
		t.Fatal(err)
	}
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	serverResult := make(chan AuthContext, 1)
	serverErr := make(chan error, 1)
	go func() {
		result, err := server.Handshake(context.Background(), left)
		serverResult <- result
		serverErr <- err
	}()
	client := ClientAuthenticator{Identity: ClientIdentity{ClientID: clientID, PrivateKey: clientPrivate, DaemonID: daemonID, DaemonPublicKey: daemonPublic}, RequestedCapabilities: []domain.Capability{domain.CapabilitySessionObserve}}
	accepted, err := client.Handshake(context.Background(), right)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
	serverAuth := <-serverResult
	if accepted.IdentityRevision != 4 || serverAuth.IdentityRevision != 4 || !slices.Equal(accepted.GrantedCapabilities, []domain.Capability{domain.CapabilitySessionObserve}) || !serverAuth.Has(domain.CapabilitySessionObserve) {
		t.Fatalf("unexpected auth: client=%+v server=%+v", accepted, serverAuth)
	}
}

func TestBootstrapAndAuthenticatedPlatformDaemon(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "device")
	result, err := Bootstrap(context.Background(), root, "test-device", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	daemon, err := LoadDaemonIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := LoadOwnerIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(context.Background(), DeviceDatabasePath(root))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := VerifyIdentityStore(context.Background(), root, store); err != nil {
		t.Fatal(err)
	}
	if result.DeviceID != daemon.DeviceID || result.ClientID != owner.ClientID {
		t.Fatal("bootstrap identities do not match")
	}
	listener, err := Listen(root)
	if err != nil {
		t.Fatalf("listen: %v; cause: %v", err, errors.Unwrap(err))
	}
	defer listener.Close()
	authenticator, err := NewServerAuthenticator(daemon, make([]byte, 32), store)
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{Listener: listener, Authenticator: authenticator, Authorizer: func(_ context.Context, auth AuthContext, method string) error {
		if method != "daemon.status" || !auth.Has(domain.CapabilityMetadataRead) {
			return domain.NewError(domain.CodePermissionDenied, "capability is not granted")
		}
		return nil
	}, Handler: HandlerFunc(func(_ context.Context, _ AuthContext, request Request) (any, error) {
		if request.Method != "daemon.status" {
			return nil, domain.NewError(domain.CodeMethodNotFound, "method is not available")
		}
		return map[string]any{"status": "ok"}, nil
	})}
	serveCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(serveCtx) }()
	connection, err := Dial(root, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	clientAuth, err := (ClientAuthenticator{Identity: owner, RequestedCapabilities: []domain.Capability{domain.CapabilityMetadataRead}}).Handshake(context.Background(), connection)
	if err != nil {
		t.Fatal(err)
	}
	client := &Client{Connection: connection, Auth: clientAuth}
	defer client.Close()
	body, _ := JSONBody(map[string]string{"probe": "ok"})
	response, err := client.Call(context.Background(), Request{ProtocolMajor: ProtocolMajor, RequestID: "status-1", Method: "daemon.status", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if !response.OK || string(response.Result) != `{"status":"ok"}` {
		t.Fatalf("unexpected response: %+v", response)
	}
	_ = server.Close()
	cancel()
	select {
	case <-serveErr:
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
}
