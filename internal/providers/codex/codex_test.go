package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

func TestCompatibilityRowsRequireExactVersionAndFingerprint(t *testing.T) {
	rows := CompatibilityRows()
	if len(rows) != 3 {
		t.Fatalf("rows=%d, want 3", len(rows))
	}
	row := rows[2]
	capabilities, err := CapabilitiesFor(BinaryDescriptor{Provider: ProviderName, Path: "/opt/codex", Version: row.Version, SchemaFingerprint: row.SchemaFingerprint})
	if err != nil || capabilities.Status != CapabilitySupported || !capabilities.Allows(MethodAccountUsage) {
		t.Fatalf("supported capabilities=%+v err=%v", capabilities, err)
	}
	if _, err := CapabilitiesFor(BinaryDescriptor{Provider: ProviderName, Path: "/opt/codex", Version: row.Version, SchemaFingerprint: "bad"}); domain.CodeOf(err) != domain.CodeProviderVersionUnsupported {
		t.Fatalf("fingerprint mismatch err=%v", err)
	}
	if _, err := CapabilitiesFor(BinaryDescriptor{Provider: ProviderName, Path: "/opt/codex", Version: "0.999.0", SchemaFingerprint: row.SchemaFingerprint}); domain.CodeOf(err) != domain.CodeProviderVersionUnsupported {
		t.Fatalf("unknown version err=%v", err)
	}
}

func TestVersionDiscoveryUsesAbsoluteExecutableAndBoundedProbe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-only")
	}
	path := filepath.Join(t.TempDir(), "codex-fixture")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'codex-cli 0.144.2\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	descriptor, err := Discover(context.Background(), DiscoverOptions{Override: path, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if descriptor.Version != "0.144.2" || descriptor.Path != path {
		t.Fatalf("descriptor=%+v", descriptor)
	}
	if _, err := Discover(context.Background(), DiscoverOptions{Override: filepath.Join(t.TempDir(), "missing")}); domain.CodeOf(err) != domain.CodeProviderFailed {
		t.Fatalf("missing binary err=%v", err)
	}
}

func TestConfiguredCodexBinaryCanonicalSchemaProbe(t *testing.T) {
	path := os.Getenv("MULTIDESK_CODEX_LIVE_BINARY")
	if path == "" {
		t.Skip("set MULTIDESK_CODEX_LIVE_BINARY for an exact local schema probe")
	}
	descriptor, err := Discover(context.Background(), DiscoverOptions{Override: path, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	capabilities, err := Probe(context.Background(), descriptor, ProbeOptions{Timeout: 20 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if capabilities.Status != CapabilitySupported || !capabilities.Allows(MethodAccountRead) || len(capabilities.SchemaFingerprint) != 64 {
		t.Fatalf("capabilities=%+v", capabilities)
	}
}

func TestConfiguredCodexBinaryEmptyHomeHandshake(t *testing.T) {
	path := os.Getenv("MULTIDESK_CODEX_LIVE_BINARY")
	if path == "" {
		t.Skip("set MULTIDESK_CODEX_LIVE_BINARY for an empty-home handshake")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	home := t.TempDir()
	command := exec.CommandContext(ctx, path, "app-server")
	command.Env = []string{"CODEX_HOME=" + home, "HOME=" + home, "PATH=" + os.Getenv("PATH")}
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = io.Discard
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	}()
	row := CompatibilityRows()[2]
	client := NewClient(stdout, stdin)
	if err := client.ConfigureMethods(row.Methods); err != nil {
		t.Fatal(err)
	}
	result, err := client.Handshake(ctx, InitializeParams{ClientInfo: ClientInfo{Name: "multi-agent-desk-test", Version: "phase2"}, Capabilities: &InitializeCapabilities{}})
	if err != nil || result.CodexHome == "" || result.PlatformOS != "macos" {
		t.Fatalf("initialize=%+v err=%v", result, err)
	}
	var raw json.RawMessage
	if err := client.Call(ctx, MethodAccountRead, map[string]any{"refreshToken": false}, &raw); err != nil {
		t.Fatal(err)
	}
	account, err := DecodeAccountResponse(raw, time.Now())
	if err != nil || !account.RequiresOpenAIAuth || account.AccountType != "" {
		t.Fatalf("account=%+v err=%v", account, err)
	}
}

func TestSchemaFingerprintCanonicalizesObjectOrderAndRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "b.json"), []byte(`{"b":2,"a":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.json"), []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := FingerprintSchema(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := FingerprintSchema(root)
	if err != nil || first != second || len(first) != 64 {
		t.Fatalf("fingerprints=%q/%q err=%v", first, second, err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.json"), []byte(`{"a":1,"b":2}`), 0o600); err != nil {
		t.Fatal(err)
	}
	reordered, err := FingerprintSchema(root)
	if err != nil || reordered != first {
		t.Fatalf("canonical fingerprints=%q/%q err=%v", first, reordered, err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink(filepath.Join(root, "a.json"), filepath.Join(root, "link.json")); err != nil {
			t.Fatal(err)
		}
		if _, err := FingerprintSchema(root); domain.CodeOf(err) != domain.CodeProviderProtocolError {
			t.Fatalf("symlink error=%v", err)
		}
	}
}

func TestJSONLRejectsDuplicateAndTrailingFields(t *testing.T) {
	if _, err := ReadFrame(strings.NewReader(`{"id":1,"id":2}`)); domain.CodeOf(err) != domain.CodeProviderProtocolError {
		t.Fatalf("duplicate frame err=%v", err)
	}
	if _, err := ReadFrame(strings.NewReader(`{"id":1} {"id":2}`)); domain.CodeOf(err) != domain.CodeProviderProtocolError {
		t.Fatalf("trailing frame err=%v", err)
	}
	var target struct {
		ID int `json:"id"`
	}
	if err := DecodeObject([]byte(`{"id":1,"unknown":2}`), &target); domain.CodeOf(err) != domain.CodeProviderProtocolError {
		t.Fatalf("unknown object field err=%v", err)
	}
}

func TestHandshakeConsumesOnlyOnePersistentFrameAtATime(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	serverDone := make(chan error, 1)
	go func() {
		reader := NewFrameReader(serverConn)
		request, err := reader.Read()
		if err != nil {
			serverDone <- err
			return
		}
		var initialize RPCRequest
		if err := DecodeObject(request, &initialize); err != nil || initialize.Method != MethodInitialize {
			serverDone <- fmt.Errorf("initialize=%s err=%v", request, err)
			return
		}
		result := json.RawMessage(`{"userAgent":"Codex/0.144.2","codexHome":"/tmp/codex","platformFamily":"unix","platformOs":"macos"}`)
		if err := WriteFrame(serverConn, RPCResponse{JSONRPC: "2.0", ID: json.RawMessage("1"), Result: result}); err != nil {
			serverDone <- err
			return
		}
		notification, err := reader.Read()
		if err != nil {
			serverDone <- err
			return
		}
		var initialized RPCRequest
		if err := DecodeObject(notification, &initialized); err != nil || initialized.Method != MethodInitialized {
			serverDone <- fmt.Errorf("initialized=%s err=%v", notification, err)
			return
		}
		serverDone <- nil
	}()
	client := NewClient(clientConn, clientConn)
	if err := client.ConfigureMethods([]string{MethodAccountUsage}); err != nil {
		t.Fatal(err)
	}
	result, err := client.Handshake(context.Background(), InitializeParams{ClientInfo: ClientInfo{Name: "test", Version: "1"}, Capabilities: &InitializeCapabilities{}})
	if err != nil || result.PlatformOS != "macos" {
		t.Fatalf("handshake result=%+v err=%v", result, err)
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
}

func TestReplayFixturesAndFailClosedMapping(t *testing.T) {
	for _, version := range []string{"0.142.5", "0.143.0", "0.144.2"} {
		result, err := ReplayFixture(version)
		if err != nil || !result.Sanitized || result.Usage.SourceVersion != version || result.Approval.PayloadDigest == "" {
			t.Fatalf("fixture %s result=%+v err=%v", version, result, err)
		}
	}
	if _, err := DecodeApprovalServerRequest(MethodApprovalCommand, []byte(`{"itemId":"a","startedAtMs":1,"threadId":"t","turnId":"u","unmapped":true}`)); domain.CodeOf(err) != domain.CodeProviderProtocolError {
		t.Fatalf("unmapped Approval field err=%v", err)
	}
	if _, err := DecodeUsageResponse([]byte(`{"dailyUsageBuckets":[],"summary":{"unmapped":1}}`), "0.144.2", time.Now()); domain.CodeOf(err) != domain.CodeProviderVersionUnsupported {
		t.Fatalf("unmapped Usage field err=%v", err)
	}
}
