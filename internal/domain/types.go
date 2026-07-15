package domain

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// ID is an opaque local entity identifier.
type ID string

// NewID creates a random identifier with a stable entity prefix.
func NewID(prefix string) (ID, error) {
	if prefix == "" || len(prefix) > 24 {
		return "", NewError(CodeInvalidArgument, "invalid identifier prefix")
	}
	for _, r := range prefix {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return "", NewError(CodeInvalidArgument, "invalid identifier prefix")
		}
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", WrapError(CodeConflict, "identifier generation failed", err)
	}
	return ID(prefix + "_" + hex.EncodeToString(b)), nil
}

// ValidateID validates an opaque identifier without inferring ownership from
// its prefix.
func ValidateID(id ID) error {
	parts := strings.Split(string(id), "_")
	if len(parts) < 2 {
		return NewError(CodeInvalidArgument, "invalid identifier")
	}
	randomPart := parts[len(parts)-1]
	decoded, err := hex.DecodeString(randomPart)
	if err != nil || len(decoded) != 16 {
		return NewError(CodeInvalidArgument, "invalid identifier")
	}
	return nil
}

// Capability names one server-authorized local action.
type Capability string

const (
	CapabilityMetadataRead          Capability = "metadata.read"
	CapabilitySessionObserve        Capability = "session.observe"
	CapabilityVaultControl          Capability = "vault.control"
	CapabilitySessionStart          Capability = "session.start"
	CapabilitySessionControlAcquire Capability = "session.control.acquire"
	CapabilitySessionControl        Capability = "session.control"
	CapabilityTerminalControl       Capability = "terminal.control"
	CapabilitySessionResume         Capability = "session.resume"
	CapabilityClientAdmin           Capability = "client.admin"
)

var validCapabilities = map[Capability]struct{}{
	CapabilityMetadataRead:          {},
	CapabilitySessionObserve:        {},
	CapabilityVaultControl:          {},
	CapabilitySessionStart:          {},
	CapabilitySessionControlAcquire: {},
	CapabilitySessionControl:        {},
	CapabilityTerminalControl:       {},
	CapabilitySessionResume:         {},
	CapabilityClientAdmin:           {},
}

// CanonicalCapabilities validates, de-duplicates, and sorts capabilities.
func CanonicalCapabilities(values []Capability) ([]Capability, error) {
	seen := make(map[Capability]struct{}, len(values))
	for _, capability := range values {
		if _, ok := validCapabilities[capability]; !ok {
			return nil, NewError(CodeInvalidArgument, "unknown capability")
		}
		seen[capability] = struct{}{}
	}
	result := make([]Capability, 0, len(seen))
	for capability := range seen {
		result = append(result, capability)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

// CapabilitiesJSON returns a deterministic JSON encoding.
func CapabilitiesJSON(values []Capability) ([]byte, error) {
	canonical, err := CanonicalCapabilities(values)
	if err != nil {
		return nil, err
	}
	return json.Marshal(canonical)
}

// HasCapability checks a frozen capability snapshot.
func HasCapability(values []Capability, target Capability) bool {
	for _, capability := range values {
		if capability == target {
			return true
		}
	}
	return false
}

type DeviceKind string

const DeviceKindDaemon DeviceKind = "daemon"

type Device struct {
	ID               ID
	Kind             DeviceKind
	DisplayName      string
	SigningPublicKey []byte
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ClientIdentityStatus string

const (
	ClientIdentityActive  ClientIdentityStatus = "active"
	ClientIdentityRevoked ClientIdentityStatus = "revoked"
)

type ClientIdentity struct {
	ID        ID
	Name      string
	PublicKey []byte
	Revision  int64
	Status    ClientIdentityStatus
	Caps      []Capability
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Workspace struct {
	ID        ID
	DeviceID  ID
	Path      string
	Label     string
	Tags      []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type RuntimeProfile struct {
	ID        ID
	DeviceID  ID
	Name      string
	Provider  string
	Settings  json.RawMessage
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CredentialStatus string

const (
	CredentialHealthy CredentialStatus = "healthy"
	CredentialExpired CredentialStatus = "expired"
	CredentialRevoked CredentialStatus = "revoked"
	CredentialUnknown CredentialStatus = "unknown"
)

type CredentialInstance struct {
	ID                 ID
	DeviceID           ID
	Provider           string
	AuthMethod         string
	SecretRef          string
	Status             CredentialStatus
	CredentialRevision int64
	SecretDigest       string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type AttachmentMode string

const (
	AttachmentObserver   AttachmentMode = "observer"
	AttachmentController AttachmentMode = "controller"
)

type SessionAttachment struct {
	ID             ID
	SessionID      ID
	ClientDeviceID ID
	Mode           AttachmentMode
	ConnectedAt    time.Time
	LastSeenAt     time.Time
}

type RuntimeEvent struct {
	ID        ID
	SessionID ID
	Sequence  int64
	Kind      string
	Metadata  json.RawMessage
	CreatedAt time.Time
}

type AuditEvent struct {
	ID         ID
	ActorID    ID
	Action     string
	TargetType string
	TargetID   ID
	Decision   string
	ErrorCode  ErrorCode
	Metadata   json.RawMessage
	CreatedAt  time.Time
}
