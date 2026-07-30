//go:build !windows

package device

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

func createPrivateDirectory(path string) error {
	if err := os.Mkdir(path, 0o700); err != nil {
		return domain.WrapError(domain.CodeConflict, "private directory could not be created", err)
	}
	return ProtectPrivateDirectory(path)
}

// ProtectPrivateDirectory applies and verifies the platform-private directory
// boundary. Unix uses owner-only mode bits; Windows supplies the exact current
// user plus LocalSystem DACL in the platform implementation.
func ProtectPrivateDirectory(path string) error {
	if err := os.Chmod(path, 0o700); err != nil {
		return domain.WrapError(domain.CodePermissionDenied, "private directory permissions could not be restricted", err)
	}
	return verifyPrivateDirectory(path)
}

func VerifyPrivateDirectory(path string) error { return verifyPrivateDirectory(path) }

func VerifyPrivateFile(path string) error { return verifyPrivateFile(path) }

// WritePrivateFileAtomic writes a file under a protected directory and applies
// the platform-specific private-file boundary.
func WritePrivateFileAtomic(path string, data []byte) error {
	return writePrivateFileAtomic(path, data)
}

// ReplacePrivateFileAtomic replaces an existing private file through a
// same-directory private temporary file. The destination must already satisfy
// the platform-private boundary.
func ReplacePrivateFileAtomic(path string, data []byte) error {
	return replacePrivateFileAtomic(path, data)
}

func verifyPrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "private directory could not be inspected", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
		return domain.NewError(domain.CodePermissionDenied, "private directory permissions are too broad")
	}
	return nil
}

func writePrivateFileAtomic(path string, data []byte) error {
	if err := verifyPrivateDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	temporary := path + ".new"
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "private file could not be created", err)
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
	if _, err := os.Lstat(path); err == nil {
		return domain.NewError(domain.CodeAlreadyExists, "private file already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return domain.WrapError(domain.CodeConflict, "private file could not be inspected", err)
	}
	if err := os.Rename(temporary, path); err != nil {
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
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "private replacement file could not be created", err)
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
	if err := os.Rename(temporary, path); err != nil {
		return domain.WrapError(domain.CodeConflict, "private replacement file could not be committed", err)
	}
	ok = true
	return verifyPrivateFile(path)
}

func verifyPrivateFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "private file could not be inspected", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
		return domain.NewError(domain.CodePermissionDenied, "private file permissions are too broad")
	}
	return nil
}
