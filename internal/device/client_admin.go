package device

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

type ClientAdministrationStore interface {
	Device(context.Context) (domain.Device, error)
	ClientIdentity(context.Context, domain.ID) (domain.ClientIdentity, error)
	CreateClientIdentity(context.Context, domain.ClientIdentity) error
	RotateClientIdentity(context.Context, domain.ID, int64, []byte, time.Time) (domain.ClientIdentity, error)
	RevokeClientIdentity(context.Context, domain.ID, int64, time.Time) (domain.ClientIdentity, error)
}

func CreateClient(ctx context.Context, store ClientAdministrationStore, name string, capabilities []domain.Capability, at time.Time) (ClientIdentity, domain.ClientIdentity, error) {
	if store == nil || name == "" || at.IsZero() {
		return ClientIdentity{}, domain.ClientIdentity{}, domain.NewError(domain.CodeInvalidArgument, "invalid client creation request")
	}
	device, err := store.Device(ctx)
	if err != nil {
		return ClientIdentity{}, domain.ClientIdentity{}, err
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return ClientIdentity{}, domain.ClientIdentity{}, domain.WrapError(domain.CodeConflict, "client identity generation failed", err)
	}
	clientID, err := domain.NewID("client")
	if err != nil {
		return ClientIdentity{}, domain.ClientIdentity{}, err
	}
	canonical, err := domain.CanonicalCapabilities(capabilities)
	if err != nil {
		return ClientIdentity{}, domain.ClientIdentity{}, err
	}
	record := domain.ClientIdentity{ID: clientID, Name: name, PublicKey: publicKey, Revision: 1, Status: domain.ClientIdentityActive, Caps: canonical, CreatedAt: at, UpdatedAt: at}
	if err := store.CreateClientIdentity(ctx, record); err != nil {
		return ClientIdentity{}, domain.ClientIdentity{}, err
	}
	return ClientIdentity{SchemaVersion: identitySchemaVersion, ClientID: clientID, PrivateKey: privateKey, DaemonID: device.ID, DaemonPublicKey: append(ed25519.PublicKey(nil), device.SigningPublicKey...)}, record, nil
}

func RotateClient(ctx context.Context, store ClientAdministrationStore, id domain.ID, expectedRevision int64, at time.Time) (ClientIdentity, domain.ClientIdentity, error) {
	if store == nil || at.IsZero() {
		return ClientIdentity{}, domain.ClientIdentity{}, domain.NewError(domain.CodeInvalidArgument, "invalid client rotation request")
	}
	device, err := store.Device(ctx)
	if err != nil {
		return ClientIdentity{}, domain.ClientIdentity{}, err
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return ClientIdentity{}, domain.ClientIdentity{}, domain.WrapError(domain.CodeConflict, "client identity generation failed", err)
	}
	record, err := store.RotateClientIdentity(ctx, id, expectedRevision, publicKey, at)
	if err != nil {
		return ClientIdentity{}, domain.ClientIdentity{}, err
	}
	return ClientIdentity{SchemaVersion: identitySchemaVersion, ClientID: id, PrivateKey: privateKey, DaemonID: device.ID, DaemonPublicKey: append(ed25519.PublicKey(nil), device.SigningPublicKey...)}, record, nil
}

func RevokeClient(ctx context.Context, store ClientAdministrationStore, id domain.ID, expectedRevision int64, at time.Time) (domain.ClientIdentity, error) {
	if store == nil || at.IsZero() {
		return domain.ClientIdentity{}, domain.NewError(domain.CodeInvalidArgument, "invalid client revocation request")
	}
	return store.RevokeClientIdentity(ctx, id, expectedRevision, at)
}
