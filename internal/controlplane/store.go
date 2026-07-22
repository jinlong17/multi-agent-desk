package controlplane

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	servermigrations "github.com/jinlong17/multi-agent-desk/migrations/server"
	_ "modernc.org/sqlite"
)

const defaultBusyTimeout = 5 * time.Second

type Store struct {
	db   *sql.DB
	path string
}

type StoreOptions struct {
	Path        string
	BusyTimeout time.Duration
	Now         func() time.Time
}

type IdempotencyResponse struct {
	Status      int
	ContentType string
	Body        []byte
}

func (s *Store) RememberIdempotentResponse(ctx context.Context, scopeDigest, requestDigest [32]byte, response IdempotencyResponse, now time.Time) (IdempotencyResponse, bool, error) {
	if response.Status < 100 || response.Status > 599 || response.ContentType != "application/json" || len(response.Body) > 1<<20 {
		return IdempotencyResponse{}, false, fmt.Errorf("idempotency response is invalid")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return IdempotencyResponse{}, false, fmt.Errorf("begin idempotency transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM idempotency_records WHERE scope_digest=? AND expires_at<=?", scopeDigest[:], formatServerTime(now)); err != nil {
		return IdempotencyResponse{}, false, fmt.Errorf("expire idempotency record: %w", err)
	}
	var storedRequest, body []byte
	var status int
	var contentType string
	err = tx.QueryRowContext(ctx, "SELECT request_digest,response_status,response_content_type,response_body FROM idempotency_records WHERE scope_digest=?", scopeDigest[:]).Scan(&storedRequest, &status, &contentType, &body)
	if err == nil {
		if !bytes.Equal(storedRequest, requestDigest[:]) {
			return IdempotencyResponse{}, false, fmt.Errorf("idempotency_key_reused")
		}
		if err := tx.Commit(); err != nil {
			return IdempotencyResponse{}, false, fmt.Errorf("commit idempotency replay: %w", err)
		}
		return IdempotencyResponse{Status: status, ContentType: contentType, Body: append([]byte(nil), body...)}, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return IdempotencyResponse{}, false, fmt.Errorf("read idempotency record: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO idempotency_records(scope_digest,request_digest,response_status,response_content_type,response_body,created_at,expires_at)
        VALUES(?,?,?,?,?,?,?)`, scopeDigest[:], requestDigest[:], response.Status, response.ContentType, response.Body,
		formatServerTime(now), formatServerTime(now.Add(24*time.Hour))); err != nil {
		return IdempotencyResponse{}, false, fmt.Errorf("store idempotency result: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return IdempotencyResponse{}, false, fmt.Errorf("commit idempotency result: %w", err)
	}
	response.Body = append([]byte(nil), response.Body...)
	return response, false, nil
}

func OpenStore(ctx context.Context, options StoreOptions) (*Store, error) {
	if !filepath.IsAbs(options.Path) || filepath.Clean(options.Path) != options.Path {
		return nil, fmt.Errorf("database path must be absolute and clean")
	}
	if options.BusyTimeout == 0 {
		options.BusyTimeout = defaultBusyTimeout
	}
	if options.BusyTimeout < 100*time.Millisecond || options.BusyTimeout > 30*time.Second {
		return nil, fmt.Errorf("database busy timeout must be between 100ms and 30s")
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	directory := filepath.Dir(options.Path)
	directoryExisted := true
	if _, err := os.Lstat(directory); errors.Is(err, os.ErrNotExist) {
		directoryExisted = false
	} else if err != nil {
		return nil, fmt.Errorf("inspect database directory: %w", err)
	}
	if directoryExisted {
		if err := verifyPrivateDirectory(directory); err != nil {
			return nil, err
		}
	} else {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
		if err := protectPrivateDirectory(directory); err != nil {
			return nil, err
		}
	}
	// Always race through O_EXCL instead of first observing the path with
	// Lstat. A concurrent creator necessarily exposes a short interval between
	// creating the file and applying the protected platform ACL. Treating an
	// Lstat hit as a fully initialized pre-existing file made that interval a
	// one-shot permission failure on Windows. An O_EXCL loser waits for the
	// creator's permission transition; a genuinely pre-existing unsafe file
	// never becomes valid and therefore still fails closed at the bounded
	// deadline.
	file, err := os.OpenFile(options.Path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if errors.Is(err, os.ErrExist) {
		if err := waitForPrivateFile(ctx, options.Path, options.BusyTimeout); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, fmt.Errorf("create private database file: %w", err)
	} else {
		if err := file.Close(); err != nil {
			return nil, fmt.Errorf("close private database file: %w", err)
		}
		if err := protectPrivateFile(options.Path); err != nil {
			return nil, err
		}
	}
	// Re-check the named objects immediately before SQLite opens them. On
	// Windows this also rejects junctions and every other reparse-point class,
	// not only the symlinks represented by os.FileMode.
	if err := verifyPrivateDirectory(directory); err != nil {
		return nil, err
	}
	if err := verifyPrivateFile(options.Path); err != nil {
		return nil, err
	}
	preexistingSidecars, err := verifyPrivateDatabaseSidecars(options.Path)
	if err != nil {
		return nil, err
	}
	dsn := "file:" + options.Path + "?_pragma=busy_timeout(" + strconv.FormatInt(options.BusyTimeout.Milliseconds(), 10) + ")&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open server database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db, path: options.Path}
	cleanup := func(err error) (*Store, error) { _ = db.Close(); return nil, err }
	if err := db.PingContext(ctx); err != nil {
		return cleanup(fmt.Errorf("open server database: %w", err))
	}
	if err := verifyPrivateDirectory(directory); err != nil {
		return cleanup(err)
	}
	if err := verifyPrivateFile(options.Path); err != nil {
		return cleanup(err)
	}
	if err := store.protectDatabaseFiles(preexistingSidecars); err != nil {
		return cleanup(err)
	}
	if err := store.backupPriorSchema(ctx, filepath.Join(directory, "backups")); err != nil {
		return cleanup(err)
	}
	if err := store.migrate(ctx, options.Now); err != nil {
		return cleanup(err)
	}
	if err := store.verifyPragmas(ctx, options.BusyTimeout); err != nil {
		return cleanup(err)
	}
	if err := store.protectDatabaseFiles(preexistingSidecars); err != nil {
		return cleanup(err)
	}
	return store, nil
}

func waitForPrivateFile(ctx context.Context, path string, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		if err := verifyPrivateFile(path); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for concurrently created private database file: %w", ctx.Err())
		case <-deadline.C:
			return lastErr
		case <-ticker.C:
		}
	}
}

func (s *Store) backupPriorSchema(ctx context.Context, directory string) error {
	migrations, err := servermigrations.List()
	if err != nil {
		return fmt.Errorf("load server migrations for backup: %w", err)
	}
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read prior schema version: %w", err)
	}
	if version <= 0 || version >= len(migrations) {
		return nil
	}
	_, err = s.Backup(ctx, directory)
	return err
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Ready(ctx context.Context) error {
	var one int
	if err := s.db.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 {
		return fmt.Errorf("database readiness check failed")
	}
	return nil
}

func (s *Store) migrate(ctx context.Context, now func() time.Time) error {
	migrations, err := servermigrations.List()
	if err != nil {
		return fmt.Errorf("load server migrations: %w", err)
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN EXCLUSIVE"); err != nil {
		return fmt.Errorf("acquire exclusive migration ownership: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
        version INTEGER PRIMARY KEY,
        name TEXT NOT NULL UNIQUE,
        checksum TEXT NOT NULL CHECK(length(checksum)=64),
        applied_at TEXT NOT NULL
    ) STRICT`); err != nil {
		return fmt.Errorf("open migration ledger: %w", err)
	}
	rows, err := conn.QueryContext(ctx, "SELECT version,name,checksum FROM schema_migrations ORDER BY version")
	if err != nil {
		return fmt.Errorf("read migration ledger: %w", err)
	}
	var applied int
	for rows.Next() {
		var version int
		var name, checksum string
		if err := rows.Scan(&version, &name, &checksum); err != nil {
			rows.Close()
			return fmt.Errorf("read migration ledger row: %w", err)
		}
		if version != applied+1 || version > len(migrations) {
			rows.Close()
			return fmt.Errorf("schema_incompatible: migration history is partial or from a future binary")
		}
		want := migrations[applied]
		if name != want.Name || !strings.EqualFold(checksum, hex.EncodeToString(want.Checksum[:])) {
			rows.Close()
			return fmt.Errorf("schema_incompatible: migration checksum mismatch")
		}
		applied++
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("read migration ledger: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close migration ledger: %w", err)
	}
	var userVersion int
	if err := conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&userVersion); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if userVersion != applied {
		return fmt.Errorf("schema_incompatible: ledger and user_version disagree")
	}
	for _, migration := range migrations[applied:] {
		if _, err := conn.ExecContext(ctx, migration.SQL); err != nil {
			return fmt.Errorf("apply server migration %s: %w", migration.Name, err)
		}
		if _, err := conn.ExecContext(ctx, "INSERT INTO schema_migrations(version,name,checksum,applied_at) VALUES(?,?,?,?)",
			migration.Version, migration.Name, hex.EncodeToString(migration.Checksum[:]), formatServerTime(now())); err != nil {
			return fmt.Errorf("record server migration %s: %w", migration.Name, err)
		}
		if _, err := conn.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version=%d", migration.Version)); err != nil {
			return fmt.Errorf("record server schema version: %w", err)
		}
	}
	var metadataCount int
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM server_metadata WHERE singleton=1").Scan(&metadataCount); err != nil {
		return fmt.Errorf("read server metadata: %w", err)
	}
	if metadataCount == 0 {
		epoch, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate schema epoch: %w", err)
		}
		stamp := formatServerTime(now())
		if _, err := conn.ExecContext(ctx, "INSERT INTO server_metadata(singleton,schema_epoch,created_at,updated_at) VALUES(1,?,?,?)", epoch.String(), stamp, stamp); err != nil {
			return fmt.Errorf("initialize server metadata: %w", err)
		}
	}
	var violations int
	fkRows, err := conn.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("check migrated foreign keys: %w", err)
	}
	for fkRows.Next() {
		violations++
	}
	if err := fkRows.Err(); err != nil {
		fkRows.Close()
		return fmt.Errorf("check migrated foreign keys: %w", err)
	}
	if err := fkRows.Close(); err != nil {
		return fmt.Errorf("close foreign key check: %w", err)
	}
	if violations != 0 {
		return fmt.Errorf("schema_incompatible: migration produced foreign key violations")
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("commit server migrations: %w", err)
	}
	committed = true
	return nil
}

func (s *Store) Backup(ctx context.Context, directory string) (string, error) {
	if !filepath.IsAbs(directory) {
		return "", fmt.Errorf("backup directory must be absolute")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}
	if err := protectPrivateDirectory(directory); err != nil {
		return "", err
	}
	path := filepath.Join(directory, "server-"+uuid.Must(uuid.NewV7()).String()+".sqlite")
	quoted := "'" + strings.ReplaceAll(path, "'", "''") + "'"
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO "+quoted); err != nil {
		return "", fmt.Errorf("create verified server backup: %w", err)
	}
	if err := protectPrivateFile(path); err != nil {
		return "", err
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return "", fmt.Errorf("open server backup for sync: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return "", fmt.Errorf("sync server backup: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close synced server backup: %w", err)
	}
	if err := verifySQLiteBackup(ctx, path); err != nil {
		return "", err
	}
	return path, nil
}

func verifyPrivateDatabaseSidecars(databasePath string) (map[string]bool, error) {
	result := make(map[string]bool, 3)
	for _, suffix := range []string{"-journal", "-shm", "-wal"} {
		path := databasePath + suffix
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("inspect database sidecar: %w", err)
		}
		if err := verifyPrivateFile(path); err != nil {
			return nil, err
		}
		result[path] = true
	}
	return result, nil
}

func (s *Store) protectDatabaseFiles(preexistingSidecars map[string]bool) error {
	if err := verifyPrivateDirectory(filepath.Dir(s.path)); err != nil {
		return err
	}
	if err := verifyPrivateFile(s.path); err != nil {
		return err
	}
	for _, suffix := range []string{"-journal", "-shm", "-wal"} {
		path := s.path + suffix
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return fmt.Errorf("inspect database sidecar: %w", err)
		}
		if preexistingSidecars[path] {
			if err := verifyPrivateFile(path); err != nil {
				return fmt.Errorf("verify database sidecar: %w", err)
			}
		} else if err := protectPrivateFile(path); err != nil {
			return fmt.Errorf("protect database sidecar: %w", err)
		}
	}
	return nil
}

func verifySQLiteBackup(ctx context.Context, path string) error {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=foreign_keys(1)")
	if err != nil {
		return fmt.Errorf("open server backup for verification: %w", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return fmt.Errorf("verify server backup integrity: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("verify server backup integrity: %s", integrity)
	}
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("verify server backup foreign keys: %w", err)
	}
	if rows.Next() {
		rows.Close()
		return fmt.Errorf("verify server backup foreign keys: violation found")
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("verify server backup foreign keys: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close server backup foreign key check: %w", err)
	}
	return nil
}

func (s *Store) verifyPragmas(ctx context.Context, busy time.Duration) error {
	checks := []struct{ query, want string }{
		{"PRAGMA journal_mode", "wal"}, {"PRAGMA foreign_keys", "1"}, {"PRAGMA busy_timeout", strconv.FormatInt(busy.Milliseconds(), 10)},
	}
	for _, check := range checks {
		var got string
		if err := s.db.QueryRowContext(ctx, check.query).Scan(&got); err != nil || !strings.EqualFold(got, check.want) {
			return fmt.Errorf("database safety pragma could not be verified")
		}
	}
	return nil
}
