//go:build !windows

package storage

import "os"

func makeExistingDeviceDirectoryUnsafeForTest(path string) error {
	return os.Chmod(path, 0o755)
}
