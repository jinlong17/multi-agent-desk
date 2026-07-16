package codex

import "time"

const (
	ProviderName       = "codex"
	MaxFrameBytes      = 256 * 1024
	MaxProbeOutput     = 16 * 1024
	MaxSummaryBytes    = 2048
	MaxMethodBytes     = 128
	MaxSchemaFiles     = 4096
	MaxSchemaFileBytes = 2 * 1024 * 1024
)

const (
	MethodInitialize               = "initialize"
	MethodInitialized              = "initialized"
	MethodAccountRead              = "account/read"
	MethodAccountRateLimits        = "account/rateLimits/read"
	MethodAccountUsage             = "account/usage/read"
	MethodApprovalCommand          = "item/commandExecution/requestApproval"
	MethodApprovalFileChange       = "item/fileChange/requestApproval"
	MethodApprovalPermissions      = "item/permissions/requestApproval"
	MethodRefreshAuthTokens        = "account/chatgptAuthTokens/refresh"
	MethodThreadStart              = "thread/start"
	MethodTurnStart                = "turn/start"
	MethodTurnInterrupt            = "turn/interrupt"
	MethodThreadStarted            = "thread/started"
	MethodTurnStarted              = "turn/started"
	MethodTurnCompleted            = "turn/completed"
	MethodAgentMessageDelta        = "item/agentMessage/delta"
	MethodAccountRateLimitsUpdated = "account/rateLimits/updated"
	MethodConfigWarning            = "configWarning"
	MethodItemStarted              = "item/started"
	MethodItemCompleted            = "item/completed"
	MethodFileChangeOutputDelta    = "item/fileChange/outputDelta"
	MethodFileChangePatchUpdated   = "item/fileChange/patchUpdated"
	MethodMCPStartupStatus         = "mcpServer/startupStatus/updated"
	MethodRemoteControlStatus      = "remoteControl/status/changed"
	MethodServerRequestResolved    = "serverRequest/resolved"
	MethodThreadStatusChanged      = "thread/status/changed"
	MethodThreadTokenUsage         = "thread/tokenUsage/updated"
	MethodTurnDiffUpdated          = "turn/diff/updated"
)

// BinaryDescriptor is diagnostic metadata only. It never contains auth or
// account material.
type BinaryDescriptor struct {
	Provider          string
	Path              string
	Version           string
	Platform          string
	Architecture      string
	SchemaFingerprint string
}

type CapabilityStatus string

const (
	CapabilitySupported   CapabilityStatus = "supported"
	CapabilityDowngraded  CapabilityStatus = "downgraded"
	CapabilityUnsupported CapabilityStatus = "unsupported"
)

type CapabilitySet struct {
	Provider          string
	BinaryPath        string
	Version           string
	Platform          string
	Architecture      string
	SchemaFingerprint string
	Methods           []string
	Experimental      []string
	Status            CapabilityStatus
}

type CompatibilityRow struct {
	Version           string
	SchemaFingerprint string
	Methods           []string
	Experimental      []string
}

// AccountSnapshot intentionally excludes email and other raw Provider
// identity. It is suitable for a bounded diagnostic projection.
type AccountSnapshot struct {
	PlanType           string
	AccountType        string
	RequiresOpenAIAuth bool
	ObservedAt         time.Time
}

type UsageProjection struct {
	Source           string
	Confidence       string
	SourceVersion    string
	CapabilityStatus string
	WindowCount      int
	SummaryKeys      []string
	RawReferenceHash string
	ObservedAt       time.Time
}

type ApprovalRequest struct {
	ProviderApprovalID string
	ThreadID           string
	TurnID             string
	Kind               string
	Summary            string
	PayloadDigest      string
}

type ProviderEvent struct {
	Method             string
	ThreadID           string
	TurnID             string
	ProviderItemID     string
	Text               string
	ProviderApprovalID string
	Kind               string
	Summary            string
	PayloadDigest      string
	Usage              *UsageProjection
	ObservedAt         time.Time
}
