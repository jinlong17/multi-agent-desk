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
		t.Fatalf("recovery set shape invalid: plaintext_count=%d hash_count=%d timestamp_match=%t", len(set.Plaintext), len(set.Hashes), set.GeneratedAt == now)
	}
	seenCodes := map[string]struct{}{}
	seenSalts := map[string]struct{}{}
	for index, code := range set.Plaintext {
		canonical, err := ParseRecoveryCode(strings.ToLower(code))
		if err != nil || canonical != code || !strings.HasPrefix(code, "MAD-RC1-") || len(code) != 47 {
			t.Fatalf("recovery code shape invalid: index=%d length=%d canonical_match=%t prefix_match=%t err_present=%t", index, len(code), canonical == code, strings.HasPrefix(code, "MAD-RC1-"), err != nil)
		}
		if _, duplicate := seenCodes[code]; duplicate {
			t.Fatal("recovery plaintext duplicated")
		}
		seenCodes[code] = struct{}{}
		hash := set.Hashes[index]
		if len(hash.Salt) != 16 || len(hash.Hash) != 32 || hash.Ordinal != index+1 {
			t.Fatalf("recovery hash shape invalid: index=%d salt_length=%d hash_length=%d ordinal_match=%t", index, len(hash.Salt), len(hash.Hash), hash.Ordinal == index+1)
		}
		if _, duplicate := seenSalts[string(hash.Salt)]; duplicate {
			t.Fatal("recovery salt duplicated")
		}
		seenSalts[string(hash.Salt)] = struct{}{}
	}
	for index, hostile := range []string{" " + set.Plaintext[0], set.Plaintext[0] + " ", strings.Replace(set.Plaintext[0], "-", "_", 1), "MAD-RC1-1111-1111-1111-1111-1111-1111-1111-1111"} {
		if _, err := ParseRecoveryCode(hostile); !errors.Is(err, ErrRecoveryInvalidOrRateLimited) {
			t.Fatalf("hostile recovery code accepted: index=%d length=%d err_present=%t", index, len(hostile), err != nil)
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
