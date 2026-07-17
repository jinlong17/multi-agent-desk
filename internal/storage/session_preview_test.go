package storage

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

func createCodexPreviewFixture(t *testing.T, store *Store) (domain.Device, domain.ClientIdentity, domain.ClientIdentity, domain.Account, domain.RuntimeProfile, domain.CredentialInstance, domain.Workspace) {
	t.Helper()
	ctx := context.Background()
	now := time.Unix(2_000, 0).UTC()
	device := domain.Device{ID: storageID("device", storageHexA), Kind: domain.DeviceKindDaemon,
		DisplayName: "selector", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}
	clientA := domain.ClientIdentity{ID: storageID("client", storageHexA), Name: "A", PublicKey: bytesOf(3, 32),
		Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityMetadataRead, domain.CapabilitySessionStart}, CreatedAt: now, UpdatedAt: now}
	clientB := domain.ClientIdentity{ID: storageID("client", storageHexB), Name: "B", PublicKey: bytesOf(4, 32),
		Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityMetadataRead, domain.CapabilitySessionStart}, CreatedAt: now, UpdatedAt: now}
	account := domain.Account{ID: storageID("account", storageHexA), Provider: domain.ProviderCodex,
		DisplayName: "Work", Enabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now}
	profile := domain.RuntimeProfile{ID: storageID("profile", storageHexA), DeviceID: device.ID, AccountID: account.ID,
		Name: "Work Linux", Provider: domain.ProviderCodex, SelectorAlias: "A", SelectorKey: "a", Settings: []byte(`{}`),
		Enabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now}
	credential := domain.CredentialInstance{ID: storageID("credential", storageHexA), DeviceID: device.ID, AccountID: account.ID,
		Provider: domain.ProviderCodex, AuthMethod: domain.AuthMethodInteractive, SecretRef: "vault:" + string(storageID("credential", storageHexA)),
		Status: domain.CredentialHealthy, CredentialRevision: 2, SecretDigest: strings.Repeat("a", 64), CreatedAt: now, UpdatedAt: now}
	workspace := domain.Workspace{ID: storageID("workspace", storageHexA), DeviceID: device.ID, Path: "/tmp/selector",
		Label: "selector", Tags: []string{}, CreatedAt: now, UpdatedAt: now}
	for _, call := range []func() error{
		func() error { return store.CreateDevice(ctx, device) },
		func() error { return store.CreateClientIdentity(ctx, clientA) },
		func() error { return store.CreateClientIdentity(ctx, clientB) },
		func() error { _, _, err := store.CreateAccountWithDefaultProfile(ctx, account, profile); return err },
		func() error { return store.CreateCredentialInstance(ctx, credential) },
		func() error { return store.CreateWorkspace(ctx, workspace) },
	} {
		if err := call(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE runtime_profiles SET credential_instance_id=? WHERE id=?`, credential.ID, profile.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO vault_items(
		credential_instance_id, account_id, device_id, provider, envelope_version, credential_revision,
		cipher_name, payload_nonce, payload_ciphertext, wrap_name, wrap_nonce, wrapped_dek,
		aad_digest, secret_digest, created_at, updated_at
	) VALUES(?, ?, ?, 'codex', 1, 2, 'aes-256-gcm', ?, ?, 'aes-256-gcm', ?, ?, ?, ?, ?, ?)`,
		credential.ID, account.ID, device.ID, make([]byte, 12), make([]byte, 18), make([]byte, 12), make([]byte, 48),
		strings.Repeat("b", 64), credential.SecretDigest, formatTime(now), formatTime(now)); err != nil {
		t.Fatal(err)
	}
	profile.CredentialInstanceID = credential.ID
	return device, clientA, clientB, account, profile, credential, workspace
}

func TestSessionStartPreviewIsOwnerBoundSingleUseAndReplaySafe(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	device, clientA, clientB, account, profile, credential, workspace := createCodexPreviewFixture(t, store)
	now := time.Unix(2_100, 0).UTC()
	fingerprint := strings.Repeat("c", 64)
	previewID := storageID("preview", storageHexA)
	preview := SessionStartPreview{ID: previewID, ClientID: clientA.ID, Provider: domain.ProviderCodex,
		AccountID: account.ID, AccountRevision: 1, RuntimeProfileID: profile.ID, ProfileRevision: 1,
		CredentialInstanceID: credential.ID, CredentialRevision: 2, DeviceID: device.ID, WorkspaceID: workspace.ID,
		ProviderVersion: "0.144.2", BinaryFingerprint: fingerprint, SchemaFingerprint: strings.Repeat("d", 64),
		CapabilityDigest: strings.Repeat("e", 64), CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute)}
	if err := store.CreateSessionStartPreview(ctx, preview); err != nil {
		t.Fatal(err)
	}
	confirmation := SessionStartConfirmation{Confirmed: true, AccountID: account.ID, AccountRevision: 1,
		RuntimeProfileID: profile.ID, ProfileRevision: 1, CredentialInstanceID: credential.ID, CredentialRevision: 2,
		DeviceID: device.ID, WorkspaceID: workspace.ID, ProviderVersion: "0.144.2"}
	request := ConsumeSessionStartPreviewRequest{PreviewID: previewID, ClientID: clientB.ID,
		RequestDigest: strings.Repeat("1", 64), At: now.Add(time.Second), BinaryFingerprint: fingerprint,
		SchemaFingerprint: preview.SchemaFingerprint, CapabilityDigest: preview.CapabilityDigest, Confirmation: confirmation,
		Session: domain.Session{ID: storageID("session", storageHexA), DeviceID: device.ID, AccountID: account.ID,
			Provider: domain.ProviderCodex, CredentialInstanceID: credential.ID, RuntimeProfileID: profile.ID,
			WorkspaceID: workspace.ID, Status: domain.SessionStarting, StartedAt: now.Add(time.Second),
			CapabilitySnapshot: []domain.Capability{domain.CapabilitySessionControl}}}
	if _, err := store.ConsumeSessionStartPreview(ctx, request); domain.CodeOf(err) != domain.CodePermissionDenied {
		t.Fatalf("cross-client preview code=%v err=%v", domain.CodeOf(err), err)
	}
	request.ClientID = clientA.ID
	request.PreviewID = storageID("preview", storageHexB)
	if _, err := store.ConsumeSessionStartPreview(ctx, request); domain.CodeOf(err) != domain.CodeIdentityConfirmationRequired {
		t.Fatalf("forged preview code=%v err=%v", domain.CodeOf(err), err)
	}
	request.PreviewID = previewID
	started, err := store.ConsumeSessionStartPreview(ctx, request)
	if err != nil || started.ID != request.Session.ID || started.Status != domain.SessionStarting {
		t.Fatalf("consume=%+v err=%v", started, err)
	}
	request.Session.ID = storageID("session", storageHexB)
	replayed, err := store.ConsumeSessionStartPreview(ctx, request)
	if err != nil || replayed.ID != started.ID {
		t.Fatalf("replay=%+v err=%v", replayed, err)
	}
	request.RequestDigest = strings.Repeat("2", 64)
	if _, err := store.ConsumeSessionStartPreview(ctx, request); domain.CodeOf(err) != domain.CodeConflict {
		t.Fatalf("different replay code=%v err=%v", domain.CodeOf(err), err)
	}
}

func TestSessionStartPreviewExpiryAndRevisionDriftCreateNoSession(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	device, clientA, _, account, profile, credential, workspace := createCodexPreviewFixture(t, store)
	now := time.Unix(3_100, 0).UTC()
	makePreview := func(id domain.ID) SessionStartPreview {
		return SessionStartPreview{ID: id, ClientID: clientA.ID, Provider: domain.ProviderCodex,
			AccountID: account.ID, AccountRevision: 1, RuntimeProfileID: profile.ID, ProfileRevision: 1,
			CredentialInstanceID: credential.ID, CredentialRevision: 2, DeviceID: device.ID, WorkspaceID: workspace.ID,
			ProviderVersion: "0.144.2", BinaryFingerprint: strings.Repeat("3", 64), SchemaFingerprint: strings.Repeat("4", 64),
			CapabilityDigest: strings.Repeat("5", 64), CreatedAt: now, ExpiresAt: now.Add(time.Minute)}
	}
	confirmation := SessionStartConfirmation{Confirmed: true, AccountID: account.ID, AccountRevision: 1,
		RuntimeProfileID: profile.ID, ProfileRevision: 1, CredentialInstanceID: credential.ID, CredentialRevision: 2,
		DeviceID: device.ID, WorkspaceID: workspace.ID, ProviderVersion: "0.144.2"}
	consume := func(preview SessionStartPreview, at time.Time, sessionID domain.ID) error {
		_, err := store.ConsumeSessionStartPreview(ctx, ConsumeSessionStartPreviewRequest{PreviewID: preview.ID, ClientID: clientA.ID,
			RequestDigest: strings.Repeat("6", 64), At: at, BinaryFingerprint: preview.BinaryFingerprint,
			SchemaFingerprint: preview.SchemaFingerprint, CapabilityDigest: preview.CapabilityDigest, Confirmation: confirmation,
			Session: domain.Session{ID: sessionID, DeviceID: device.ID, AccountID: account.ID, Provider: domain.ProviderCodex,
				CredentialInstanceID: credential.ID, RuntimeProfileID: profile.ID, WorkspaceID: workspace.ID,
				Status: domain.SessionStarting, StartedAt: at, CapabilitySnapshot: []domain.Capability{domain.CapabilitySessionControl}}})
		return err
	}
	expired := makePreview(storageID("preview", storageHexA))
	if err := store.CreateSessionStartPreview(ctx, expired); err != nil {
		t.Fatal(err)
	}
	if err := consume(expired, expired.ExpiresAt, storageID("session", storageHexA)); domain.CodeOf(err) != domain.CodeConfirmationExpired {
		t.Fatalf("expired code=%v err=%v", domain.CodeOf(err), err)
	}
	drifted := makePreview(storageID("preview", storageHexB))
	if err := store.CreateSessionStartPreview(ctx, drifted); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE runtime_profiles SET revision=revision+1 WHERE id=?`, profile.ID); err != nil {
		t.Fatal(err)
	}
	if err := consume(drifted, now.Add(time.Second), storageID("session", storageHexB)); domain.CodeOf(err) != domain.CodeProfileBindingChanged {
		t.Fatalf("drift code=%v err=%v", domain.CodeOf(err), err)
	}
	sessions, err := store.ListSessions(ctx)
	if err != nil || len(sessions) != 0 {
		t.Fatalf("failed previews created sessions=%+v err=%v", sessions, err)
	}
}

func TestSessionStartPreviewConcurrentConsumersCreateOneSession(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	device, clientA, _, account, profile, credential, workspace := createCodexPreviewFixture(t, store)
	now := time.Unix(4_100, 0).UTC()
	preview := SessionStartPreview{ID: storageID("preview", storageHexA), ClientID: clientA.ID, Provider: domain.ProviderCodex,
		AccountID: account.ID, AccountRevision: 1, RuntimeProfileID: profile.ID, ProfileRevision: 1,
		CredentialInstanceID: credential.ID, CredentialRevision: 2, DeviceID: device.ID, WorkspaceID: workspace.ID,
		ProviderVersion: "0.144.2", BinaryFingerprint: strings.Repeat("7", 64), SchemaFingerprint: strings.Repeat("8", 64),
		CapabilityDigest: strings.Repeat("9", 64), CreatedAt: now, ExpiresAt: now.Add(time.Minute)}
	if err := store.CreateSessionStartPreview(ctx, preview); err != nil {
		t.Fatal(err)
	}
	confirmation := SessionStartConfirmation{Confirmed: true, AccountID: account.ID, AccountRevision: 1,
		RuntimeProfileID: profile.ID, ProfileRevision: 1, CredentialInstanceID: credential.ID, CredentialRevision: 2,
		DeviceID: device.ID, WorkspaceID: workspace.ID, ProviderVersion: "0.144.2"}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for index, values := range []struct {
		session domain.ID
		digest  string
	}{
		{storageID("session", storageHexA), strings.Repeat("a", 64)},
		{storageID("session", storageHexB), strings.Repeat("b", 64)},
	} {
		index, values := index, values
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := store.ConsumeSessionStartPreview(ctx, ConsumeSessionStartPreviewRequest{PreviewID: preview.ID, ClientID: clientA.ID,
				RequestDigest: values.digest, At: now.Add(time.Duration(index+1) * time.Second), BinaryFingerprint: preview.BinaryFingerprint,
				SchemaFingerprint: preview.SchemaFingerprint, CapabilityDigest: preview.CapabilityDigest, Confirmation: confirmation,
				Session: domain.Session{ID: values.session, DeviceID: device.ID, AccountID: account.ID, Provider: domain.ProviderCodex,
					CredentialInstanceID: credential.ID, RuntimeProfileID: profile.ID, WorkspaceID: workspace.ID,
					Status: domain.SessionStarting, StartedAt: now.Add(time.Second), CapabilitySnapshot: []domain.Capability{domain.CapabilitySessionControl}}})
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	sessions, err := store.ListSessions(ctx)
	if err != nil || successes != 1 || len(sessions) != 1 {
		t.Fatalf("successes=%d sessions=%+v err=%v", successes, sessions, err)
	}
}
