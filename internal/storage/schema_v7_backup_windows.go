//go:build windows

package storage

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"
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
	if _, err := os.Lstat(path); err == nil {
		return verifyDevicePrivateDirectory(path)
	} else if !os.IsNotExist(err) {
		return domain.WrapError(domain.CodeConflict, "backup root could not be inspected", err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		return domain.WrapError(domain.CodeConflict, "backup root could not be created", err)
	}
	return protectDevicePrivateDirectory(path)
}

func backupCurrentUserSID() (*windows.SID, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("read current Windows token user: %w", err)
	}
	sid, err := user.User.Sid.Copy()
	if err != nil {
		return nil, fmt.Errorf("copy current Windows token user SID: %w", err)
	}
	return sid, nil
}

func backupSystemSID() (*windows.SID, error) {
	return windows.CreateWellKnownSid(windows.WinLocalSystemSid)
}

func protectDevicePrivateDirectory(path string) error { return protectDeviceWindowsPath(path, true) }
func protectDevicePrivateFile(path string) error      { return protectDeviceWindowsPath(path, false) }

func protectDeviceWindowsPath(path string, wantDirectory bool) error {
	if err := verifyDeviceWindowsPathKind(path, wantDirectory); err != nil {
		return err
	}
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
	return verifyDeviceWindowsPath(path, wantDirectory)
}

func verifyDevicePrivateDirectory(path string) error {
	return verifyDeviceWindowsPath(path, true)
}

func verifyDevicePrivateFile(path string) error {
	return verifyDeviceWindowsPath(path, false)
}

func prepareExistingDevicePrivateDirectory(ctx context.Context, path string, timeout time.Duration) error {
	return waitForDeviceWindowsPath(ctx, path, true, timeout)
}

func prepareExistingDevicePrivateFile(ctx context.Context, path string, timeout time.Duration) error {
	return waitForDeviceWindowsPath(ctx, path, false, timeout)
}

// prepareExistingDevicePrivateSidecar is called only after the SQLite sidecar
// has been observed with Lstat. Unlike stable database files, sidecars may be
// removed by another SQLite connection while their exact DACL is being
// checked. A vanished sidecar is therefore not pre-existing; a sidecar that
// remains present must still converge to the exact private Windows boundary.
func prepareExistingDevicePrivateSidecar(ctx context.Context, path string, timeout time.Duration) (bool, error) {
	return waitForDevicePrivateSidecar(ctx, path, timeout, verifyDevicePrivateFile)
}

func waitForDevicePrivateSidecar(ctx context.Context, path string, timeout time.Duration, verify func(string) error) (bool, error) {
	if timeout <= 0 {
		return false, domain.NewError(domain.CodeInvalidArgument, "private Windows sidecar wait is invalid")
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		lastErr = verify(path)
		if lastErr == nil {
			return true, nil
		}
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			return false, nil
		} else if err != nil {
			return false, domain.WrapError(domain.CodeConflict, "database sidecar cannot be inspected", err)
		}
		select {
		case <-ctx.Done():
			return false, domain.WrapError(domain.CodePermissionDenied, "private Windows sidecar did not become safe", ctx.Err())
		case <-deadline.C:
			return false, lastErr
		case <-ticker.C:
		}
	}
}

func waitForDeviceWindowsPath(ctx context.Context, path string, wantDirectory bool, timeout time.Duration) error {
	if timeout <= 0 {
		return domain.NewError(domain.CodeInvalidArgument, "private Windows path wait is invalid")
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		lastErr = verifyDeviceWindowsPath(path, wantDirectory)
		if lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return domain.WrapError(domain.CodePermissionDenied, "private Windows path did not become safe", ctx.Err())
		case <-deadline.C:
			return lastErr
		case <-ticker.C:
		}
	}
}

func verifyDeviceWindowsPath(path string, wantDirectory bool) error {
	if err := verifyDeviceWindowsPathKind(path, wantDirectory); err != nil {
		return err
	}
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
	if err != nil || defaulted {
		return domain.NewError(domain.CodePermissionDenied, "private Windows DACL is unavailable")
	}
	if err := verifyDeviceWindowsDACL(dacl, ownerSID, systemSID); err != nil {
		return err
	}
	return verifyDeviceWindowsPathKind(path, wantDirectory)
}

func verifyDeviceWindowsDACL(dacl *windows.ACL, ownerSID, systemSID *windows.SID) error {
	if dacl == nil || ownerSID == nil || systemSID == nil || dacl.AceCount != 2 {
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
		case aceSID.Equals(ownerSID) && !seenOwner:
			seenOwner = true
		case aceSID.Equals(systemSID) && !seenSystem:
			seenSystem = true
		default:
			return domain.NewError(domain.CodePermissionDenied, "private Windows DACL grants an unexpected or duplicate principal")
		}
	}
	if !seenOwner || !seenSystem {
		return domain.NewError(domain.CodePermissionDenied, "private Windows DACL is missing owner or SYSTEM")
	}
	return nil
}

func verifyDeviceWindowsPathKind(path string, wantDirectory bool) error {
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return domain.WrapError(domain.CodePermissionDenied, "private Windows path is invalid", err)
	}
	attributes, err := windows.GetFileAttributes(pointer)
	if err != nil {
		return domain.WrapError(domain.CodePermissionDenied, "private Windows path attributes are unavailable", err)
	}
	if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return domain.NewError(domain.CodePermissionDenied, "private Windows path is a reparse point")
	}
	if attributes&windows.FILE_ATTRIBUTE_DEVICE != 0 {
		return domain.NewError(domain.CodePermissionDenied, "private Windows path is a device")
	}
	isDirectory := attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0
	if isDirectory != wantDirectory {
		return domain.NewError(domain.CodePermissionDenied, "private Windows path kind is invalid")
	}
	return nil
}

func syncDirectory(path string) error {
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "private Windows directory path is invalid", err)
	}
	// FlushFileBuffers requires a handle opened with GENERIC_WRITE. The exact
	// private owner+SYSTEM DACL grants this directory handle the needed access.
	handle, err := windows.CreateFile(pointer, windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
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
