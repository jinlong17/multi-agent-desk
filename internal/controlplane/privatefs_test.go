package controlplane

import (
	"os"
	"path/filepath"
	"testing"
)

func privateTestDirectory(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	if err := protectPrivateDirectory(directory); err != nil {
		t.Fatal(err)
	}
	return directory
}

func privateTestFile(t *testing.T, directory, name string, contents []byte) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := protectPrivateFile(path); err != nil {
		t.Fatal(err)
	}
	return path
}
