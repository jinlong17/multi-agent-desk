//go:build windows

package controlplane

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const fileAllAccess windows.ACCESS_MASK = windows.STANDARD_RIGHTS_ALL |
	windows.FILE_READ_DATA |
	windows.FILE_WRITE_DATA |
	windows.FILE_APPEND_DATA |
	windows.FILE_READ_EA |
	windows.FILE_WRITE_EA |
	windows.FILE_EXECUTE |
	0x40 | // FILE_DELETE_CHILD is part of the Win32 FILE_ALL_ACCESS mask.
	windows.FILE_READ_ATTRIBUTES |
	windows.FILE_WRITE_ATTRIBUTES

func currentUserSID() (*windows.SID, error) {
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

func localSystemSID() (*windows.SID, error) {
	sid, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return nil, fmt.Errorf("resolve Windows LocalSystem SID: %w", err)
	}
	return sid, nil
}

func protectPrivateDirectory(path string) error { return protectWindowsPath(path, true) }
func verifyPrivateDirectory(path string) error  { return verifyWindowsPath(path, true) }
func protectPrivateFile(path string) error      { return protectWindowsPath(path, false) }
func verifyPrivateFile(path string) error       { return verifyWindowsPath(path, false) }

func protectWindowsPath(path string, wantDirectory bool) error {
	if err := verifyWindowsPathKind(path, wantDirectory); err != nil {
		return err
	}
	ownerSID, err := currentUserSID()
	if err != nil {
		return err
	}
	descriptor, err := windows.SecurityDescriptorFromString("O:" + ownerSID.String() + "D:P(A;;FA;;;SY)(A;;FA;;;" + ownerSID.String() + ")")
	if err != nil {
		return fmt.Errorf("build private Windows DACL: %w", err)
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		return fmt.Errorf("read private Windows owner: %w", err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return fmt.Errorf("read private Windows DACL: %w", err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		owner, nil, dacl, nil); err != nil {
		return fmt.Errorf("apply private Windows DACL: %w", err)
	}
	return verifyWindowsPath(path, wantDirectory)
}

func verifyWindowsPath(path string, wantDirectory bool) error {
	if err := verifyWindowsPathKind(path, wantDirectory); err != nil {
		return err
	}
	ownerSID, err := currentUserSID()
	if err != nil {
		return err
	}
	systemSID, err := localSystemSID()
	if err != nil {
		return err
	}
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("read Windows path security descriptor: %w", err)
	}
	if descriptor == nil {
		return fmt.Errorf("Windows path has no security descriptor")
	}
	owner, defaulted, err := descriptor.Owner()
	if err != nil || owner == nil || defaulted || !owner.Equals(ownerSID) {
		return fmt.Errorf("Windows path owner is not the current logon SID")
	}
	control, _, err := descriptor.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return fmt.Errorf("Windows path DACL inheritance is not protected")
	}
	dacl, defaulted, err := descriptor.DACL()
	if err != nil || defaulted {
		return fmt.Errorf("Windows path DACL is unavailable")
	}
	if err := verifyControlPlaneWindowsDACL(dacl, ownerSID, systemSID); err != nil {
		return err
	}
	return verifyWindowsPathKind(path, wantDirectory)
}

func verifyControlPlaneWindowsDACL(dacl *windows.ACL, ownerSID, systemSID *windows.SID) error {
	if dacl == nil || ownerSID == nil || systemSID == nil || dacl.AceCount != 2 {
		return fmt.Errorf("Windows path DACL does not contain exactly current logon and LocalSystem")
	}
	seenOwner, seenSystem := false, false
	for index := uint32(0); index < uint32(dacl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, index, &ace); err != nil || ace == nil {
			return fmt.Errorf("read Windows path DACL ACE %d: %w", index, err)
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags != 0 || ace.Mask != fileAllAccess {
			return fmt.Errorf("Windows path DACL contains an unsafe ACE")
		}
		aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		switch {
		case aceSID.Equals(ownerSID) && !seenOwner:
			seenOwner = true
		case aceSID.Equals(systemSID) && !seenSystem:
			seenSystem = true
		default:
			return fmt.Errorf("Windows path DACL grants an unexpected or duplicate principal")
		}
	}
	if !seenOwner || !seenSystem {
		return fmt.Errorf("Windows path DACL is missing current logon or LocalSystem")
	}
	return nil
}

func verifyWindowsPathKind(path string, wantDirectory bool) error {
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("encode Windows path: %w", err)
	}
	attributes, err := windows.GetFileAttributes(pointer)
	if err != nil {
		return fmt.Errorf("read Windows path attributes: %w", err)
	}
	if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("Windows path is a reparse point")
	}
	if attributes&windows.FILE_ATTRIBUTE_DEVICE != 0 {
		return fmt.Errorf("Windows path is a device")
	}
	isDirectory := attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0
	if isDirectory != wantDirectory {
		return fmt.Errorf("Windows path kind is invalid")
	}
	return nil
}
