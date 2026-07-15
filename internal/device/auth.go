package device

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"slices"
	"sync"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

type ClientIdentityLookup interface {
	ClientIdentity(context.Context, domain.ID) (domain.ClientIdentity, error)
}

type ServerAuthenticator struct {
	DaemonID         domain.ID
	PrivateKey       ed25519.PrivateKey
	EndpointInstance []byte
	Lookup           ClientIdentityLookup
	Now              func() time.Time

	mu   sync.Mutex
	seen map[string]time.Time
}

func NewServerAuthenticator(daemon DaemonIdentity, endpointInstance []byte, lookup ClientIdentityLookup) (*ServerAuthenticator, error) {
	if domain.ValidateID(daemon.DeviceID) != nil || len(daemon.PrivateKey) != ed25519.PrivateKeySize || len(endpointInstance) != 32 || lookup == nil {
		return nil, domain.NewError(domain.CodeInvalidArgument, "invalid daemon authenticator")
	}
	return &ServerAuthenticator{
		DaemonID: daemon.DeviceID, PrivateKey: append(ed25519.PrivateKey(nil), daemon.PrivateKey...),
		EndpointInstance: append([]byte(nil), endpointInstance...), Lookup: lookup,
		Now: time.Now, seen: make(map[string]time.Time),
	}, nil
}

func (a *ServerAuthenticator) Handshake(ctx context.Context, rw interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
}) (AuthContext, error) {
	if err := setContextDeadline(rw, ctx, HandshakeTimeout); err != nil {
		return AuthContext{}, err
	}
	defer clearDeadline(rw)
	body, err := readFrame(rw)
	if err != nil {
		return AuthContext{}, authFailure()
	}
	var hello ClientHello
	if err := decodeStrict(body, &hello); err != nil || validateHello(hello) != nil {
		return AuthContext{}, authFailure()
	}
	if !a.markNonce(hello.ClientID, hello.ClientNonce) {
		return AuthContext{}, authFailure()
	}
	client, err := a.Lookup.ClientIdentity(ctx, hello.ClientID)
	if err != nil || client.Status != domain.ClientIdentityActive || len(client.PublicKey) != ed25519.PublicKeySize {
		return AuthContext{}, authFailure()
	}
	daemonPublic := a.PrivateKey.Public().(ed25519.PublicKey)
	daemonNonce, err := randomBytes(32)
	if err != nil {
		return AuthContext{}, authFailure()
	}
	proof := ServerProof{DaemonID: a.DaemonID, DaemonNonce: daemonNonce,
		EndpointInstance: append([]byte(nil), a.EndpointInstance...), NegotiatedMinor: min(hello.ProtocolMinor, ProtocolMinor)}
	proof.DaemonSignature = ed25519.Sign(a.PrivateKey, transcript(hello, proof, client.PublicKey, daemonPublic))
	if err := writeFrame(rw, proof); err != nil {
		return AuthContext{}, authFailure()
	}
	body, err = readFrame(rw)
	if err != nil {
		return AuthContext{}, authFailure()
	}
	var clientProof ClientProof
	if err := decodeStrict(body, &clientProof); err != nil || len(clientProof.ClientSignature) != ed25519SignatureSize ||
		!ed25519.Verify(ed25519.PublicKey(client.PublicKey), transcript(hello, proof, client.PublicKey, daemonPublic), clientProof.ClientSignature) {
		return AuthContext{}, authFailure()
	}
	granted := intersectCapabilities(hello.RequestedCapabilities, client.Caps)
	connectionID, err := randomBytes(16)
	if err != nil {
		return AuthContext{}, authFailure()
	}
	now := a.Now()
	if now.IsZero() {
		now = time.Now()
	}
	if err := writeFrame(rw, AuthOK{ConnectionID: connectionID, GrantedCapabilities: granted,
		ExpiresAt: now.Add(ConnectionLifetime), IdentityRevision: client.Revision}); err != nil {
		return AuthContext{}, authFailure()
	}
	return AuthContext{ClientID: client.ID, IdentityRevision: client.Revision,
		GrantedCapabilities: granted, ConnectionID: connectionID,
		AuthenticatedAt: now, ExpiresAt: now.Add(ConnectionLifetime)}, nil
}

func (a *ServerAuthenticator) markNonce(id domain.ID, nonce []byte) bool {
	now := time.Now()
	key := string(id) + ":" + string(nonce)
	a.mu.Lock()
	defer a.mu.Unlock()
	for seen, at := range a.seen {
		if now.Sub(at) > ConnectionLifetime {
			delete(a.seen, seen)
		}
	}
	if _, exists := a.seen[key]; exists || len(a.seen) >= 4096 {
		return false
	}
	a.seen[key] = now
	return true
}

type ClientAuthenticator struct {
	Identity              ClientIdentity
	RequestedCapabilities []domain.Capability
	Now                   func() time.Time
}

func (a ClientAuthenticator) Handshake(ctx context.Context, rw interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
}) (AuthOK, error) {
	if domain.ValidateID(a.Identity.ClientID) != nil || domain.ValidateID(a.Identity.DaemonID) != nil ||
		len(a.Identity.PrivateKey) != ed25519.PrivateKeySize || len(a.Identity.DaemonPublicKey) != ed25519.PublicKeySize {
		return AuthOK{}, authFailure()
	}
	if err := setContextDeadline(rw, ctx, HandshakeTimeout); err != nil {
		return AuthOK{}, err
	}
	defer clearDeadline(rw)
	nonce, err := randomBytes(32)
	if err != nil {
		return AuthOK{}, authFailure()
	}
	caps := a.RequestedCapabilities
	if len(caps) == 0 {
		caps = ownerCapabilities
	}
	caps, err = domain.CanonicalCapabilities(caps)
	if err != nil {
		return AuthOK{}, authFailure()
	}
	hello := ClientHello{ProtocolMajor: ProtocolMajor, ProtocolMinor: ProtocolMinor,
		ClientID: a.Identity.ClientID, ClientNonce: nonce, RequestedCapabilities: caps}
	if err := writeFrame(rw, hello); err != nil {
		return AuthOK{}, authFailure()
	}
	body, err := readFrame(rw)
	if err != nil {
		return AuthOK{}, authFailure()
	}
	var proof ServerProof
	if err := decodeStrict(body, &proof); err != nil || validateServerProof(proof) != nil || proof.DaemonID != a.Identity.DaemonID ||
		!ed25519.Verify(a.Identity.DaemonPublicKey, transcript(hello, proof, a.Identity.PublicKey(), a.Identity.DaemonPublicKey), proof.DaemonSignature) {
		return AuthOK{}, authFailure()
	}
	signature := ed25519.Sign(a.Identity.PrivateKey, transcript(hello, proof, a.Identity.PublicKey(), a.Identity.DaemonPublicKey))
	if err := writeFrame(rw, ClientProof{ClientSignature: signature}); err != nil {
		return AuthOK{}, authFailure()
	}
	body, err = readFrame(rw)
	if err != nil {
		return AuthOK{}, authFailure()
	}
	var accepted AuthOK
	if err := decodeStrict(body, &accepted); err != nil || len(accepted.ConnectionID) != 16 || accepted.ExpiresAt.IsZero() {
		return AuthOK{}, authFailure()
	}
	canonical, err := domain.CanonicalCapabilities(accepted.GrantedCapabilities)
	if err != nil || !slices.Equal(canonical, accepted.GrantedCapabilities) || !capabilitiesSubset(accepted.GrantedCapabilities, caps) || !nowBefore(a.Now, accepted.ExpiresAt) {
		return AuthOK{}, authFailure()
	}
	return accepted, nil
}

func intersectCapabilities(requested, allowed []domain.Capability) []domain.Capability {
	result := make([]domain.Capability, 0, len(requested))
	for _, requestedCapability := range requested {
		if domain.HasCapability(allowed, requestedCapability) {
			result = append(result, requestedCapability)
		}
	}
	return result
}

func capabilitiesSubset(values, allowed []domain.Capability) bool {
	for _, value := range values {
		if !domain.HasCapability(allowed, value) {
			return false
		}
	}
	return true
}

func authFailure() *domain.Error {
	return domain.NewError(domain.CodeUnauthenticated, "peer authentication failed")
}

func randomBytes(size int) ([]byte, error) {
	result := make([]byte, size)
	if _, err := rand.Read(result); err != nil {
		return nil, err
	}
	return result, nil
}

func NewEndpointInstance() ([]byte, error) { return randomBytes(32) }

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func nowBefore(now func() time.Time, deadline time.Time) bool {
	if now == nil {
		now = time.Now
	}
	return now().Before(deadline)
}

func setContextDeadline(rw any, ctx context.Context, fallback time.Duration) error {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return domain.NewError(domain.CodeDeadlineExceeded, "peer deadline exceeded")
		default:
		}
	}
	setter, ok := rw.(interface{ SetDeadline(time.Time) error })
	if !ok {
		return nil
	}
	deadline := time.Now().Add(fallback)
	if ctx != nil {
		if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
			deadline = ctxDeadline
		}
	}
	if err := setter.SetDeadline(deadline); err != nil {
		return domain.WrapError(domain.CodeDaemonUnavailable, "peer deadline could not be set", err)
	}
	return nil
}

func clearDeadline(rw any) {
	if setter, ok := rw.(interface{ SetDeadline(time.Time) error }); ok {
		_ = setter.SetDeadline(time.Time{})
	}
}
