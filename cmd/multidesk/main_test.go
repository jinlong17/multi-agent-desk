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
