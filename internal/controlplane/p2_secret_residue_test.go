package controlplane

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestP2SecretCanariesAbsentFromClosedDatabaseAndSidecars(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "server.sqlite")
	store, err := OpenStore(context.Background(), StoreOptions{Path: path, BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	canaries := [][]byte{
		[]byte("Bootstrap AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
		[]byte("__Host-mad_session=BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"),
		[]byte(`{"csrfToken":"CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"}`),
		[]byte("MAD-RC1-DEFG-HJKM-NPQR-STVW-XYZ2-3456-789A-BCDE"),
		[]byte(`{"challenge":"webauthn-ceremony-canary"}`),
		[]byte(`{"signingProof":"bootstrap-proof-canary"}`),
	}
	if _, err := store.db.Exec(`CREATE TABLE p2_secret_canary(value BLOB NOT NULL) STRICT`); err != nil {
		t.Fatal(err)
	}
	for _, canary := range canaries {
		if _, err := store.db.Exec(`INSERT INTO p2_secret_canary(value) VALUES(?)`, canary); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.db.Exec(`DELETE FROM p2_secret_canary`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`DROP TABLE p2_secret_canary`); err != nil {
		t.Fatal(err)
	}
	var busy, logPages, checkpointed int
	if err := store.db.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logPages, &checkpointed); err != nil || busy != 0 || logPages != checkpointed {
		t.Fatalf("checkpoint failed: busy=%d pages=%d checkpointed=%d err=%v", busy, logPages, checkpointed, err)
	}
	backup := filepath.Join(root, "server-backup.sqlite")
	if _, err := store.db.Exec(`VACUUM INTO ?`, backup); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	assertNoP2CanaryBytes(t, append(sqliteResiduePaths(path), backup), canaries)
}

func TestP2OneTimeSecretsAbsentFromRetainedLogsAndArtifacts(t *testing.T) {
	root := t.TempDir()
	canary := []byte("MAD-RC1-DEFG-HJKM-NPQR-STVW-XYZ2-3456-789A-BCDE")
	safe := filepath.Join(root, "sanitized-summary.txt")
	if err := os.WriteFile(safe, []byte("tests=2 failures=0 retained_logs=0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	assertNoP2CanaryBytes(t, []string{safe}, [][]byte{canary})
	leaked := filepath.Join(root, "retained-test-output.txt")
	if err := os.WriteFile(leaked, canary, 0o600); err != nil {
		t.Fatal(err)
	}
	err := p2CanaryScanError([]string{leaked}, [][]byte{canary})
	if err == nil || strings.Contains(err.Error(), string(canary)) {
		t.Fatalf("artifact detector result unsafe: detected=%t leaked_value_rendered=%t", err != nil, err != nil && strings.Contains(err.Error(), string(canary)))
	}
}

func sqliteResiduePaths(database string) []string {
	return []string{database, database + "-wal", database + "-shm", database + "-journal"}
}

func assertNoP2CanaryBytes(t *testing.T, paths []string, canaries [][]byte) {
	t.Helper()
	if err := p2CanaryScanError(paths, canaries); err != nil {
		t.Fatal(err)
	}
}

func p2CanaryScanError(paths []string, canaries [][]byte) error {
	for _, path := range paths {
		contents, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read residue file: %w", err)
		}
		for _, canary := range canaries {
			if bytes.Contains(contents, canary) {
				return fmt.Errorf("secret canary detected in retained SQLite or artifact bytes")
			}
		}
	}
	return nil
}
