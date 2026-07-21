//go:build windows

package controlplane

import (
	"path/filepath"
	"testing"

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

func TestWindowsPrivateFixturesUseExactCurrentLogonDACL(t *testing.T) {
	directory := privateTestDirectory(t)
	if err := verifyPrivateDirectory(directory); err != nil {
		t.Fatal(err)
	}
	path := privateTestFile(t, directory, "fixture", []byte("private"))
	if err := verifyPrivateFile(path); err != nil {
		t.Fatal(err)
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
