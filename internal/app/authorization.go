package app

import (
	"context"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/device"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

type ClientLookup interface {
	ClientIdentity(context.Context, domain.ID) (domain.ClientIdentity, error)
}

type Authorizer struct {
	Clients ClientLookup
	Now     func() time.Time
}

func RequiredCapability(method string) (domain.Capability, error) {
	switch method {
	case "daemon.status", "vault.status", "sessions.list", "sessions.show", "sessions.observe":
		return domain.CapabilityMetadataRead, nil
	case "accounts.list", "accounts.show", "accounts.create", "accounts.disable", "profiles.list", "profiles.create", "profiles.edit", "profiles.delete", "credentials.status":
		return domain.CapabilityMetadataRead, nil
	case "provider.describe", "provider.health", "profile.validate":
		return domain.CapabilityProviderMetadataRead, nil
	case "auth.begin", "auth.complete", "auth.cancel", "auth.status", "auth.logout":
		return domain.CapabilityProviderAuth, nil
	case "usage.read":
		return domain.CapabilityProviderUsageRead, nil
	case "approval.list", "approval.observe":
		return domain.CapabilityApprovalRead, nil
	case "approval.respond":
		return domain.CapabilityApprovalRespond, nil
	case "sessions.attach", "sessions.detach":
		return domain.CapabilitySessionObserve, nil
	case "vault.initialize", "vault.unlock", "vault.lock":
		return domain.CapabilityVaultControl, nil
	case "sessions.start", "session.start":
		return domain.CapabilitySessionStart, nil
	case "control.acquire":
		return domain.CapabilitySessionControlAcquire, nil
	case "control.heartbeat", "control.release", "sessions.stop", "sessions.kill", "session.stop":
		return domain.CapabilitySessionControl, nil
	case "terminal.input", "terminal.resize", "session.input", "session.resize":
		return domain.CapabilityTerminalControl, nil
	case "sessions.resume", "session.resume":
		return domain.CapabilitySessionResume, nil
	case "client.create", "client.list", "client.rotate", "client.revoke":
		return domain.CapabilityClientAdmin, nil
	default:
		return "", domain.NewError(domain.CodeMethodNotFound, "method is not available")
	}
}

func (a Authorizer) Authorize(ctx context.Context, auth device.AuthContext, method string) error {
	capability, err := RequiredCapability(method)
	if err != nil {
		return err
	}
	if a.Clients == nil || !auth.ValidAt(a.now()) || !auth.Has(capability) {
		return domain.NewError(domain.CodeUnauthenticated, "peer authentication failed")
	}
	client, err := a.Clients.ClientIdentity(ctx, auth.ClientID)
	if err != nil || client.Status != domain.ClientIdentityActive || client.Revision != auth.IdentityRevision {
		return domain.NewError(domain.CodeUnauthenticated, "peer authentication failed")
	}
	if !domain.HasCapability(client.Caps, capability) {
		return domain.NewError(domain.CodePermissionDenied, "capability is not granted")
	}
	return nil
}

func (a Authorizer) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}
