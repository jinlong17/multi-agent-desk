package domain

import (
	"encoding/hex"
	"strings"
	"time"
)

const (
	ProviderFake   = "fake"
	ProviderCodex  = "codex"
	ProviderClaude = "claude"
)

const (
	AuthMethodFake        = "fake"
	AuthMethodInteractive = "interactive"
	AuthMethodDeviceCode  = "device_code"
)

func ProviderKnown(provider string) bool {
	return provider == ProviderFake || provider == ProviderCodex || provider == ProviderClaude
}

func AuthMethodKnown(method string) bool {
	switch method {
	case AuthMethodFake, AuthMethodInteractive, AuthMethodDeviceCode:
		return true
	default:
		return false
	}
}

func ValidateAccount(account Account) error {
	if err := ValidateID(account.ID); err != nil {
		return err
	}
	if !PublicProvider(account.Provider) || account.Internal || strings.TrimSpace(account.DisplayName) == "" || len(account.DisplayName) > 128 ||
		len(account.SubscriptionHint) > 64 || account.Revision < 1 ||
		!validCreatedUpdated(account.CreatedAt, account.UpdatedAt) {
		return NewError(CodeInvalidArgument, "invalid account")
	}
	if account.ProviderSubjectDigest != "" {
		decoded, err := hex.DecodeString(account.ProviderSubjectDigest)
		if err != nil || len(decoded) != 32 {
			return NewError(CodeInvalidArgument, "account subject digest is invalid")
		}
	}
	return nil
}

func ValidateApproval(approval Approval) error {
	for _, id := range []ID{approval.ID, approval.SessionID} {
		if err := ValidateID(id); err != nil {
			return err
		}
	}
	if approval.ProviderApprovalID == "" || len(approval.ProviderApprovalID) > 256 || approval.Kind == "" || len(approval.Kind) > 64 ||
		len(approval.PayloadDigest) != 64 || !isHexDigest(approval.PayloadDigest) || len(approval.Summary) > 2048 ||
		!ApprovalStatusKnown(approval.Status) || approval.IdempotencyKey == "" || len(approval.IdempotencyKey) > 128 || approval.RequestedAt.IsZero() {
		return NewError(CodeInvalidArgument, "invalid approval")
	}
	if approval.RespondedByDeviceID != "" {
		if err := ValidateID(approval.RespondedByDeviceID); err != nil {
			return err
		}
	}
	if approval.RespondedAt != nil && approval.RespondedAt.Before(approval.RequestedAt) {
		return NewError(CodeInvalidArgument, "approval response precedes request")
	}
	if approval.ResponseState == "" {
		approval.ResponseState = ApprovalResponseIdle
	}
	if approval.ResponseState != ApprovalResponseIdle && approval.ResponseState != ApprovalResponseDispatching && approval.ResponseState != ApprovalResponseWritten && approval.ResponseState != ApprovalResponseAmbiguous {
		return NewError(CodeInvalidArgument, "approval response state is invalid")
	}
	if approval.RequestedDecision != "" && approval.RequestedDecision != ApprovalDecisionApprove && approval.RequestedDecision != ApprovalDecisionDeny && approval.RequestedDecision != ApprovalDecisionCancel {
		return NewError(CodeInvalidArgument, "approval decision is invalid")
	}
	if approval.Status == ApprovalPending && (approval.RespondedByDeviceID != "" || approval.RespondedAt != nil) {
		return NewError(CodeInvalidArgument, "pending approval cannot have a response")
	}
	if approval.Status != ApprovalPending && approval.RespondedAt == nil {
		return NewError(CodeInvalidArgument, "terminal approval requires a response time")
	}
	if (approval.Status == ApprovalApproved || approval.Status == ApprovalDenied || approval.Status == ApprovalCancelled) && approval.RespondedByDeviceID == "" {
		return NewError(CodeInvalidArgument, "operator approval requires a responder")
	}
	return nil
}

func ApprovalStatusKnown(status ApprovalStatus) bool {
	switch status {
	case ApprovalPending, ApprovalApproved, ApprovalDenied, ApprovalExpired, ApprovalCancelled:
		return true
	default:
		return false
	}
}

func ValidateUsageSnapshot(snapshot UsageSnapshot) error {
	for _, id := range []ID{snapshot.ID, snapshot.AccountID, snapshot.DeviceID} {
		if err := ValidateID(id); err != nil {
			return err
		}
	}
	if snapshot.Provider != ProviderCodex || snapshot.WindowKind == "" || len(snapshot.WindowKind) > 64 || snapshot.ObservedAt.IsZero() ||
		snapshot.SourceVersion == "" || len(snapshot.SourceVersion) > 128 || !UsageSourceKnown(snapshot.Source) ||
		!UsageConfidenceKnown(snapshot.Confidence) || !UsageCapabilityStatusKnown(snapshot.CapabilityStatus) || len(snapshot.ErrorCode) > 64 {
		return NewError(CodeInvalidArgument, "invalid usage snapshot")
	}
	for _, value := range []*float64{snapshot.UsedValue, snapshot.LimitValue} {
		if value != nil && *value < 0 {
			return NewError(CodeInvalidArgument, "usage value cannot be negative")
		}
	}
	if snapshot.UsedPercent != nil && (*snapshot.UsedPercent < 0 || *snapshot.UsedPercent > 100) {
		return NewError(CodeInvalidArgument, "usage percent is invalid")
	}
	if snapshot.ResetsAt != nil && snapshot.ResetsAt.Before(snapshot.ObservedAt) {
		return NewError(CodeInvalidArgument, "usage reset precedes observation")
	}
	if snapshot.RawReferenceHash != "" && !isHexDigest(snapshot.RawReferenceHash) {
		return NewError(CodeInvalidArgument, "usage reference hash is invalid")
	}
	return nil
}

func UsageSourceKnown(source UsageSource) bool {
	switch source {
	case UsageSourceOfficial, UsageSourceCLIDerived, UsageSourceLocalEstimate, UsageSourceUnofficial:
		return true
	default:
		return false
	}
}

func UsageConfidenceKnown(confidence UsageConfidence) bool {
	switch confidence {
	case UsageConfidenceHigh, UsageConfidenceMedium, UsageConfidenceLow:
		return true
	default:
		return false
	}
}

func UsageCapabilityStatusKnown(status UsageCapabilityStatus) bool {
	switch status {
	case UsageSupported, UsageUnavailable, UsageSchemaChanged, UsageError:
		return true
	default:
		return false
	}
}

func isHexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validCreatedUpdated(createdAt, updatedAt time.Time) bool {
	return !createdAt.IsZero() && !updatedAt.IsZero() && !updatedAt.Before(createdAt)
}
