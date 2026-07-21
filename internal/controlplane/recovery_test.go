package controlplane

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRecoveryCodeGenerationParsingHashingAndRateLimit(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	set, err := GenerateRecoveryCodeSet(now)
	if err != nil {
		t.Fatal(err)
	}
	defer set.ZeroPlaintext()
	if len(set.Plaintext) != 10 || len(set.Hashes) != 10 || set.GeneratedAt != now {
		t.Fatalf("recovery set shape=%+v", set)
	}
	seenCodes := map[string]struct{}{}
	seenSalts := map[string]struct{}{}
	for index, code := range set.Plaintext {
		canonical, err := ParseRecoveryCode(strings.ToLower(code))
		if err != nil || canonical != code || !strings.HasPrefix(code, "MAD-RC1-") || len(code) != 47 {
			t.Fatalf("code[%d]=%q canonical=%q err=%v", index, code, canonical, err)
		}
		if _, duplicate := seenCodes[code]; duplicate {
			t.Fatal("recovery plaintext duplicated")
		}
		seenCodes[code] = struct{}{}
		hash := set.Hashes[index]
		if len(hash.Salt) != 16 || len(hash.Hash) != 32 || hash.Ordinal != index+1 {
			t.Fatalf("hash[%d]=%+v", index, hash)
		}
		if _, duplicate := seenSalts[string(hash.Salt)]; duplicate {
			t.Fatal("recovery salt duplicated")
		}
		seenSalts[string(hash.Salt)] = struct{}{}
	}
	for _, hostile := range []string{" " + set.Plaintext[0], set.Plaintext[0] + " ", strings.Replace(set.Plaintext[0], "-", "_", 1), "MAD-RC1-1111-1111-1111-1111-1111-1111-1111-1111"} {
		if _, err := ParseRecoveryCode(hostile); !errors.Is(err, ErrRecoveryInvalidOrRateLimited) {
			t.Fatalf("hostile code accepted: %q err=%v", hostile, err)
		}
	}
	candidate := RecoveryCandidate{ID: set.Hashes[0].ID, Salt: set.Hashes[0].Salt, Hash: set.Hashes[0].Hash, Status: "active"}
	matched, err := MatchRecoveryCode(context.Background(), set.Plaintext[0], []RecoveryCandidate{candidate})
	if err != nil || matched != candidate.ID {
		t.Fatalf("matched=%q err=%v", matched, err)
	}
	candidate.Status = "consumed"
	if _, err := MatchRecoveryCode(context.Background(), set.Plaintext[0], []RecoveryCandidate{candidate}); !errors.Is(err, ErrRecoveryConsumed) {
		t.Fatalf("consumed recovery code accepted: %v", err)
	}
	limiter := &RecoveryLimiter{Now: func() time.Time { return now }}
	for index := 0; index < 5; index++ {
		if !limiter.Allow("192.0.2.1") {
			t.Fatalf("source limited early at %d", index)
		}
	}
	if limiter.Allow("192.0.2.1") {
		t.Fatal("source limit was not enforced")
	}
}
