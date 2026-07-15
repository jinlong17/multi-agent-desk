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
	return verifyPrivateDirectory(path)
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
