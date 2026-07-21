//go:build windows

package device

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"os"
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
	_ = server.Close()
	cancel()
	select {
	case <-serveErr:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop")
	}
}

func TestNamedPipeListenerCloseUnblocksPendingAccept(t *testing.T) {
	root := filepath.Join(t.TempDir(), "device")
	if err := createPrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	listenerValue, err := Listen(root)
	if err != nil {
		t.Fatal(err)
	}
	listener := listenerValue.(*windowsListener)
	acceptErr := make(chan error, 1)
	go func() {
		connection, acceptError := listener.Accept()
		if connection != nil {
			_ = connection.Close()
		}
		acceptErr <- acceptError
	}()
	waitForPendingAccept(t, listener)
	closeDone := make(chan error, 1)
	go func() { closeDone <- listener.Close() }()
	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("listener close did not cancel pending accept")
	}
	select {
	case err := <-acceptErr:
		if !errors.Is(err, os.ErrClosed) {
			t.Fatalf("pending accept error=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pending accept did not return")
	}
}

func TestNamedPipeCloseUnblocksPendingRead(t *testing.T) {
	for iteration := 0; iteration < 32; iteration++ {
		testNamedPipeCloseUnblocksPendingRead(t, iteration)
	}
}

func testNamedPipeCloseUnblocksPendingRead(t *testing.T, iteration int) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "device")
	if err := createPrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	listener, err := Listen(root)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	type acceptResult struct {
		connection io.ReadWriteCloser
		err        error
	}
	accepted := make(chan acceptResult, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		accepted <- acceptResult{connection: connection, err: acceptErr}
	}()
	client, err := Dial(root, 5*time.Second)
	if err != nil {
		t.Fatalf("iteration %d dial: %v", iteration, err)
	}
	defer client.Close()
	var result acceptResult
	select {
	case result = <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatalf("iteration %d accept did not return", iteration)
	}
	if result.err != nil {
		t.Fatalf("iteration %d accept: %v", iteration, result.err)
	}
	serverConnection := result.connection.(*pipeConn)
	defer serverConnection.Close()
	readDone := make(chan error, 1)
	go func() {
		_, readErr := serverConnection.Read(make([]byte, 1))
		readDone <- readErr
	}()
	waitForPipeOperation(t, serverConnection, iteration)
	closeDone := make(chan error, 1)
	go func() { closeDone <- serverConnection.Close() }()
	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("iteration %d close: %v", iteration, err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("iteration %d close did not cancel pending read", iteration)
	}
	select {
	case err := <-readDone:
		if !errors.Is(err, os.ErrClosed) {
			t.Fatalf("iteration %d read error=%v", iteration, err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("iteration %d pending read did not return", iteration)
	}
	if err := serverConnection.Close(); err != nil {
		t.Fatalf("iteration %d repeated close: %v", iteration, err)
	}
}

func TestNamedPipeReadDeadlineCancelsPendingRead(t *testing.T) {
	root := filepath.Join(t.TempDir(), "device")
	if err := createPrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	listener, err := Listen(root)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	type acceptResult struct {
		connection io.ReadWriteCloser
		err        error
	}
	accepted := make(chan acceptResult, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		accepted <- acceptResult{connection: connection, err: acceptErr}
	}()
	client, err := Dial(root, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var result acceptResult
	select {
	case result = <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("accept did not return")
	}
	if result.err != nil {
		t.Fatal(result.err)
	}
	serverConnection := result.connection.(*pipeConn)
	defer serverConnection.Close()
	if err := serverConnection.SetDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	readDone := make(chan error, 1)
	go func() {
		_, readErr := serverConnection.Read(make([]byte, 1))
		readDone <- readErr
	}()
	waitForPipeOperation(t, serverConnection, 0)
	select {
	case err := <-readDone:
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("deadline read error=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("deadline did not cancel pending read")
	}
}

func waitForPendingAccept(t *testing.T, listener *windowsListener) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		listener.mu.Lock()
		pending := listener.accepting != 0
		listener.mu.Unlock()
		if pending {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("accept did not become pending")
		}
		time.Sleep(time.Millisecond)
	}
}

func waitForPipeOperation(t *testing.T, connection *pipeConn, iteration int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		connection.mu.Lock()
		active := connection.active
		connection.mu.Unlock()
		if active > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("iteration %d read did not become pending", iteration)
		}
		time.Sleep(time.Millisecond)
	}
}

type windowsClientLookup struct{ value domain.ClientIdentity }

func (l windowsClientLookup) ClientIdentity(_ context.Context, id domain.ID) (domain.ClientIdentity, error) {
	if id != l.value.ID {
		return domain.ClientIdentity{}, domain.NewError(domain.CodeNotFound, "client not found")
	}
	return l.value, nil
}
