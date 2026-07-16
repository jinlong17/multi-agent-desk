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
	DailyUsageBuckets []json.RawMessage `json:"dailyUsageBuckets"`
	Summary           json.RawMessage   `json:"summary"`
}

type usageDailyBucket struct {
	StartDate string `json:"startDate"`
	Tokens    *int64 `json:"tokens"`
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
	if len(envelope.Summary) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Summary), []byte("null")) {
		return UsageProjection{}, domain.NewError(domain.CodeProviderVersionUnsupported, "codex usage summary is missing")
	}
	var summary map[string]json.RawMessage
	if err := DecodeObject(envelope.Summary, &summary); err != nil {
		return UsageProjection{}, domain.NewError(domain.CodeProviderVersionUnsupported, "codex usage summary schema changed")
	}
	keys := make([]string, 0, len(summary))
	for key, raw := range summary {
		if key == "" || len(key) > 128 {
			return UsageProjection{}, domain.NewError(domain.CodeProviderProtocolError, "codex usage summary key is invalid")
		}
		if _, ok := allowedUsageSummaryKeys[key]; !ok {
			return UsageProjection{}, domain.NewError(domain.CodeProviderVersionUnsupported, "codex usage summary field is unmapped")
		}
		if !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			var value int64
			if json.Unmarshal(raw, &value) != nil {
				return UsageProjection{}, domain.NewError(domain.CodeProviderVersionUnsupported, "codex usage summary value schema changed")
			}
		}
		keys = append(keys, key)
	}
	for _, raw := range envelope.DailyUsageBuckets {
		var bucket usageDailyBucket
		if err := DecodeObject(raw, &bucket); err != nil || bucket.Tokens == nil || len(bucket.StartDate) != len("2006-01-02") {
			return UsageProjection{}, domain.NewError(domain.CodeProviderVersionUnsupported, "codex usage daily bucket schema changed")
		}
		if _, err := time.Parse("2006-01-02", bucket.StartDate); err != nil {
			return UsageProjection{}, domain.NewError(domain.CodeProviderVersionUnsupported, "codex usage daily bucket date changed")
		}
	}
	sort.Strings(keys)
	digest := sha256.Sum256(frame)
	return UsageProjection{Source: string(domain.UsageSourceOfficial), Confidence: string(domain.UsageConfidenceHigh),
		SourceVersion: version, CapabilityStatus: string(domain.UsageSupported), WindowCount: len(envelope.DailyUsageBuckets),
		SummaryKeys: keys, RawReferenceHash: hex.EncodeToString(digest[:]), ObservedAt: observedAt.UTC()}, nil
}

// DecodeApprovalServerRequest maps only the exact bounded identity fields of a
// Provider-initiated Approval request. Command/path/permission payloads are
// retained only through a digest and never enter summaries or logs.
func DecodeApprovalServerRequest(method string, frame []byte) (ApprovalRequest, error) {
	var itemID, approvalID, threadID, turnID string
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
		if nonNullJSON(request.ProposedExecpolicyAmendment) || nonNullJSON(request.ProposedNetworkPolicyAmendments) {
			return ApprovalRequest{}, domain.NewError(domain.CodeProviderVersionUnsupported, "codex policy-amendment Approval is disabled")
		}
		if request.TurnID == "" || request.ThreadID == "" || request.ItemID == "" || request.StartedAtMS < 0 {
			return ApprovalRequest{}, domain.NewError(domain.CodeApprovalUnknown, "codex command Approval request is incomplete")
		}
		itemID = request.ItemID
		threadID = request.ThreadID
		turnID = request.TurnID
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
		threadID = request.ThreadID
		turnID = request.TurnID
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
		threadID = request.ThreadID
		turnID = request.TurnID
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
	return ApprovalRequest{ProviderApprovalID: providerID, ThreadID: threadID, TurnID: turnID, Kind: kind,
		Summary: "Codex " + kind + " approval", PayloadDigest: hex.EncodeToString(digest[:])}, nil
}

func nonNullJSON(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	return len(trimmed) != 0 && !bytes.Equal(trimmed, []byte("null"))
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
		return ProviderEvent{Method: method, ThreadID: approval.ThreadID, TurnID: approval.TurnID,
			ProviderApprovalID: approval.ProviderApprovalID, Kind: approval.Kind,
			Summary: approval.Summary, PayloadDigest: approval.PayloadDigest, ObservedAt: observedAt.UTC()}, nil
	case MethodThreadStarted:
		var notification struct {
			Thread json.RawMessage `json:"thread"`
		}
		if err := DecodeObject(frame, &notification); err != nil {
			return ProviderEvent{}, err
		}
		threadID, err := boundedObjectID(notification.Thread, "thread")
		if err != nil {
			return ProviderEvent{}, err
		}
		return ProviderEvent{Method: method, ThreadID: threadID, ObservedAt: observedAt.UTC()}, nil
	case MethodTurnStarted, MethodTurnCompleted:
		var notification struct {
			ThreadID string          `json:"threadId"`
			Turn     json.RawMessage `json:"turn"`
		}
		if err := DecodeObject(frame, &notification); err != nil {
			return ProviderEvent{}, err
		}
		turnID, err := boundedObjectID(notification.Turn, "turn")
		if err != nil || notification.ThreadID == "" || len(notification.ThreadID) > 256 {
			return ProviderEvent{}, domain.NewError(domain.CodeProviderProtocolError, "codex turn notification is invalid")
		}
		return ProviderEvent{Method: method, ThreadID: notification.ThreadID, TurnID: turnID, ObservedAt: observedAt.UTC()}, nil
	case MethodAgentMessageDelta:
		var notification struct {
			Delta    string `json:"delta"`
			ItemID   string `json:"itemId"`
			ThreadID string `json:"threadId"`
			TurnID   string `json:"turnId"`
		}
		if err := DecodeObject(frame, &notification); err != nil {
			return ProviderEvent{}, err
		}
		if notification.ThreadID == "" || notification.TurnID == "" || notification.ItemID == "" ||
			len(notification.ThreadID) > 256 || len(notification.TurnID) > 256 || len(notification.ItemID) > 256 || len(notification.Delta) > MaxSummaryBytes {
			return ProviderEvent{}, domain.NewError(domain.CodeProviderProtocolError, "codex output notification is invalid")
		}
		return ProviderEvent{Method: method, ThreadID: notification.ThreadID, TurnID: notification.TurnID,
			ProviderItemID: notification.ItemID, Text: notification.Delta, ObservedAt: observedAt.UTC()}, nil
	case MethodConfigWarning:
		var notification struct {
			Details json.RawMessage `json:"details"`
			Summary json.RawMessage `json:"summary"`
		}
		if err := DecodeObject(frame, &notification); err != nil || len(notification.Details) == 0 || len(notification.Summary) == 0 {
			return ProviderEvent{}, domain.NewError(domain.CodeProviderProtocolError, "codex config warning is invalid")
		}
		return ProviderEvent{Method: method, ObservedAt: observedAt.UTC()}, nil
	case MethodRemoteControlStatus:
		var notification struct {
			EnvironmentID  json.RawMessage `json:"environmentId"`
			InstallationID json.RawMessage `json:"installationId"`
			ServerName     json.RawMessage `json:"serverName"`
			Status         json.RawMessage `json:"status"`
		}
		if err := DecodeObject(frame, &notification); err != nil || len(notification.EnvironmentID) == 0 ||
			len(notification.InstallationID) == 0 || len(notification.ServerName) == 0 || len(notification.Status) == 0 {
			return ProviderEvent{}, domain.NewError(domain.CodeProviderProtocolError, "codex remote-control status is invalid")
		}
		return ProviderEvent{Method: method, ObservedAt: observedAt.UTC()}, nil
	case MethodAccountRateLimitsUpdated:
		var notification struct {
			RateLimits json.RawMessage `json:"rateLimits"`
		}
		if err := DecodeObject(frame, &notification); err != nil || len(notification.RateLimits) == 0 {
			return ProviderEvent{}, domain.NewError(domain.CodeProviderProtocolError, "codex rate-limit update is invalid")
		}
		return ProviderEvent{Method: method, ObservedAt: observedAt.UTC()}, nil
	case MethodMCPStartupStatus:
		var notification struct {
			Error         json.RawMessage `json:"error"`
			FailureReason json.RawMessage `json:"failureReason"`
			Name          json.RawMessage `json:"name"`
			Status        json.RawMessage `json:"status"`
			ThreadID      string          `json:"threadId"`
		}
		if err := DecodeObject(frame, &notification); err != nil || len(notification.Error) == 0 || len(notification.FailureReason) == 0 ||
			len(notification.Name) == 0 || len(notification.Status) == 0 || notification.ThreadID == "" || len(notification.ThreadID) > 256 {
			return ProviderEvent{}, domain.NewError(domain.CodeProviderProtocolError, "codex MCP startup status is invalid")
		}
		return ProviderEvent{Method: method, ThreadID: notification.ThreadID, ObservedAt: observedAt.UTC()}, nil
	case MethodThreadStatusChanged:
		var notification struct {
			Status   json.RawMessage `json:"status"`
			ThreadID string          `json:"threadId"`
		}
		if err := DecodeObject(frame, &notification); err != nil || len(notification.Status) == 0 || notification.ThreadID == "" || len(notification.ThreadID) > 256 {
			return ProviderEvent{}, domain.NewError(domain.CodeProviderProtocolError, "codex thread status is invalid")
		}
		return ProviderEvent{Method: method, ThreadID: notification.ThreadID, ObservedAt: observedAt.UTC()}, nil
	case MethodItemStarted, MethodItemCompleted:
		var notification struct {
			CompletedAtMS *int64          `json:"completedAtMs"`
			Item          json.RawMessage `json:"item"`
			StartedAtMS   *int64          `json:"startedAtMs"`
			ThreadID      string          `json:"threadId"`
			TurnID        string          `json:"turnId"`
		}
		if err := DecodeObject(frame, &notification); err != nil || len(notification.Item) == 0 || notification.ThreadID == "" ||
			notification.TurnID == "" || len(notification.ThreadID) > 256 || len(notification.TurnID) > 256 ||
			(method == MethodItemStarted && (notification.StartedAtMS == nil || *notification.StartedAtMS < 0)) ||
			(method == MethodItemCompleted && (notification.CompletedAtMS == nil || *notification.CompletedAtMS < 0)) {
			return ProviderEvent{}, domain.NewError(domain.CodeProviderProtocolError, "codex item status is invalid")
		}
		return ProviderEvent{Method: method, ThreadID: notification.ThreadID, TurnID: notification.TurnID, ObservedAt: observedAt.UTC()}, nil
	case MethodThreadTokenUsage:
		var notification struct {
			ThreadID   string          `json:"threadId"`
			TokenUsage json.RawMessage `json:"tokenUsage"`
			TurnID     string          `json:"turnId"`
		}
		if err := DecodeObject(frame, &notification); err != nil || len(notification.TokenUsage) == 0 || notification.ThreadID == "" ||
			notification.TurnID == "" || len(notification.ThreadID) > 256 || len(notification.TurnID) > 256 {
			return ProviderEvent{}, domain.NewError(domain.CodeProviderProtocolError, "codex token-usage update is invalid")
		}
		return ProviderEvent{Method: method, ThreadID: notification.ThreadID, TurnID: notification.TurnID, ObservedAt: observedAt.UTC()}, nil
	case MethodServerRequestResolved:
		var notification struct {
			RequestID json.RawMessage `json:"requestId"`
			ThreadID  string          `json:"threadId"`
		}
		if err := DecodeObject(frame, &notification); err != nil || len(notification.RequestID) == 0 || notification.ThreadID == "" || len(notification.ThreadID) > 256 {
			return ProviderEvent{}, domain.NewError(domain.CodeProviderProtocolError, "codex resolved request notification is invalid")
		}
		return ProviderEvent{Method: method, ThreadID: notification.ThreadID, ObservedAt: observedAt.UTC()}, nil
	case MethodFileChangeOutputDelta:
		var notification struct {
			Delta    string `json:"delta"`
			ItemID   string `json:"itemId"`
			ThreadID string `json:"threadId"`
			TurnID   string `json:"turnId"`
		}
		if err := DecodeObject(frame, &notification); err != nil || notification.ItemID == "" || notification.ThreadID == "" || notification.TurnID == "" ||
			len(notification.Delta) > MaxFrameBytes || len(notification.ItemID) > 256 || len(notification.ThreadID) > 256 || len(notification.TurnID) > 256 {
			return ProviderEvent{}, domain.NewError(domain.CodeProviderProtocolError, "codex file-change output notification is invalid")
		}
		return ProviderEvent{Method: method, ThreadID: notification.ThreadID, TurnID: notification.TurnID,
			ProviderItemID: notification.ItemID, ObservedAt: observedAt.UTC()}, nil
	case MethodFileChangePatchUpdated:
		var notification struct {
			Changes  json.RawMessage `json:"changes"`
			ItemID   string          `json:"itemId"`
			ThreadID string          `json:"threadId"`
			TurnID   string          `json:"turnId"`
		}
		if err := DecodeObject(frame, &notification); err != nil || len(notification.Changes) == 0 || notification.ItemID == "" ||
			notification.ThreadID == "" || notification.TurnID == "" || len(notification.ItemID) > 256 || len(notification.ThreadID) > 256 || len(notification.TurnID) > 256 {
			return ProviderEvent{}, domain.NewError(domain.CodeProviderProtocolError, "codex file-change patch notification is invalid")
		}
		return ProviderEvent{Method: method, ThreadID: notification.ThreadID, TurnID: notification.TurnID,
			ProviderItemID: notification.ItemID, ObservedAt: observedAt.UTC()}, nil
	case MethodTurnDiffUpdated:
		var notification struct {
			Diff     json.RawMessage `json:"diff"`
			ThreadID string          `json:"threadId"`
			TurnID   string          `json:"turnId"`
		}
		if err := DecodeObject(frame, &notification); err != nil || len(notification.Diff) == 0 || notification.ThreadID == "" ||
			notification.TurnID == "" || len(notification.ThreadID) > 256 || len(notification.TurnID) > 256 {
			return ProviderEvent{}, domain.NewError(domain.CodeProviderProtocolError, "codex turn diff notification is invalid")
		}
		return ProviderEvent{Method: method, ThreadID: notification.ThreadID, TurnID: notification.TurnID, ObservedAt: observedAt.UTC()}, nil
	default:
		return ProviderEvent{}, domain.NewError(domain.CodeProviderProtocolError, "codex event method is unmapped")
	}
}

func boundedObjectID(frame json.RawMessage, kind string) (string, error) {
	var value map[string]json.RawMessage
	if len(frame) == 0 || json.Unmarshal(frame, &value) != nil {
		return "", domain.NewError(domain.CodeProviderProtocolError, "codex "+kind+" result is invalid")
	}
	var id string
	if raw := value["id"]; len(raw) == 0 || json.Unmarshal(raw, &id) != nil || id == "" || len(id) > 256 {
		return "", domain.NewError(domain.CodeProviderProtocolError, "codex "+kind+" identity is invalid")
	}
	return id, nil
}

func boundedString(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) > max {
		return value[:max]
	}
	return value
}
