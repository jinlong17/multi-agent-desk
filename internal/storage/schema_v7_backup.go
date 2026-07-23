package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	moderncsqlite "modernc.org/sqlite"
)

const schemaV7BackupVersion = 7

// DeviceBinaryVersion is embedded in every pre-v8 recovery manifest. Release
// builds replace it with their exact version; tests may pin it explicitly.
var DeviceBinaryVersion = "devel"

type SchemaV7BackupManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	Size          int64  `json:"size"`
	SHA256        string `json:"sha256"`
	CreatedAt     string `json:"createdAt"`
	BinaryVersion string `json:"binaryVersion"`
}

func (s *Store) backupSchemaV7BeforeMigration(ctx context.Context, binaryVersion string, now func() time.Time) error {
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return domain.WrapError(domain.CodeSchemaIncompatible, "schema version could not be read before backup", err)
	}
	if version != schemaV7BackupVersion {
		return nil
	}
	if strings.TrimSpace(binaryVersion) == "" || len(binaryVersion) > 128 || now == nil {
		return domain.NewError(domain.CodeSchemaIncompatible, "backup binary version is invalid")
	}
	var vaultRows int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM vault_config WHERE singleton_id=1").Scan(&vaultRows); err != nil || vaultRows != 1 {
		return domain.NewError(domain.CodeSchemaIncompatible, "schema v7 backup requires an initialized portable Vault")
	}
	// Bring the main file fully current before deriving its path digest. The
	// subsequent exclusive connection prevents a second writer from changing
	// the source between that digest and the online backup snapshot.
	var busy, logPages, checkpointed int
	if err := s.db.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logPages, &checkpointed); err != nil || busy != 0 || logPages != checkpointed {
		return domain.NewError(domain.CodeConflict, "schema v7 backup could not checkpoint an exclusively stopped Device")
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "schema v7 backup could not acquire connection", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "PRAGMA locking_mode=EXCLUSIVE"); err != nil {
		return domain.WrapError(domain.CodeConflict, "schema v7 backup could not request exclusive ownership", err)
	}
	if _, err := conn.ExecContext(ctx, "BEGIN EXCLUSIVE"); err != nil {
		return domain.WrapError(domain.CodeConflict, "schema v7 backup requires the Daemon to be stopped", err)
	}
	// In EXCLUSIVE locking mode, committing this probe retains the connection's
	// exclusive database lock while leaving the connection transaction-free;
	// modernc's online-backup API cannot step a source that is inside a write
	// transaction.
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		return domain.WrapError(domain.CodeConflict, "schema v7 exclusive ownership could not be established", err)
	}

	sourceDigest, _, err := digestFile(s.path)
	if err != nil {
		return domain.WrapError(domain.CodeSchemaIncompatible, "schema v7 source digest failed", err)
	}
	createdAt := now().UTC()
	if createdAt.IsZero() {
		return domain.NewError(domain.CodeSchemaIncompatible, "schema v7 backup time is invalid")
	}
	databaseDirectory := filepath.Dir(s.path)
	if err := verifyDevicePrivateDirectory(databaseDirectory); err != nil {
		return err
	}
	backupBase := filepath.Join(databaseDirectory, "backups")
	if err := ensurePrivateBackupDirectory(backupBase); err != nil {
		return err
	}
	backupRoot := filepath.Join(backupBase, "schema-v7")
	if err := ensurePrivateBackupDirectory(backupRoot); err != nil {
		return err
	}
	directoryName := createdAt.Format("20060102T150405Z") + "-" + hex.EncodeToString(sourceDigest[:6])
	backupDirectory := filepath.Join(backupRoot, directoryName)
	if _, err := os.Lstat(backupDirectory); err == nil {
		return domain.NewError(domain.CodeConflict, "schema v7 backup destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return domain.WrapError(domain.CodeConflict, "schema v7 backup destination cannot be inspected", err)
	}
	if err := os.Mkdir(backupDirectory, 0o700); err != nil {
		return domain.WrapError(domain.CodeConflict, "schema v7 backup directory could not be created", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(backupDirectory)
		}
	}()
	if err := protectDevicePrivateDirectory(backupDirectory); err != nil {
		return err
	}
	backupPath := filepath.Join(backupDirectory, "device.sqlite")
	type onlineBackuper interface {
		NewBackup(string) (*moderncsqlite.Backup, error)
	}
	if err := conn.Raw(func(driverConn any) error {
		backuper, ok := driverConn.(onlineBackuper)
		if !ok {
			return fmt.Errorf("SQLite driver does not expose online backup")
		}
		backup, err := backuper.NewBackup(backupPath)
		if err != nil {
			return err
		}
		for more := true; more; {
			more, err = backup.Step(128)
			if err != nil {
				_ = backup.Finish()
				return err
			}
		}
		return backup.Finish()
	}); err != nil {
		return domain.WrapError(domain.CodeConflict, "schema v7 online backup failed", err)
	}
	if err := protectDevicePrivateFile(backupPath); err != nil {
		return err
	}
	if err := syncFile(backupPath); err != nil {
		return err
	}
	backupDigest, backupSize, err := digestFile(backupPath)
	if err != nil {
		return domain.WrapError(domain.CodeSchemaIncompatible, "schema v7 backup digest failed", err)
	}
	manifest := SchemaV7BackupManifest{
		SchemaVersion: schemaV7BackupVersion,
		Size:          backupSize,
		SHA256:        hex.EncodeToString(backupDigest[:]),
		CreatedAt:     createdAt.Format(time.RFC3339Nano),
		BinaryVersion: binaryVersion,
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "schema v7 manifest could not be encoded", err)
	}
	manifestPath := filepath.Join(backupDirectory, "manifest.json")
	manifestFile, err := os.OpenFile(manifestPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "schema v7 manifest could not be created", err)
	}
	if _, err := manifestFile.Write(manifestBytes); err != nil {
		manifestFile.Close()
		return domain.WrapError(domain.CodeConflict, "schema v7 manifest could not be written", err)
	}
	if err := manifestFile.Sync(); err != nil {
		manifestFile.Close()
		return domain.WrapError(domain.CodeConflict, "schema v7 manifest could not be synced", err)
	}
	if err := manifestFile.Close(); err != nil {
		return domain.WrapError(domain.CodeConflict, "schema v7 manifest could not be closed", err)
	}
	if err := protectDevicePrivateFile(manifestPath); err != nil {
		return err
	}
	if err := syncDirectory(backupDirectory); err != nil {
		return err
	}
	if err := syncDirectory(backupRoot); err != nil {
		return err
	}
	if err := syncDirectory(backupBase); err != nil {
		return err
	}
	if err := syncDirectory(databaseDirectory); err != nil {
		return err
	}
	if err := verifySchemaV7Backup(ctx, backupDirectory, binaryVersion); err != nil {
		return err
	}
	cleanup = false
	return nil
}

// RestoreSchemaV7Backup verifies a complete v7 backup and atomically replaces
// the stopped Device database. The exact prior binary version must be supplied;
// this is recovery, not a down migration.
func RestoreSchemaV7Backup(ctx context.Context, databasePath, backupDirectory, binaryVersion string) error {
	if ctx == nil || !filepath.IsAbs(databasePath) || filepath.Clean(databasePath) != databasePath || !filepath.IsAbs(backupDirectory) || filepath.Clean(backupDirectory) != backupDirectory {
		return domain.NewError(domain.CodeInvalidArgument, "restore paths must be absolute and clean")
	}
	targetDirectory := filepath.Dir(databasePath)
	if err := verifyDevicePrivateDirectory(targetDirectory); err != nil {
		return err
	}
	if err := verifyDevicePrivateDirectory(backupDirectory); err != nil {
		return err
	}
	if err := verifySchemaV7Backup(ctx, backupDirectory, binaryVersion); err != nil {
		return err
	}
	if err := verifyDevicePrivateFile(databasePath); err != nil {
		return err
	}
	if err := rejectRestoreSidecars(databasePath); err != nil {
		return err
	}
	// An exclusive probe rejects a concurrently running Daemon before any
	// replacement bytes are written.
	probeDSN, err := sqliteDSNForOS(runtimeGOOS(), databasePath)
	if err != nil {
		return err
	}
	probe, err := sql.Open("sqlite", probeDSN+"?_pragma=busy_timeout(100)&_pragma=locking_mode(EXCLUSIVE)")
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "restore target could not be opened exclusively", err)
	}
	if _, err := probe.ExecContext(ctx, "BEGIN EXCLUSIVE"); err != nil {
		probe.Close()
		return domain.NewError(domain.CodeConflict, "restore requires the Daemon to be stopped")
	}
	_, _ = probe.ExecContext(ctx, "ROLLBACK")
	if err := probe.Close(); err != nil {
		return domain.WrapError(domain.CodeConflict, "restore exclusive probe could not close", err)
	}
	if err := verifyDevicePrivateFile(databasePath); err != nil {
		return err
	}
	if err := verifyDevicePrivateDirectory(targetDirectory); err != nil {
		return err
	}
	if err := rejectRestoreSidecars(databasePath); err != nil {
		return err
	}

	sourcePath := filepath.Join(backupDirectory, "device.sqlite")
	source, err := os.Open(sourcePath)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "restore backup could not be opened", err)
	}
	defer source.Close()
	temporary, err := os.OpenFile(databasePath+".restore.tmp", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "restore temporary database could not be created", err)
	}
	temporaryPath := temporary.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err := io.Copy(temporary, source); err != nil {
		temporary.Close()
		return domain.WrapError(domain.CodeConflict, "restore database copy failed", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return domain.WrapError(domain.CodeConflict, "restore database sync failed", err)
	}
	if err := temporary.Close(); err != nil {
		return domain.WrapError(domain.CodeConflict, "restore temporary database could not close", err)
	}
	if err := protectDevicePrivateFile(temporaryPath); err != nil {
		return err
	}
	if err := verifySQLiteFile(ctx, temporaryPath, schemaV7BackupVersion); err != nil {
		return err
	}
	if err := verifyDevicePrivateDirectory(targetDirectory); err != nil {
		return err
	}
	if err := verifyDevicePrivateFile(databasePath); err != nil {
		return err
	}
	if err := rejectRestoreSidecars(databasePath); err != nil {
		return err
	}
	if err := replaceDeviceDatabaseFile(temporaryPath, databasePath); err != nil {
		return domain.WrapError(domain.CodeConflict, "restore atomic replacement failed", err)
	}
	cleanup = false
	if err := syncDirectory(targetDirectory); err != nil {
		return err
	}
	if err := verifyDevicePrivateDirectory(targetDirectory); err != nil {
		return err
	}
	if err := protectDevicePrivateFile(databasePath); err != nil {
		return err
	}
	return rejectRestoreSidecars(databasePath)
}

func verifySchemaV7Backup(ctx context.Context, directory, binaryVersion string) error {
	if err := verifyDevicePrivateDirectory(directory); err != nil {
		return err
	}
	manifestPath := filepath.Join(directory, "manifest.json")
	backupPath := filepath.Join(directory, "device.sqlite")
	if err := verifyDevicePrivateFile(manifestPath); err != nil {
		return err
	}
	if err := verifyDevicePrivateFile(backupPath); err != nil {
		return err
	}
	manifestFile, err := os.Open(manifestPath)
	if err != nil {
		return domain.WrapError(domain.CodeSchemaIncompatible, "schema v7 manifest could not be read", err)
	}
	defer manifestFile.Close()
	var manifest SchemaV7BackupManifest
	decoder := json.NewDecoder(io.LimitReader(manifestFile, 4097))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return domain.NewError(domain.CodeSchemaIncompatible, "schema v7 manifest is invalid")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.NewError(domain.CodeSchemaIncompatible, "schema v7 manifest has trailing data")
	}
	if manifest.SchemaVersion != schemaV7BackupVersion || manifest.Size < 1 || len(manifest.SHA256) != 64 || manifest.BinaryVersion != binaryVersion {
		return domain.NewError(domain.CodeSchemaIncompatible, "schema v7 manifest does not match this binary")
	}
	if _, err := time.Parse(time.RFC3339Nano, manifest.CreatedAt); err != nil || !strings.HasSuffix(manifest.CreatedAt, "Z") {
		return domain.NewError(domain.CodeSchemaIncompatible, "schema v7 manifest time is invalid")
	}
	digest, size, err := digestFile(backupPath)
	if err != nil || size != manifest.Size || !strings.EqualFold(hex.EncodeToString(digest[:]), manifest.SHA256) {
		return domain.NewError(domain.CodeSchemaIncompatible, "schema v7 backup digest does not match manifest")
	}
	return verifySQLiteFile(ctx, backupPath, schemaV7BackupVersion)
}

func verifySQLiteFile(ctx context.Context, path string, wantVersion int) error {
	if err := verifyDevicePrivateFile(path); err != nil {
		return err
	}
	dsn, err := sqliteDSNForOS(runtimeGOOS(), path)
	if err != nil {
		return err
	}
	db, err := sql.Open("sqlite", dsn+"?mode=ro&_pragma=foreign_keys(1)")
	if err != nil {
		return domain.WrapError(domain.CodeSchemaIncompatible, "backup could not be opened read-only", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		return domain.NewError(domain.CodeSchemaIncompatible, "backup integrity check failed")
	}
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return domain.WrapError(domain.CodeSchemaIncompatible, "backup foreign-key check failed", err)
	}
	violated := rows.Next()
	if closeErr := rows.Close(); closeErr != nil {
		return domain.WrapError(domain.CodeSchemaIncompatible, "backup foreign-key check could not close", closeErr)
	}
	if violated {
		return domain.NewError(domain.CodeSchemaIncompatible, "backup contains a foreign-key violation")
	}
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil || version != wantVersion {
		return domain.NewError(domain.CodeSchemaIncompatible, "backup schema version is invalid")
	}
	return nil
}

func rejectRestoreSidecars(databasePath string) error {
	for _, suffix := range []string{"-journal", "-shm", "-wal"} {
		path := databasePath + suffix
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return domain.WrapError(domain.CodeConflict, "restore target sidecar could not be inspected", err)
		}
		return domain.NewError(domain.CodeConflict, "restore target contains a SQLite sidecar")
	}
	return nil
}

func digestFile(path string) ([sha256.Size]byte, int64, error) {
	var digest [sha256.Size]byte
	file, err := os.Open(path)
	if err != nil {
		return digest, 0, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return digest, 0, err
	}
	copy(digest[:], hash.Sum(nil))
	return digest, size, nil
}

func syncFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "private file could not be opened for sync", err)
	}
	defer file.Close()
	if err := file.Sync(); err != nil {
		return domain.WrapError(domain.CodeConflict, "private file could not be synced", err)
	}
	return nil
}
