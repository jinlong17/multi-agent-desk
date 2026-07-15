package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	runErr := run(args, writer, writer)
	_ = writer.Close()
	data, readErr := io.ReadAll(reader)
	_ = reader.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	return string(data), runErr
}

func TestServiceSpecCommandIsStableJSONAndNonMutating(t *testing.T) {
	root := filepath.Join(t.TempDir(), "device")
	executable := filepath.Join(t.TempDir(), "multidesk")
	output, err := captureCLI(t, "daemon", "install", "--root", root, "--executable", executable, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		SchemaVersion int            `json:"schema_version"`
		RequestID     string         `json:"request_id"`
		OK            bool           `json:"ok"`
		Result        map[string]any `json:"result"`
	}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("invalid JSON output %q: %v", output, err)
	}
	if envelope.SchemaVersion != 1 || envelope.RequestID != "daemon-install" || !envelope.OK || envelope.Result["goos"] == nil {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("service rendering mutated root: %v", err)
	}
}

func TestCLIRejectsUnsupportedClientProvisioningWithoutLeakingKey(t *testing.T) {
	output, err := captureCLI(t, "client", "create", "--root", filepath.Join(t.TempDir(), "device"), "--json")
	// The command returns a safe error to stderr; it must not manufacture a
	// private key or silently fall back to direct database access.
	if err == nil {
		t.Fatal("client create unexpectedly succeeded")
	}
	if strings.Contains(output, "private_key") {
		t.Fatalf("private key appeared in CLI output: %q", output)
	}
}

func TestCLIRequestIdentityBindsBodyAndRevision(t *testing.T) {
	firstID, firstKey := cliRequestIdentity("terminal.input", []byte(`{"session_id":"session-a","sequence":1,"payload":"a"}`), ptrInt64(1))
	secondID, secondKey := cliRequestIdentity("terminal.input", []byte(`{"session_id":"session-a","sequence":2,"payload":"b"}`), ptrInt64(1))
	retryID, retryKey := cliRequestIdentity("terminal.input", []byte(`{"session_id":"session-a","sequence":1,"payload":"a"}`), ptrInt64(1))
	if firstID == secondID || firstKey == secondKey {
		t.Fatal("different CLI operations reused the same request identity")
	}
	if firstID != retryID || firstKey != retryKey {
		t.Fatal("exact CLI retry did not retain its request identity")
	}
}

func TestVaultSecretReaderIsBoundedAndDoesNotEcho(t *testing.T) {
	secret, err := readVaultSecret(strings.NewReader("unlock-value\n"))
	if err != nil || secret != "unlock-value" {
		t.Fatalf("stdin secret=%q err=%v", secret, err)
	}
	if _, err := readVaultSecret(strings.NewReader(strings.Repeat("x", maxVaultUnlockInput+1))); err == nil {
		t.Fatal("oversized stdin secret accepted")
	}
}

func TestVaultUnlockRejectsArgvSecret(t *testing.T) {
	_, err := captureCLI(t, "vault", "unlock", "--root", filepath.Join(t.TempDir(), "device"), "--secret", "argv-secret")
	if err == nil {
		t.Fatal("argv Vault secret was accepted")
	}
}

func TestVaultUnlockRejectsPositionalSecret(t *testing.T) {
	_, err := captureCLI(t, "vault", "unlock", "--root", filepath.Join(t.TempDir(), "device"), "--secret-stdin", "argv-secret")
	if err == nil || !strings.Contains(err.Error(), "accepts no positional arguments") {
		t.Fatalf("positional Vault secret was not rejected at parse boundary: %v", err)
	}
}

func ptrInt64(value int64) *int64 { return &value }
