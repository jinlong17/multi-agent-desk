//go:build windows

package device

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

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

// ProtectPrivateDirectory applies the current-logon DACL to an existing
// directory and verifies that it is a real directory. Windows mode bits are
// intentionally not used as an access-control primitive.
func ProtectPrivateDirectory(path string) error {
	sa, cleanup, err := currentLogonSecurityAttributes()
	if err != nil {
		return domain.WrapError(domain.CodePermissionDenied, "private directory policy could not be created", err)
	}
	defer cleanup()
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	ok, _, callErr := procSetFileSecurity.Call(uintptr(unsafe.Pointer(name)), daclSecurityInformation|protectedDACLInformation, sa.SecurityDescriptor)
	if ok == 0 {
		return domain.WrapError(domain.CodePermissionDenied, "private directory policy could not be applied", callErr)
	}
	return verifyPrivateDirectory(path)
}

// WritePrivateFileAtomic writes a file with the current-logon DACL and commits
// it with an atomic rename.
func WritePrivateFileAtomic(path string, data []byte) error {
	return writePrivateFileAtomic(path, data)
}

func verifyPrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "private directory could not be inspected", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return domain.NewError(domain.CodePermissionDenied, "private directory is not protected")
	}
	// Named Pipe and identity files are protected by the current-logon DACL;
	// mode bits are not an access-control primitive on Windows.
	return nil
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
	sa, cleanup, err := currentLogonSecurityAttributes()
	if err != nil {
		return domain.WrapError(domain.CodePermissionDenied, "private file policy could not be created", err)
	}
	defer cleanup()
	name, err := syscall.UTF16PtrFromString(temporary)
	if err != nil {
		return err
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_WRITE, 0, sa, syscall.CREATE_NEW, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "private file could not be created", err)
	}
	file := os.NewFile(uintptr(handle), temporary)
	if file == nil {
		_ = syscall.CloseHandle(handle)
		return domain.NewError(domain.CodeConflict, "private file handle could not be created")
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
	if err := os.Rename(temporary, path); err != nil {
		return domain.WrapError(domain.CodeConflict, "private file could not be committed", err)
	}
	ok = true
	return verifyPrivateFile(path)
}

func verifyPrivateFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "private file could not be inspected", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return domain.NewError(domain.CodePermissionDenied, "private file is not protected")
	}
	return nil
}
