package storage

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	devicemigrations "github.com/jinlong17/multi-agent-desk/migrations/device"
)

func numberedID(prefix string, number int) domain.ID {
	return domain.ID(fmt.Sprintf("%s_%032x", prefix, number))
}

func publicRegistryStore(t *testing.T) (*Store, domain.Device) {
	t.Helper()
	store, _ := openTestStore(t)
	now := time.Unix(1_000, 0).UTC()
	device := domain.Device{ID: numberedID("device", 1), Kind: domain.DeviceKindDaemon,
		DisplayName: "registry", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now}
	if err := store.CreateDevice(context.Background(), device); err != nil {
		t.Fatal(err)
	}
	return store, device
}

func createPublicAccount(t *testing.T, store *Store, device domain.Device, number int, provider, alias string, at time.Time) (domain.Account, domain.RuntimeProfile) {
	t.Helper()
	account := domain.Account{ID: numberedID("account", number), Provider: provider,
		DisplayName: fmt.Sprintf("Account %02d", number), Enabled: true, Revision: 1,
		CreatedAt: at, UpdatedAt: at}
	profile := domain.RuntimeProfile{ID: numberedID("profile", number), AccountID: account.ID,
		DeviceID: device.ID, Name: account.DisplayName, Provider: provider, SelectorAlias: alias,
		Settings: []byte(`{}`), Enabled: true, Revision: 1, CreatedAt: at, UpdatedAt: at}
	createdAccount, createdProfile, err := store.CreateAccountWithDefaultProfile(context.Background(), account, profile)
	if err != nil {
		t.Fatal(err)
	}
	return createdAccount, createdProfile
}

func TestManualRegistryPaginationRevisionAndAtomicDeletion(t *testing.T) {
	store, device := publicRegistryStore(t)
	ctx := context.Background()
	base := time.Unix(2_000, 0).UTC()
	accounts := make([]domain.Account, 0, 8)
	profiles := make([]domain.RuntimeProfile, 0, 8)
	for index := 1; index <= 8; index++ {
		provider := domain.ProviderCodex
		if index%2 == 0 {
			provider = domain.ProviderClaude
		}
		account, profile := createPublicAccount(t, store, device, index+10, provider,
			fmt.Sprintf("A%d", index), base.Add(time.Duration(index)*time.Second))
		accounts = append(accounts, account)
		profiles = append(profiles, profile)
	}

	var listed []domain.Account
	cursor := ""
	for {
		page, err := store.ListAccountPage(ctx, AccountListOptions{Limit: 3, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		listed = append(listed, page.Items...)
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if len(listed) != 8 {
		t.Fatalf("listed %d accounts, want 8", len(listed))
	}
	for index := 0; index < 4; index++ {
		account := accounts[index+4]
		profile := domain.RuntimeProfile{ID: numberedID("profile", 201+index), AccountID: account.ID,
			DeviceID: device.ID, Name: fmt.Sprintf("Extra %d", index+1), Provider: account.Provider,
			SelectorAlias: fmt.Sprintf("Extra%d", index+1), Settings: []byte(`{}`), Enabled: true,
			Revision: 1, CreatedAt: base.Add(time.Duration(20+index) * time.Second),
			UpdatedAt: base.Add(time.Duration(20+index) * time.Second)}
		if _, err := store.CreateProfile(ctx, account, profile); err != nil {
			t.Fatal(err)
		}
	}
	var profileCount int
	profileCursor := ""
	for {
		page, err := store.ListProfiles(ctx, ProfileListOptions{Limit: 3, Cursor: profileCursor})
		if err != nil {
			t.Fatal(err)
		}
		profileCount += len(page.Items)
		if page.NextCursor == "" {
			break
		}
		profileCursor = page.NextCursor
	}
	if profileCount != 12 {
		t.Fatalf("listed %d profiles, want 12", profileCount)
	}
	conflictAccount := domain.Account{ID: numberedID("account", 500), Provider: domain.ProviderCodex,
		DisplayName: "Conflict", Enabled: true, Revision: 1, CreatedAt: base, UpdatedAt: base}
	conflictProfile := domain.RuntimeProfile{ID: numberedID("profile", 500), AccountID: conflictAccount.ID,
		DeviceID: device.ID, Name: "Conflict", Provider: domain.ProviderCodex, SelectorAlias: "a1",
		Settings: []byte(`{}`), Enabled: true, Revision: 1, CreatedAt: base, UpdatedAt: base}
	if _, _, err := store.CreateAccountWithDefaultProfile(ctx, conflictAccount, conflictProfile); domain.CodeOf(err) != domain.CodeAliasConflict {
		t.Fatalf("case-folded alias conflict got %v", err)
	}
	firstPage, err := store.ListAccountPage(ctx, AccountListOptions{Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ListAccountPage(ctx, AccountListOptions{Provider: domain.ProviderCodex, Limit: 3, Cursor: firstPage.NextCursor}); domain.CodeOf(err) != domain.CodeInvalidArgument {
		t.Fatalf("filter/cursor reuse got %v", err)
	}

	binding, err := store.ResolveProfile(ctx, "@a1")
	if err != nil || binding.Account.ID != accounts[0].ID || binding.Profile.ID != profiles[0].ID || binding.Credential != nil {
		t.Fatalf("unexpected binding %+v, %v", binding, err)
	}
	rawBinding, err := store.ResolveProfileTarget(ctx, string(profiles[0].ID))
	if err != nil || rawBinding.Account.ID != accounts[0].ID || rawBinding.Profile.ID != profiles[0].ID {
		t.Fatalf("unexpected raw-ID admin binding %+v, %v", rawBinding, err)
	}
	if _, err := store.ResolveProfile(ctx, string(profiles[0].ID)); domain.CodeOf(err) != domain.CodeAliasInvalid {
		t.Fatalf("public raw-ID selector was accepted: %v", err)
	}
	newName := "Renamed"
	updated, err := store.UpdateAccount(ctx, accounts[0].ID, 1, AccountPatch{DisplayName: &newName}, base.Add(time.Minute))
	if err != nil || updated.Revision != 2 || updated.DisplayName != newName {
		t.Fatalf("unexpected update %+v, %v", updated, err)
	}
	if _, err := store.UpdateAccount(ctx, accounts[0].ID, 1, AccountPatch{DisplayName: &newName}, base.Add(2*time.Minute)); domain.CodeOf(err) != domain.CodeSyncConflict {
		t.Fatalf("stale update got %v", err)
	}

	profile, err := store.SetProfileEnabled(ctx, profiles[0].ID, 1, false, base.Add(3*time.Minute))
	if err != nil || profile.Revision != 2 || profile.Enabled {
		t.Fatalf("unexpected profile disable %+v, %v", profile, err)
	}
	if err := store.DeleteProfile(ctx, profile.ID, profile.Revision, base.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	replacement := domain.RuntimeProfile{ID: numberedID("profile", 99), AccountID: accounts[0].ID,
		DeviceID: device.ID, Name: "Replacement", Provider: accounts[0].Provider,
		SelectorAlias: "A1", Settings: []byte(`{}`), Enabled: true, Revision: 1,
		CreatedAt: base.Add(5 * time.Minute), UpdatedAt: base.Add(5 * time.Minute)}
	if _, err := store.CreateProfile(ctx, updated, replacement); err != nil {
		t.Fatalf("deleted alias was not reusable: %v", err)
	}

	second := accounts[1]
	extra := domain.RuntimeProfile{ID: numberedID("profile", 100), AccountID: second.ID,
		DeviceID: device.ID, Name: "Second config", Provider: second.Provider,
		SelectorAlias: "B2", Settings: []byte(`{}`), Enabled: true, Revision: 1,
		CreatedAt: base.Add(6 * time.Minute), UpdatedAt: base.Add(6 * time.Minute)}
	if _, err := store.CreateProfile(ctx, second, extra); err != nil {
		t.Fatal(err)
	}
	disabled, err := store.SetAccountEnabledRevision(ctx, second.ID, second.Revision, false, base.Add(7*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteAccount(ctx, second.ID, disabled.Revision, base.Add(8*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Account(ctx, second.ID); domain.CodeOf(err) != domain.CodeAccountNotFound {
		t.Fatalf("deleted account lookup got %v", err)
	}
	var tombstones int
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM metadata_tombstones WHERE entity_id IN (?, ?, ?)", second.ID, profiles[1].ID, extra.ID).Scan(&tombstones); err != nil || tombstones != 3 {
		t.Fatalf("tombstones=%d, err=%v", tombstones, err)
	}
}

func TestGenericUsageWindowsRoundTripUnknownAndMissingValues(t *testing.T) {
	store, device := publicRegistryStore(t)
	ctx := context.Background()
	now := time.Unix(3_000, 0).UTC()
	account, _ := createPublicAccount(t, store, device, 21, domain.ProviderCodex, "Quota", now)
	duration, used, remaining := int64(18_000), 34.0, 66.0
	reset := now.Add(5 * time.Hour)
	snapshot := domain.UsageSnapshot{ID: numberedID("usage", 1), AccountID: account.ID,
		DeviceID: device.ID, Provider: domain.ProviderCodex, ProviderVersion: "test-1",
		Source: domain.UsageSourceOfficial, Confidence: domain.UsageConfidenceHigh,
		Availability: domain.AvailabilityAvailable, ObservedAt: now, StaleAt: now.Add(time.Minute),
		Windows: []domain.UsageWindow{
			{ProviderLimitID: "primary", Kind: domain.UsageWindowRolling, Label: "5 hours", DurationSeconds: &duration, UsedPercent: &used, RemainingPercent: &remaining, ResetsAt: &reset},
			{ProviderLimitID: "future", Kind: domain.UsageWindowUnknown, Label: "Provider window"},
		}}
	if err := store.CreateUsageSnapshotWithWindows(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	replay := snapshot
	replay.ID = numberedID("usage", 2)
	if err := store.CreateUsageSnapshotWithWindows(ctx, replay); err != nil {
		t.Fatalf("usage replay was not idempotent: %v", err)
	}
	got, err := store.ListUsageSnapshotsWithWindows(ctx, account.ID)
	if err != nil || len(got) != 1 || len(got[0].Windows) != 2 {
		t.Fatalf("usage got %+v, %v", got, err)
	}
	if got[0].Windows[1].UsedPercent != nil || got[0].Windows[1].RemainingPercent != nil || got[0].Windows[1].Kind != domain.UsageWindowUnknown {
		t.Fatalf("unknown window was reinterpreted: %+v", got[0].Windows[1])
	}
	if *got[0].Windows[0].UsedPercent != used || !got[0].Windows[0].ResetsAt.Equal(reset) {
		t.Fatalf("primary window changed: %+v", got[0].Windows[0])
	}
}

func TestAccountCreateDoesNotAllocateProviderHome(t *testing.T) {
	store, device := publicRegistryStore(t)
	root := filepath.Dir(store.Path())
	before, err := directoryNames(root)
	if err != nil {
		t.Fatal(err)
	}
	createPublicAccount(t, store, device, 31, domain.ProviderClaude, "NoHome", time.Unix(4_000, 0).UTC())
	after, err := directoryNames(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("metadata create changed Device root entries: before=%v after=%v", before, after)
	}
}

func directoryNames(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry.Name())
	}
	sort.Strings(result)
	return result, nil
}

func TestMigrationV6PreservesPopulatedFakeTuplesAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "device")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "device.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	migrations, err := devicemigrations.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations[:3] {
		if _, err := raw.ExecContext(ctx, migration.SQL); err != nil {
			t.Fatalf("apply %s: %v", migration.Name, err)
		}
	}
	if _, err := raw.ExecContext(ctx, `CREATE TABLE schema_migrations(
		version INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE, checksum TEXT NOT NULL,
		applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations[:3] {
		if _, err := raw.ExecContext(ctx, "INSERT INTO schema_migrations VALUES(?, ?, ?, ?)",
			migration.Version, migration.Name, hex.EncodeToString(migration.Checksum[:]), formatTime(time.Unix(10, 0))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := raw.ExecContext(ctx, "PRAGMA user_version=3"); err != nil {
		t.Fatal(err)
	}
	deviceID := numberedID("device", 41)
	profileA, profileB := numberedID("profile", 41), numberedID("profile", 42)
	credentialA, credentialB := numberedID("credential", 41), numberedID("credential", 42)
	sessionA, sessionB := numberedID("session", 41), numberedID("session", 42)
	workspace := numberedID("workspace", 41)
	at := formatTime(time.Unix(20, 0))
	statements := []struct {
		query string
		args  []any
	}{
		{"INSERT INTO device_identity VALUES(?, 'daemon', 'legacy', ?, ?, ?)", []any{deviceID, make([]byte, 32), at, at}},
		{"INSERT INTO workspaces VALUES(?, ?, '/tmp/legacy', 'legacy', '[]', ?, ?)", []any{workspace, deviceID, at, at}},
		{"INSERT INTO runtime_profiles VALUES(?, ?, 'one', 'fake', '{}', ?, ?)", []any{profileA, deviceID, at, at}},
		{"INSERT INTO runtime_profiles VALUES(?, ?, 'two', 'fake', '{}', ?, ?)", []any{profileB, deviceID, at, at}},
		{"INSERT INTO credential_instances VALUES(?, ?, 'fake', 'fake', 'fake:a', 'healthy', 0, ?, ?, ?)", []any{credentialA, deviceID, fmt.Sprintf("%064x", 1), at, at}},
		{"INSERT INTO credential_instances VALUES(?, ?, 'fake', 'fake', 'fake:b', 'healthy', 0, ?, ?, ?)", []any{credentialB, deviceID, fmt.Sprintf("%064x", 2), at, at}},
		{"INSERT INTO sessions(id, device_id, provider, credential_instance_id, runtime_profile_id, workspace_id, status, started_at, capability_snapshot_json) VALUES(?, ?, 'fake', ?, ?, ?, 'starting', ?, '[]')", []any{sessionA, deviceID, credentialA, profileA, workspace, at}},
		{"INSERT INTO sessions(id, device_id, provider, credential_instance_id, runtime_profile_id, workspace_id, status, started_at, capability_snapshot_json) VALUES(?, ?, 'fake', ?, ?, ?, 'starting', ?, '[]')", []any{sessionB, deviceID, credentialB, profileB, workspace, at}},
	}
	for _, statement := range statements {
		if _, err := raw.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed legacy fixture: %v", err)
		}
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	internalAccount := domain.ID("account_" + stringsTrimPrefix(string(deviceID), "device_"))
	var provider string
	var internal bool
	if err := store.db.QueryRowContext(ctx, "SELECT provider, internal FROM accounts WHERE id = ?", internalAccount).Scan(&provider, &internal); err != nil || provider != domain.ProviderFake || !internal {
		t.Fatalf("synthetic account invalid: provider=%s internal=%v err=%v", provider, internal, err)
	}
	if _, err := store.PublicAccount(ctx, internalAccount); domain.CodeOf(err) != domain.CodeAccountNotFound {
		t.Fatalf("internal account became publicly addressable: %v", err)
	}
	for _, expected := range []struct {
		session, profile, credential domain.ID
	}{{sessionA, profileA, credentialA}, {sessionB, profileB, credentialB}} {
		stored, err := store.Session(ctx, expected.session)
		if err != nil || stored.AccountID != internalAccount || stored.RuntimeProfileID != expected.profile || stored.CredentialInstanceID != expected.credential {
			t.Fatalf("migrated tuple %+v, err=%v", stored, err)
		}
		profile, err := store.RuntimeProfile(ctx, expected.profile)
		if err != nil || !profile.Internal || profile.SelectorAlias != "" || profile.AccountID != internalAccount {
			t.Fatalf("migrated profile %+v, err=%v", profile, err)
		}
	}
	var violations int
	rows, err := store.db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		violations++
	}
	_ = rows.Close()
	if violations != 0 {
		t.Fatalf("foreign key violations=%d", violations)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var accounts int
	if err := reopened.db.QueryRowContext(ctx, "SELECT count(*) FROM accounts WHERE internal = 1").Scan(&accounts); err != nil || accounts != 1 {
		t.Fatalf("restart synthetic accounts=%d, err=%v", accounts, err)
	}
}

func stringsTrimPrefix(value, prefix string) string {
	if len(value) >= len(prefix) && value[:len(prefix)] == prefix {
		return value[len(prefix):]
	}
	return value
}
