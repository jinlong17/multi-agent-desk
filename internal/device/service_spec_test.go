package device

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceSpecRendersAllSupportedPlatforms(t *testing.T) {
	root := filepath.Join(t.TempDir(), "device")
	executable := filepath.Join(t.TempDir(), "multidesk")
	for _, goos := range []string{"darwin", "linux", "windows"} {
		spec, err := RenderServiceSpec(goos, root, executable)
		if err != nil {
			t.Fatalf("%s: %v", goos, err)
		}
		if spec.Contents == "" || !strings.Contains(spec.Contents, "multidesk") || !strings.Contains(spec.Contents, executable) || !strings.Contains(spec.Contents, root) {
			t.Fatalf("%s spec incomplete: %q", goos, spec.Contents)
		}
	}
	if _, err := RenderServiceSpec("plan9", root, executable); err == nil {
		t.Fatal("unsupported service platform accepted")
	}
	if _, err := RenderServiceSpec("linux", "relative", executable); err == nil {
		t.Fatal("relative service root accepted")
	}
}
