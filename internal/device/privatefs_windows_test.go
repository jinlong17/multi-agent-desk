//go:build windows

package device

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"golang.org/x/sys/windows"
)

func setDeviceDiskTestDACL(path, aces string) error {
	owner, err := deviceDiskCurrentUserSID()
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

func TestWindowsPrivateDiskPathsUseExactCurrentUserAndSystemDACL(t *testing.T) {
	if deviceDiskFileAllAccess != windows.ACCESS_MASK(0x001f01ff) {
		t.Fatalf("FILE_ALL_ACCESS mask=%#x", deviceDiskFileAllAccess)
	}
	root := filepath.Join(t.TempDir(), "device-private")
	if err := createPrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "identity.json")
	if err := WritePrivateFileAtomic(path, []byte("private")); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPrivateFile(path); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsPrivateDiskFileHasExactPolicyBeforeFirstWrite(t *testing.T) {
	root := filepath.Join(t.TempDir(), "device-private")
	if err := createPrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "identity.json.new")
	file, err := createDeviceDiskPrivateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = file.Close()
		_ = os.Remove(path)
	})
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Fatalf("new private file already contains %d bytes", info.Size())
	}
	if err := verifyDeviceDiskHandle(windows.Handle(file.Fd())); err != nil {
		t.Fatalf("creation-time private policy: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPrivateFile(path); err != nil {
		t.Fatalf("closed creation-time private policy: %v", err)
	}
}

func TestWindowsPrivateDiskPathRejectsExtraPrincipal(t *testing.T) {
	root := filepath.Join(t.TempDir(), "device-private")
	if err := createPrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "identity.json")
	if err := WritePrivateFileAtomic(path, []byte("private")); err != nil {
		t.Fatal(err)
	}
	owner, err := deviceDiskCurrentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	if err := setDeviceDiskTestDACL(path, "(A;;FA;;;SY)(A;;FA;;;"+owner.String()+")(A;;FR;;;WD)"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = protectPrivateFile(path) })
	if err := VerifyPrivateFile(path); domain.CodeOf(err) != domain.CodePermissionDenied {
		t.Fatalf("code=%s err=%v", domain.CodeOf(err), err)
	}
}
