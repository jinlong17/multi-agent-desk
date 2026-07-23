package storage

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestP2SecretCanariesAbsentFromClosedDeviceDatabaseAndSidecars(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	path := filepath.Join(root, "device.db")
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	canaries := [][]byte{
		[]byte("Bootstrap AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
		[]byte("__Host-mad_session=BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"),
		[]byte(`{"csrfToken":"CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"}`),
		[]byte("MAD-RC1-DEFG-HJKM-NPQR-STVW-XYZ2-3456-789A-BCDE"),
		[]byte(`{"challenge":"webauthn-ceremony-canary"}`),
		[]byte(`{"exchangeProof":"bootstrap-proof-canary"}`),
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
	backup := filepath.Join(filepath.Dir(path), "device-backup.db")
	if _, err := store.db.Exec(`VACUUM INTO ?`, backup); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range append([]string{path, path + "-wal", path + "-shm", path + "-journal"}, backup) {
		contents, err := os.ReadFile(candidate)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, canary := range canaries {
			if bytes.Contains(contents, canary) {
				t.Fatal(fmt.Errorf("secret canary detected in retained Device SQLite bytes"))
			}
		}
	}
}
