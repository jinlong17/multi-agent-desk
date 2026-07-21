package controlplane

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ProcessLock serializes the live server and offline bootstrap maintenance
// against the same database. The OS lock is released on process exit even if
// the lock file remains on disk.
type ProcessLock struct {
	mu   sync.Mutex
	file *os.File
}

func AcquireProcessLock(databasePath string) (*ProcessLock, error) {
	if !filepath.IsAbs(databasePath) || filepath.Clean(databasePath) != databasePath {
		return nil, fmt.Errorf("database path must be absolute and clean")
	}
	if err := verifyPrivateDirectory(filepath.Dir(databasePath)); err != nil {
		return nil, err
	}
	path := databasePath + ".process.lock"
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open control-plane process lock: %w", err)
	}
	if err := protectPrivateFile(path); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := lockProcessFile(file); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("control-plane database is already in use: %w", err)
	}
	return &ProcessLock{file: file}, nil
}

func (l *ProcessLock) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	file := l.file
	l.file = nil
	unlockErr := unlockProcessFile(file)
	closeErr := file.Close()
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
