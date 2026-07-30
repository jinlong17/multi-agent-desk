package device

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

func protectedPrivateFSTestDirectory(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := ProtectPrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestWritePrivateFileAtomicHasOneCreateOnlyWinner(t *testing.T) {
	root := protectedPrivateFSTestDirectory(t)
	path := filepath.Join(root, "writer.lock")
	const writers = 16
	start := make(chan struct{})
	results := make(chan error, writers)
	var wait sync.WaitGroup
	for index := 0; index < writers; index++ {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			results <- WritePrivateFileAtomic(path, []byte{byte(index)})
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
			continue
		}
		if code := domain.CodeOf(err); code != domain.CodeAlreadyExists && code != domain.CodeConflict {
			t.Fatalf("create-only loser code=%s err=%v", code, err)
		}
	}
	if winners != 1 {
		t.Fatalf("create-only winners=%d want=1", winners)
	}
	if err := VerifyPrivateFile(path); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || len(data) != 1 {
		t.Fatalf("winner payload=%v err=%v", data, err)
	}
}

func TestReplacePrivateFileAtomicPreservesBoundaryAndCleansTemporary(t *testing.T) {
	root := protectedPrivateFSTestDirectory(t)
	path := filepath.Join(root, "manifest.json")
	if err := WritePrivateFileAtomic(path, []byte("before")); err != nil {
		t.Fatal(err)
	}
	if err := ReplacePrivateFileAtomic(path, []byte("after")); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "after" {
		t.Fatalf("replacement=%q err=%v", data, err)
	}
	if err := VerifyPrivateFile(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path + ".replace"); !os.IsNotExist(err) {
		t.Fatalf("replacement residue err=%v", err)
	}
}

func TestReplacePrivateFileAtomicFailsClosedOnCandidateCollision(t *testing.T) {
	root := protectedPrivateFSTestDirectory(t)
	path := filepath.Join(root, "manifest.json")
	if err := WritePrivateFileAtomic(path, []byte("before")); err != nil {
		t.Fatal(err)
	}
	if err := WritePrivateFileAtomic(path+".replace", []byte("stale")); err != nil {
		t.Fatal(err)
	}
	if err := ReplacePrivateFileAtomic(path, []byte("after")); domain.CodeOf(err) != domain.CodeConflict {
		t.Fatalf("candidate collision code=%s err=%v", domain.CodeOf(err), err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "before" {
		t.Fatalf("destination changed=%q err=%v", data, err)
	}
}
