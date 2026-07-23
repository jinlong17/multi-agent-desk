//go:build !windows

package storage

import (
	"context"
	"errors"
	"os"
	"runtime"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

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

func prepareExistingDevicePrivateDirectory(_ context.Context, path string, _ time.Duration) error {
	return verifyDevicePrivateDirectory(path)
}

func prepareExistingDevicePrivateFile(_ context.Context, path string, _ time.Duration) error {
	// Pre-P2 Device databases may have been created with a process umask rather
	// than the explicit 0600 contract. Restrict them before SQLite opens them.
	return protectDevicePrivateFile(path)
}

func prepareExistingDevicePrivateSidecar(_ context.Context, path string, _ time.Duration) (bool, error) {
	// The caller already observed this SQLite sidecar. Preserve the historical
	// Unix permission tightening, but allow SQLite teardown to win the race
	// between that observation and the sidecar-specific protection attempt.
	if err := protectDevicePrivateFile(path); err != nil {
		if _, statErr := os.Lstat(path); errors.Is(statErr, os.ErrNotExist) {
			return false, nil
		} else if statErr != nil {
			return false, domain.WrapError(domain.CodeConflict, "database sidecar cannot be inspected", statErr)
		}
		return false, err
	}
	return true, nil
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
