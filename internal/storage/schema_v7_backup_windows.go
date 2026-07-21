//go:build windows

package storage

import (
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"golang.org/x/sys/windows"
)

const backupFileAllAccess windows.ACCESS_MASK = windows.STANDARD_RIGHTS_ALL |
	windows.FILE_READ_DATA |
	windows.FILE_WRITE_DATA |
	windows.FILE_APPEND_DATA |
	windows.FILE_READ_EA |
	windows.FILE_WRITE_EA |
	windows.FILE_EXECUTE |
	0x40 |
	windows.FILE_READ_ATTRIBUTES |
	windows.FILE_WRITE_ATTRIBUTES

func runtimeGOOS() string { return runtime.GOOS }

func ensurePrivateBackupDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return domain.WrapError(domain.CodeConflict, "backup root could not be created", err)
	}
	return protectDevicePrivateDirectory(path)
}

func backupCurrentUserSID() (*windows.SID, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("read current Windows token user: %w", err)
	}
	return user.User.Sid, nil
}

func backupSystemSID() (*windows.SID, error) {
	return windows.StringToSid("S-1-5-18")
}

func protectDevicePrivateDirectory(path string) error { return protectDeviceWindowsPath(path) }
func protectDevicePrivateFile(path string) error      { return protectDeviceWindowsPath(path) }

func protectDeviceWindowsPath(path string) error {
	owner, err := backupCurrentUserSID()
	if err != nil {
		return domain.WrapError(domain.CodePermissionDenied, "private Windows owner could not be resolved", err)
	}
	descriptor, err := windows.SecurityDescriptorFromString("O:" + owner.String() + "D:P(A;;FA;;;SY)(A;;FA;;;" + owner.String() + ")")
	if err != nil {
		return domain.WrapError(domain.CodePermissionDenied, "private Windows DACL could not be built", err)
	}
	descriptorOwner, _, err := descriptor.Owner()
	if err != nil {
		return domain.WrapError(domain.CodePermissionDenied, "private Windows owner could not be read", err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return domain.WrapError(domain.CodePermissionDenied, "private Windows DACL could not be read", err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		descriptorOwner, nil, dacl, nil); err != nil {
		return domain.WrapError(domain.CodePermissionDenied, "private Windows DACL could not be applied", err)
	}
	return verifyDeviceWindowsPath(path)
}

func verifyDevicePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return domain.NewError(domain.CodePermissionDenied, "private Windows directory is unsafe")
	}
	return verifyDeviceWindowsPath(path)
}

func verifyDevicePrivateFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return domain.NewError(domain.CodePermissionDenied, "private Windows file is unsafe")
	}
	return verifyDeviceWindowsPath(path)
}

func verifyDeviceWindowsPath(path string) error {
	ownerSID, err := backupCurrentUserSID()
	if err != nil {
		return err
	}
	systemSID, err := backupSystemSID()
	if err != nil {
		return err
	}
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || descriptor == nil {
		return domain.NewError(domain.CodePermissionDenied, "private Windows descriptor is unavailable")
	}
	owner, defaulted, err := descriptor.Owner()
	if err != nil || owner == nil || defaulted || !owner.Equals(ownerSID) {
		return domain.NewError(domain.CodePermissionDenied, "private Windows owner is not the current logon SID")
	}
	control, _, err := descriptor.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return domain.NewError(domain.CodePermissionDenied, "private Windows DACL inheritance is enabled")
	}
	dacl, defaulted, err := descriptor.DACL()
	if err != nil || dacl == nil || defaulted || dacl.AceCount != 2 {
		return domain.NewError(domain.CodePermissionDenied, "private Windows DACL does not contain exactly owner and SYSTEM")
	}
	seenOwner, seenSystem := false, false
	for index := uint32(0); index < uint32(dacl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, index, &ace); err != nil || ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags != 0 || ace.Mask != backupFileAllAccess {
			return domain.NewError(domain.CodePermissionDenied, "private Windows DACL contains an unsafe ACE")
		}
		aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		switch {
		case aceSID.Equals(ownerSID):
			seenOwner = true
		case aceSID.Equals(systemSID):
			seenSystem = true
		default:
			return domain.NewError(domain.CodePermissionDenied, "private Windows DACL grants an unexpected principal")
		}
	}
	if !seenOwner || !seenSystem {
		return domain.NewError(domain.CodePermissionDenied, "private Windows DACL is missing owner or SYSTEM")
	}
	return nil
}

func syncDirectory(path string) error {
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "private Windows directory path is invalid", err)
	}
	handle, err := windows.CreateFile(pointer, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "private Windows directory could not be opened for sync", err)
	}
	defer windows.CloseHandle(handle)
	if err := windows.FlushFileBuffers(handle); err != nil {
		return domain.WrapError(domain.CodeConflict, "private Windows directory could not be synced", err)
	}
	return nil
}
