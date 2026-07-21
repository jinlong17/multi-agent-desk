package storage

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

func makeExactSchemaV7Fixture(t *testing.T) (string, string) {
	t.Helper()
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "device")
	path := filepath.Join(root, "device.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_100, 0).UTC()
	deviceID := storageID("device", storageHexA)
	clientID := storageID("client", storageHexA)
	if err := store.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "backup", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: clientID, Name: "owner", PublicKey: bytesOf(1, 32), Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityVaultControl}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO vault_config(
		singleton_id,format_version,kdf_name,kdf_salt,argon_time,argon_memory_kib,
		argon_parallelism,key_check_nonce,key_check_ciphertext,initialized_at,
		initialized_by_device_id,init_request_digest,created_at,updated_at)
		VALUES(1,1,'argon2id-v19',?,3,65536,1,?,?,?, ?,?,?,?)`,
		bytesOf(2, 16), bytesOf(3, 12), bytesOf(4, 49), formatTime(now), clientID,
		strings.Repeat("a", 64), formatTime(now), formatTime(now)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TABLE remote_device_identities; DROP TABLE controlplane_id_mappings;
		DELETE FROM schema_migrations WHERE version=8; PRAGMA user_version=7`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	return path, root
}

func TestSchemaV7OnlineBackupUpgradeAndAtomicRestore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native Windows DACL and atomic replacement run in the Windows gate")
	}
	path, root := makeExactSchemaV7Fixture(t)
	previousVersion := DeviceBinaryVersion
	DeviceBinaryVersion = "0.1.0-p2-backup-test"
	t.Cleanup(func() { DeviceBinaryVersion = previousVersion })
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("%v cause=%v", err, errors.Unwrap(err))
	}
	if version, err := store.SchemaVersion(context.Background()); err != nil || version != 8 {
		t.Fatalf("upgraded schema=%d err=%v", version, err)
	}
	backups, err := filepath.Glob(filepath.Join(root, "backups", "schema-v7", "*-*"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backup directories=%v err=%v", backups, err)
	}
	manifestBytes, err := os.ReadFile(filepath.Join(backups[0], "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest SchemaV7BackupManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != 7 || manifest.BinaryVersion != DeviceBinaryVersion || manifest.Size < 1 || len(manifest.SHA256) != 64 {
		t.Fatalf("manifest=%+v", manifest)
	}
	for _, item := range []struct {
		path string
		mode os.FileMode
	}{
		{backups[0], 0o700},
		{filepath.Join(backups[0], "device.sqlite"), 0o600},
		{filepath.Join(backups[0], "manifest.json"), 0o600},
	} {
		info, err := os.Stat(item.path)
		if err != nil || info.Mode().Perm() != item.mode {
			t.Fatalf("path=%s mode=%v err=%v", item.path, info.Mode().Perm(), err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := RestoreSchemaV7Backup(context.Background(), path, backups[0], DeviceBinaryVersion); err != nil {
		t.Fatal(err)
	}
	if err := verifySQLiteFile(context.Background(), path, 7); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaV7BackupRequiresVaultAndRejectsManifestTamper(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native Windows DACL runs in the Windows gate")
	}
	path, root := makeExactSchemaV7Fixture(t)
	raw, err := Open(context.Background(), path)
	if err != nil {
		// Opening performs the upgrade; this branch is intentionally unreachable
		// and guards fixture drift.
		t.Fatalf("%v cause=%v", err, errors.Unwrap(err))
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	backups, _ := filepath.Glob(filepath.Join(root, "backups", "schema-v7", "*-*"))
	if len(backups) != 1 {
		t.Fatalf("backup directories=%v", backups)
	}
	manifestPath := filepath.Join(backups[0], "manifest.json")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest[len(manifest)-2] ^= 1
	if err := os.WriteFile(manifestPath, manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifySchemaV7Backup(context.Background(), backups[0], DeviceBinaryVersion); domain.CodeOf(err) != domain.CodeSchemaIncompatible {
		t.Fatalf("tampered manifest code=%v err=%v", domain.CodeOf(err), err)
	}
}
