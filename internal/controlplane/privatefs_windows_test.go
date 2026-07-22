//go:build windows

package controlplane

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func makeTestFileUnsafe(path string) error {
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION)
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

func setControlPlaneWindowsDACL(path, aces string) error {
	owner, err := currentUserSID()
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

func TestWindowsPrivateFixturesUseExactCurrentLogonAndSystemDACL(t *testing.T) {
	if fileAllAccess != windows.ACCESS_MASK(0x001f01ff) {
		t.Fatalf("FILE_ALL_ACCESS mask=%#x", fileAllAccess)
	}
	directory := privateTestDirectory(t)
	if err := verifyPrivateDirectory(directory); err != nil {
		t.Fatal(err)
	}
	path := privateTestFile(t, directory, "fixture", []byte("private"))
	if err := verifyPrivateFile(path); err != nil {
		t.Fatal(err)
	}
}

func controlPlaneWindowsDirectoryNames(t *testing.T, path string) []string {
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

func TestWindowsServerStoreRejectsUnsafeSidecarBeforeSQLiteOpen(t *testing.T) {
	directory := privateTestDirectory(t)
	path := filepath.Join(directory, "server.sqlite")
	store, err := OpenStore(context.Background(), StoreOptions{Path: path, BusyTimeout: 100 * time.Millisecond})
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
	if err := protectPrivateFile(sidecar); err != nil {
		t.Fatal(err)
	}
	if err := makeTestFileUnsafe(sidecar); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = protectPrivateFile(sidecar) })
	beforeNames := controlPlaneWindowsDirectoryNames(t, directory)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if reopened, err := OpenStore(ctx, StoreOptions{Path: path, BusyTimeout: 100 * time.Millisecond}); err == nil {
		_ = reopened.Close()
		t.Fatal("unsafe preexisting SQLite sidecar was accepted")
	}
	got, err := os.ReadFile(sidecar)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("sidecar changed got=%q err=%v", got, err)
	}
	if afterNames := controlPlaneWindowsDirectoryNames(t, directory); !slices.Equal(afterNames, beforeNames) {
		t.Fatalf("directory changed before=%v after=%v", beforeNames, afterNames)
	}
}

func TestWindowsServerObservedUnsafeSidecarMayDisappearBeforeVerification(t *testing.T) {
	directory := privateTestDirectory(t)
	sidecar := filepath.Join(directory, "server.sqlite-journal")
	if err := os.WriteFile(sidecar, []byte("ephemeral"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := protectPrivateFile(sidecar); err != nil {
		t.Fatal(err)
	}
	if err := makeTestFileUnsafe(sidecar); err != nil {
		t.Fatal(err)
	}
	if err := verifyPrivateFile(sidecar); err == nil {
		t.Fatal("unsafe sidecar fixture was accepted")
	}
	if _, err := os.Lstat(sidecar); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(sidecar); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	present, err := waitForPrivateDatabaseSidecar(ctx, sidecar, time.Second)
	if err != nil {
		t.Fatalf("vanished observed sidecar was rejected: %v", err)
	}
	if present {
		t.Fatal("vanished sidecar was marked preexisting")
	}
}

func TestWindowsServerSidecarWaitsForConcurrentExactProtection(t *testing.T) {
	directory := privateTestDirectory(t)
	sidecar := filepath.Join(directory, "server.sqlite-wal")
	if err := os.WriteFile(sidecar, []byte("ephemeral"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := protectPrivateFile(sidecar); err != nil {
		t.Fatal(err)
	}
	if err := makeTestFileUnsafe(sidecar); err != nil {
		t.Fatal(err)
	}
	if err := verifyPrivateFile(sidecar); err == nil {
		t.Fatal("unsafe sidecar fixture was accepted")
	}

	observedUnsafe := make(chan struct{})
	protectResult := make(chan error, 1)
	var observedOnce sync.Once
	go func() {
		<-observedUnsafe
		protectResult <- protectPrivateFile(sidecar)
	}()
	present, waitErr := waitForPrivateDatabaseSidecarWithVerifier(context.Background(), sidecar, time.Second, func(path string) error {
		err := verifyPrivateFile(path)
		if err != nil {
			observedOnce.Do(func() { close(observedUnsafe) })
		}
		return err
	})
	observedOnce.Do(func() { close(observedUnsafe) })
	if protectErr := <-protectResult; protectErr != nil {
		t.Fatalf("protect sidecar after observed unsafe state: %v", protectErr)
	}
	if waitErr != nil {
		t.Fatalf("wait for exact sidecar protection: %v", waitErr)
	}
	if !present {
		t.Fatal("exact sidecar was not marked preexisting")
	}
	if err := verifyPrivateFile(sidecar); err != nil {
		t.Fatalf("protected sidecar is not exact: %v", err)
	}
}

func TestWindowsServerObservedSidecarMayDisappearDuringPostOpenProtection(t *testing.T) {
	directory := privateTestDirectory(t)
	for _, test := range []struct {
		name      string
		operation func(string) error
	}{
		{name: "verify-preexisting", operation: verifyObservedPrivateDatabaseSidecar},
		{name: "protect-new", operation: protectObservedPrivateDatabaseSidecar},
	} {
		t.Run(test.name, func(t *testing.T) {
			sidecar := filepath.Join(directory, test.name+".sqlite-shm")
			if err := os.WriteFile(sidecar, []byte("ephemeral"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := protectPrivateFile(sidecar); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(sidecar); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(sidecar); err != nil {
				t.Fatal(err)
			}
			if err := test.operation(sidecar); err != nil {
				t.Fatalf("vanished observed sidecar was rejected: %v", err)
			}
		})
	}
}

func TestWindowsPrivatePathKindIsExact(t *testing.T) {
	directory := privateTestDirectory(t)
	path := privateTestFile(t, directory, "kind", []byte("private"))
	if err := verifyPrivateFile(directory); err == nil {
		t.Fatal("directory was accepted as a private file")
	}
	if err := verifyPrivateDirectory(path); err == nil {
		t.Fatal("file was accepted as a private directory")
	}
}

func TestWindowsServerPrivateDACLNegativeMatrixAndReverseOrder(t *testing.T) {
	directory := privateTestDirectory(t)
	path := privateTestFile(t, directory, "acl-matrix", []byte("private"))
	owner, err := currentUserSID()
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
			if err := setControlPlaneWindowsDACL(path, test.aces); err != nil {
				t.Fatal(err)
			}
			if err := verifyPrivateFile(path); err == nil {
				t.Fatal("unsafe DACL was accepted")
			}
			if err := protectPrivateFile(path); err != nil {
				t.Fatal(err)
			}
		})
	}
	if err := setControlPlaneWindowsDACL(path, "(A;;FA;;;"+ownerText+")(A;;FA;;;SY)"); err != nil {
		t.Fatal(err)
	}
	if err := verifyPrivateFile(path); err != nil {
		t.Fatalf("reverse-order exact DACL was rejected: %v", err)
	}
}

func TestWindowsServerPrivateDACLRejectsRawNonzeroACEFlags(t *testing.T) {
	owner, err := currentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	system, err := localSystemSID()
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
			if err := verifyControlPlaneWindowsDACL(dacl, owner, system); err == nil {
				t.Fatal("raw DACL with nonzero ACE flags was accepted")
			}
		})
	}
}

func TestWindowsServerPrivateFileAndDirectoryReparsePointsAreRejected(t *testing.T) {
	directory := privateTestDirectory(t)
	targetFile := privateTestFile(t, directory, "target-file", []byte("private"))
	fileLink := filepath.Join(directory, "file-link")
	if err := os.Symlink(targetFile, fileLink); err != nil {
		t.Fatalf("create file reparse fixture: %v", err)
	}
	if err := verifyPrivateFile(fileLink); err == nil {
		t.Fatal("file reparse point was accepted")
	}
	targetDirectory := filepath.Join(directory, "target-directory")
	if err := os.Mkdir(targetDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := protectPrivateDirectory(targetDirectory); err != nil {
		t.Fatal(err)
	}
	directoryLink := filepath.Join(directory, "directory-link")
	if err := os.Symlink(targetDirectory, directoryLink); err != nil {
		t.Fatalf("create directory reparse fixture: %v", err)
	}
	if err := verifyPrivateDirectory(directoryLink); err == nil {
		t.Fatal("directory reparse point was accepted")
	}
}

func TestWindowsUnprotectedDACLIsRejected(t *testing.T) {
	directory := privateTestDirectory(t)
	path := privateTestFile(t, directory, "unprotected", []byte("private"))
	if err := makeTestFileUnsafe(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = protectPrivateFile(path) })
	if err := verifyPrivateFile(path); err == nil {
		t.Fatal("unprotected Windows DACL was accepted")
	}
	if err := protectPrivateFile(path); err != nil {
		t.Fatal(err)
	}
	if err := verifyPrivateFile(path); err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != directory {
		t.Fatal("fixture escaped its protected directory")
	}
}
