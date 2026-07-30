package transport

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

func TestBootstrapKeyEnvelopeAssertionJCSV1Vector(t *testing.T) {
	sealedAt := time.Date(2030, 3, 1, 0, 0, 0, 123456000, time.UTC)
	encoded, err := BootstrapKeyEnvelopeAssertionJCSV1(1, 1, 7, sealedAt, "pending")
	if err != nil {
		t.Fatal(err)
	}
	want := `{"formatVersion":1,"keyRevision":1,"recordRevision":7,"sealedAt":"2030-03-01T00:00:00.123456Z","status":"pending"}`
	if string(encoded) != want {
		t.Fatalf("canonical payload=%s", encoded)
	}
	digest := sha256.Sum256(encoded)
	if got := hex.EncodeToString(digest[:]); got != "7751a4ba39de9fe89e03c63e8c8d946b03431ac3f0f185aa15b07b671bc89efe" {
		t.Fatalf("canonical digest=%s", got)
	}
}
