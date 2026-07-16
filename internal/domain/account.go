package domain

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

const (
	ProviderFake   = "fake"
	ProviderCodex  = "codex"
	ProviderClaude = "claude"
)

var selectorAliasPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,31}$`)

type Account struct {
	ID                    ID
	Provider              string
	DisplayName           string
	ProviderSubjectDigest string
	SubscriptionHint      string
	Internal              bool
	Enabled               bool
	Revision              int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func NewAccount(account Account) (Account, error) {
	if ValidateID(account.ID) != nil || !ValidProvider(account.Provider) ||
		len(account.DisplayName) < 1 || len(account.DisplayName) > 128 ||
		len(account.ProviderSubjectDigest) > 128 || len(account.SubscriptionHint) > 64 || account.Revision < 1 ||
		account.CreatedAt.IsZero() || account.UpdatedAt.Before(account.CreatedAt) {
		return Account{}, NewError(CodeInvalidArgument, "invalid account")
	}
	if account.Internal != (account.Provider == ProviderFake) {
		return Account{}, NewError(CodeInvalidArgument, "invalid account visibility")
	}
	return account, nil
}

func ValidProvider(provider string) bool {
	return provider == ProviderFake || provider == ProviderCodex || provider == ProviderClaude
}

func PublicProvider(provider string) bool {
	return provider == ProviderCodex || provider == ProviderClaude
}

// CanonicalSelectorAlias validates a stored alias. A leading @ belongs to the
// selector surface and is intentionally not accepted here.
func CanonicalSelectorAlias(alias string) (string, error) {
	if !selectorAliasPattern.MatchString(alias) {
		return "", NewError(CodeAliasInvalid, "profile alias is invalid")
	}
	return strings.ToLower(alias), nil
}

// ParseProfileSelector accepts exactly one optional leading @ and returns the
// canonical lookup key.
func ParseProfileSelector(selector string) (string, error) {
	if strings.HasPrefix(selector, "@@") {
		return "", NewError(CodeAliasInvalid, "profile selector is invalid")
	}
	selector = strings.TrimPrefix(selector, "@")
	return CanonicalSelectorAlias(selector)
}

type Availability string

const (
	AvailabilityAvailable   Availability = "available"
	AvailabilityLimited     Availability = "limited"
	AvailabilityUnavailable Availability = "unavailable"
	AvailabilityUnknown     Availability = "unknown"
)

func validAvailability(value Availability) bool {
	switch value {
	case AvailabilityAvailable, AvailabilityLimited, AvailabilityUnavailable, AvailabilityUnknown:
		return true
	default:
		return false
	}
}

type UsageSource string

const (
	UsageSourceOfficial      UsageSource = "official"
	UsageSourceCLIDerived    UsageSource = "cli_derived"
	UsageSourceLocalEstimate UsageSource = "local_estimate"
	UsageSourceUnavailable   UsageSource = "unavailable"
)

type UsageConfidence string

const (
	UsageConfidenceHigh   UsageConfidence = "high"
	UsageConfidenceMedium UsageConfidence = "medium"
	UsageConfidenceLow    UsageConfidence = "low"
	UsageConfidenceNone   UsageConfidence = "none"
)

type UsageWindowKind string

const (
	UsageWindowRolling      UsageWindowKind = "rolling"
	UsageWindowCalendar     UsageWindowKind = "calendar"
	UsageWindowSpendControl UsageWindowKind = "spend_control"
	UsageWindowSDKCredit    UsageWindowKind = "sdk_credit"
	UsageWindowUnknown      UsageWindowKind = "unknown"
)

type UsageWindow struct {
	ProviderLimitID  string
	Kind             UsageWindowKind
	Label            string
	DurationSeconds  *int64
	UsedValue        *float64
	LimitValue       *float64
	UsedPercent      *float64
	RemainingPercent *float64
	ResetsAt         *time.Time
}

type UsageSnapshot struct {
	ID                   ID
	AccountID            ID
	CredentialInstanceID ID
	DeviceID             ID
	Provider             string
	ProviderVersion      string
	Source               UsageSource
	Confidence           UsageConfidence
	Availability         Availability
	ObservedAt           time.Time
	StaleAt              time.Time
	RawReferenceHash     string
	Windows              []UsageWindow
}

func NewUsageSnapshot(snapshot UsageSnapshot) (UsageSnapshot, error) {
	if ValidateID(snapshot.ID) != nil || ValidateID(snapshot.AccountID) != nil ||
		ValidateID(snapshot.DeviceID) != nil || !ValidProvider(snapshot.Provider) ||
		snapshot.Provider == ProviderFake || snapshot.ProviderVersion == "" ||
		!validUsageSource(snapshot.Source) || !validUsageConfidence(snapshot.Confidence) ||
		!validAvailability(snapshot.Availability) || snapshot.ObservedAt.IsZero() ||
		snapshot.StaleAt.Before(snapshot.ObservedAt) || len(snapshot.RawReferenceHash) > 128 {
		return UsageSnapshot{}, NewError(CodeInvalidArgument, "invalid usage snapshot")
	}
	if snapshot.CredentialInstanceID != "" && ValidateID(snapshot.CredentialInstanceID) != nil {
		return UsageSnapshot{}, NewError(CodeInvalidArgument, "invalid usage credential")
	}
	for _, window := range snapshot.Windows {
		if err := validateUsageWindow(window); err != nil {
			return UsageSnapshot{}, err
		}
	}
	snapshot.Windows = append([]UsageWindow(nil), snapshot.Windows...)
	return snapshot, nil
}

func validUsageSource(value UsageSource) bool {
	switch value {
	case UsageSourceOfficial, UsageSourceCLIDerived, UsageSourceLocalEstimate, UsageSourceUnavailable:
		return true
	default:
		return false
	}
}

func validUsageConfidence(value UsageConfidence) bool {
	switch value {
	case UsageConfidenceHigh, UsageConfidenceMedium, UsageConfidenceLow, UsageConfidenceNone:
		return true
	default:
		return false
	}
}

func validateUsageWindow(window UsageWindow) error {
	if len(window.Label) < 1 || len(window.Label) > 128 || len(window.ProviderLimitID) > 128 ||
		!validUsageWindowKind(window.Kind) {
		return NewError(CodeInvalidArgument, "invalid usage window")
	}
	if window.DurationSeconds != nil && *window.DurationSeconds <= 0 {
		return NewError(CodeInvalidArgument, "invalid usage duration")
	}
	for _, value := range []*float64{window.UsedPercent, window.RemainingPercent} {
		if value != nil && (*value < 0 || *value > 100) {
			return NewError(CodeInvalidArgument, "invalid usage percentage")
		}
	}
	for _, value := range []*float64{window.UsedValue, window.LimitValue} {
		if value != nil && *value < 0 {
			return NewError(CodeInvalidArgument, "invalid usage value")
		}
	}
	return nil
}

func validUsageWindowKind(value UsageWindowKind) bool {
	switch value {
	case UsageWindowRolling, UsageWindowCalendar, UsageWindowSpendControl, UsageWindowSDKCredit, UsageWindowUnknown:
		return true
	default:
		return false
	}
}

// CanonicalSettings retains only valid non-secret JSON supplied internally.
func CanonicalSettings(settings json.RawMessage) (json.RawMessage, error) {
	if len(settings) == 0 {
		return json.RawMessage(`{}`), nil
	}
	if !json.Valid(settings) || len(settings) > 64*1024 {
		return nil, NewError(CodeInvalidArgument, "invalid profile settings")
	}
	return append(json.RawMessage(nil), settings...), nil
}
