//go:build windows

package device

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"golang.org/x/sys/windows"
)

const deviceDiskFileAllAccess windows.ACCESS_MASK = windows.STANDARD_RIGHTS_ALL |
	windows.FILE_READ_DATA |
	windows.FILE_WRITE_DATA |
	windows.FILE_APPEND_DATA |
	windows.FILE_READ_EA |
	windows.FILE_WRITE_EA |
	windows.FILE_EXECUTE |
	0x40 |
	windows.FILE_READ_ATTRIBUTES |
	windows.FILE_WRITE_ATTRIBUTES

func createPrivateDirectory(path string) error {
	parent := filepath.Dir(path)
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		return domain.NewError(domain.CodeConflict, "private directory parent is unavailable")
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		return domain.WrapError(domain.CodeConflict, "private directory could not be created", err)
	}
	return ProtectPrivateDirectory(path)
}

// ProtectPrivateDirectory applies the exact current-user plus LocalSystem DACL
// used by Device storage and verifies the complete boundary. Windows mode bits
// are intentionally not used as an access-control primitive.
func ProtectPrivateDirectory(path string) error {
	return protectDeviceDiskPath(path, true)
}

func VerifyPrivateDirectory(path string) error { return verifyPrivateDirectory(path) }

func VerifyPrivateFile(path string) error { return verifyPrivateFile(path) }

// WritePrivateFileAtomic writes a file with the exact current-user plus
// LocalSystem DACL and commits it with an atomic rename.
func WritePrivateFileAtomic(path string, data []byte) error {
	return writePrivateFileAtomic(path, data)
}

// ReplacePrivateFileAtomic replaces an existing private file through a
// same-directory private temporary file. The destination must already satisfy
// the exact current-user plus LocalSystem boundary.
func ReplacePrivateFileAtomic(path string, data []byte) error {
	return replacePrivateFileAtomic(path, data)
}

func verifyPrivateDirectory(path string) error {
	return verifyDeviceDiskPath(path, true)
}

func writePrivateFileAtomic(path string, data []byte) error {
	if err := verifyPrivateDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	temporary := path + ".new"
	if _, err := os.Lstat(path); err == nil {
		return domain.NewError(domain.CodeAlreadyExists, "private file already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return domain.WrapError(domain.CodeConflict, "private file could not be inspected", err)
	}
	file, err := createDeviceDiskPrivateFile(temporary)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(temporary)
		}
	}()
	if _, err := file.Write(data); err != nil {
		return domain.WrapError(domain.CodeConflict, "private file could not be written", err)
	}
	if err := file.Sync(); err != nil {
		return domain.WrapError(domain.CodeConflict, "private file could not be synchronized", err)
	}
	if err := file.Close(); err != nil {
		return domain.WrapError(domain.CodeConflict, "private file could not be closed", err)
	}
	if err := verifyPrivateFile(temporary); err != nil {
		return err
	}
	if err := moveDeviceDiskPrivateFile(temporary, path, false); err != nil {
		return domain.WrapError(domain.CodeConflict, "private file could not be committed", err)
	}
	ok = true
	return verifyPrivateFile(path)
}

func replacePrivateFileAtomic(path string, data []byte) error {
	if err := verifyPrivateDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	if err := verifyPrivateFile(path); err != nil {
		return err
	}
	temporary := path + ".replace"
	file, err := createDeviceDiskPrivateFile(temporary)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(temporary)
		}
	}()
	if _, err := file.Write(data); err != nil {
		return domain.WrapError(domain.CodeConflict, "private replacement file could not be written", err)
	}
	if err := file.Sync(); err != nil {
		return domain.WrapError(domain.CodeConflict, "private replacement file could not be synchronized", err)
	}
	if err := file.Close(); err != nil {
		return domain.WrapError(domain.CodeConflict, "private replacement file could not be closed", err)
	}
	if err := verifyPrivateFile(temporary); err != nil {
		return err
	}
	if err := moveDeviceDiskPrivateFile(temporary, path, true); err != nil {
		return domain.WrapError(domain.CodeConflict, "private replacement file could not be committed", err)
	}
	ok = true
	return verifyPrivateFile(path)
}

func verifyPrivateFile(path string) error {
	return verifyDeviceDiskPath(path, false)
}

func protectPrivateFile(path string) error {
	return protectDeviceDiskPath(path, false)
}

func deviceDiskCurrentUserSID() (*windows.SID, error) {
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

func deviceDiskSystemSID() (*windows.SID, error) {
	return windows.CreateWellKnownSid(windows.WinLocalSystemSid)
}

func deviceDiskSecurityDescriptor() (*windows.SECURITY_DESCRIPTOR, error) {
	owner, err := deviceDiskCurrentUserSID()
	if err != nil {
		return nil, domain.WrapError(domain.CodePermissionDenied, "private Windows owner could not be resolved", err)
	}
	descriptor, err := windows.SecurityDescriptorFromString("O:" + owner.String() + "D:P(A;;FA;;;SY)(A;;FA;;;" + owner.String() + ")")
	if err != nil {
		return nil, domain.WrapError(domain.CodePermissionDenied, "private Windows DACL could not be built", err)
	}
	if err := verifyDeviceDiskSecurityDescriptor(descriptor); err != nil {
		return nil, err
	}
	return descriptor, nil
}

func deviceDiskSecurityAttributes() (*windows.SecurityAttributes, error) {
	descriptor, err := deviceDiskSecurityDescriptor()
	if err != nil {
		return nil, err
	}
	return &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: descriptor,
	}, nil
}

func createDeviceDiskPrivateFile(path string) (*os.File, error) {
	sa, err := deviceDiskSecurityAttributes()
	if err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "private file path is invalid", err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, sa, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
	runtime.KeepAlive(sa.SecurityDescriptor)
	runtime.KeepAlive(sa)
	if err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "private file could not be created", err)
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		_ = windows.CloseHandle(handle)
		return nil, domain.NewError(domain.CodeConflict, "private file handle could not be created")
	}
	if err := verifyDeviceDiskHandle(handle); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	return file, nil
}

func moveDeviceDiskPrivateFile(source, destination string, replace bool) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "private source path is invalid", err)
	}
	to, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "private destination path is invalid", err)
	}
	flags := uint32(windows.MOVEFILE_WRITE_THROUGH)
	if replace {
		flags |= windows.MOVEFILE_REPLACE_EXISTING
	}
	if err := windows.MoveFileEx(from, to, flags); err != nil {
		return domain.WrapError(domain.CodeConflict, "private file move failed", err)
	}
	return nil
}

func protectDeviceDiskPath(path string, wantDirectory bool) error {
	if err := verifyDeviceDiskPathKind(path, wantDirectory); err != nil {
		return err
	}
	descriptor, err := deviceDiskSecurityDescriptor()
	if err != nil {
		return err
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
	return verifyDeviceDiskPath(path, wantDirectory)
}

func verifyDeviceDiskPath(path string, wantDirectory bool) error {
	if err := verifyDeviceDiskPathKind(path, wantDirectory); err != nil {
		return err
	}
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || descriptor == nil {
		return domain.NewError(domain.CodePermissionDenied, "private Windows descriptor is unavailable")
	}
	if err := verifyDeviceDiskSecurityDescriptor(descriptor); err != nil {
		return err
	}
	return verifyDeviceDiskPathKind(path, wantDirectory)
}

func verifyDeviceDiskHandle(handle windows.Handle) error {
	if handle == 0 || handle == windows.InvalidHandle {
		return domain.NewError(domain.CodePermissionDenied, "private Windows handle is unavailable")
	}
	descriptor, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || descriptor == nil {
		return domain.NewError(domain.CodePermissionDenied, "private Windows descriptor is unavailable")
	}
	return verifyDeviceDiskSecurityDescriptor(descriptor)
}

func verifyDeviceDiskSecurityDescriptor(descriptor *windows.SECURITY_DESCRIPTOR) error {
	if descriptor == nil {
		return domain.NewError(domain.CodePermissionDenied, "private Windows descriptor is unavailable")
	}
	ownerSID, err := deviceDiskCurrentUserSID()
	if err != nil {
		return domain.WrapError(domain.CodePermissionDenied, "private Windows owner could not be resolved", err)
	}
	systemSID, err := deviceDiskSystemSID()
	if err != nil {
		return domain.WrapError(domain.CodePermissionDenied, "private Windows SYSTEM SID could not be resolved", err)
	}
	owner, defaulted, err := descriptor.Owner()
	if err != nil || owner == nil || defaulted || !owner.Equals(ownerSID) {
		return domain.NewError(domain.CodePermissionDenied, "private Windows owner is not the current user SID")
	}
	control, _, err := descriptor.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return domain.NewError(domain.CodePermissionDenied, "private Windows DACL inheritance is enabled")
	}
	dacl, defaulted, err := descriptor.DACL()
	if err != nil || defaulted {
		return domain.NewError(domain.CodePermissionDenied, "private Windows DACL is unavailable")
	}
	if err := verifyDeviceDiskDACL(dacl, ownerSID, systemSID); err != nil {
		return err
	}
	return nil
}

func verifyDeviceDiskDACL(dacl *windows.ACL, ownerSID, systemSID *windows.SID) error {
	if dacl == nil || ownerSID == nil || systemSID == nil || dacl.AceCount != 2 {
		return domain.NewError(domain.CodePermissionDenied, "private Windows DACL does not contain exactly current user and SYSTEM")
	}
	seenOwner, seenSystem := false, false
	for index := uint32(0); index < uint32(dacl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, index, &ace); err != nil || ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags != 0 || ace.Mask != deviceDiskFileAllAccess {
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
		return domain.NewError(domain.CodePermissionDenied, "private Windows DACL is missing current user or SYSTEM")
	}
	return nil
}

func verifyDeviceDiskPathKind(path string, wantDirectory bool) error {
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
