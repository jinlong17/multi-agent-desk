package controlplane

import (
	"path/filepath"
	"testing"
)

func TestProcessLockExcludesConcurrentServerAndMaintenance(t *testing.T) {
	path := filepath.Join(privateTestDirectory(t), "server.sqlite")
	first, err := AcquireProcessLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if second, err := AcquireProcessLock(path); err == nil {
		_ = second.Close()
		t.Fatal("concurrent process lock was acquired")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := AcquireProcessLock(path)
	if err != nil {
		t.Fatalf("process lock was not released: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}
