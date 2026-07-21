package controlplane

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	servermigrations "github.com/jinlong17/multi-agent-desk/migrations/server"
	_ "modernc.org/sqlite"
)

func openTestStore(t *testing.T, path string) *Store {
	t.Helper()
	if err := protectPrivateDirectory(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(context.Background(), StoreOptions{Path: path, BusyTimeout: 500 * time.Millisecond, Now: func() time.Time { return time.Unix(1_900_000_000, 0) }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func assertStoreFilesPrivate(t *testing.T, path string) {
	t.Helper()
	if err := verifyPrivateDirectory(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(path + "*")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("database and sidecar files are missing")
	}
	for _, match := range matches {
		if err := verifyPrivateFile(match); err != nil {
			t.Fatalf("%s: %v", filepath.Base(match), err)
		}
	}
}

func TestStoreEmptyRestartPragmasAndIdempotency(t *testing.T) {
	path := filepath.Join(privateTestDirectory(t), "server.sqlite")
	store := openTestStore(t, path)
	var migrationCount, userVersion int
	if err := store.db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&migrationCount); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow("PRAGMA user_version").Scan(&userVersion); err != nil {
		t.Fatal(err)
	}
	if migrationCount != 2 || userVersion != 2 {
		t.Fatalf("migrationCount=%d userVersion=%d", migrationCount, userVersion)
	}
	if err := store.verifyPragmas(context.Background(), 500*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	assertStoreFilesPrivate(t, path)
	var userCount, deviceCount int
	if err := store.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&userCount); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='devices'").Scan(&deviceCount); err != nil {
		t.Fatal(err)
	}
	if userCount != 0 || deviceCount != 0 {
		t.Fatal("P1 activated a user or Device table")
	}
	now := time.Unix(1_900_000_000, 0)
	var scope, request, changed [32]byte
	copy(scope[:], bytes.Repeat([]byte{1}, 32))
	copy(request[:], bytes.Repeat([]byte{2}, 32))
	copy(changed[:], bytes.Repeat([]byte{3}, 32))
	response := IdempotencyResponse{Status: 201, ContentType: "application/json", Body: []byte(`{"ok":true}`)}
	if got, replay, err := store.RememberIdempotentResponse(context.Background(), scope, request, response, now); err != nil || replay || !bytes.Equal(got.Body, response.Body) {
		t.Fatalf("got=%+v replay=%v err=%v", got, replay, err)
	}
	if got, replay, err := store.RememberIdempotentResponse(context.Background(), scope, request, IdempotencyResponse{Status: 500, ContentType: "application/json", Body: []byte(`{}`)}, now); err != nil || !replay || got.Status != 201 {
		t.Fatalf("got=%+v replay=%v err=%v", got, replay, err)
	}
	if _, _, err := store.RememberIdempotentResponse(context.Background(), scope, changed, response, now); err == nil {
		t.Fatal("accepted reused idempotency key with changed digest")
	}
	if got, replay, err := store.RememberIdempotentResponse(context.Background(), scope, changed, response, now.Add(25*time.Hour)); err != nil || replay || !bytes.Equal(got.Body, response.Body) {
		t.Fatalf("expired idempotency entry was not safely replaced: got=%+v replay=%v err=%v", got, replay, err)
	}
}

func TestStoreRestartPreservesFoundationDataWithoutMigrationReplay(t *testing.T) {
	path := filepath.Join(privateTestDirectory(t), "server.sqlite")
	first, err := OpenStore(t.Context(), StoreOptions{Path: path, BusyTimeout: 500 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	var epoch string
	if err := first.db.QueryRow("SELECT schema_epoch FROM server_metadata WHERE singleton=1").Scan(&epoch); err != nil {
		t.Fatal(err)
	}
	if _, err := first.db.Exec("INSERT INTO pre_user_audit_events(id,action,decision,created_at) VALUES(?,?,?,?)", "018f47a0-7b1c-7cc2-8000-000000000001", "foundation_restart", "allowed", "2030-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := OpenStore(t.Context(), StoreOptions{Path: path, BusyTimeout: 500 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	var gotEpoch string
	var auditCount, migrationCount int
	if err := second.db.QueryRow("SELECT schema_epoch FROM server_metadata WHERE singleton=1").Scan(&gotEpoch); err != nil {
		t.Fatal(err)
	}
	if err := second.db.QueryRow("SELECT count(*) FROM pre_user_audit_events").Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if err := second.db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&migrationCount); err != nil {
		t.Fatal(err)
	}
	if gotEpoch != epoch || auditCount != 1 || migrationCount != 2 {
		t.Fatalf("restart changed foundation data: epoch=%q/%q audit=%d migrations=%d", epoch, gotEpoch, auditCount, migrationCount)
	}
}

func TestStoreConcurrentMigrationHasOneCompleteLedger(t *testing.T) {
	path := filepath.Join(privateTestDirectory(t), "server.sqlite")
	start := make(chan struct{})
	results := make(chan error, 2)
	var storesMu sync.Mutex
	var stores []*Store
	for range 2 {
		go func() {
			<-start
			store, err := OpenStore(context.Background(), StoreOptions{Path: path, BusyTimeout: 3 * time.Second})
			if store != nil {
				storesMu.Lock()
				stores = append(stores, store)
				storesMu.Unlock()
			}
			results <- err
		}()
	}
	close(start)
	busyFailures := 0
	for range 2 {
		if err := <-results; err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "locked") && !strings.Contains(strings.ToLower(err.Error()), "busy") {
				t.Fatal(err)
			}
			busyFailures++
		}
	}
	defer func() {
		for _, store := range stores {
			_ = store.Close()
		}
	}()
	if len(stores) == 0 || busyFailures > 1 {
		t.Fatalf("concurrent migration stores=%d busyFailures=%d", len(stores), busyFailures)
	}
	var count int
	if err := stores[0].db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("ledger count=%d", count)
	}
}

func TestStorePriorSchemaBacksUpAndUpgrades(t *testing.T) {
	directory := privateTestDirectory(t)
	path := filepath.Join(directory, "server.sqlite")
	migrations, err := servermigrations.List()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(migrations[0].SQL); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,name TEXT NOT NULL UNIQUE,checksum TEXT NOT NULL,applied_at TEXT NOT NULL) STRICT`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("INSERT INTO schema_migrations VALUES(1,?,?,?)", migrations[0].Name, hex.EncodeToString(migrations[0].Checksum[:]), "prior"); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("PRAGMA user_version=1"); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("INSERT INTO server_metadata VALUES(1,'018f47a0-7b1c-7cc2-8000-000000000001','prior','prior')"); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := protectPrivateFile(path); err != nil {
		t.Fatal(err)
	}
	store := openTestStore(t, path)
	var count int
	if err := store.db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("migration count=%d", count)
	}
	backups, err := filepath.Glob(filepath.Join(directory, "backups", "server-*.sqlite"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups=%v err=%v", backups, err)
	}
	if err := verifyPrivateDirectory(filepath.Dir(backups[0])); err != nil {
		t.Fatal(err)
	}
	if err := verifyPrivateFile(backups[0]); err != nil {
		t.Fatal(err)
	}
	synced, err := os.OpenFile(backups[0], os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open protected backup read-write: %v", err)
	}
	if err := synced.Sync(); err != nil {
		_ = synced.Close()
		t.Fatalf("sync protected backup read-write: %v", err)
	}
	if err := synced.Close(); err != nil {
		t.Fatalf("close protected backup read-write: %v", err)
	}
	if err := verifyPrivateFile(backups[0]); err != nil {
		t.Fatal(err)
	}
	if err := verifySQLiteBackup(t.Context(), backups[0]); err != nil {
		t.Fatal(err)
	}
	restored, err := sql.Open("sqlite", "file:"+backups[0]+"?mode=ro&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var version int
	if err := restored.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 1 {
		t.Fatalf("restored version=%d err=%v", version, err)
	}
	var epoch string
	if err := restored.QueryRow("SELECT schema_epoch FROM server_metadata WHERE singleton=1").Scan(&epoch); err != nil || epoch != "018f47a0-7b1c-7cc2-8000-000000000001" {
		t.Fatalf("restored epoch=%q err=%v", epoch, err)
	}
}

func TestStoreRejectsFuturePartialChecksumCorruptAndBusy(t *testing.T) {
	for _, setup := range []struct {
		name  string
		apply func(*testing.T, string)
	}{
		{"future", func(t *testing.T, path string) {
			raw, _ := sql.Open("sqlite", "file:"+path)
			_, _ = raw.Exec("CREATE TABLE future(x INTEGER); PRAGMA user_version=99")
			_ = raw.Close()
		}},
		{"partial", func(t *testing.T, path string) {
			raw, _ := sql.Open("sqlite", "file:"+path)
			_, _ = raw.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,name TEXT UNIQUE,checksum TEXT,applied_at TEXT); INSERT INTO schema_migrations VALUES(2,'bad',?, 'x'); PRAGMA user_version=2`, bytes.Repeat([]byte{'0'}, 64))
			_ = raw.Close()
		}},
		{"corrupt", func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("not sqlite"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(setup.name, func(t *testing.T) {
			path := filepath.Join(privateTestDirectory(t), "server.sqlite")
			setup.apply(t, path)
			if err := protectPrivateFile(path); err != nil {
				t.Fatal(err)
			}
			if store, err := OpenStore(context.Background(), StoreOptions{Path: path, BusyTimeout: 200 * time.Millisecond}); err == nil {
				_ = store.Close()
				t.Fatal("unsafe database was accepted")
			}
		})
	}
	t.Run("busy", func(t *testing.T) {
		path := filepath.Join(privateTestDirectory(t), "server.sqlite")
		first := openTestStore(t, path)
		conn, err := first.db.Conn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.ExecContext(context.Background(), "BEGIN EXCLUSIVE"); err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = conn.ExecContext(context.Background(), "ROLLBACK"); _ = conn.Close() }()
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancel()
		if store, err := OpenStore(ctx, StoreOptions{Path: path, BusyTimeout: 200 * time.Millisecond}); err == nil {
			_ = store.Close()
			t.Fatal("busy database did not fail within bound")
		}
	})
}
