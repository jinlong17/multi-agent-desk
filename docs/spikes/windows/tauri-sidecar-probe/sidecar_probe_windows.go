//go:build windows

// Command mad-sidecar is a lifecycle fixture for the Windows Tauri sidecar
// Spike. It is not production daemon or authorization code.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const schemaVersion = 1

type readyState struct {
	SchemaVersion int    `json:"schema_version"`
	DaemonPID     int    `json:"daemon_pid"`
	GrandchildPID int    `json:"grandchild_pid"`
	OwnerHash     string `json:"owner_hash"`
	StartedAtUTC  string `json:"started_at_utc"`
}

type controlCommand struct {
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	Action        string `json:"action"`
	OwnerToken    string `json:"owner_token"`
}

type controlResult struct {
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	Accepted      bool   `json:"accepted"`
	Reason        string `json:"reason"`
	DaemonPID     int    `json:"daemon_pid"`
	GrandchildPID int    `json:"grandchild_pid"`
	CompletedUTC  string `json:"completed_at_utc"`
}

func main() {
	mode := flag.String("mode", "daemon", "daemon or grandchild")
	stateDir := flag.String("state", "", "probe state directory")
	flag.Parse()
	if *stateDir == "" {
		fail(errors.New("-state is required"))
	}
	var err error
	switch *mode {
	case "daemon":
		err = daemonMain(*stateDir)
	case "grandchild":
		err = grandchildMain(*stateDir)
	default:
		err = fmt.Errorf("unknown mode %q", *mode)
	}
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "sidecar probe failed:", err)
	os.Exit(1)
}

func daemonMain(stateDir string) error {
	ownerToken := os.Getenv("MAD_OWNER_TOKEN")
	if len(ownerToken) < 16 {
		return errors.New("MAD_OWNER_TOKEN must contain at least 16 characters")
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	lockPath := filepath.Join(stateDir, "daemon.lock")
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("acquire exclusive daemon lock: %w", err)
	}
	if _, err := fmt.Fprintf(lock, "%d\n", os.Getpid()); err != nil {
		lock.Close()
		return err
	}
	if err := lock.Close(); err != nil {
		return err
	}
	cleanExit := false
	defer func() {
		if cleanExit {
			_ = os.Remove(lockPath)
			_ = os.Remove(filepath.Join(stateDir, "ready.json"))
			_ = os.Remove(filepath.Join(stateDir, "grandchild-heartbeat.json"))
		}
	}()

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve sidecar executable: %w", err)
	}
	grandchild := exec.Command(executable, "-mode", "grandchild", "-state", stateDir)
	grandchild.Env = withoutOwnerToken(os.Environ())
	if err := grandchild.Start(); err != nil {
		return fmt.Errorf("start grandchild: %w", err)
	}
	grandchildPID := grandchild.Process.Pid
	defer func() {
		if grandchild.Process != nil {
			_ = grandchild.Process.Kill()
			_, _ = grandchild.Process.Wait()
		}
	}()

	ready := readyState{
		SchemaVersion: schemaVersion,
		DaemonPID:     os.Getpid(),
		GrandchildPID: grandchildPID,
		OwnerHash:     tokenHash(ownerToken),
		StartedAtUTC:  time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := writeJSON(filepath.Join(stateDir, "ready.json"), ready); err != nil {
		return fmt.Errorf("write ready state: %w", err)
	}
	fmt.Printf("ready|%d|%d\n", ready.DaemonPID, ready.GrandchildPID)

	controlPath := filepath.Join(stateDir, "control.json")
	resultPath := filepath.Join(stateDir, "control-result.json")
	for {
		commandBytes, err := os.ReadFile(controlPath)
		if errors.Is(err, os.ErrNotExist) {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if err != nil {
			return fmt.Errorf("read control command: %w", err)
		}
		_ = os.Remove(controlPath)
		var command controlCommand
		if err := json.Unmarshal(commandBytes, &command); err != nil {
			return fmt.Errorf("decode control command: %w", err)
		}
		result := controlResult{
			SchemaVersion: schemaVersion,
			ID:            command.ID,
			DaemonPID:     os.Getpid(),
			GrandchildPID: grandchildPID,
			CompletedUTC:  time.Now().UTC().Format(time.RFC3339Nano),
		}
		if command.SchemaVersion != schemaVersion || command.Action != "shutdown" {
			result.Reason = "invalid_command"
			if err := writeJSON(resultPath, result); err != nil {
				return err
			}
			continue
		}
		if command.OwnerToken != ownerToken {
			result.Reason = "owner_mismatch"
			if err := writeJSON(resultPath, result); err != nil {
				return err
			}
			continue
		}
		result.Accepted = true
		result.Reason = "owner_shutdown"
		if err := grandchild.Process.Kill(); err != nil {
			return fmt.Errorf("kill grandchild: %w", err)
		}
		if _, err := grandchild.Process.Wait(); err != nil {
			return fmt.Errorf("wait grandchild: %w", err)
		}
		grandchild.Process = nil
		if err := writeJSON(resultPath, result); err != nil {
			return err
		}
		cleanExit = true
		return nil
	}
}

func grandchildMain(stateDir string) error {
	heartbeatPath := filepath.Join(stateDir, "grandchild-heartbeat.json")
	for sequence := 1; ; sequence++ {
		value := map[string]any{
			"schema_version": schemaVersion,
			"pid":            os.Getpid(),
			"sequence":       sequence,
			"updated_at_utc": time.Now().UTC().Format(time.RFC3339Nano),
		}
		if err := writeJSON(heartbeatPath, value); err != nil {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func withoutOwnerToken(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if !strings.HasPrefix(strings.ToUpper(value), "MAD_OWNER_TOKEN=") {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func tokenHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func writeJSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	temporary := fmt.Sprintf("%s.%d.tmp", path, os.Getpid())
	if err := os.WriteFile(temporary, encoded, 0o600); err != nil {
		return err
	}
	_ = os.Remove(path)
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}
