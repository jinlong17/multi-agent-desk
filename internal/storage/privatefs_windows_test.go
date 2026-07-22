//go:build windows

package storage

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"golang.org/x/sys/windows"
)

func makeDeviceWindowsPathUnprotected(path string) error {
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.UNPROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil)
}

func makeDeviceWindowsPathExtraACE(path string) error {
	owner, err := backupCurrentUserSID()
	if err != nil {
		return err
	}
	return setDeviceWindowsDACL(path, "(A;;FA;;;SY)(A;;FA;;;"+owner.String()+")(A;;FR;;;WD)")
}

func makeExistingDeviceDirectoryUnsafeForTest(path string) error {
	return makeDeviceWindowsPathExtraACE(path)
}

func setDeviceWindowsDACL(path, aces string) error {
	owner, err := backupCurrentUserSID()
	if err != nil {
		return err
	}
	descriptor, err := windows.SecurityDescriptorFromString("O:" + owner.String() + "D:P" + aces)
	if err != nil {
		return err
	}
	descriptorOwner, _, err := descriptor.Owner()
	if err != nil {
		return err
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		descriptorOwner, nil, dacl, nil)
}

func deviceWindowsSecurityDescriptorText(path string) (string, error) {
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return "", err
	}
	text := descriptor.String()
	if text == "" {
		return "", windows.ERROR_INVALID_SECURITY_DESCR
	}
	return text, nil
}

func windowsDirectoryNames(t *testing.T, path string) []string {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry.Name())
	}
	slices.Sort(result)
	return result
}

func TestWindowsDeviceStoreUsesExactCurrentLogonAndSystemDACL(t *testing.T) {
	if backupFileAllAccess != windows.ACCESS_MASK(0x001f01ff) {
		t.Fatalf("FILE_ALL_ACCESS mask=%#x", backupFileAllAccess)
	}
	root := filepath.Join(t.TempDir(), "device-private")
	path := filepath.Join(root, "device.db")
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := verifyDevicePrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	if err := verifyDevicePrivateFile(path); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"-journal", "-shm", "-wal"} {
		sidecar := path + suffix
		if _, err := os.Lstat(sidecar); err == nil {
			if err := verifyDevicePrivateFile(sidecar); err != nil {
				t.Fatalf("sidecar %s: %v", suffix, err)
			}
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
}

func TestWindowsDeviceStoreRejectsUnsafeSidecarBeforeSQLiteOpen(t *testing.T) {
	root := filepath.Join(t.TempDir(), "device-private")
	path := filepath.Join(root, "device.db")
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	sidecar := path + "-journal"
	want := []byte("must-not-be-opened-or-modified")
	if err := os.WriteFile(sidecar, want, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := protectDevicePrivateFile(sidecar); err != nil {
		t.Fatal(err)
	}
	if err := makeDeviceWindowsPathUnprotected(sidecar); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = protectDevicePrivateFile(sidecar) })
	beforeNames := windowsDirectoryNames(t, root)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if reopened, err := Open(ctx, path); err == nil {
		_ = reopened.Close()
		t.Fatal("unsafe preexisting SQLite sidecar was accepted")
	} else if domain.CodeOf(err) != domain.CodePermissionDenied {
		t.Fatalf("code=%s err=%v", domain.CodeOf(err), err)
	}
	got, err := os.ReadFile(sidecar)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("sidecar changed got=%q err=%v", got, err)
	}
	if afterNames := windowsDirectoryNames(t, root); !slices.Equal(afterNames, beforeNames) {
		t.Fatalf("directory changed before=%v after=%v", beforeNames, afterNames)
	}
}

func TestWindowsBackupRootRejectsPreexistingExtraACE(t *testing.T) {
	root := filepath.Join(t.TempDir(), "backup-private")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := protectDevicePrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	if err := makeDeviceWindowsPathExtraACE(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = protectDevicePrivateDirectory(root) })
	if err := ensurePrivateBackupDirectory(root); domain.CodeOf(err) != domain.CodePermissionDenied {
		t.Fatalf("code=%s err=%v", domain.CodeOf(err), err)
	}
}

func TestWindowsSyncDirectoryFlushesWithWritableHandle(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sync-private")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := protectDevicePrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	if err := syncDirectory(root); err != nil {
		t.Fatalf("flush protected directory: %v", err)
	}
}

func TestWindowsPrivateDACLNegativeMatrixAndReverseOrder(t *testing.T) {
	root := filepath.Join(t.TempDir(), "acl-matrix")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := protectDevicePrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "fixture")
	if err := os.WriteFile(path, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := protectDevicePrivateFile(path); err != nil {
		t.Fatal(err)
	}
	owner, err := backupCurrentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	ownerText := owner.String()
	for _, test := range []struct {
		name string
		aces string
	}{
		{"missing-system", "(A;;FA;;;" + ownerText + ")"},
		{"extra-principal", "(A;;FA;;;SY)(A;;FA;;;" + ownerText + ")(A;;FR;;;WD)"},
		{"wrong-mask", "(A;;FR;;;SY)(A;;FA;;;" + ownerText + ")"},
		{"duplicate", "(A;;FA;;;" + ownerText + ")(A;;FA;;;" + ownerText + ")"},
		{"deny", "(D;;FR;;;SY)(A;;FA;;;" + ownerText + ")"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := setDeviceWindowsDACL(path, test.aces); err != nil {
				t.Fatal(err)
			}
			if err := verifyDevicePrivateFile(path); err == nil {
				t.Fatal("unsafe DACL was accepted")
			}
			if err := protectDevicePrivateFile(path); err != nil {
				t.Fatal(err)
			}
		})
	}
	if err := setDeviceWindowsDACL(path, "(A;;FA;;;"+ownerText+")(A;;FA;;;SY)"); err != nil {
		t.Fatal(err)
	}
	if err := verifyDevicePrivateFile(path); err != nil {
		t.Fatalf("reverse-order exact DACL was rejected: %v", err)
	}
}

func TestWindowsPrivateDACLRejectsRawNonzeroACEFlags(t *testing.T) {
	owner, err := backupCurrentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	system, err := backupSystemSID()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		aces string
	}{
		{"object-inherit", "(A;OI;FA;;;SY)(A;;FA;;;" + owner.String() + ")"},
		{"inherited", "(A;ID;FA;;;SY)(A;;FA;;;" + owner.String() + ")"},
	} {
		t.Run(test.name, func(t *testing.T) {
			descriptor, err := windows.SecurityDescriptorFromString("D:P" + test.aces)
			if err != nil {
				t.Fatal(err)
			}
			dacl, _, err := descriptor.DACL()
			if err != nil {
				t.Fatal(err)
			}
			var first *windows.ACCESS_ALLOWED_ACE
			if err := windows.GetAce(dacl, 0, &first); err != nil || first == nil {
				t.Fatalf("read raw flag fixture: %v", err)
			}
			if first.Header.AceFlags == 0 {
				t.Fatal("raw flag fixture was normalized before verification")
			}
			if err := verifyDeviceWindowsDACL(dacl, owner, system); err == nil {
				t.Fatal("raw DACL with nonzero ACE flags was accepted")
			}
		})
	}
}

func TestWindowsPrivateFileAndDirectoryReparsePointsAreRejected(t *testing.T) {
	root := filepath.Join(t.TempDir(), "reparse-matrix")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := protectDevicePrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	targetFile := filepath.Join(root, "target-file")
	if err := os.WriteFile(targetFile, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := protectDevicePrivateFile(targetFile); err != nil {
		t.Fatal(err)
	}
	fileLink := filepath.Join(root, "file-link")
	if err := os.Symlink(targetFile, fileLink); err != nil {
		t.Fatalf("create file reparse fixture: %v", err)
	}
	if err := verifyDevicePrivateFile(fileLink); err == nil {
		t.Fatal("file reparse point was accepted")
	}

	targetDirectory := filepath.Join(root, "target-directory")
	if err := os.Mkdir(targetDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := protectDevicePrivateDirectory(targetDirectory); err != nil {
		t.Fatal(err)
	}
	directoryLink := filepath.Join(root, "directory-link")
	if err := os.Symlink(targetDirectory, directoryLink); err != nil {
		t.Fatalf("create directory reparse fixture: %v", err)
	}
	if err := verifyDevicePrivateDirectory(directoryLink); err == nil {
		t.Fatal("directory reparse point was accepted")
	}
}

func windowsSchemaV7RestoreFixture(t *testing.T) (string, string, string) {
	t.Helper()
	path, root := makeExactSchemaV7Fixture(t)
	previousVersion := DeviceBinaryVersion
	DeviceBinaryVersion = "0.1.0-windows-restore-test"
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

func TestWindowsRestoreRejectsUnsafeOrReparseTargetWithoutMutation(t *testing.T) {
	t.Run("unsafe-main", func(t *testing.T) {
		path, root, backup := windowsSchemaV7RestoreFixture(t)
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := makeDeviceWindowsPathUnprotected(path); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = protectDevicePrivateFile(path) })
		beforeNames := windowsDirectoryNames(t, root)
		if err := RestoreSchemaV7Backup(context.Background(), path, backup, DeviceBinaryVersion); domain.CodeOf(err) != domain.CodePermissionDenied {
			t.Fatalf("code=%s err=%v", domain.CodeOf(err), err)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(after, before) {
			t.Fatalf("target changed err=%v", err)
		}
		if afterNames := windowsDirectoryNames(t, root); !slices.Equal(afterNames, beforeNames) {
			t.Fatalf("directory changed before=%v after=%v", beforeNames, afterNames)
		}
	})

	t.Run("preexisting-sidecar", func(t *testing.T) {
		path, root, backup := windowsSchemaV7RestoreFixture(t)
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sidecar := path + "-journal"
		sidecarBytes := []byte("unsafe-restore-sidecar")
		if err := os.WriteFile(sidecar, sidecarBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := protectDevicePrivateFile(sidecar); err != nil {
			t.Fatal(err)
		}
		if err := makeDeviceWindowsPathExtraACE(sidecar); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = protectDevicePrivateFile(sidecar) })
		beforeNames := windowsDirectoryNames(t, root)
		if err := RestoreSchemaV7Backup(context.Background(), path, backup, DeviceBinaryVersion); domain.CodeOf(err) != domain.CodeConflict {
			t.Fatalf("code=%s err=%v", domain.CodeOf(err), err)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(after, before) {
			t.Fatalf("target changed err=%v", err)
		}
		gotSidecar, err := os.ReadFile(sidecar)
		if err != nil || !bytes.Equal(gotSidecar, sidecarBytes) {
			t.Fatalf("sidecar changed got=%q err=%v", gotSidecar, err)
		}
		if afterNames := windowsDirectoryNames(t, root); !slices.Equal(afterNames, beforeNames) {
			t.Fatalf("directory changed before=%v after=%v", beforeNames, afterNames)
		}
	})

	t.Run("reparse-parent", func(t *testing.T) {
		path, root, backup := windowsSchemaV7RestoreFixture(t)
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		parentLink := filepath.Join(filepath.Dir(root), "device-parent-link")
		if err := os.Symlink(root, parentLink); err != nil {
			t.Fatalf("create restore parent reparse fixture: %v", err)
		}
		linkedPath := filepath.Join(parentLink, filepath.Base(path))
		if err := RestoreSchemaV7Backup(context.Background(), linkedPath, backup, DeviceBinaryVersion); domain.CodeOf(err) != domain.CodePermissionDenied {
			t.Fatalf("code=%s err=%v", domain.CodeOf(err), err)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(after, before) {
			t.Fatalf("database changed err=%v", err)
		}
	})

	t.Run("wrong-kind-parent", func(t *testing.T) {
		_, root, backup := windowsSchemaV7RestoreFixture(t)
		parentFile := filepath.Join(filepath.Dir(root), "not-a-directory")
		if err := os.WriteFile(parentFile, []byte("private"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := protectDevicePrivateFile(parentFile); err != nil {
			t.Fatal(err)
		}
		if err := RestoreSchemaV7Backup(context.Background(), filepath.Join(parentFile, "device.db"), backup, DeviceBinaryVersion); domain.CodeOf(err) != domain.CodePermissionDenied {
			t.Fatalf("code=%s err=%v", domain.CodeOf(err), err)
		}
	})

	t.Run("reparse-main", func(t *testing.T) {
		path, root, backup := windowsSchemaV7RestoreFixture(t)
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		link := path + ".link"
		if err := os.Symlink(path, link); err != nil {
			t.Fatalf("create restore reparse fixture: %v", err)
		}
		beforeNames := windowsDirectoryNames(t, root)
		if err := RestoreSchemaV7Backup(context.Background(), link, backup, DeviceBinaryVersion); domain.CodeOf(err) != domain.CodePermissionDenied {
			t.Fatalf("code=%s err=%v", domain.CodeOf(err), err)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(after, before) {
			t.Fatalf("target changed err=%v", err)
		}
		if afterNames := windowsDirectoryNames(t, root); !slices.Equal(afterNames, beforeNames) {
			t.Fatalf("directory changed before=%v after=%v", beforeNames, afterNames)
		}
	})
}

func TestWindowsVerifySQLiteFileRejectsUnsafeFileBeforeOpen(t *testing.T) {
	path, _, backup := windowsSchemaV7RestoreFixture(t)
	_ = path
	backupPath := filepath.Join(backup, "device.sqlite")
	if err := makeDeviceWindowsPathUnprotected(backupPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = protectDevicePrivateFile(backupPath) })
	if err := verifySQLiteFile(context.Background(), backupPath, schemaV7BackupVersion); domain.CodeOf(err) != domain.CodePermissionDenied {
		t.Fatalf("code=%s err=%v", domain.CodeOf(err), err)
	}
}

func TestWindowsDeviceStoreRejectsUnprotectedPreexistingFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "device-private")
	path := filepath.Join(root, "device.db")
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := makeDeviceWindowsPathUnprotected(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = protectDevicePrivateFile(path) })
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	beforeSecurity, err := deviceWindowsSecurityDescriptorText(path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened, err := Open(context.Background(), path); err == nil {
		_ = reopened.Close()
		t.Fatal("unprotected database DACL was accepted")
	} else if domain.CodeOf(err) != domain.CodePermissionDenied {
		t.Fatalf("code=%s err=%v", domain.CodeOf(err), err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("unsafe database content changed err=%v", err)
	}
	afterSecurity, err := deviceWindowsSecurityDescriptorText(path)
	if err != nil || afterSecurity != beforeSecurity {
		t.Fatalf("unsafe database DACL changed before=%q after=%q err=%v", beforeSecurity, afterSecurity, err)
	}
	if err := verifyDevicePrivateFile(path); domain.CodeOf(err) != domain.CodePermissionDenied {
		t.Fatalf("unsafe database was repaired during rejection: code=%s err=%v", domain.CodeOf(err), err)
	}
}

func TestWindowsDevicePrivatePathKindIsExact(t *testing.T) {
	root := filepath.Join(t.TempDir(), "device-private")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := protectDevicePrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "fixture")
	if err := os.WriteFile(path, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := protectDevicePrivateFile(path); err != nil {
		t.Fatal(err)
	}
	if err := verifyDevicePrivateFile(root); err == nil {
		t.Fatal("directory was accepted as a private file")
	}
	if err := verifyDevicePrivateDirectory(path); err == nil {
		t.Fatal("file was accepted as a private directory")
	}
}
