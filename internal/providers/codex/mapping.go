package codex

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

type accountEnvelope struct {
	Account            json.RawMessage `json:"account"`
	RequiresOpenAIAuth bool            `json:"requiresOpenaiAuth"`
}

type accountBody struct {
	PlanType string `json:"planType"`
	Type     string `json:"type"`
}

func DecodeAccountResponse(frame []byte, observedAt time.Time) (AccountSnapshot, error) {
	var envelope accountEnvelope
	if err := DecodeObject(frame, &envelope); err != nil {
		return AccountSnapshot{}, err
	}
	if len(envelope.Account) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Account), []byte("null")) {
		return AccountSnapshot{RequiresOpenAIAuth: envelope.RequiresOpenAIAuth, ObservedAt: observedAt}, nil
	}
	var account accountBody
	if err := DecodeObject(envelope.Account, &account); err != nil {
		return AccountSnapshot{}, err
	}
	if account.PlanType == "" || account.Type == "" {
		return AccountSnapshot{}, domain.NewError(domain.CodeProviderProtocolError, "codex account response is incomplete")
	}
	if len(account.PlanType) > 128 || len(account.Type) > 64 {
		return AccountSnapshot{}, domain.NewError(domain.CodeFrameTooLarge, "codex account response is too large")
	}
	return AccountSnapshot{PlanType: strings.TrimSpace(account.PlanType), AccountType: strings.TrimSpace(account.Type),
		RequiresOpenAIAuth: envelope.RequiresOpenAIAuth, ObservedAt: observedAt.UTC()}, nil
}

type usageEnvelope struct {
	DailyUsageBuckets []json.RawMessage          `json:"dailyUsageBuckets"`
	Summary           map[string]json.RawMessage `json:"summary"`
}

var allowedUsageSummaryKeys = map[string]struct{}{
	"currentStreakDays":     {},
	"lifetimeTokens":        {},
	"longestRunningTurnSec": {},
	"longestStreakDays":     {},
	"peakDailyTokens":       {},
}

func DecodeUsageResponse(frame []byte, version string, observedAt time.Time) (UsageProjection, error) {
	var envelope usageEnvelope
	if err := DecodeObject(frame, &envelope); err != nil {
		return UsageProjection{}, err
	}
	keys := make([]string, 0, len(envelope.Summary))
	for key := range envelope.Summary {
		if key == "" || len(key) > 128 {
			return UsageProjection{}, domain.NewError(domain.CodeProviderProtocolError, "codex usage summary key is invalid")
		}
		if _, ok := allowedUsageSummaryKeys[key]; !ok {
			return UsageProjection{}, domain.NewError(domain.CodeProviderVersionUnsupported, "codex usage summary field is unmapped")
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return UsageProjection{Source: string(domain.UsageSourceOfficial), Confidence: string(domain.UsageConfidenceHigh),
		SourceVersion: version, CapabilityStatus: string(domain.UsageSupported), WindowCount: len(envelope.DailyUsageBuckets),
		SummaryKeys: keys, ObservedAt: observedAt.UTC()}, nil
}

// DecodeApprovalServerRequest maps only the exact bounded identity fields of a
// Provider-initiated Approval request. Command/path/permission payloads are
// retained only through a digest and never enter summaries or logs.
func DecodeApprovalServerRequest(method string, frame []byte) (ApprovalRequest, error) {
	var itemID, approvalID string
	switch method {
	case MethodApprovalCommand:
		var request struct {
			TurnID                          string          `json:"turnId"`
			ApprovalID                      *string         `json:"approvalId"`
			ThreadID                        string          `json:"threadId"`
			Command                         json.RawMessage `json:"command"`
			CommandActions                  json.RawMessage `json:"commandActions"`
			CWD                             json.RawMessage `json:"cwd"`
			EnvironmentID                   json.RawMessage `json:"environmentId"`
			ItemID                          string          `json:"itemId"`
			NetworkApprovalContext          json.RawMessage `json:"networkApprovalContext"`
			ProposedExecpolicyAmendment     json.RawMessage `json:"proposedExecpolicyAmendment"`
			ProposedNetworkPolicyAmendments json.RawMessage `json:"proposedNetworkPolicyAmendments"`
			Reason                          json.RawMessage `json:"reason"`
			StartedAtMS                     int64           `json:"startedAtMs"`
		}
		if err := DecodeObject(frame, &request); err != nil {
			return ApprovalRequest{}, err
		}
		if request.TurnID == "" || request.ThreadID == "" || request.ItemID == "" || request.StartedAtMS < 0 {
			return ApprovalRequest{}, domain.NewError(domain.CodeApprovalUnknown, "codex command Approval request is incomplete")
		}
		itemID = request.ItemID
		if request.ApprovalID != nil {
			approvalID = *request.ApprovalID
		}
	case MethodApprovalFileChange:
		var request struct {
			GrantRoot   json.RawMessage `json:"grantRoot"`
			ItemID      string          `json:"itemId"`
			Reason      json.RawMessage `json:"reason"`
			StartedAtMS int64           `json:"startedAtMs"`
			ThreadID    string          `json:"threadId"`
			TurnID      string          `json:"turnId"`
		}
		if err := DecodeObject(frame, &request); err != nil {
			return ApprovalRequest{}, err
		}
		if request.TurnID == "" || request.ThreadID == "" || request.ItemID == "" || request.StartedAtMS < 0 {
			return ApprovalRequest{}, domain.NewError(domain.CodeApprovalUnknown, "codex file-change Approval request is incomplete")
		}
		itemID = request.ItemID
	case MethodApprovalPermissions:
		var request struct {
			CWD           json.RawMessage `json:"cwd"`
			EnvironmentID json.RawMessage `json:"environmentId"`
			ItemID        string          `json:"itemId"`
			Permissions   json.RawMessage `json:"permissions"`
			Reason        json.RawMessage `json:"reason"`
			StartedAtMS   int64           `json:"startedAtMs"`
			ThreadID      string          `json:"threadId"`
			TurnID        string          `json:"turnId"`
		}
		if err := DecodeObject(frame, &request); err != nil {
			return ApprovalRequest{}, err
		}
		if request.TurnID == "" || request.ThreadID == "" || request.ItemID == "" || request.StartedAtMS < 0 || len(request.Permissions) == 0 {
			return ApprovalRequest{}, domain.NewError(domain.CodeApprovalUnknown, "codex permissions Approval request is incomplete")
		}
		itemID = request.ItemID
	default:
		return ApprovalRequest{}, domain.NewError(domain.CodeApprovalUnknown, "codex Approval request method is unsupported")
	}
	providerID := strings.TrimSpace(approvalID)
	if providerID == "" {
		providerID = strings.TrimSpace(itemID)
	}
	if providerID == "" || len(providerID) > 256 {
		return ApprovalRequest{}, domain.NewError(domain.CodeApprovalUnknown, "codex Approval request identity is invalid")
	}
	digest := sha256.Sum256(frame)
	kind := strings.TrimSuffix(strings.TrimPrefix(method, "item/"), "/requestApproval")
	return ApprovalRequest{ProviderApprovalID: providerID, Kind: kind,
		Summary: "Codex " + kind + " approval", PayloadDigest: hex.EncodeToString(digest[:])}, nil
}

func MapEvent(method string, frame []byte, version string, observedAt time.Time) (ProviderEvent, error) {
	switch method {
	case MethodAccountRead:
		if _, err := DecodeAccountResponse(frame, observedAt); err != nil {
			return ProviderEvent{}, err
		}
		return ProviderEvent{Method: method, ObservedAt: observedAt.UTC()}, nil
	case MethodAccountUsage:
		usage, err := DecodeUsageResponse(frame, version, observedAt)
		if err != nil {
			return ProviderEvent{}, err
		}
		return ProviderEvent{Method: method, Usage: &usage, ObservedAt: observedAt.UTC()}, nil
	case MethodApprovalCommand, MethodApprovalFileChange, MethodApprovalPermissions:
		approval, err := DecodeApprovalServerRequest(method, frame)
		if err != nil {
			return ProviderEvent{}, err
		}
		return ProviderEvent{Method: method, ProviderApprovalID: approval.ProviderApprovalID, Kind: approval.Kind,
			Summary: approval.Summary, PayloadDigest: approval.PayloadDigest, ObservedAt: observedAt.UTC()}, nil
	default:
		return ProviderEvent{}, domain.NewError(domain.CodeProviderProtocolError, "codex event method is unmapped")
	}
}

func boundedString(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) > max {
		return value[:max]
	}
	return value
}
