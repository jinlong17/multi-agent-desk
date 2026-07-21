package generated

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestUsageWindowGeneratedOptionalAndEnumGolden(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve generated contract test path")
	}
	fixturePath := filepath.Join(filepath.Dir(source), "..", "..", "..", "..", "api", "openapi", "fixtures", "usage-window-v1.json")
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	fixture = bytes.TrimSpace(fixture)
	var window UsageWindowV1
	if err := json.Unmarshal(fixture, &window); err != nil {
		t.Fatal(err)
	}
	if !window.Kind.Valid() || !window.Unit.Valid() || window.UsedScaled == nil || *window.UsedScaled != "10" {
		t.Fatalf("generated enum/optional semantics drifted: %+v", window)
	}
	if window.LimitScaled != nil || window.ProviderLimitId != nil || window.RemainingBasisPoints != nil || window.ResetsAt != nil {
		t.Fatalf("omitted fields materialized: %+v", window)
	}
	encoded, err := json.Marshal(window)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, fixture) {
		t.Fatalf("generated Go golden drift\nwant %s\n got %s", fixture, encoded)
	}
}
