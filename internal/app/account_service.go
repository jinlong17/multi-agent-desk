package app

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/device"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
)

type registryAccountCreateBody struct {
	Provider         string `json:"provider"`
	Name             string `json:"name"`
	Alias            string `json:"alias"`
	SubscriptionHint string `json:"subscription_hint,omitempty"`
}

type listBody struct {
	Provider  string    `json:"provider,omitempty"`
	AccountID domain.ID `json:"account_id,omitempty"`
	Limit     int       `json:"limit,omitempty"`
	Cursor    string    `json:"cursor,omitempty"`
}

type targetBody struct {
	Target           string  `json:"target"`
	ExpectedRevision int64   `json:"expected_revision,omitempty"`
	Name             *string `json:"name,omitempty"`
	Alias            *string `json:"alias,omitempty"`
	SubscriptionHint *string `json:"subscription_hint,omitempty"`
	BrowserProfile   string  `json:"browser_profile,omitempty"`
}

type registryProfileCreateBody struct {
	AccountID domain.ID `json:"account_id"`
	Name      string    `json:"name"`
	Alias     string    `json:"alias"`
}

type registryUsageBody struct {
	Profile string `json:"profile,omitempty"`
}

func registryMethod(method string) bool {
	switch method {
	case "accounts.create", "accounts.list", "accounts.show", "accounts.update", "accounts.disable", "accounts.enable", "accounts.delete",
		"profiles.create", "profiles.list", "profiles.show", "profiles.resolveAlias", "profiles.update", "profiles.disable", "profiles.enable", "profiles.delete", "profiles.validate",
		"provider.login.begin", "provider.login.status", "provider.login.cancel", "provider.logout", "provider.shell", "usage.list", "usage.refresh":
		return true
	default:
		return false
	}
}

func (s *SessionService) handleAccountMethod(ctx context.Context, request device.Request) (any, error) {
	switch request.Method {
	case "accounts.create":
		var body registryAccountCreateBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		if !domain.PublicProvider(body.Provider) {
			return nil, domain.NewError(domain.CodeInvalidArgument, "public provider must be codex or claude")
		}
		deviceRecord, err := s.Store.Device(ctx)
		if err != nil {
			return nil, err
		}
		accountID, err := domain.NewID("account")
		if err != nil {
			return nil, err
		}
		profileID, err := domain.NewID("profile")
		if err != nil {
			return nil, err
		}
		now := s.now()
		account := domain.Account{ID: accountID, Provider: body.Provider, DisplayName: body.Name,
			SubscriptionHint: body.SubscriptionHint, Enabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now}
		profile := domain.RuntimeProfile{ID: profileID, AccountID: accountID, DeviceID: deviceRecord.ID,
			Name: body.Name, Provider: body.Provider, SelectorAlias: body.Alias, Settings: json.RawMessage(`{}`),
			Enabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now}
		createdAccount, createdProfile, err := s.Store.CreateAccountWithDefaultProfile(ctx, account, profile)
		if err != nil {
			return nil, err
		}
		return map[string]any{"account": registryAccountView(createdAccount), "profile": registryProfileView(createdProfile, nil)}, nil

	case "accounts.list":
		var body listBody
		if err := decodeOptionalBody(request.Body, &body); err != nil {
			return nil, err
		}
		page, err := s.Store.ListAccountPage(ctx, storage.AccountListOptions{Provider: body.Provider, Limit: body.Limit, Cursor: body.Cursor})
		if err != nil {
			return nil, err
		}
		items := make([]any, 0, len(page.Items))
		for _, account := range page.Items {
			items = append(items, registryAccountView(account))
		}
		return map[string]any{"items": items, "next_cursor": nullableString(page.NextCursor)}, nil

	case "accounts.show":
		var body targetBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		account, err := s.Store.AccountBySelector(ctx, body.Target)
		if err != nil {
			return nil, err
		}
		profiles, err := s.Store.ListProfiles(ctx, storage.ProfileListOptions{AccountID: account.ID, Limit: storage.MaxPageLimit})
		if err != nil {
			return nil, err
		}
		profileViews := make([]any, 0, len(profiles.Items))
		for _, profile := range profiles.Items {
			profileViews = append(profileViews, registryProfileView(profile, nil))
		}
		return map[string]any{"account": registryAccountView(account), "profiles": profileViews, "next_cursor": nullableString(profiles.NextCursor)}, nil

	case "accounts.update", "accounts.disable", "accounts.enable", "accounts.delete":
		var body targetBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		account, err := s.Store.AccountBySelector(ctx, body.Target)
		if err != nil {
			return nil, err
		}
		switch request.Method {
		case "accounts.update":
			updated, err := s.Store.UpdateAccount(ctx, account.ID, body.ExpectedRevision,
				storage.AccountPatch{DisplayName: body.Name, SubscriptionHint: body.SubscriptionHint}, s.now())
			if err != nil {
				return nil, err
			}
			return registryAccountView(updated), nil
		case "accounts.disable", "accounts.enable":
			updated, err := s.Store.SetAccountEnabledRevision(ctx, account.ID, body.ExpectedRevision, request.Method == "accounts.enable", s.now())
			if err != nil {
				return nil, err
			}
			return registryAccountView(updated), nil
		default:
			if err := s.Store.DeleteAccount(ctx, account.ID, body.ExpectedRevision, s.now()); err != nil {
				return nil, err
			}
			return map[string]any{"id": account.ID, "deleted": true}, nil
		}

	case "profiles.create":
		var body registryProfileCreateBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		account, err := s.Store.PublicAccount(ctx, body.AccountID)
		if err != nil {
			return nil, err
		}
		deviceRecord, err := s.Store.Device(ctx)
		if err != nil {
			return nil, err
		}
		profileID, err := domain.NewID("profile")
		if err != nil {
			return nil, err
		}
		now := s.now()
		profile, err := s.Store.CreateProfile(ctx, account, domain.RuntimeProfile{ID: profileID,
			AccountID: account.ID, DeviceID: deviceRecord.ID, Name: body.Name, Provider: account.Provider,
			SelectorAlias: body.Alias, Settings: json.RawMessage(`{}`), Enabled: true, Revision: 1,
			CreatedAt: now, UpdatedAt: now})
		if err != nil {
			return nil, err
		}
		return registryProfileView(profile, nil), nil

	case "profiles.list":
		var body listBody
		if err := decodeOptionalBody(request.Body, &body); err != nil {
			return nil, err
		}
		page, err := s.Store.ListProfiles(ctx, storage.ProfileListOptions{AccountID: body.AccountID, Limit: body.Limit, Cursor: body.Cursor})
		if err != nil {
			return nil, err
		}
		items := make([]any, 0, len(page.Items))
		for _, profile := range page.Items {
			items = append(items, registryProfileView(profile, nil))
		}
		return map[string]any{"items": items, "next_cursor": nullableString(page.NextCursor)}, nil

	case "profiles.show":
		var body targetBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		binding, err := s.Store.ResolveProfileTarget(ctx, body.Target)
		if err != nil {
			return nil, err
		}
		return bindingView(binding), nil

	case "profiles.resolveAlias":
		var body targetBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		binding, err := s.Store.ResolveProfile(ctx, body.Target)
		if err != nil {
			return nil, err
		}
		return bindingView(binding), nil

	case "profiles.update", "profiles.disable", "profiles.enable", "profiles.delete":
		var body targetBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		binding, err := s.Store.ResolveProfileTarget(ctx, body.Target)
		if err != nil {
			return nil, err
		}
		switch request.Method {
		case "profiles.update":
			profile, err := s.Store.UpdateProfile(ctx, binding.Profile.ID, body.ExpectedRevision,
				storage.ProfilePatch{Name: body.Name, SelectorAlias: body.Alias}, s.now())
			if err != nil {
				return nil, err
			}
			return registryProfileView(profile, binding.Credential), nil
		case "profiles.disable", "profiles.enable":
			profile, err := s.Store.SetProfileEnabled(ctx, binding.Profile.ID, body.ExpectedRevision, request.Method == "profiles.enable", s.now())
			if err != nil {
				return nil, err
			}
			return registryProfileView(profile, binding.Credential), nil
		default:
			if err := s.Store.DeleteProfile(ctx, binding.Profile.ID, body.ExpectedRevision, s.now()); err != nil {
				return nil, err
			}
			return map[string]any{"id": binding.Profile.ID, "deleted": true}, nil
		}

	case "usage.list":
		var body registryUsageBody
		if err := decodeOptionalBody(request.Body, &body); err != nil {
			return nil, err
		}
		var accountID domain.ID
		if body.Profile != "" {
			binding, err := s.Store.ResolveProfile(ctx, body.Profile)
			if err != nil {
				return nil, err
			}
			accountID = binding.Account.ID
		}
		snapshots, err := s.Store.ListUsageSnapshotsWithWindows(ctx, accountID)
		if err != nil {
			return nil, err
		}
		items := make([]any, 0, len(snapshots))
		for _, snapshot := range snapshots {
			items = append(items, registryUsageView(snapshot, s.now()))
		}
		return map[string]any{"items": items}, nil

	case "profiles.validate":
		var body targetBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		if _, err := s.Store.ResolveProfileTarget(ctx, body.Target); err != nil {
			return nil, err
		}
		return nil, domain.NewError(domain.CodeProviderUnavailable, "provider operation is not available in P1")
	case "provider.login.begin", "provider.login.status", "provider.login.cancel", "provider.logout", "provider.shell":
		var body targetBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		if _, err := s.Store.ResolveProfile(ctx, body.Target); err != nil {
			return nil, err
		}
		return nil, domain.NewError(domain.CodeProviderUnavailable, "provider operation is not available in P1")
	case "usage.refresh":
		var body registryUsageBody
		if err := decodeOptionalBody(request.Body, &body); err != nil {
			return nil, err
		}
		if body.Profile == "" {
			return nil, domain.NewError(domain.CodeInvalidArgument, "usage refresh requires an explicit profile selector")
		}
		binding, err := s.resolveCodexProfile(ctx, body.Profile, "")
		if err != nil {
			return nil, err
		}
		if binding.Credential == nil || binding.Credential.Status != domain.CredentialHealthy {
			return nil, domain.NewError(domain.CodeIdentityConfirmationRequired, "profile login is not confirmed")
		}
		if s.Runtime == nil || s.Runtime.Codex == nil {
			return nil, domain.NewError(domain.CodeUsageUnavailable, "codex usage is unavailable without an active supported runtime")
		}
		snapshot, err := s.Runtime.Codex.ReadUsage(ctx, binding.Account.ID)
		if err != nil {
			return nil, err
		}
		return registryUsageView(snapshot, s.now()), nil
	default:
		return nil, domain.NewError(domain.CodeMethodNotFound, "method is not available")
	}
}

func registryAccountView(account domain.Account) map[string]any {
	return map[string]any{"id": account.ID, "provider": account.Provider,
		"display_name": account.DisplayName, "subscription_hint": account.SubscriptionHint,
		"enabled": account.Enabled, "revision": account.Revision,
		"created_at": account.CreatedAt, "updated_at": account.UpdatedAt}
}

func registryProfileView(profile domain.RuntimeProfile, credential *domain.CredentialInstance) map[string]any {
	authStatus := "login_required"
	availability := domain.AvailabilityUnknown
	var lastValidated *time.Time
	if credential != nil {
		authStatus = string(credential.Status)
		if credential.Availability != "" {
			availability = credential.Availability
		}
		lastValidated = credential.LastValidatedAt
	}
	return map[string]any{"id": profile.ID, "account_id": profile.AccountID,
		"device_id": profile.DeviceID, "name": profile.Name, "provider": profile.Provider,
		"selector": "@" + profile.SelectorAlias, "enabled": profile.Enabled,
		"revision": profile.Revision, "auth_status": authStatus,
		"availability": availability, "last_validated_at": lastValidated,
		"created_at": profile.CreatedAt, "updated_at": profile.UpdatedAt}
}

func bindingView(binding storage.ProfileBinding) map[string]any {
	return map[string]any{"account": registryAccountView(binding.Account), "profile": registryProfileView(binding.Profile, binding.Credential)}
}

func registryUsageView(snapshot domain.UsageSnapshot, now time.Time) map[string]any {
	windows := make([]any, 0, len(snapshot.Windows))
	for _, window := range snapshot.Windows {
		value := map[string]any{"provider_limit_id": window.ProviderLimitID, "kind": window.Kind, "label": window.Label}
		putOptional(value, "duration_seconds", window.DurationSeconds)
		putOptional(value, "used_value", window.UsedValue)
		putOptional(value, "limit_value", window.LimitValue)
		putOptional(value, "used_percent", window.UsedPercent)
		putOptional(value, "remaining_percent", window.RemainingPercent)
		putOptional(value, "resets_at", window.ResetsAt)
		windows = append(windows, value)
	}
	return map[string]any{"snapshot_id": snapshot.ID, "account_id": snapshot.AccountID,
		"credential_instance_id": nullableID(snapshot.CredentialInstanceID), "device_id": snapshot.DeviceID,
		"provider": snapshot.Provider, "provider_version": snapshot.ProviderVersion,
		"source": snapshot.Source, "confidence": snapshot.Confidence,
		"availability": snapshot.Availability, "observed_at": snapshot.ObservedAt,
		"stale_at": snapshot.StaleAt, "stale": now.After(snapshot.StaleAt), "windows": windows}
}

func putOptional(target map[string]any, key string, value any) {
	switch typed := value.(type) {
	case *int64:
		if typed != nil {
			target[key] = *typed
		}
	case *float64:
		if typed != nil {
			target[key] = *typed
		}
	case *time.Time:
		if typed != nil {
			target[key] = *typed
		}
	}
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableID(value domain.ID) any {
	if value == "" {
		return nil
	}
	return value
}

func decodeOptionalBody(body json.RawMessage, target any) error {
	if len(body) == 0 || string(body) == "null" {
		return nil
	}
	return decodeBody(body, target)
}
