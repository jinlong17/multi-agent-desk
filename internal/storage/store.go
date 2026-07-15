package storage

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	devicemigrations "github.com/jinlong17/multi-agent-desk/migrations/device"
	_ "modernc.org/sqlite"
)

const (
	DefaultBusyTimeout = 5 * time.Second
	databaseFileMode   = 0o600
	databaseDirMode    = 0o700
)

type Pragmas struct {
	JournalMode string
	ForeignKeys bool
	BusyTimeout time.Duration
}

// Store owns the single Device SQLite connection pool. MaxOpenConns is one so
// connection-local foreign-key and busy-timeout settings apply to every query
// and one process remains the only writer.
type Store struct {
	db   *sql.DB
	path string
}

// Open creates or opens the Device database, verifies its settings, and
// applies every ordered embedded migration.
func Open(ctx context.Context, path string) (*Store, error) {
	if ctx == nil || path == "" {
		return nil, domain.NewError(domain.CodeInvalidArgument, "database path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, domain.WrapError(domain.CodeInvalidArgument, "database path is invalid", err)
	}
	if err := ensurePrivateDirectory(filepath.Dir(absolute)); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(absolute); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, domain.NewError(domain.CodeConflict, "database path is not a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, domain.WrapError(domain.CodeConflict, "database path cannot be inspected", err)
	}

	dsn, err := sqliteDSNForOS(runtime.GOOS, absolute)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "database open failed", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	store := &Store{db: db, path: absolute}
	ok := false
	defer func() {
		if !ok {
			_ = db.Close()
		}
	}()
	if err := db.PingContext(ctx); err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "database open failed", err)
	}
	if err := os.Chmod(absolute, databaseFileMode); err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "database permissions could not be restricted", err)
	}
	if err := store.configure(ctx); err != nil {
		return nil, err
	}
	if err := store.migrate(ctx); err != nil {
		return nil, err
	}
	ok = true
	return store, nil
}

func sqliteDSNForOS(goos, absolute string) (string, error) {
	if absolute == "" {
		return "", domain.NewError(domain.CodeInvalidArgument, "database path is required")
	}
	if goos != "windows" {
		if !strings.HasPrefix(absolute, "/") {
			return "", domain.NewError(domain.CodeInvalidArgument, "database path must be absolute")
		}
		return (&url.URL{Scheme: "file", Path: absolute}).String(), nil
	}

	normalized := strings.ReplaceAll(absolute, `\`, "/")
	if strings.HasPrefix(normalized, "//") {
		remainder := strings.TrimPrefix(normalized, "//")
		separator := strings.IndexByte(remainder, '/')
		if separator <= 0 || separator == len(remainder)-1 {
			return "", domain.NewError(domain.CodeInvalidArgument, "database UNC path is invalid")
		}
		return (&url.URL{
			Scheme: "file",
			Host:   remainder[:separator],
			Path:   remainder[separator:],
		}).String(), nil
	}
	if len(normalized) < 3 || normalized[1] != ':' || normalized[2] != '/' ||
		((normalized[0] < 'A' || normalized[0] > 'Z') && (normalized[0] < 'a' || normalized[0] > 'z')) {
		return "", domain.NewError(domain.CodeInvalidArgument, "database Windows path must be drive-rooted or UNC")
	}
	return (&url.URL{Scheme: "file", Path: "/" + normalized}).String(), nil
}

func ensurePrivateDirectory(path string) error {
	if err := os.MkdirAll(path, databaseDirMode); err != nil {
		return domain.WrapError(domain.CodeConflict, "database directory could not be created", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "database directory cannot be inspected", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return domain.NewError(domain.CodeConflict, "database directory is not private")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return domain.NewError(domain.CodePermissionDenied, "database directory permissions are too broad")
	}
	return nil
}

func (s *Store) configure(ctx context.Context) error {
	var mode string
	if err := s.db.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&mode); err != nil {
		return domain.WrapError(domain.CodeConflict, "database journal configuration failed", err)
	}
	if !strings.EqualFold(mode, "wal") {
		return domain.NewError(domain.CodeConflict, "database did not enter WAL mode")
	}
	if _, err := s.db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		return domain.WrapError(domain.CodeConflict, "database foreign-key configuration failed", err)
	}
	milliseconds := DefaultBusyTimeout.Milliseconds()
	if _, err := s.db.ExecContext(ctx, "PRAGMA busy_timeout="+strconv.FormatInt(milliseconds, 10)); err != nil {
		return domain.WrapError(domain.CodeConflict, "database busy-timeout configuration failed", err)
	}
	pragmas, err := s.Pragmas(ctx)
	if err != nil {
		return err
	}
	if pragmas.JournalMode != "wal" || !pragmas.ForeignKeys || pragmas.BusyTimeout != DefaultBusyTimeout {
		return domain.NewError(domain.CodeConflict, "database pragma verification failed")
	}
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	migrations, err := devicemigrations.List()
	if err != nil {
		return domain.WrapError(domain.CodeSchemaIncompatible, "embedded migrations are invalid", err)
	}
	current := migrations[len(migrations)-1].Version
	var userVersion int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&userVersion); err != nil {
		return domain.WrapError(domain.CodeSchemaIncompatible, "schema version could not be read", err)
	}
	if userVersion > current {
		return domain.NewError(domain.CodeSchemaIncompatible, "database schema is newer than this binary")
	}

	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY CHECK (version >= 1),
			name TEXT NOT NULL UNIQUE,
			checksum TEXT NOT NULL CHECK (length(checksum) = 64),
			applied_at TEXT NOT NULL
		)`); err != nil {
		return domain.WrapError(domain.CodeSchemaIncompatible, "migration ledger could not be opened", err)
	}

	type appliedMigration struct {
		version  int
		name     string
		checksum string
	}
	rows, err := s.db.QueryContext(ctx, "SELECT version, name, checksum FROM schema_migrations ORDER BY version")
	if err != nil {
		return domain.WrapError(domain.CodeSchemaIncompatible, "migration ledger could not be read", err)
	}
	applied := make([]appliedMigration, 0, current)
	for rows.Next() {
		var item appliedMigration
		if err := rows.Scan(&item.version, &item.name, &item.checksum); err != nil {
			_ = rows.Close()
			return domain.WrapError(domain.CodeSchemaIncompatible, "migration ledger is invalid", err)
		}
		applied = append(applied, item)
	}
	if err := rows.Close(); err != nil {
		return domain.WrapError(domain.CodeSchemaIncompatible, "migration ledger could not be closed", err)
	}
	if len(applied) > current {
		return domain.NewError(domain.CodeSchemaIncompatible, "database contains unknown migrations")
	}
	for index, item := range applied {
		want := migrations[index]
		if item.version != want.Version || item.name != want.Name || item.checksum != hex.EncodeToString(want.Checksum[:]) {
			return domain.NewError(domain.CodeSchemaIncompatible, "database migration history does not match this binary")
		}
	}
	if userVersion != 0 && userVersion != len(applied) {
		return domain.NewError(domain.CodeSchemaIncompatible, "database schema version is inconsistent")
	}

	for _, migration := range migrations[len(applied):] {
		if err := s.applyMigration(ctx, migration); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) applyMigration(ctx context.Context, migration devicemigrations.Migration) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "migration transaction could not start", err)
	}
	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		_ = tx.Rollback()
		return domain.WrapError(domain.CodeSchemaIncompatible, "database migration failed", err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations(version, name, checksum, applied_at) VALUES(?, ?, ?, ?)",
		migration.Version, migration.Name, hex.EncodeToString(migration.Checksum[:]), formatTime(time.Now()),
	); err != nil {
		_ = tx.Rollback()
		return domain.WrapError(domain.CodeSchemaIncompatible, "migration ledger update failed", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version=%d", migration.Version)); err != nil {
		_ = tx.Rollback()
		return domain.WrapError(domain.CodeSchemaIncompatible, "schema version update failed", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.WrapError(domain.CodeSchemaIncompatible, "database migration commit failed", err)
	}
	return nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *Store) Pragmas(ctx context.Context) (Pragmas, error) {
	var result Pragmas
	var foreignKeys int
	var busyMilliseconds int64
	if err := s.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&result.JournalMode); err != nil {
		return Pragmas{}, domain.WrapError(domain.CodeConflict, "database journal mode could not be read", err)
	}
	result.JournalMode = strings.ToLower(result.JournalMode)
	if err := s.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		return Pragmas{}, domain.WrapError(domain.CodeConflict, "database foreign-key mode could not be read", err)
	}
	if err := s.db.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyMilliseconds); err != nil {
		return Pragmas{}, domain.WrapError(domain.CodeConflict, "database busy timeout could not be read", err)
	}
	result.ForeignKeys = foreignKeys == 1
	result.BusyTimeout = time.Duration(busyMilliseconds) * time.Millisecond
	return result, nil
}

func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, domain.WrapError(domain.CodeSchemaIncompatible, "schema version could not be read", err)
	}
	return version, nil
}

func (s *Store) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "database transaction could not start", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return domain.WrapError(domain.CodeConflict, "database transaction could not commit", err)
	}
	return nil
}

func writeError(message string, err error) error {
	if err == nil {
		return nil
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "constraint") || strings.Contains(text, "unique") {
		return domain.WrapError(domain.CodeConflict, message, err)
	}
	return domain.WrapError(domain.CodeConflict, message, err)
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, domain.WrapError(domain.CodeSchemaIncompatible, "stored timestamp is invalid", err)
	}
	return parsed, nil
}

func parseOptionalTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := parseTime(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
