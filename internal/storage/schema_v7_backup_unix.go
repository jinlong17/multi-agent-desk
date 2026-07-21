//go:build !windows

package storage

import (
	"errors"
	"os"
	"runtime"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

func runtimeGOOS() string { return runtime.GOOS }

func ensurePrivateBackupDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return domain.WrapError(domain.CodeConflict, "backup root could not be created", err)
	}
	return protectDevicePrivateDirectory(path)
}

func protectDevicePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return domain.NewError(domain.CodeConflict, "private directory is unsafe")
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return domain.WrapError(domain.CodeConflict, "private directory permissions could not be restricted", err)
	}
	return verifyDevicePrivateDirectory(path)
}

func verifyDevicePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
		return domain.NewError(domain.CodePermissionDenied, "private directory permissions are unsafe")
	}
	return nil
}

func protectDevicePrivateFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return domain.NewError(domain.CodeConflict, "private file is unsafe")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return domain.WrapError(domain.CodeConflict, "private file permissions could not be restricted", err)
	}
	return verifyDevicePrivateFile(path)
}

func verifyDevicePrivateFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 {
		return domain.NewError(domain.CodePermissionDenied, "private file permissions are unsafe")
	}
	return nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "private directory could not be opened for sync", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil && !errors.Is(err, os.ErrInvalid) {
		return domain.WrapError(domain.CodeConflict, "private directory could not be synced", err)
	}
	return nil
}
