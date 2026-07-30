//go:build !windows

package storage

import "os"

func replaceDeviceDatabaseFile(temporaryPath, databasePath string) error {
	return os.Rename(temporaryPath, databasePath)
}
