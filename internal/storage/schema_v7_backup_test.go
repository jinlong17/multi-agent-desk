package storage

import (
	"bytes"
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
		{filepath.Join(root, "backups"), 0o700},
		{filepath.Join(root, "backups", "schema-v7"), 0o700},
		{backups[0], 0o700},
		{filepath.Join(backups[0], "device.sqlite"), 0o600},
		{filepath.Join(backups[0], "manifest.json"), 0o600},
	} {
		if runtime.GOOS == "windows" {
			info, err := os.Stat(item.path)
			if err != nil {
				t.Fatalf("path=%s err=%v", item.path, err)
			}
			if info.IsDir() {
				err = verifyDevicePrivateDirectory(item.path)
			} else {
				err = verifyDevicePrivateFile(item.path)
			}
			if err != nil {
				t.Fatalf("path=%s err=%v", item.path, err)
			}
		} else {
			info, err := os.Stat(item.path)
			if err != nil || info.Mode().Perm() != item.mode {
				t.Fatalf("path=%s mode=%v err=%v", item.path, info.Mode().Perm(), err)
			}
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

func schemaV7RestoreFixture(t *testing.T) (string, string, string) {
	t.Helper()
	path, root := makeExactSchemaV7Fixture(t)
	previousVersion := DeviceBinaryVersion
	DeviceBinaryVersion = "0.1.0-p2-restore-fixture"
	t.Cleanup(func() { DeviceBinaryVersion = previousVersion })
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(filepath.Join(root, "backups", "schema-v7", "*-*"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups=%v err=%v", backups, err)
	}
	return path, root, backups[0]
}

func TestSchemaV7RestoreRejectsEveryPreexistingSidecarWithoutMutation(t *testing.T) {
	for _, suffix := range []string{"-journal", "-shm", "-wal"} {
		t.Run(suffix, func(t *testing.T) {
			path, _, backup := schemaV7RestoreFixture(t)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			sidecar := path + suffix
			contents := []byte("stale-sidecar-must-not-replay")
			if err := os.WriteFile(sidecar, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := protectDevicePrivateFile(sidecar); err != nil {
				t.Fatal(err)
			}
			if err := RestoreSchemaV7Backup(context.Background(), path, backup, DeviceBinaryVersion); domain.CodeOf(err) != domain.CodeConflict {
				t.Fatalf("code=%s err=%v", domain.CodeOf(err), err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(after, before) {
				t.Fatalf("database changed err=%v", err)
			}
			got, err := os.ReadFile(sidecar)
			if err != nil || !bytes.Equal(got, contents) {
				t.Fatalf("sidecar changed got=%q err=%v", got, err)
			}
		})
	}
}

func TestSchemaV7RestoreRejectsUnsafeTargetParentWithoutMutation(t *testing.T) {
	path, root, backup := schemaV7RestoreFixture(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := makeExistingDeviceDirectoryUnsafeForTest(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = protectDevicePrivateDirectory(root) })
	if err := RestoreSchemaV7Backup(context.Background(), path, backup, DeviceBinaryVersion); domain.CodeOf(err) != domain.CodePermissionDenied {
		t.Fatalf("code=%s err=%v", domain.CodeOf(err), err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("database changed err=%v", err)
	}
}

func TestSchemaV7BackupRejectsUnsafeIntermediateDirectory(t *testing.T) {
	for _, unsafeLevel := range []string{"backups", "schema-v7"} {
		t.Run(unsafeLevel, func(t *testing.T) {
			path, root := makeExactSchemaV7Fixture(t)
			base := filepath.Join(root, "backups")
			if err := os.Mkdir(base, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := protectDevicePrivateDirectory(base); err != nil {
				t.Fatal(err)
			}
			target := base
			if unsafeLevel == "schema-v7" {
				target = filepath.Join(base, "schema-v7")
				if err := os.Mkdir(target, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := protectDevicePrivateDirectory(target); err != nil {
					t.Fatal(err)
				}
			}
			if err := makeExistingDeviceDirectoryUnsafeForTest(target); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = protectDevicePrivateDirectory(target) })
			if store, err := Open(context.Background(), path); err == nil {
				_ = store.Close()
				t.Fatal("unsafe backup path was accepted")
			} else if domain.CodeOf(err) != domain.CodePermissionDenied {
				t.Fatalf("code=%s err=%v", domain.CodeOf(err), err)
			}
		})
	}
}

func TestSchemaV7BackupRequiresVaultAndRejectsManifestTamper(t *testing.T) {
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
