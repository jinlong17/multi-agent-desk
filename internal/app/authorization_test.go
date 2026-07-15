package app

import (
	"context"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/device"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

type testClients struct{ value domain.ClientIdentity }

func (c testClients) ClientIdentity(_ context.Context, id domain.ID) (domain.ClientIdentity, error) {
	if id != c.value.ID {
		return domain.ClientIdentity{}, domain.NewError(domain.CodeNotFound, "client not found")
	}
	return c.value, nil
}

func TestAuthorizerDeniesUnknownAndRevokedCapabilities(t *testing.T) {
	id, _ := domain.NewID("client")
	now := time.Now().UTC()
	client := domain.ClientIdentity{ID: id, Revision: 2, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityMetadataRead}, CreatedAt: now, UpdatedAt: now}
	authorizer := Authorizer{Clients: testClients{value: client}, Now: func() time.Time { return now }}
	auth := device.AuthContext{ClientID: id, IdentityRevision: 2, GrantedCapabilities: []domain.Capability{domain.CapabilityMetadataRead}, AuthenticatedAt: now.Add(-time.Second), ExpiresAt: now.Add(time.Minute)}
	if err := authorizer.Authorize(context.Background(), auth, "daemon.status"); err != nil {
		t.Fatal(err)
	}
	if code := domain.CodeOf(authorizer.Authorize(context.Background(), auth, "sessions.start")); code != domain.CodeUnauthenticated {
		t.Fatalf("missing capability code = %v", code)
	}
	if code := domain.CodeOf(authorizer.Authorize(context.Background(), auth, "unknown.method")); code != domain.CodeMethodNotFound {
		t.Fatalf("unknown method code = %v", code)
	}
	client.Status = domain.ClientIdentityRevoked
	if code := domain.CodeOf((Authorizer{Clients: testClients{value: client}, Now: func() time.Time { return now }}).Authorize(context.Background(), auth, "daemon.status")); code != domain.CodeUnauthenticated {
		t.Fatalf("revoked code = %v", code)
	}
}
