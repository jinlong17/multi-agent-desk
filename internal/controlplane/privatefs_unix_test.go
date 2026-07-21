//go:build !windows

package controlplane

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivatePathsRejectSymlinksAndBroadExistingDirectory(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target")
	if err := os.WriteFile(target, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := verifyPrivateFile(link); err == nil {
		t.Fatal("private file verifier followed a symlink")
	}

	broad := filepath.Join(directory, "broad")
	if err := os.Mkdir(broad, 0o755); err != nil {
		t.Fatal(err)
	}
	if store, err := OpenStore(t.Context(), StoreOptions{Path: filepath.Join(broad, "server.sqlite")}); err == nil {
		_ = store.Close()
		t.Fatal("store silently accepted an existing broad database directory")
	}
}
