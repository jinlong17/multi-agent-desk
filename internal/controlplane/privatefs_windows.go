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
	return user.User.Sid, nil
}

func protectPrivateDirectory(path string) error { return protectWindowsPath(path) }
func verifyPrivateDirectory(path string) error  { return verifyWindowsPathDACL(path) }
func protectPrivateFile(path string) error      { return protectWindowsPath(path) }
func verifyPrivateFile(path string) error       { return verifyWindowsPathDACL(path) }

func protectWindowsPath(path string) error {
	sid, err := currentUserSID()
	if err != nil {
		return err
	}
	descriptor, err := windows.SecurityDescriptorFromString("O:" + sid.String() + "D:P(A;;FA;;;" + sid.String() + ")")
	if err != nil {
		return fmt.Errorf("build owner-only Windows DACL: %w", err)
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		return fmt.Errorf("read owner-only Windows owner: %w", err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return fmt.Errorf("read owner-only Windows DACL: %w", err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		owner, nil, dacl, nil); err != nil {
		return fmt.Errorf("apply owner-only Windows DACL: %w", err)
	}
	return verifyWindowsPathDACL(path)
}

func verifyWindowsPathDACL(path string) error {
	want, err := currentUserSID()
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
	if err != nil || owner == nil || defaulted || !owner.Equals(want) {
		return fmt.Errorf("Windows path owner is not the current logon SID")
	}
	control, _, err := descriptor.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return fmt.Errorf("Windows path DACL inheritance is not protected")
	}
	dacl, defaulted, err := descriptor.DACL()
	if err != nil || dacl == nil || defaulted || dacl.AceCount != 1 {
		return fmt.Errorf("Windows path DACL is not an explicit single-principal ACL")
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil || ace == nil {
		return fmt.Errorf("read Windows path DACL ACE: %w", err)
	}
	if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags != 0 {
		return fmt.Errorf("Windows path DACL contains a non-allow ACE")
	}
	aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if !aceSID.Equals(want) || ace.Mask != fileAllAccess {
		return fmt.Errorf("Windows path DACL does not grant only current-logon full control")
	}
	return nil
}
