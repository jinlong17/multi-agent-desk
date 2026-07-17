package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/device"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/providers/codex"
	"github.com/jinlong17/multi-agent-desk/internal/runtime"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
	"github.com/jinlong17/multi-agent-desk/internal/vault"
)

type SessionService struct {
	Store               *storage.Store
	Runtime             *runtime.Manager
	Vault               *vault.Manager
	EnrollmentValidator func(context.Context, codex.BinaryDescriptor, string) error
	CredentialHomeRoot  string
	Now                 func() time.Time
	SelectorPreflight   func(context.Context) (SelectorPreflight, error)
	mu                  sync.Mutex
}

type SelectorPreflight struct {
	ProviderVersion   string
	BinaryFingerprint string
	SchemaFingerprint string
	CapabilityDigest  string
	Capabilities      []domain.Capability
}

func NewSessionService(store *storage.Store, manager *runtime.Manager) *SessionService {
	return &SessionService{Store: store, Runtime: manager, Now: func() time.Time { return time.Now().UTC() }}
}

// RecoverPendingApprovals records restart expiration before the server starts
// accepting new local requests. It never replays a Provider mutation.
func (s *SessionService) RecoverPendingApprovals(ctx context.Context) error {
	if s == nil || s.Store == nil {
		return domain.NewError(domain.CodeInvalidArgument, "session service is incomplete")
	}
	if err := s.Store.ExpirePendingApprovals(ctx, s.now()); err != nil {
		return err
	}
	paths, err := s.Store.ExpireAuthEnrollments(ctx, s.now())
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := s.removeEnrollmentStaging(path); err != nil {
			return err
		}
	}
	if err := s.Store.DeleteExpiredSessionStartPreviews(ctx, s.now()); err != nil {
		return err
	}
	return s.removeOrphanEnrollmentStaging()
}

func (s *SessionService) selectorPreflight(ctx context.Context) (SelectorPreflight, error) {
	if s.SelectorPreflight != nil {
		return s.SelectorPreflight(ctx)
	}
	descriptor, err := codex.Discover(ctx, codex.DiscoverOptions{})
	if err != nil {
		return SelectorPreflight{}, err
	}
	if descriptor.Platform != "linux" || descriptor.Architecture != "amd64" {
		return SelectorPreflight{}, domain.NewError(domain.CodeProviderPlatformUnsupported, "Codex multi-account selector is supported only on the accepted Linux target")
	}
	capabilities, err := codex.Probe(ctx, descriptor, codex.ProbeOptions{})
	if err != nil {
		return SelectorPreflight{}, err
	}
	if descriptor.Version != "0.144.2" || capabilities.Status != codex.CapabilitySupported {
		return SelectorPreflight{}, domain.NewError(domain.CodeProviderVersionUnsupported, "Codex selector compatibility is unsupported")
	}
	binaryFingerprint, err := codex.BinaryFingerprint(descriptor)
	if err != nil {
		return SelectorPreflight{}, err
	}
	encoded, _ := json.Marshal(capabilities.Methods)
	digest := sha256.Sum256(encoded)
	return SelectorPreflight{ProviderVersion: descriptor.Version, BinaryFingerprint: binaryFingerprint,
		SchemaFingerprint: capabilities.SchemaFingerprint, CapabilityDigest: hex.EncodeToString(digest[:]),
		Capabilities: []domain.Capability{domain.CapabilityProviderUsageRead, domain.CapabilitySessionControl}}, nil
}

func (s *SessionService) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *SessionService) resolveCodexProfile(ctx context.Context, selector string, profileID domain.ID) (storage.ProfileBinding, error) {
	if (selector == "") == (profileID == "") {
		return storage.ProfileBinding{}, domain.NewError(domain.CodeInvalidArgument, "exactly one profile selector or profile ID is required")
	}
	if selector != "" {
		binding, err := s.Store.ResolveProfile(ctx, selector)
		if err != nil {
			return storage.ProfileBinding{}, err
		}
		if binding.Profile.Provider != domain.ProviderCodex || binding.Account.Provider != domain.ProviderCodex {
			return storage.ProfileBinding{}, domain.NewError(domain.CodeProfileBindingChanged, "profile is not a public Codex profile")
		}
		return binding, nil
	}
	profile, err := s.Store.RuntimeProfile(ctx, profileID)
	if err != nil {
		return storage.ProfileBinding{}, err
	}
	account, err := s.Store.Account(ctx, profile.AccountID)
	if err != nil {
		return storage.ProfileBinding{}, err
	}
	binding := storage.ProfileBinding{Account: account, Profile: profile}
	if profile.CredentialInstanceID != "" {
		credential, err := s.Store.CredentialInstance(ctx, profile.CredentialInstanceID)
		if err != nil {
			return storage.ProfileBinding{}, err
		}
		binding.Credential = &credential
	}
	if profile.Provider != domain.ProviderCodex || account.Provider != domain.ProviderCodex || profile.Internal || account.Internal {
		return storage.ProfileBinding{}, domain.NewError(domain.CodeProfileBindingChanged, "profile is not a public Codex profile")
	}
	return binding, nil
}

func enrollmentAliasDigest(enrollmentID domain.ID, alias string) (string, error) {
	key, err := domain.ParseProfileSelector(alias)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte("@" + key + "\x00" + string(enrollmentID)))
	return hex.EncodeToString(digest[:]), nil
}

func (s *SessionService) removeEnrollmentStaging(path string) error {
	if s == nil || s.Store == nil || path == "" {
		return domain.NewError(domain.CodeInvalidArgument, "enrollment staging path is invalid")
	}
	base := filepath.Clean(filepath.Join(filepath.Dir(s.Store.Path()), "enrollments"))
	clean := filepath.Clean(path)
	if filepath.Dir(clean) != base || clean == base || domain.ValidateID(domain.ID(filepath.Base(clean))) != nil {
		return domain.NewError(domain.CodeConflict, "enrollment staging path escaped its root")
	}
	info, err := os.Lstat(base)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return domain.WrapError(domain.CodeConflict, "enrollment staging root could not be checked", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return domain.NewError(domain.CodeConflict, "enrollment staging root is unsafe")
	}
	if err := os.RemoveAll(clean); err != nil {
		return domain.WrapError(domain.CodeConflict, "enrollment staging could not be removed", err)
	}
	return nil
}

func (s *SessionService) createEnrollmentStaging(enrollmentID domain.ID) (string, error) {
	if s == nil || s.Store == nil || domain.ValidateID(enrollmentID) != nil {
		return "", domain.NewError(domain.CodeInvalidArgument, "enrollment staging identity is invalid")
	}
	base := filepath.Clean(filepath.Join(filepath.Dir(s.Store.Path()), "enrollments"))
	if err := os.Mkdir(base, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return "", domain.WrapError(domain.CodeConflict, "enrollment staging root could not be created", err)
	}
	info, err := os.Lstat(base)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", domain.NewError(domain.CodeConflict, "enrollment staging root is unsafe")
	}
	if err := device.ProtectPrivateDirectory(base); err != nil {
		return "", err
	}
	staging := filepath.Join(base, string(enrollmentID))
	if err := os.Mkdir(staging, 0o700); err != nil {
		return "", domain.WrapError(domain.CodeConflict, "enrollment staging could not be created", err)
	}
	if err := device.ProtectPrivateDirectory(staging); err != nil {
		_ = os.Remove(staging)
		return "", err
	}
	return staging, nil
}

func (s *SessionService) removeOrphanEnrollmentStaging() error {
	if s == nil || s.Store == nil {
		return domain.NewError(domain.CodeInvalidArgument, "session service is incomplete")
	}
	base := filepath.Clean(filepath.Join(filepath.Dir(s.Store.Path()), "enrollments"))
	info, err := os.Lstat(base)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "enrollment staging root could not be checked", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return domain.NewError(domain.CodeConflict, "enrollment staging root is unsafe")
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "enrollment staging root could not be listed", err)
	}
	for _, entry := range entries {
		path := filepath.Join(base, entry.Name())
		if filepath.Dir(path) != base {
			return domain.NewError(domain.CodeConflict, "enrollment staging entry escaped its root")
		}
		if err := os.RemoveAll(path); err != nil {
			return domain.WrapError(domain.CodeConflict, "orphan enrollment staging could not be removed", err)
		}
	}
	return nil
}

func (s *SessionService) removeCredentialHome(credentialID domain.ID) error {
	if s == nil || domain.ValidateID(credentialID) != nil {
		return domain.NewError(domain.CodeInvalidArgument, "credential home identity is invalid")
	}
	if s.CredentialHomeRoot == "" {
		return nil
	}
	base := filepath.Clean(s.CredentialHomeRoot)
	info, err := os.Lstat(base)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return domain.NewError(domain.CodeConflict, "credential home root is unsafe")
	}
	home := filepath.Join(base, string(credentialID))
	if filepath.Dir(home) != base {
		return domain.NewError(domain.CodeConflict, "credential home escaped its root")
	}
	if err := os.RemoveAll(home); err != nil {
		return domain.WrapError(domain.CodeConflict, "credential home could not be removed", err)
	}
	lockPath := filepath.Join(base, string(credentialID)+".writer.lock")
	if filepath.Dir(lockPath) != base {
		return domain.NewError(domain.CodeConflict, "credential writer lock escaped its root")
	}
	if err := os.RemoveAll(lockPath); err != nil {
		return domain.WrapError(domain.CodeConflict, "credential writer lock could not be removed", err)
	}
	return nil
}

func (s *SessionService) Handle(ctx context.Context, auth device.AuthContext, request device.Request) (any, error) {
	if s == nil || s.Store == nil || s.Runtime == nil {
		return nil, domain.NewError(domain.CodeInvalidArgument, "session service is incomplete")
	}
	if request.Method == "vault.initialize" || request.Method == "vault.unlock" {
		if request.IdempotencyKey == "" {
			return nil, domain.NewError(domain.CodeInvalidArgument, "idempotency key is required")
		}
		// Password-bearing Vault operations never enter the generic body-digest
		// ledger. Initialization has its own Store-level replay contract; unlock
		// is state-aware and safe to execute again after a transport retry.
		return s.dispatch(ctx, auth, request)
	}
	if requiresIdempotency(request.Method) {
		if request.IdempotencyKey == "" {
			return nil, domain.NewError(domain.CodeInvalidArgument, "idempotency key is required")
		}
		return s.withIdempotency(ctx, auth.ClientID, request, func() (any, error) {
			return s.dispatch(ctx, auth, request)
		})
	}
	return s.dispatch(ctx, auth, request)
}

func requiresIdempotency(method string) bool {
	switch method {
	case "sessions.start", "session.start", "sessions.attach", "sessions.detach", "control.acquire",
		"terminal.input", "terminal.resize", "session.input", "session.resize", "sessions.stop", "session.stop", "sessions.kill",
		"sessions.resume", "session.resume", "accounts.create", "accounts.disable", "profiles.create", "profiles.edit", "profiles.delete", "auth.begin", "auth.complete", "auth.confirm", "auth.cancel", "auth.logout",
		"approval.respond":
		return true
	default:
		return false
	}
}

func (s *SessionService) withIdempotency(ctx context.Context, clientID domain.ID, request device.Request, fn func() (any, error)) (any, error) {
	digest := sha256.Sum256(append([]byte(request.Method+"\x00"+request.IdempotencyKey+"\x00"), request.Body...))
	digestText := hex.EncodeToString(digest[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.Store.IdempotencyRecord(ctx, clientID, request.Method, request.IdempotencyKey)
	if err == nil {
		if record.RequestDigest != digestText {
			return nil, domain.NewError(domain.CodeConflict, "idempotency key was reused with a different request")
		}
		return json.RawMessage(record.ResponseMetadata), nil
	}
	if domain.CodeOf(err) != domain.CodeNotFound {
		return nil, err
	}
	result, err := fn()
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) > device.MaxFrameBytes {
		return nil, domain.NewError(domain.CodeFrameTooLarge, "idempotent response exceeds limit")
	}
	if err := s.Store.SaveIdempotencyRecord(ctx, storage.IdempotencyRecord{ClientID: clientID, Method: request.Method,
		IdempotencyKey: request.IdempotencyKey, RequestDigest: digestText, ResponseCode: "ok",
		ResponseMetadata: encoded, CreatedAt: s.now()}); err != nil {
		// A concurrent writer may have committed the same key. Re-read and
		// compare instead of ever returning an unverified duplicate result.
		if domain.CodeOf(err) == domain.CodeConflict {
			stored, readErr := s.Store.IdempotencyRecord(ctx, clientID, request.Method, request.IdempotencyKey)
			if readErr == nil && stored.RequestDigest == digestText {
				return json.RawMessage(stored.ResponseMetadata), nil
			}
		}
		return nil, err
	}
	return result, nil
}

func (s *SessionService) dispatch(ctx context.Context, auth device.AuthContext, request device.Request) (any, error) {
	if registryMethod(request.Method) {
		return s.handleAccountMethod(ctx, request)
	}
	switch request.Method {
	case "daemon.status":
		return map[string]any{"status": "ok", "schema_version": 1}, nil
	case "vault.status":
		if s.Vault == nil {
			return map[string]any{"state": vault.StateLocked}, nil
		}
		return map[string]any{"state": s.Vault.Status()}, nil
	case "vault.initialize":
		if s.Vault == nil {
			return nil, domain.NewError(domain.CodeVaultLocked, "vault is unavailable")
		}
		var body vaultBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		secret := []byte(body.Secret)
		body.Secret = ""
		defer zeroSecretBytes(secret)
		defer zeroSecretBytes(request.Body)
		state, err := s.Vault.Initialize(ctx, auth.ClientID, request.IdempotencyKey, secret, s.now())
		if err != nil {
			return nil, err
		}
		return map[string]any{"state": state, "initialized": true}, nil
	case "vault.unlock":
		if s.Vault == nil {
			return nil, domain.NewError(domain.CodeVaultLocked, "vault is unavailable")
		}
		var body vaultBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		secret := []byte(body.Secret)
		body.Secret = ""
		defer zeroSecretBytes(secret)
		defer zeroSecretBytes(request.Body)
		if err := s.Vault.Unlock(secret); err != nil {
			return nil, err
		}
		return map[string]any{"state": s.Vault.Status()}, nil
	case "vault.lock":
		if s.Vault == nil {
			return nil, domain.NewError(domain.CodeVaultLocked, "vault is unavailable")
		}
		if err := s.Vault.Lock(); err != nil {
			return nil, err
		}
		return map[string]any{"state": s.Vault.Status()}, nil
	case "sessions.list":
		sessions, err := s.Store.ListSessions(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]any, 0, len(sessions))
		for _, session := range sessions {
			result = append(result, sessionView(session))
		}
		return result, nil
	case "client.list":
		clients, err := s.Store.ListClientIdentities(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]any, 0, len(clients))
		for _, client := range clients {
			result = append(result, clientView(client))
		}
		return result, nil
	case "accounts.list":
		accounts, err := s.Store.ListAccounts(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]any, 0, len(accounts))
		for _, account := range accounts {
			result = append(result, accountView(account))
		}
		return result, nil
	case "accounts.show":
		var body accountBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		account, err := s.Store.Account(ctx, body.AccountID)
		if err != nil {
			return nil, err
		}
		return accountView(account), nil
	case "accounts.create":
		var body accountCreateBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		id, err := domain.NewID("account")
		if err != nil {
			return nil, err
		}
		now := s.now()
		account := domain.Account{ID: id, Provider: body.Provider, DisplayName: body.DisplayName,
			ProviderSubjectDigest: body.ProviderSubjectDigest, Enabled: true, CreatedAt: now, UpdatedAt: now}
		if err := s.Store.CreateAccount(ctx, account); err != nil {
			return nil, err
		}
		return accountView(account), nil
	case "accounts.disable":
		var body accountBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		account, err := s.Store.SetAccountEnabled(ctx, body.AccountID, false, s.now())
		if err != nil {
			return nil, err
		}
		return accountView(account), nil
	case "profiles.list":
		profiles, err := s.Store.ListRuntimeProfiles(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]any, 0, len(profiles))
		for _, profile := range profiles {
			result = append(result, profileView(profile))
		}
		return result, nil
	case "profiles.create":
		var body profileCreateBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		id, err := domain.NewID("profile")
		if err != nil {
			return nil, err
		}
		settings := body.Settings
		if len(settings) == 0 {
			settings = json.RawMessage(`{}`)
		}
		now := s.now()
		profile := domain.RuntimeProfile{ID: id, DeviceID: body.DeviceID, AccountID: body.AccountID,
			Name: body.Name, Provider: body.Provider, Settings: settings, CreatedAt: now, UpdatedAt: now}
		if err := s.Store.CreateRuntimeProfile(ctx, profile); err != nil {
			return nil, err
		}
		return profileView(profile), nil
	case "profiles.edit":
		var body profileEditBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		current, err := s.Store.RuntimeProfile(ctx, body.ProfileID)
		if err != nil {
			return nil, err
		}
		if body.Name != "" {
			current.Name = body.Name
		}
		if body.Provider != "" {
			current.Provider = body.Provider
		}
		if body.AccountID != "" {
			current.AccountID = body.AccountID
		}
		if len(body.Settings) != 0 {
			current.Settings = body.Settings
		}
		current.UpdatedAt = s.now()
		updated, err := s.Store.UpdateRuntimeProfile(ctx, current)
		if err != nil {
			return nil, err
		}
		return profileView(updated), nil
	case "profiles.delete":
		var body profileBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		if err := s.Store.DeleteRuntimeProfile(ctx, body.ProfileID); err != nil {
			return nil, err
		}
		return map[string]any{"profile_id": body.ProfileID, "deleted": true}, nil
	case "credentials.status":
		credentials, err := s.Store.ListCredentialInstances(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]any, 0, len(credentials))
		for _, credential := range credentials {
			result = append(result, credentialView(credential))
		}
		return result, nil
	case "provider.describe":
		return s.codexDescribe(ctx), nil
	case "provider.health":
		describe := s.codexDescribe(ctx)
		return map[string]any{"provider": domain.ProviderCodex, "status": describe["status"], "version": describe["version"],
			"binary_path": describe["binary_path"], "reason": describe["reason"]}, nil
	case "profile.validate":
		var body profileBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		profile, err := s.Store.RuntimeProfile(ctx, body.ProfileID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"profile_id": profile.ID, "provider": profile.Provider, "valid": true, "account_id": profile.AccountID}, nil
	case "auth.begin":
		var body authBeginBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		if body.Mode != "" && body.Mode != domain.AuthMethodInteractive {
			return nil, domain.NewError(domain.CodeProviderUnsupported, "Codex auth mode is disabled")
		}
		duePaths, err := s.Store.ExpireDueAuthEnrollments(ctx, s.now())
		if err != nil {
			return nil, err
		}
		for _, path := range duePaths {
			if err := s.removeEnrollmentStaging(path); err != nil {
				return nil, err
			}
		}
		if s.Vault == nil {
			return nil, domain.NewError(domain.CodeVaultLocked, "vault is unavailable")
		}
		if err := s.Vault.RequireUnlocked(); err != nil {
			return nil, err
		}
		binding, err := s.resolveCodexProfile(ctx, body.ProfileSelector, body.ProfileID)
		if err != nil {
			return nil, err
		}
		profile := binding.Profile
		if profile.Provider != domain.ProviderCodex || profile.AccountID == "" {
			return nil, domain.NewError(domain.CodeConflict, "profile is not Codex-auth capable")
		}
		descriptor, err := codex.Discover(ctx, codex.DiscoverOptions{})
		if err != nil {
			return nil, err
		}
		fingerprint, err := codex.BinaryFingerprint(descriptor)
		if err != nil {
			return nil, err
		}
		idempotencyBytes := sha256.Sum256([]byte(string(auth.ClientID) + "\x00" + request.IdempotencyKey))
		beginDigest := hex.EncodeToString(idempotencyBytes[:])
		existing, err := s.Store.AuthEnrollmentByBeginDigest(ctx, auth.ClientID, beginDigest)
		if err == nil {
			if existing.RuntimeProfileID != profile.ID {
				return nil, domain.NewError(domain.CodeConflict, "auth begin idempotency key was reused for another profile")
			}
			if existing.BinaryFingerprint != fingerprint {
				return nil, domain.NewError(domain.CodeProviderVersionUnsupported, "enrollment binary changed")
			}
			return map[string]any{"enrollment_id": existing.ID, "binary_path": descriptor.Path, "argv": []string{"login"}, "staging_path": existing.StagingPath, "expires_at": existing.ExpiresAt}, nil
		}
		if domain.CodeOf(err) != domain.CodeNotFound {
			return nil, err
		}
		enrollmentID, err := domain.NewID("enrollment")
		if err != nil {
			return nil, err
		}
		credentialID := body.CredentialID
		var credential *domain.CredentialInstance
		if credentialID == "" {
			credentialID, err = domain.NewID("credential")
			if err != nil {
				return nil, err
			}
			zeroDigest := strings.Repeat("0", 64)
			value := domain.CredentialInstance{ID: credentialID, DeviceID: profile.DeviceID, AccountID: profile.AccountID,
				Provider: domain.ProviderCodex, AuthMethod: domain.AuthMethodInteractive, SecretRef: "vault:" + string(credentialID),
				Status: domain.CredentialUnknown, CredentialRevision: 1, SecretDigest: zeroDigest, CreatedAt: s.now(), UpdatedAt: s.now()}
			credential = &value
		}
		staging, err := s.createEnrollmentStaging(enrollmentID)
		if err != nil {
			return nil, err
		}
		expires := s.now().Add(10 * time.Minute)
		enrollment := storage.AuthEnrollment{ID: enrollmentID, ClientDeviceID: auth.ClientID, RuntimeProfileID: profile.ID,
			CredentialInstanceID: credentialID, BinaryFingerprint: fingerprint, StagingPath: staging, State: storage.EnrollmentBegun,
			IdempotencyDigest: beginDigest, ExpiresAt: expires, CreatedAt: s.now(), UpdatedAt: s.now()}
		if err := s.Store.BeginAuthEnrollment(ctx, enrollment, credential); err != nil {
			_ = s.removeEnrollmentStaging(staging)
			replayed, replayErr := s.Store.AuthEnrollmentByBeginDigest(ctx, auth.ClientID, beginDigest)
			if replayErr == nil && replayed.RuntimeProfileID == profile.ID && replayed.BinaryFingerprint == fingerprint {
				return map[string]any{"enrollment_id": replayed.ID, "binary_path": descriptor.Path, "argv": []string{"login"}, "staging_path": replayed.StagingPath, "expires_at": replayed.ExpiresAt}, nil
			}
			return nil, err
		}
		return map[string]any{"enrollment_id": enrollmentID, "binary_path": descriptor.Path, "argv": []string{"login"}, "staging_path": staging, "expires_at": expires}, nil
	case "auth.complete":
		var body authEnrollmentBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		completionBytes := sha256.Sum256([]byte(string(auth.ClientID) + "\x00" + request.IdempotencyKey))
		completionDigest := hex.EncodeToString(completionBytes[:])
		enrollment, err := s.Store.ClaimAuthEnrollment(ctx, body.EnrollmentID, auth.ClientID, completionDigest, s.now())
		if err != nil {
			if domain.CodeOf(err) == domain.CodeDeadlineExceeded {
				if expired, readErr := s.Store.AuthEnrollment(ctx, body.EnrollmentID); readErr == nil && expired.ClientDeviceID == auth.ClientID {
					_ = s.removeEnrollmentStaging(expired.StagingPath)
				}
			}
			return nil, err
		}
		if enrollment.State == storage.EnrollmentSucceeded {
			credential, err := s.Store.CredentialInstance(ctx, enrollment.CredentialInstanceID)
			if err != nil {
				return nil, err
			}
			if err := s.removeEnrollmentStaging(enrollment.StagingPath); err != nil {
				return nil, err
			}
			return map[string]any{"enrollment_id": enrollment.ID, "credential_id": credential.ID, "state": storage.EnrollmentSucceeded, "credential_revision": credential.CredentialRevision}, nil
		}
		if enrollment.State == storage.EnrollmentAwaitingConfirmation {
			profile, err := s.Store.RuntimeProfile(ctx, enrollment.RuntimeProfileID)
			if err != nil {
				return nil, err
			}
			return map[string]any{"enrollment_id": enrollment.ID, "credential_id": enrollment.CredentialInstanceID,
				"account_id": enrollment.ConfirmationAccountID, "runtime_profile_id": enrollment.ConfirmationProfileID,
				"profile_selector": "@" + profile.SelectorAlias, "state": storage.EnrollmentAwaitingConfirmation,
				"confirmation_expires_at": enrollment.ExpiresAt}, nil
		}
		fail := func(cause error) (any, error) {
			_, _ = s.Store.FinishAuthEnrollment(ctx, enrollment.ID, auth.ClientID, storage.EnrollmentFailed, s.now())
			_ = s.removeEnrollmentStaging(enrollment.StagingPath)
			return nil, cause
		}
		descriptor, err := codex.Discover(ctx, codex.DiscoverOptions{})
		if err != nil {
			return fail(err)
		}
		fingerprint, err := codex.BinaryFingerprint(descriptor)
		if err != nil {
			return fail(err)
		}
		if fingerprint != enrollment.BinaryFingerprint {
			return fail(domain.NewError(domain.CodeProviderVersionUnsupported, "enrollment binary changed"))
		}
		validateEnrollment := codex.ValidateEnrollment
		if s.EnrollmentValidator != nil {
			validateEnrollment = s.EnrollmentValidator
		}
		if err := validateEnrollment(ctx, descriptor, enrollment.StagingPath); err != nil {
			return fail(err)
		}
		postValidationFingerprint, err := codex.BinaryFingerprint(descriptor)
		if err != nil {
			return fail(err)
		}
		if postValidationFingerprint != enrollment.BinaryFingerprint {
			return fail(domain.NewError(domain.CodeProviderVersionUnsupported, "enrollment binary changed during validation"))
		}
		credentialBytes, err := codex.ReadEnrollmentAuth(enrollment.StagingPath)
		if err != nil {
			return fail(err)
		}
		defer func() {
			for index := range credentialBytes {
				credentialBytes[index] = 0
			}
		}()
		profile, err := s.Store.RuntimeProfile(ctx, enrollment.RuntimeProfileID)
		if err != nil {
			return fail(err)
		}
		if profile.SelectorAlias == "" || profile.Internal || !profile.Enabled {
			return fail(domain.NewError(domain.CodeIdentityConfirmationRequired, "public profile alias is required for auth confirmation"))
		}
		credential, err := s.Store.CredentialInstance(ctx, enrollment.CredentialInstanceID)
		if err != nil {
			return fail(err)
		}
		account, err := s.Store.Account(ctx, profile.AccountID)
		if err != nil {
			return fail(err)
		}
		if !account.Enabled || account.Internal || credential.AccountID != account.ID || credential.DeviceID != profile.DeviceID {
			return fail(domain.NewError(domain.CodeProfileBindingChanged, "auth profile binding changed"))
		}
		aliasDigest, err := enrollmentAliasDigest(enrollment.ID, "@"+profile.SelectorAlias)
		if err != nil {
			return fail(err)
		}
		awaiting, err := s.Store.AwaitAuthEnrollmentConfirmation(ctx, enrollment.ID, auth.ClientID,
			account.ID, profile.ID, credential.ID, account.Revision, profile.Revision,
			credential.CredentialRevision, aliasDigest, s.now())
		if err != nil {
			return fail(err)
		}
		return map[string]any{"enrollment_id": awaiting.ID, "credential_id": credential.ID,
			"account_id": account.ID, "runtime_profile_id": profile.ID, "profile_selector": "@" + profile.SelectorAlias,
			"state": storage.EnrollmentAwaitingConfirmation, "confirmation_expires_at": awaiting.ExpiresAt}, nil
	case "auth.confirm":
		var body authConfirmBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		if !body.Confirmed {
			return nil, domain.NewError(domain.CodeIdentityConfirmationRequired, "explicit auth confirmation is required")
		}
		enrollment, err := s.Store.AuthEnrollment(ctx, body.EnrollmentID)
		if err != nil {
			return nil, err
		}
		if enrollment.ClientDeviceID != auth.ClientID {
			return nil, domain.NewError(domain.CodePermissionDenied, "auth enrollment owner is required")
		}
		if enrollment.State == storage.EnrollmentSucceeded {
			credential, err := s.Store.CredentialInstance(ctx, enrollment.CredentialInstanceID)
			if err != nil {
				return nil, err
			}
			return map[string]any{"enrollment_id": enrollment.ID, "credential_id": credential.ID,
				"state": enrollment.State, "credential_revision": credential.CredentialRevision}, nil
		}
		binding, err := s.resolveCodexProfile(ctx, body.ProfileSelector, "")
		if err != nil {
			return nil, err
		}
		aliasDigest, err := enrollmentAliasDigest(enrollment.ID, body.ProfileSelector)
		if err != nil {
			return nil, err
		}
		if binding.Account.ID != enrollment.ConfirmationAccountID || binding.Profile.ID != enrollment.ConfirmationProfileID ||
			enrollment.CredentialInstanceID != enrollment.ConfirmationCredentialID ||
			binding.Account.Revision != enrollment.ConfirmationAccountRevision ||
			binding.Profile.Revision != enrollment.ConfirmationProfileRevision ||
			aliasDigest != enrollment.ConfirmationAliasDigest {
			return nil, domain.NewError(domain.CodeIdentityConfirmationMismatch, "auth confirmation does not match enrollment")
		}
		credentialBeforeConfirm, err := s.Store.CredentialInstance(ctx, enrollment.CredentialInstanceID)
		if err != nil || credentialBeforeConfirm.CredentialRevision != enrollment.ConfirmationCredentialRevision ||
			credentialBeforeConfirm.AccountID != enrollment.ConfirmationAccountID ||
			credentialBeforeConfirm.DeviceID != binding.Profile.DeviceID ||
			credentialBeforeConfirm.Provider != domain.ProviderCodex ||
			credentialBeforeConfirm.Status == domain.CredentialRevoked || credentialBeforeConfirm.Status == domain.CredentialExpired {
			return nil, domain.NewError(domain.CodeProfileBindingChanged, "auth credential changed before confirmation")
		}
		stagingPath := enrollment.StagingPath
		enrollment, err = s.Store.ConfirmAuthEnrollmentAttestation(ctx, enrollment.ID, auth.ClientID, aliasDigest, s.now())
		if err != nil {
			if domain.CodeOf(err) == domain.CodeConfirmationExpired {
				_ = s.removeEnrollmentStaging(stagingPath)
			}
			return nil, err
		}
		failConfirm := func(cause error) (any, error) {
			_, _ = s.Store.FinishAuthEnrollment(ctx, enrollment.ID, auth.ClientID, storage.EnrollmentFailed, s.now())
			_ = s.removeEnrollmentStaging(enrollment.StagingPath)
			return nil, cause
		}
		if s.Vault == nil {
			return failConfirm(domain.NewError(domain.CodeVaultLocked, "vault is unavailable"))
		}
		if err := s.Vault.RequireUnlocked(); err != nil {
			return failConfirm(err)
		}
		descriptor, err := codex.Discover(ctx, codex.DiscoverOptions{})
		if err != nil {
			return failConfirm(err)
		}
		fingerprint, err := codex.BinaryFingerprint(descriptor)
		if err != nil || fingerprint != enrollment.BinaryFingerprint {
			return failConfirm(domain.NewError(domain.CodeProviderVersionUnsupported, "enrollment binary changed before confirmation"))
		}
		validateEnrollment := codex.ValidateEnrollment
		if s.EnrollmentValidator != nil {
			validateEnrollment = s.EnrollmentValidator
		}
		if err := validateEnrollment(ctx, descriptor, enrollment.StagingPath); err != nil {
			return failConfirm(err)
		}
		credentialBytes, err := codex.ReadEnrollmentAuth(enrollment.StagingPath)
		if err != nil {
			return failConfirm(err)
		}
		defer zeroSecretBytes(credentialBytes)
		credential, err := s.Store.CredentialInstance(ctx, enrollment.CredentialInstanceID)
		if err != nil {
			return failConfirm(err)
		}
		revision, err := s.Vault.SealEnrollmentCredential(ctx, enrollment.ID, auth.ClientID,
			enrollment.CompletionIdempotencyDigest, vault.CredentialMetadata{CredentialInstanceID: credential.ID,
				AccountID: credential.AccountID, DeviceID: credential.DeviceID, Provider: credential.Provider,
				ExpectedRevision: credential.CredentialRevision, CreatedAt: credential.CreatedAt, UpdatedAt: s.now()}, credentialBytes)
		if err != nil {
			return failConfirm(err)
		}
		if err := s.removeEnrollmentStaging(enrollment.StagingPath); err != nil {
			return nil, err
		}
		return map[string]any{"enrollment_id": enrollment.ID, "credential_id": credential.ID,
			"state": storage.EnrollmentSucceeded, "credential_revision": revision}, nil
	case "auth.cancel":
		var body authEnrollmentBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		enrollment, err := s.Store.AuthEnrollment(ctx, body.EnrollmentID)
		if err != nil {
			return nil, err
		}
		if enrollment.ClientDeviceID != auth.ClientID {
			return nil, domain.NewError(domain.CodePermissionDenied, "auth enrollment owner is required")
		}
		finished, err := s.Store.FinishAuthEnrollment(ctx, body.EnrollmentID, auth.ClientID, storage.EnrollmentCancelled, s.now())
		if err != nil {
			return nil, err
		}
		if err := s.removeEnrollmentStaging(enrollment.StagingPath); err != nil {
			return nil, err
		}
		return map[string]any{"enrollment_id": finished.ID, "state": finished.State}, nil
	case "auth.status":
		var body authStatusBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		if body.EnrollmentID != "" {
			if body.CredentialID != "" || body.ProfileSelector != "" {
				return nil, domain.NewError(domain.CodeInvalidArgument, "auth status accepts one target")
			}
			enrollment, err := s.Store.AuthEnrollment(ctx, body.EnrollmentID)
			if err != nil {
				return nil, err
			}
			if enrollment.ClientDeviceID != auth.ClientID {
				return nil, domain.NewError(domain.CodePermissionDenied, "auth enrollment owner is required")
			}
			return map[string]any{"enrollment_id": enrollment.ID, "credential_id": enrollment.CredentialInstanceID, "state": enrollment.State, "expires_at": enrollment.ExpiresAt}, nil
		}
		credentialID := body.CredentialID
		if body.ProfileSelector != "" {
			if credentialID != "" {
				return nil, domain.NewError(domain.CodeInvalidArgument, "auth status accepts one target")
			}
			binding, err := s.resolveCodexProfile(ctx, body.ProfileSelector, "")
			if err != nil {
				return nil, err
			}
			if binding.Credential == nil {
				return nil, domain.NewError(domain.CodeIdentityConfirmationRequired, "profile login is not confirmed")
			}
			credentialID = binding.Credential.ID
		}
		if credentialID == "" {
			return nil, domain.NewError(domain.CodeInvalidArgument, "auth status target is required")
		}
		credential, err := s.Store.CredentialInstance(ctx, credentialID)
		if err != nil {
			return nil, err
		}
		return credentialView(credential), nil
	case "auth.logout":
		var body credentialBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		credentialID := body.CredentialID
		if body.ProfileSelector != "" {
			if credentialID != "" {
				return nil, domain.NewError(domain.CodeInvalidArgument, "auth logout accepts one target")
			}
			binding, err := s.resolveCodexProfile(ctx, body.ProfileSelector, "")
			if err != nil {
				return nil, err
			}
			if binding.Credential == nil {
				return nil, domain.NewError(domain.CodeIdentityConfirmationRequired, "profile login is not confirmed")
			}
			credentialID = binding.Credential.ID
		}
		if credentialID == "" {
			return nil, domain.NewError(domain.CodeInvalidArgument, "auth logout target is required")
		}
		if err := s.Store.ReserveVaultCredentialRevocation(ctx, credentialID, s.now()); err != nil {
			return nil, err
		}
		if err := s.removeCredentialHome(credentialID); err != nil {
			return nil, err
		}
		credential, err := s.Store.FinalizeVaultCredentialRevocation(ctx, credentialID, s.now())
		if err != nil {
			return nil, err
		}
		return credentialView(credential), nil
	case "usage.read":
		var body usageBody
		if len(request.Body) != 0 {
			if err := decodeBody(request.Body, &body); err != nil {
				return nil, err
			}
		}
		if body.AccountID == "" {
			return map[string]any{"provider": domain.ProviderCodex, "available": false, "capability_status": domain.UsageUnavailable, "snapshots": []any{}}, nil
		}
		capabilityStatus := domain.UsageUnavailable
		if s.Runtime != nil && s.Runtime.Codex != nil {
			if snapshot, readErr := s.Runtime.Codex.ReadUsage(ctx, body.AccountID); readErr == nil {
				capabilityStatus = snapshot.CapabilityStatus
			} else if domain.CodeOf(readErr) != domain.CodeUsageUnavailable {
				return nil, readErr
			}
		}
		snapshots, err := s.Store.ListUsageSnapshots(ctx, body.AccountID)
		if err != nil {
			return nil, err
		}
		result := make([]any, 0, len(snapshots))
		for _, snapshot := range snapshots {
			result = append(result, usageView(snapshot))
		}
		return map[string]any{"provider": domain.ProviderCodex, "available": capabilityStatus == domain.UsageSupported,
			"capability_status": capabilityStatus, "snapshots": result}, nil
	case "approval.list", "approval.observe":
		var body sessionBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		approvals, err := s.Store.ListApprovals(ctx, body.SessionID)
		if err != nil {
			return nil, err
		}
		result := make([]any, 0, len(approvals))
		for _, approval := range approvals {
			result = append(result, approvalView(approval))
		}
		return result, nil
	case "approval.respond":
		var body approvalResponseBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		revision, err := requestRevision(request)
		if err != nil {
			return nil, err
		}
		storedApproval, err := s.Store.Approval(ctx, body.ApprovalID)
		if err != nil {
			return nil, err
		}
		if storedApproval.SessionID != body.SessionID {
			return nil, domain.NewError(domain.CodeConflict, "approval does not belong to session")
		}
		lease, err := s.Store.ControllerLease(ctx, body.SessionID)
		if err != nil {
			return nil, domain.NewError(domain.CodeApprovalLeaseRequired, "controller lease is required for approval response")
		}
		if err := lease.RequireControl(auth.ClientID, revision, s.now()); err != nil {
			return nil, domain.NewError(domain.CodeApprovalLeaseRequired, "controller lease is required for approval response")
		}
		if s.Runtime == nil || s.Runtime.Codex == nil {
			return nil, domain.NewError(domain.CodeProviderUnsupported, "approval dispatch requires the Codex runtime bridge")
		}
		decision := domain.ApprovalDecision(body.Decision)
		approval, err := s.Runtime.Codex.RespondApproval(ctx, codex.ApprovalDispatchRequest{SessionID: body.SessionID,
			ApprovalID: body.ApprovalID, ProviderApprovalID: body.ProviderApprovalID, ResponderID: auth.ClientID,
			ResponseKey: request.IdempotencyKey, Decision: decision, LeaseRevision: revision})
		if err != nil {
			return nil, err
		}
		return approvalView(approval), nil
	case "sessions.show":
		var body sessionBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		session, err := s.Store.Session(ctx, body.SessionID)
		if err != nil {
			return nil, err
		}
		return sessionView(session), nil
	case "sessions.preview":
		var body previewBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		if body.Provider != "" && body.Provider != domain.ProviderCodex {
			return nil, domain.NewError(domain.CodeProfileBindingChanged, "selector preview is Codex-only")
		}
		if s.Vault == nil {
			return nil, domain.NewError(domain.CodeVaultLocked, "vault is unavailable")
		}
		if err := s.Vault.RequireUnlocked(); err != nil {
			return nil, err
		}
		preflight, err := s.selectorPreflight(ctx)
		if err != nil {
			return nil, err
		}
		binding, err := s.resolveCodexProfile(ctx, body.ProfileSelector, "")
		if err != nil {
			return nil, err
		}
		if !binding.Account.Enabled {
			return nil, domain.NewError(domain.CodeAccountDisabled, "account is disabled")
		}
		if !binding.Profile.Enabled {
			return nil, domain.NewError(domain.CodeProfileDisabled, "profile is disabled")
		}
		if binding.Credential == nil || binding.Credential.Status != domain.CredentialHealthy {
			return nil, domain.NewError(domain.CodeIdentityConfirmationRequired, "confirmed profile login is required")
		}
		workspace, err := s.Store.Workspace(ctx, body.WorkspaceID)
		if err != nil {
			return nil, err
		}
		if workspace.DeviceID != binding.Profile.DeviceID {
			return nil, domain.NewError(domain.CodeProfileBindingChanged, "workspace does not match profile device")
		}
		snapshots, err := s.Store.ListUsageSnapshotsWithWindows(ctx, binding.Account.ID)
		if err != nil {
			return nil, err
		}
		var usageID domain.ID
		var usageObservedAt any
		usageStale := true
		if len(snapshots) != 0 {
			usageID, usageObservedAt = snapshots[0].ID, snapshots[0].ObservedAt
			usageStale = !s.now().Before(snapshots[0].StaleAt)
		}
		previewID, err := domain.NewID("preview")
		if err != nil {
			return nil, err
		}
		preview := storage.SessionStartPreview{ID: previewID, ClientID: auth.ClientID, Provider: domain.ProviderCodex,
			AccountID: binding.Account.ID, AccountRevision: binding.Account.Revision,
			RuntimeProfileID: binding.Profile.ID, ProfileRevision: binding.Profile.Revision,
			CredentialInstanceID: binding.Credential.ID, CredentialRevision: binding.Credential.CredentialRevision,
			DeviceID: binding.Profile.DeviceID, WorkspaceID: workspace.ID, UsageSnapshotID: usageID,
			ProviderVersion: preflight.ProviderVersion, BinaryFingerprint: preflight.BinaryFingerprint,
			SchemaFingerprint: preflight.SchemaFingerprint, CapabilityDigest: preflight.CapabilityDigest,
			CreatedAt: s.now(), ExpiresAt: s.now().Add(10 * time.Minute)}
		if err := s.Store.CreateSessionStartPreview(ctx, preview); err != nil {
			return nil, err
		}
		return map[string]any{"schema_version": 1, "preview_id": preview.ID, "expires_at": preview.ExpiresAt,
			"provider": domain.ProviderCodex, "account_id": binding.Account.ID, "account_revision": binding.Account.Revision,
			"account_label": binding.Account.DisplayName, "runtime_profile_id": binding.Profile.ID,
			"profile_revision": binding.Profile.Revision, "profile_alias": binding.Profile.SelectorAlias,
			"profile_label": binding.Profile.Name, "credential_instance_id": binding.Credential.ID,
			"credential_revision": binding.Credential.CredentialRevision, "auth_status": binding.Credential.Status,
			"device_id": binding.Profile.DeviceID, "workspace_id": workspace.ID,
			"provider_version": preflight.ProviderVersion, "compatibility_status": "supported",
			"capability_snapshot": preflight.Capabilities, "usage_snapshot_id": usageID,
			"usage_observed_at": usageObservedAt, "usage_stale": usageStale}, nil
	case "sessions.start", "session.start":
		var body startBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		if body.ProfileSelector != "" || body.Provider == domain.ProviderCodex || body.AccountID != "" {
			if body.ProfileSelector == "" || body.PreviewID == "" || body.Confirmation == nil || !body.Confirmation.Confirmed {
				return nil, domain.NewError(domain.CodeIdentityConfirmationRequired, "daemon-issued preview and explicit confirmation are required")
			}
			preflight, err := s.selectorPreflight(ctx)
			if err != nil {
				return nil, err
			}
			binding, err := s.Store.ResolveProfile(ctx, body.ProfileSelector)
			if err != nil {
				return nil, err
			}
			if !binding.Account.Enabled {
				return nil, domain.NewError(domain.CodeAccountDisabled, "account is disabled")
			}
			if !binding.Profile.Enabled {
				return nil, domain.NewError(domain.CodeProfileDisabled, "profile is disabled")
			}
			if binding.Credential == nil {
				return nil, domain.NewError(domain.CodeProfileBindingChanged, "profile credential changed after preview")
			}
			sessionID, err := domain.NewID("session")
			if err != nil {
				return nil, err
			}
			digest := sha256.Sum256(append([]byte(request.Method+"\x00"+request.IdempotencyKey+"\x00"), request.Body...))
			session, err := s.Store.ConsumeSessionStartPreview(ctx, storage.ConsumeSessionStartPreviewRequest{
				PreviewID: body.PreviewID, ClientID: auth.ClientID, RequestDigest: hex.EncodeToString(digest[:]), At: s.now(),
				BinaryFingerprint: preflight.BinaryFingerprint, SchemaFingerprint: preflight.SchemaFingerprint,
				CapabilityDigest: preflight.CapabilityDigest,
				Confirmation: storage.SessionStartConfirmation{Confirmed: body.Confirmation.Confirmed,
					AccountID: body.Confirmation.AccountID, AccountRevision: body.Confirmation.AccountRevision,
					RuntimeProfileID: body.Confirmation.RuntimeProfileID, ProfileRevision: body.Confirmation.ProfileRevision,
					CredentialInstanceID: body.Confirmation.CredentialInstanceID, CredentialRevision: body.Confirmation.CredentialRevision,
					DeviceID: body.Confirmation.DeviceID, WorkspaceID: body.Confirmation.WorkspaceID,
					UsageSnapshotID: body.Confirmation.UsageSnapshotID, ProviderVersion: body.Confirmation.ProviderVersion},
				Session: domain.Session{ID: sessionID, DeviceID: binding.Profile.DeviceID, AccountID: binding.Account.ID,
					Provider: domain.ProviderCodex, CredentialInstanceID: binding.Credential.ID,
					RuntimeProfileID: binding.Profile.ID, WorkspaceID: body.WorkspaceID, Status: domain.SessionStarting,
					StartedAt: s.now(), CapabilitySnapshot: preflight.Capabilities},
			})
			if err != nil {
				return nil, err
			}
			// P1 proves the reservation boundary without launching a Provider process.
			// P2 replaces this deterministic terminal receipt with StartReserved.
			if session.Status == domain.SessionStarting {
				session, err = s.Store.TransitionSession(ctx, session.ID, domain.SessionStarting, domain.SessionFailed,
					s.now(), nil, string(domain.CodeProviderCapabilityUnavailable))
				if err != nil {
					return nil, err
				}
			}
			return sessionView(session), nil
		}
		session, err := s.Runtime.Start(ctx, runtime.StartRequest{Provider: body.Provider, AccountID: body.AccountID, DeviceID: body.DeviceID,
			CredentialInstanceID: body.CredentialInstanceID, RuntimeProfileID: body.RuntimeProfileID,
			WorkspaceID: body.WorkspaceID, Capabilities: body.Capabilities})
		if err != nil {
			return nil, err
		}
		return sessionView(session), nil
	case "sessions.attach":
		var body attachBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		mode := domain.AttachmentObserver
		if body.Mode != "" {
			mode = domain.AttachmentMode(body.Mode)
		}
		if err := s.Runtime.Attach(ctx, body.SessionID, auth.ClientID, mode); err != nil {
			return nil, err
		}
		return map[string]any{"session_id": body.SessionID, "client_id": auth.ClientID, "mode": mode}, nil
	case "sessions.detach":
		var body sessionBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		if err := s.Runtime.Detach(ctx, body.SessionID, auth.ClientID); err != nil {
			return nil, err
		}
		return map[string]any{"session_id": body.SessionID, "client_id": auth.ClientID, "detached": true}, nil
	case "sessions.observe":
		var body observeBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		replay, replayErr := s.Runtime.Replay(ctx, body.SessionID, body.FromSequence)
		result := map[string]any{"session_id": body.SessionID, "next_sequence": replay.NextSequence, "truncated": replay.Truncated, "chunks": replay.Chunks}
		if replayErr != nil && domain.CodeOf(replayErr) != domain.CodeReplayUnavailable {
			return nil, replayErr
		}
		return result, nil
	case "control.acquire":
		var body sessionBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		lease, err := s.Runtime.Acquire(ctx, body.SessionID, auth.ClientID)
		if err != nil {
			return nil, err
		}
		return leaseView(lease), nil
	case "control.heartbeat":
		var body sessionBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		revision, err := requestRevision(request)
		if err != nil {
			return nil, err
		}
		lease, err := s.Runtime.Heartbeat(ctx, body.SessionID, auth.ClientID, revision)
		if err != nil {
			return nil, err
		}
		return leaseView(lease), nil
	case "control.release":
		var body sessionBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		revision, err := requestRevision(request)
		if err != nil {
			return nil, err
		}
		lease, err := s.Runtime.Release(ctx, body.SessionID, auth.ClientID, revision)
		if err != nil {
			return nil, err
		}
		return leaseView(lease), nil
	case "terminal.input", "session.input":
		var body inputBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		revision, err := requestRevision(request)
		if err != nil {
			return nil, err
		}
		return s.Runtime.Input(ctx, runtime.InputRequest{SessionID: body.SessionID, ClientID: auth.ClientID,
			Revision: revision, Sequence: body.Sequence, Payload: body.Payload})
	case "terminal.resize", "session.resize":
		var body resizeBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		revision, err := requestRevision(request)
		if err != nil {
			return nil, err
		}
		if err := s.Runtime.Resize(ctx, runtime.ResizeRequest{SessionID: body.SessionID, ClientID: auth.ClientID,
			Revision: revision, Rows: body.Rows, Cols: body.Cols}); err != nil {
			return nil, err
		}
		return map[string]any{"session_id": body.SessionID, "rows": body.Rows, "cols": body.Cols}, nil
	case "sessions.stop", "session.stop", "sessions.kill":
		var body sessionBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		revision, err := requestRevision(request)
		if err != nil {
			return nil, err
		}
		var session domain.Session
		if request.Method == "sessions.stop" || request.Method == "session.stop" {
			session, err = s.Runtime.Stop(ctx, body.SessionID, auth.ClientID, revision)
		} else {
			session, err = s.Runtime.Kill(ctx, body.SessionID, auth.ClientID, revision)
		}
		if err != nil {
			return nil, err
		}
		return sessionView(session), nil
	case "sessions.resume", "session.resume":
		var body sessionBody
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		session, err := s.Runtime.Resume(ctx, body.SessionID)
		if err != nil {
			return nil, err
		}
		return sessionView(session), nil
	default:
		return nil, domain.NewError(domain.CodeMethodNotFound, "method is not available")
	}
}

func (s *SessionService) codexDescribe(ctx context.Context) map[string]any {
	descriptor, err := codex.Discover(ctx, codex.DiscoverOptions{})
	if err != nil {
		return map[string]any{"provider": domain.ProviderCodex, "status": "unsupported", "phase": "P1",
			"capabilities": []string{}, "reason": "codex binary was not discovered"}
	}
	capabilities, probeErr := codex.Probe(ctx, descriptor, codex.ProbeOptions{})
	if probeErr != nil {
		status := capabilities.Status
		if status == "" {
			status = "unsupported"
		}
		return map[string]any{"provider": domain.ProviderCodex, "status": status, "phase": "P1",
			"binary_path": descriptor.Path, "version": descriptor.Version, "platform": descriptor.Platform,
			"architecture": descriptor.Architecture, "schema_fingerprint": descriptor.SchemaFingerprint,
			"capabilities": capabilities.Methods, "reason": domain.CodeOf(probeErr)}
	}
	return map[string]any{"provider": domain.ProviderCodex, "status": capabilities.Status, "phase": "P1",
		"binary_path": capabilities.BinaryPath, "version": capabilities.Version, "platform": capabilities.Platform,
		"architecture": capabilities.Architecture, "schema_fingerprint": capabilities.SchemaFingerprint,
		"capabilities": capabilities.Methods, "experimental": capabilities.Experimental}
}

type sessionBody struct {
	SessionID domain.ID `json:"session_id"`
}
type accountBody struct {
	AccountID domain.ID `json:"account_id"`
}
type accountCreateBody struct {
	Provider              string `json:"provider"`
	DisplayName           string `json:"display_name"`
	ProviderSubjectDigest string `json:"provider_subject_digest,omitempty"`
}
type profileBody struct {
	ProfileID domain.ID `json:"profile_id"`
}
type profileCreateBody struct {
	DeviceID  domain.ID       `json:"device_id"`
	AccountID domain.ID       `json:"account_id,omitempty"`
	Name      string          `json:"name"`
	Provider  string          `json:"provider"`
	Settings  json.RawMessage `json:"settings,omitempty"`
}
type profileEditBody struct {
	ProfileID domain.ID       `json:"profile_id"`
	Name      string          `json:"name,omitempty"`
	Provider  string          `json:"provider,omitempty"`
	AccountID domain.ID       `json:"account_id,omitempty"`
	Settings  json.RawMessage `json:"settings,omitempty"`
}
type credentialBody struct {
	CredentialID    domain.ID `json:"credential_id"`
	ProfileSelector string    `json:"profile_selector,omitempty"`
}
type authBeginBody struct {
	ProfileID       domain.ID `json:"profile_id,omitempty"`
	ProfileSelector string    `json:"profile_selector,omitempty"`
	CredentialID    domain.ID `json:"credential_id,omitempty"`
	Mode            string    `json:"mode,omitempty"`
}
type authEnrollmentBody struct {
	EnrollmentID domain.ID `json:"enrollment_id"`
}
type authConfirmBody struct {
	EnrollmentID    domain.ID `json:"enrollment_id"`
	ProfileSelector string    `json:"profile_selector"`
	Confirmed       bool      `json:"confirmed"`
}
type authStatusBody struct {
	EnrollmentID    domain.ID `json:"enrollment_id,omitempty"`
	CredentialID    domain.ID `json:"credential_id,omitempty"`
	ProfileSelector string    `json:"profile_selector,omitempty"`
}
type usageBody struct {
	AccountID domain.ID `json:"account_id,omitempty"`
}
type approvalResponseBody struct {
	SessionID          domain.ID `json:"session_id"`
	ApprovalID         domain.ID `json:"approval_id"`
	ProviderApprovalID string    `json:"provider_approval_id"`
	Decision           string    `json:"decision"`
}
type vaultBody struct {
	Secret string `json:"secret"`
}
type observeBody struct {
	SessionID    domain.ID `json:"session_id"`
	FromSequence int64     `json:"from_sequence,omitempty"`
}
type attachBody struct {
	SessionID domain.ID `json:"session_id"`
	Mode      string    `json:"mode,omitempty"`
}
type previewBody struct {
	Provider        string    `json:"provider,omitempty"`
	ProfileSelector string    `json:"profile_selector"`
	WorkspaceID     domain.ID `json:"workspace_id"`
}
type startBody struct {
	DeviceID             domain.ID            `json:"device_id"`
	Provider             string               `json:"provider,omitempty"`
	AccountID            domain.ID            `json:"account_id,omitempty"`
	CredentialInstanceID domain.ID            `json:"credential_instance_id"`
	RuntimeProfileID     domain.ID            `json:"runtime_profile_id"`
	WorkspaceID          domain.ID            `json:"workspace_id"`
	Capabilities         []domain.Capability  `json:"capabilities"`
	ProfileSelector      string               `json:"profile_selector,omitempty"`
	PreviewID            domain.ID            `json:"preview_id,omitempty"`
	WorkspacePath        string               `json:"workspace_path,omitempty"`
	Confirmation         *profileConfirmation `json:"confirmation,omitempty"`
	ProviderArgs         []string             `json:"provider_args,omitempty"`
	NonInteractive       bool                 `json:"non_interactive,omitempty"`
}

type profileConfirmation struct {
	Confirmed            bool      `json:"confirmed"`
	AccountID            domain.ID `json:"account_id"`
	AccountRevision      int64     `json:"account_revision"`
	CredentialInstanceID domain.ID `json:"credential_instance_id"`
	CredentialRevision   int64     `json:"credential_revision"`
	RuntimeProfileID     domain.ID `json:"runtime_profile_id"`
	ProfileRevision      int64     `json:"profile_revision"`
	DeviceID             domain.ID `json:"device_id"`
	WorkspaceID          domain.ID `json:"workspace_id"`
	UsageSnapshotID      domain.ID `json:"usage_snapshot_id"`
	ProviderVersion      string    `json:"provider_version"`
}
type inputBody struct {
	SessionID domain.ID `json:"session_id"`
	Sequence  int64     `json:"sequence"`
	Payload   string    `json:"payload"`
}
type resizeBody struct {
	SessionID domain.ID `json:"session_id"`
	Rows      int       `json:"rows"`
	Cols      int       `json:"cols"`
}

func zeroSecretBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

func decodeBody(body json.RawMessage, target any) error {
	if len(body) == 0 {
		return domain.NewError(domain.CodeInvalidArgument, "request body is required")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return domain.NewError(domain.CodeInvalidArgument, "request body is invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return domain.NewError(domain.CodeInvalidArgument, "request body is invalid")
	}
	return nil
}

func requestRevision(request device.Request) (int64, error) {
	if request.LeaseRevision == nil || *request.LeaseRevision < 1 {
		return 0, domain.NewError(domain.CodeInvalidArgument, "lease revision is required")
	}
	return *request.LeaseRevision, nil
}

func sessionView(session domain.Session) map[string]any {
	return map[string]any{"id": session.ID, "device_id": session.DeviceID, "provider": session.Provider, "account_id": session.AccountID,
		"credential_instance_id": session.CredentialInstanceID, "runtime_profile_id": session.RuntimeProfileID,
		"workspace_id": session.WorkspaceID, "provider_session_id": session.ProviderSessionID,
		"resumed_from_session_id": session.ResumedFromSessionID, "status": session.Status,
		"started_at": session.StartedAt, "ended_at": session.EndedAt, "exit_code": session.ExitCode,
		"capability_snapshot": session.CapabilitySnapshot, "failure_code": session.FailureCode}
}

func accountView(account domain.Account) map[string]any {
	return map[string]any{"id": account.ID, "provider": account.Provider, "display_name": account.DisplayName,
		"provider_subject_digest": account.ProviderSubjectDigest, "enabled": account.Enabled,
		"created_at": account.CreatedAt, "updated_at": account.UpdatedAt}
}

func profileView(profile domain.RuntimeProfile) map[string]any {
	return map[string]any{"id": profile.ID, "device_id": profile.DeviceID, "account_id": profile.AccountID,
		"name": profile.Name, "provider": profile.Provider, "settings": profile.Settings,
		"created_at": profile.CreatedAt, "updated_at": profile.UpdatedAt}
}

func credentialView(credential domain.CredentialInstance) map[string]any {
	return map[string]any{"id": credential.ID, "device_id": credential.DeviceID, "account_id": credential.AccountID,
		"provider": credential.Provider, "auth_method": credential.AuthMethod, "secret_ref": credential.SecretRef,
		"status": credential.Status, "credential_revision": credential.CredentialRevision,
		"secret_digest": credential.SecretDigest, "created_at": credential.CreatedAt, "updated_at": credential.UpdatedAt}
}

func approvalView(approval domain.Approval) map[string]any {
	return map[string]any{"id": approval.ID, "session_id": approval.SessionID, "provider_approval_id": approval.ProviderApprovalID,
		"kind": approval.Kind, "payload_digest": approval.PayloadDigest, "summary": approval.Summary,
		"status": approval.Status, "response_state": approval.ResponseState,
		"requested_decision": approval.RequestedDecision, "responded_by_device_id": approval.RespondedByDeviceID,
		"idempotency_key": approval.IdempotencyKey, "requested_at": approval.RequestedAt,
		"dispatch_started_at": approval.DispatchStartedAt, "responded_at": approval.RespondedAt,
		"dispatch_error_code": approval.DispatchErrorCode}
}

func usageView(snapshot domain.UsageSnapshot) map[string]any {
	return map[string]any{"id": snapshot.ID, "provider": snapshot.Provider, "account_id": snapshot.AccountID,
		"device_id": snapshot.DeviceID, "source": snapshot.Source, "confidence": snapshot.Confidence,
		"window_kind": snapshot.WindowKind, "used_value": snapshot.UsedValue, "limit_value": snapshot.LimitValue,
		"used_percent": snapshot.UsedPercent, "resets_at": snapshot.ResetsAt, "observed_at": snapshot.ObservedAt,
		"raw_reference_hash": snapshot.RawReferenceHash, "source_version": snapshot.SourceVersion,
		"capability_status": snapshot.CapabilityStatus, "error_code": snapshot.ErrorCode}
}

func leaseView(lease domain.ControllerLease) map[string]any {
	return map[string]any{"session_id": lease.SessionID, "holder_device_id": lease.HolderDeviceID,
		"revision": lease.Revision, "expires_at": lease.ExpiresAt, "last_heartbeat_at": lease.LastHeartbeat,
		"released_at": lease.ReleasedAt}
}

func clientView(client domain.ClientIdentity) map[string]any {
	return map[string]any{"id": client.ID, "name": client.Name, "revision": client.Revision,
		"status": client.Status, "capabilities": client.Caps, "created_at": client.CreatedAt,
		"updated_at": client.UpdatedAt}
}
