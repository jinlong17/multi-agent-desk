package device

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

const (
	ProtocolMajor      = 1
	ProtocolMinor      = 0
	MaxFrameBytes      = 256 * 1024
	MaxRequestIDBytes  = 128
	MaxMethodBytes     = 128
	MaxCapabilities    = 32
	HandshakeTimeout   = 5 * time.Second
	ConnectionLifetime = 30 * time.Minute
)

type ClientHello struct {
	ProtocolMajor         int                 `json:"protocol_major"`
	ProtocolMinor         int                 `json:"protocol_minor"`
	ClientID              domain.ID           `json:"client_id"`
	ClientNonce           []byte              `json:"client_nonce"`
	RequestedCapabilities []domain.Capability `json:"requested_capabilities"`
}

type ServerProof struct {
	DaemonID         domain.ID `json:"daemon_id"`
	DaemonNonce      []byte    `json:"daemon_nonce"`
	EndpointInstance []byte    `json:"endpoint_instance"`
	NegotiatedMinor  int       `json:"negotiated_minor"`
	DaemonSignature  []byte    `json:"daemon_signature"`
}

type ClientProof struct {
	ClientSignature []byte `json:"client_signature"`
}

type AuthOK struct {
	ConnectionID        []byte              `json:"connection_id"`
	GrantedCapabilities []domain.Capability `json:"granted_capabilities"`
	ExpiresAt           time.Time           `json:"expires_at"`
	IdentityRevision    int64               `json:"identity_revision"`
}

type Request struct {
	ProtocolMajor  int             `json:"protocol_major"`
	RequestID      string          `json:"request_id"`
	Method         string          `json:"method"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	LeaseRevision  *int64          `json:"lease_revision,omitempty"`
	Body           json.RawMessage `json:"body,omitempty"`
}

type Response struct {
	ProtocolMajor int             `json:"protocol_major"`
	RequestID     string          `json:"request_id"`
	OK            bool            `json:"ok"`
	Result        json.RawMessage `json:"result,omitempty"`
	Error         *WireError      `json:"error,omitempty"`
}

type Event struct {
	ProtocolMajor int             `json:"protocol_major"`
	StreamID      string          `json:"stream_id"`
	Sequence      int64           `json:"sequence"`
	Kind          string          `json:"kind"`
	Truncated     bool            `json:"truncated"`
	Body          json.RawMessage `json:"body,omitempty"`
}

type WireError struct {
	Code     domain.ErrorCode `json:"code"`
	Message  string           `json:"message"`
	Metadata json.RawMessage  `json:"metadata,omitempty"`
}

type AuthContext struct {
	ClientID            domain.ID
	IdentityRevision    int64
	GrantedCapabilities []domain.Capability
	ConnectionID        []byte
	AuthenticatedAt     time.Time
	ExpiresAt           time.Time
}

func (a AuthContext) Has(capability domain.Capability) bool {
	return domain.HasCapability(a.GrantedCapabilities, capability)
}

func (a AuthContext) ValidAt(now time.Time) bool {
	return !now.Before(a.AuthenticatedAt) && now.Before(a.ExpiresAt)
}

func marshalFrame(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, domain.WrapError(domain.CodeInvalidArgument, "message could not be encoded", err)
	}
	if len(encoded) > MaxFrameBytes {
		return nil, domain.NewError(domain.CodeFrameTooLarge, "message exceeds frame limit")
	}
	return encoded, nil
}

func writeFrame(w io.Writer, value any) error {
	body, err := marshalFrame(value)
	if err != nil {
		return err
	}
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(body)))
	if err := writeAll(w, length[:]); err != nil {
		return domain.WrapError(domain.CodeDaemonUnavailable, "message header could not be written", err)
	}
	if err := writeAll(w, body); err != nil {
		return domain.WrapError(domain.CodeDaemonUnavailable, "message could not be written", err)
	}
	return nil
}

func readFrame(r io.Reader) ([]byte, error) {
	var length [4]byte
	if _, err := io.ReadFull(r, length[:]); err != nil {
		return nil, domain.WrapError(domain.CodeDaemonUnavailable, "message header could not be read", err)
	}
	size := binary.BigEndian.Uint32(length[:])
	if size == 0 || size > MaxFrameBytes {
		return nil, domain.NewError(domain.CodeFrameTooLarge, "message exceeds frame limit")
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, domain.WrapError(domain.CodeDaemonUnavailable, "message could not be read", err)
	}
	if err := validateJSON(body); err != nil {
		return nil, domain.NewError(domain.CodeInvalidArgument, "message JSON is invalid")
	}
	return body, nil
}

func writeAll(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 || n > len(data) {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	return nil
}

// validateJSON rejects duplicate object keys and trailing JSON before a typed
// decoder is allowed to interpret a frame.
func validateJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := consumeJSONValue(decoder); err != nil {
		return err
	}
	var token any
	if err := decoder.Decode(&token); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	return nil
}

func consumeJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			seen := map[string]struct{}{}
			for decoder.More() {
				key, ok := mustStringToken(decoder.Token())
				if !ok {
					return errors.New("object key is not a string")
				}
				if _, exists := seen[key]; exists {
					return errors.New("duplicate object key")
				}
				seen[key] = struct{}{}
				if err := consumeJSONValue(decoder); err != nil {
					return err
				}
			}
			close, err := decoder.Token()
			if err != nil || close != json.Delim('}') {
				return errors.New("object is not closed")
			}
		case '[':
			for decoder.More() {
				if err := consumeJSONValue(decoder); err != nil {
					return err
				}
			}
			close, err := decoder.Token()
			if err != nil || close != json.Delim(']') {
				return errors.New("array is not closed")
			}
		default:
			return errors.New("unexpected JSON delimiter")
		}
	}
	return nil
}

func mustStringToken(token json.Token, err error) (string, bool) {
	if err != nil {
		return "", false
	}
	value, ok := token.(string)
	return value, ok
}

func validateHello(hello ClientHello) error {
	if hello.ProtocolMajor != ProtocolMajor || hello.ProtocolMinor < 0 || hello.ProtocolMinor > ProtocolMinor {
		return domain.NewError(domain.CodeUnsupportedVersion, "protocol version is unsupported")
	}
	if domain.ValidateID(hello.ClientID) != nil || len(hello.ClientNonce) != 32 || len(hello.RequestedCapabilities) > MaxCapabilities {
		return domain.NewError(domain.CodeUnauthenticated, "peer authentication failed")
	}
	canonical, err := domain.CanonicalCapabilities(hello.RequestedCapabilities)
	if err != nil || !slices.Equal(canonical, hello.RequestedCapabilities) {
		return domain.NewError(domain.CodeUnauthenticated, "peer authentication failed")
	}
	return nil
}

func validateServerProof(proof ServerProof) error {
	if domain.ValidateID(proof.DaemonID) != nil || len(proof.DaemonNonce) != 32 || len(proof.EndpointInstance) != 32 ||
		proof.NegotiatedMinor < 0 || proof.NegotiatedMinor > ProtocolMinor || len(proof.DaemonSignature) != ed25519SignatureSize {
		return domain.NewError(domain.CodeUnauthenticated, "peer authentication failed")
	}
	return nil
}

const ed25519SignatureSize = 64

func transcript(hello ClientHello, proof ServerProof, clientPublic, daemonPublic []byte) []byte {
	clientDigest := sha256.Sum256(clientPublic)
	daemonDigest := sha256.Sum256(daemonPublic)
	value := struct {
		Domain                string              `json:"domain"`
		Major                 int                 `json:"major"`
		Minor                 int                 `json:"minor"`
		ClientID              domain.ID           `json:"client_id"`
		DaemonID              domain.ID           `json:"daemon_id"`
		ClientKeyDigest       [32]byte            `json:"client_key_digest"`
		DaemonKeyDigest       [32]byte            `json:"daemon_key_digest"`
		ClientNonce           []byte              `json:"client_nonce"`
		DaemonNonce           []byte              `json:"daemon_nonce"`
		EndpointInstance      []byte              `json:"endpoint_instance"`
		RequestedCapabilities []domain.Capability `json:"requested_capabilities"`
	}{"multidesk/device-handshake/v1", hello.ProtocolMajor, proof.NegotiatedMinor, hello.ClientID,
		proof.DaemonID, clientDigest, daemonDigest, hello.ClientNonce, proof.DaemonNonce,
		proof.EndpointInstance, hello.RequestedCapabilities}
	encoded, _ := json.Marshal(value)
	return encoded
}
