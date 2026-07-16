package domain

import (
	"strings"
	"testing"
	"time"
)

func TestProfileSelectorAliasIsBoundedAndCanonical(t *testing.T) {
	valid := map[string]string{"A": "a", "team_1": "team_1", "Codex-West": "codex-west"}
	for value, expected := range valid {
		got, err := CanonicalSelectorAlias(value)
		if err != nil || got != expected {
			t.Fatalf("alias %q got %q, %v", value, got, err)
		}
		selector, err := ParseProfileSelector("@" + value)
		if err != nil || selector != expected {
			t.Fatalf("selector %q got %q, %v", value, selector, err)
		}
	}
	invalid := []string{"", "@A", "@@A", "1A", "a/b", "a b", "a;rm", "账号", strings.Repeat("a", 33)}
	for _, value := range invalid {
		if _, err := CanonicalSelectorAlias(value); CodeOf(err) != CodeAliasInvalid {
			t.Fatalf("invalid alias %q got %v", value, err)
		}
	}
}

func TestAccountMetadataIsBounded(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	account := Account{ID: testID("account", fixedHex), Provider: ProviderCodex,
		DisplayName: "A", Enabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now}
	if _, err := NewAccount(account); err != nil {
		t.Fatalf("valid account: %v", err)
	}
	account.ProviderSubjectDigest = strings.Repeat("x", 129)
	if _, err := NewAccount(account); CodeOf(err) != CodeInvalidArgument {
		t.Fatalf("oversized provider digest got %v", err)
	}
}

func TestUsageValidationKeepsUnknownOptional(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	snapshot := UsageSnapshot{ID: testID("usage", fixedHex), AccountID: testID("account", fixedHex),
		DeviceID: testID("device", fixedHex), Provider: ProviderCodex, ProviderVersion: "test",
		Source: UsageSourceUnavailable, Confidence: UsageConfidenceNone,
		Availability: AvailabilityUnknown, ObservedAt: now, StaleAt: now,
		Windows: []UsageWindow{{Kind: UsageWindowUnknown, Label: "Unknown"}}}
	validated, err := NewUsageSnapshot(snapshot)
	if err != nil || validated.Windows[0].UsedPercent != nil {
		t.Fatalf("unknown usage changed: %+v, %v", validated, err)
	}
	bad := 101.0
	snapshot.Windows[0].UsedPercent = &bad
	if _, err := NewUsageSnapshot(snapshot); CodeOf(err) != CodeInvalidArgument {
		t.Fatalf("invalid percent got %v", err)
	}
}
