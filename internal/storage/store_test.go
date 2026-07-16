package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	devicemigrations "github.com/jinlong17/multi-agent-desk/migrations/device"
	_ "modernc.org/sqlite"
)

const (
	storageHexA = "00112233445566778899aabbccddeeff"
	storageHexB = "ffeeddccbbaa99887766554433221100"
)

func storageID(prefix, suffix string) domain.ID {
	return domain.ID(prefix + "_" + suffix)
}

func openTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "device-root", "device.db")
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, path
}

func TestSQLiteDSNPreservesAbsolutePlatformPaths(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		path     string
		expected string
	}{
		{
			name:     "unix special characters",
			goos:     "linux",
			path:     "/tmp/device root/a?#%.db",
			expected: "file:///tmp/device%20root/a%3F%23%25.db",
		},
		{
			name:     "windows drive",
			goos:     "windows",
			path:     `C:\Users\runner admin\device?#%.db`,
			expected: "file:///C:/Users/runner%20admin/device%3F%23%25.db",
		},
		{
			name:     "windows unc",
			goos:     "windows",
			path:     `\\server\private share\device.db`,
			expected: "file://server/private%20share/device.db",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := sqliteDSNForOS(test.goos, test.path)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.expected {
				t.Fatalf("got %q, want %q", got, test.expected)
			}
		})
	}
	for _, test := range []struct {
		goos string
		path string
	}{
		{"linux", "relative/device.db"},
		{"windows", `relative\device.db`},
		{"windows", `\\server`},
	} {
		if _, err := sqliteDSNForOS(test.goos, test.path); domain.CodeOf(err) != domain.CodeInvalidArgument {
			t.Fatalf("%s path %q got %v", test.goos, test.path, err)
		}
	}
}

func TestOpenConfiguresAndRestartsDeviceDatabase(t *testing.T) {
	store, path := openTestStore(t)
	ctx := context.Background()
	pragmas, err := store.Pragmas(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantPragmas := Pragmas{JournalMode: "wal", ForeignKeys: true, BusyTimeout: DefaultBusyTimeout}
	if pragmas != wantPragmas {
		t.Fatalf("got %+v, want %+v", pragmas, wantPragmas)
	}
	version, err := store.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != 5 {
		t.Fatalf("got schema version %d, want 5", version)
	}
	var migrationCount int
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&migrationCount); err != nil {
		t.Fatal(err)
	}
	if migrationCount != 5 {
		t.Fatalf("got %d applied migrations, want 5", migrationCount)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != databaseFileMode {
			t.Fatalf("got database mode %o, want %o", info.Mode().Perm(), databaseFileMode)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if err := reopened.db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&migrationCount); err != nil {
		t.Fatal(err)
	}
	if migrationCount != 5 {
		t.Fatalf("restart reapplied migrations: count=%d", migrationCount)
	}
}

func TestOpenRejectsBroadDirectoryPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file modes do not encode the Device root ACL")
	}
	dir := filepath.Join(t.TempDir(), "broad")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Open(context.Background(), filepath.Join(dir, "device.db"))
	if domain.CodeOf(err) != domain.CodePermissionDenied {
		t.Fatalf("got %v, want permission denied", err)
	}
}

func TestOpenRejectsFutureAndChangedMigrationWithoutDeletingData(t *testing.T) {
	store, path := openTestStore(t)
	ctx := context.Background()
	if _, err := store.db.ExecContext(ctx, "CREATE TABLE preservation_marker(value TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "INSERT INTO preservation_marker(value) VALUES('keep')"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "PRAGMA user_version=999"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(ctx, path); domain.CodeOf(err) != domain.CodeSchemaIncompatible {
		t.Fatalf("got %v, want schema incompatible", err)
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	var marker string
	if err := raw.QueryRowContext(ctx, "SELECT value FROM preservation_marker").Scan(&marker); err != nil {
		t.Fatal(err)
	}
	if marker != "keep" {
		t.Fatalf("future-schema rejection changed data: %q", marker)
	}
	if _, err := raw.ExecContext(ctx, "PRAGMA user_version=4"); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, "UPDATE schema_migrations SET checksum = ? WHERE version = 2", strings.Repeat("0", 64)); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(ctx, path); domain.CodeOf(err) != domain.CodeSchemaIncompatible {
		t.Fatalf("changed checksum got %v, want schema incompatible", err)
	}
}

func TestCodexMigrationPreservesLegacyFakeRows(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	deviceRoot := filepath.Join(root, "device")
	if err := os.Mkdir(deviceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(deviceRoot, "device.db")
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	migrations, err := devicemigrations.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 5 {
		t.Fatalf("migration count=%d", len(migrations))
	}
	if _, err := raw.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, migrations[0].SQL); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, migrations[1].SQL); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, migrations[2].SQL); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `CREATE TABLE schema_migrations(
		version INTEGER PRIMARY KEY CHECK (version >= 1), name TEXT NOT NULL UNIQUE,
		checksum TEXT NOT NULL CHECK (length(checksum) = 64), applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations[:3] {
		if _, err := raw.ExecContext(ctx, "INSERT INTO schema_migrations(version, name, checksum, applied_at) VALUES(?, ?, ?, ?)", migration.Version, migration.Name, hex.EncodeToString(migration.Checksum[:]), "legacy"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := raw.ExecContext(ctx, "PRAGMA user_version=3"); err != nil {
		t.Fatal(err)
	}
	deviceID := storageID("device", storageHexA)
	profileID := storageID("profile", storageHexA)
	credentialID := storageID("credential", storageHexA)
	workspaceID := storageID("workspace", storageHexA)
	sessionID := storageID("session", storageHexA)
	if _, err := raw.ExecContext(ctx, `INSERT INTO device_identity(id, kind, display_name, signing_public_key, created_at, updated_at) VALUES(?, 'daemon', 'legacy', ?, '1970-01-01T00:00:00Z', '1970-01-01T00:00:00Z')`, deviceID, make([]byte, 32)); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `INSERT INTO workspaces(id, device_id, path, label, tags_json, created_at, updated_at) VALUES(?, ?, '/tmp/legacy', 'legacy', '[]', '1970-01-01T00:00:00Z', '1970-01-01T00:00:00Z')`, workspaceID, deviceID); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `INSERT INTO runtime_profiles(id, device_id, name, provider, settings_json, created_at, updated_at) VALUES(?, ?, 'legacy', 'fake', '{}', '1970-01-01T00:00:00Z', '1970-01-01T00:00:00Z')`, profileID, deviceID); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `INSERT INTO credential_instances(id, device_id, provider, auth_method, secret_ref, status, credential_revision, secret_digest, created_at, updated_at) VALUES(?, ?, 'fake', 'fake', 'fake:legacy', 'healthy', 0, ?, '1970-01-01T00:00:00Z', '1970-01-01T00:00:00Z')`, credentialID, deviceID, storageDigestA); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `INSERT INTO sessions(id, device_id, provider, credential_instance_id, runtime_profile_id, workspace_id, status, started_at, capability_snapshot_json, failure_code) VALUES(?, ?, 'fake', ?, ?, ?, 'starting', '1970-01-01T00:00:00Z', '[]', '')`, sessionID, deviceID, credentialID, profileID, workspaceID); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if version, err := store.SchemaVersion(ctx); err != nil || version != 5 {
		t.Fatalf("upgraded schema=%d err=%v", version, err)
	}
	profile, err := store.RuntimeProfile(ctx, profileID)
	if err != nil || profile.Provider != domain.ProviderFake || profile.AccountID != "" {
		t.Fatalf("legacy profile=%+v err=%v", profile, err)
	}
	credential, err := store.CredentialInstance(ctx, credentialID)
	if err != nil || credential.Provider != domain.ProviderFake || credential.AccountID != "" || credential.CredentialRevision != 0 {
		t.Fatalf("legacy credential=%+v err=%v", credential, err)
	}
	session, err := store.Session(ctx, sessionID)
	if err != nil || session.Provider != domain.ProviderFake || session.AccountID != "" {
		t.Fatalf("legacy session=%+v err=%v", session, err)
	}
}

func TestMigrationFailureRollsBackDDLAndLedger(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	contents := []byte("CREATE TABLE should_rollback(id INTEGER); THIS IS NOT SQL;")
	migration := devicemigrations.Migration{
		Version:  6,
		Name:     "0006_invalid.sql",
		SQL:      string(contents),
		Checksum: sha256.Sum256(contents),
	}
	if err := store.applyMigration(ctx, migration); domain.CodeOf(err) != domain.CodeSchemaIncompatible {
		t.Fatalf("got %v, want schema incompatible", err)
	}
	var tableCount int
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='should_rollback'").Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if tableCount != 0 {
		t.Fatal("failed migration left partial DDL")
	}
	var ledgerCount int
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations WHERE version=6").Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 0 {
		t.Fatal("failed migration was recorded")
	}
	version, err := store.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != 5 {
		t.Fatalf("failed migration changed user_version to %d", version)
	}
}

type storageFixture struct {
	device     domain.Device
	clientA    domain.ClientIdentity
	clientB    domain.ClientIdentity
	workspace  domain.Workspace
	profile    domain.RuntimeProfile
	credential domain.CredentialInstance
	session    domain.Session
}

func createStorageFixture(t *testing.T, store *Store) storageFixture {
	return createStorageFixtureWithCapabilities(t, store, []domain.Capability{
		domain.CapabilityMetadataRead,
		domain.CapabilitySessionResume,
	})
}

func createStorageFixtureWithCapabilities(t *testing.T, store *Store, capabilities []domain.Capability) storageFixture {
	t.Helper()
	ctx := context.Background()
	now := time.Unix(100, 0).UTC()
	fixture := storageFixture{
		device: domain.Device{
			ID: storageID("device", storageHexA), Kind: domain.DeviceKindDaemon,
			DisplayName: "test device", SigningPublicKey: make([]byte, 32), CreatedAt: now, UpdatedAt: now,
		},
		clientA: domain.ClientIdentity{
			ID: storageID("client", storageHexA), Name: "owner", PublicKey: bytesOf(1, 32), Revision: 1,
			Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityClientAdmin, domain.CapabilityMetadataRead},
			CreatedAt: now, UpdatedAt: now,
		},
		clientB: domain.ClientIdentity{
			ID: storageID("client", storageHexB), Name: "observer", PublicKey: bytesOf(2, 32), Revision: 1,
			Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityMetadataRead},
			CreatedAt: now, UpdatedAt: now,
		},
		workspace: domain.Workspace{
			ID: storageID("workspace", storageHexA), DeviceID: storageID("device", storageHexA),
			Path: "/tmp/workspace", Label: "workspace", Tags: []string{"phase1"}, CreatedAt: now, UpdatedAt: now,
		},
		profile: domain.RuntimeProfile{
			ID: storageID("profile", storageHexA), DeviceID: storageID("device", storageHexA),
			Name: "fake-default", Provider: "fake", Settings: []byte(`{"echo":true}`), CreatedAt: now, UpdatedAt: now,
		},
		credential: domain.CredentialInstance{
			ID: storageID("credential", storageHexA), DeviceID: storageID("device", storageHexA), Provider: "fake",
			AuthMethod: "fake", SecretRef: "fake:phase1", Status: domain.CredentialHealthy,
			CredentialRevision: 0, SecretDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			CreatedAt: now, UpdatedAt: now,
		},
	}
	fixture.session = domain.Session{
		ID: storageID("session", storageHexA), DeviceID: fixture.device.ID, Provider: "fake",
		CredentialInstanceID: fixture.credential.ID, RuntimeProfileID: fixture.profile.ID,
		WorkspaceID: fixture.workspace.ID, Status: domain.SessionStarting, StartedAt: now,
		CapabilitySnapshot: append([]domain.Capability(nil), capabilities...),
	}
	for _, call := range []func() error{
		func() error { return store.CreateDevice(ctx, fixture.device) },
		func() error { return store.CreateClientIdentity(ctx, fixture.clientA) },
		func() error { return store.CreateClientIdentity(ctx, fixture.clientB) },
		func() error { return store.CreateWorkspace(ctx, fixture.workspace) },
		func() error { return store.CreateRuntimeProfile(ctx, fixture.profile) },
		func() error { return store.CreateCredentialInstance(ctx, fixture.credential) },
		func() error { return store.CreateSession(ctx, fixture.session) },
	} {
		if err := call(); err != nil {
			t.Fatal(err)
		}
	}
	return fixture
}

func bytesOf(value byte, count int) []byte {
	result := make([]byte, count)
	for index := range result {
		result[index] = value
	}
	return result
}

func TestRepositoriesPersistSessionAttachmentLeaseAndEvents(t *testing.T) {
	store, path := openTestStore(t)
	ctx := context.Background()
	fixture := createStorageFixture(t, store)

	device, err := store.Device(ctx)
	if err != nil || device.ID != fixture.device.ID {
		t.Fatalf("device got %+v, %v", device, err)
	}
	client, err := store.ClientIdentity(ctx, fixture.clientA.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantCaps := []domain.Capability{domain.CapabilityClientAdmin, domain.CapabilityMetadataRead}
	if !reflect.DeepEqual(client.Caps, wantCaps) {
		t.Fatalf("got caps %v, want %v", client.Caps, wantCaps)
	}
	workspace, err := store.Workspace(ctx, fixture.workspace.ID)
	if err != nil || workspace.Path != fixture.workspace.Path || !reflect.DeepEqual(workspace.Tags, fixture.workspace.Tags) {
		t.Fatalf("workspace got %+v, %v", workspace, err)
	}
	profile, err := store.RuntimeProfile(ctx, fixture.profile.ID)
	if err != nil || profile.Provider != "fake" || string(profile.Settings) != string(fixture.profile.Settings) {
		t.Fatalf("profile got %+v, %v", profile, err)
	}
	credential, err := store.CredentialInstance(ctx, fixture.credential.ID)
	if err != nil || credential.CredentialRevision != 0 || credential.SecretDigest != fixture.credential.SecretDigest {
		t.Fatalf("credential got %+v, %v", credential, err)
	}

	running, err := store.TransitionSession(ctx, fixture.session.ID, domain.SessionStarting, domain.SessionRunning, time.Unix(110, 0), nil, "")
	if err != nil || running.Status != domain.SessionRunning {
		t.Fatalf("running transition got %+v, %v", running, err)
	}
	if _, err := store.TransitionSession(ctx, fixture.session.ID, domain.SessionStarting, domain.SessionRunning, time.Unix(111, 0), nil, ""); domain.CodeOf(err) != domain.CodeConflict {
		t.Fatalf("stale transition got %v", err)
	}

	attachment := domain.SessionAttachment{
		ID: storageID("attachment", storageHexA), SessionID: fixture.session.ID,
		ClientDeviceID: fixture.clientB.ID, Mode: domain.AttachmentObserver,
		ConnectedAt: time.Unix(112, 0).UTC(), LastSeenAt: time.Unix(112, 0).UTC(),
	}
	if err := store.CreateAttachment(ctx, attachment); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAttachment(ctx, attachment); domain.CodeOf(err) != domain.CodeConflict {
		t.Fatalf("duplicate attachment got %v", err)
	}

	lease, err := domain.AcquireControllerLease(nil, fixture.session.ID, fixture.clientA.ID, time.Unix(113, 0), domain.DefaultLeaseDuration)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveControllerLease(ctx, lease, 0); err != nil {
		t.Fatal(err)
	}
	heartbeat, err := lease.Heartbeat(fixture.clientA.ID, lease.Revision, time.Unix(120, 0), domain.DefaultLeaseDuration)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveControllerLease(ctx, heartbeat, lease.Revision); err != nil {
		t.Fatal(err)
	}
	stale := heartbeat
	stale.ExpiresAt = stale.ExpiresAt.Add(time.Second)
	if err := store.SaveControllerLease(ctx, stale, 999); domain.CodeOf(err) != domain.CodeStaleLease {
		t.Fatalf("stale lease got %v", err)
	}
	storedLease, err := store.ControllerLease(ctx, fixture.session.ID)
	if err != nil || storedLease.Revision != heartbeat.Revision || !storedLease.ExpiresAt.Equal(heartbeat.ExpiresAt) {
		t.Fatalf("stored lease got %+v, %v", storedLease, err)
	}

	event := domain.RuntimeEvent{
		ID: storageID("event", storageHexA), SessionID: fixture.session.ID, Sequence: 1,
		Kind: "session.running", Metadata: []byte(`{"source":"fake"}`), CreatedAt: time.Unix(121, 0).UTC(),
	}
	if err := store.AppendRuntimeEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	audit := domain.AuditEvent{
		ID: storageID("audit", storageHexA), ActorID: fixture.clientA.ID, Action: "session.transition",
		TargetType: "session", TargetID: fixture.session.ID, Decision: "allowed", Metadata: []byte(`{"status":"running"}`),
		CreatedAt: time.Unix(122, 0).UTC(),
	}
	if err := store.AppendAuditEvent(ctx, audit); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteAttachment(ctx, fixture.session.ID, fixture.clientB.ID); err != nil {
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	persisted, err := reopened.Session(ctx, fixture.session.ID)
	if err != nil || persisted.Status != domain.SessionRunning {
		t.Fatalf("persisted session got %+v, %v", persisted, err)
	}
	for table, want := range map[string]int{
		"session_events":      1,
		"audit_events":        1,
		"session_attachments": 0,
		"controller_leases":   1,
	} {
		var got int
		if err := reopened.db.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s count=%d, want %d", table, got, want)
		}
	}
}

func TestResumeRepositoryRequiresTerminalSourceAndPreservesOriginal(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	fixture := createStorageFixture(t, store)
	newID := storageID("session", storageHexB)

	direct := fixture.session
	direct.ID = newID
	direct.ResumedFromSessionID = fixture.session.ID
	direct.StartedAt = time.Unix(120, 0).UTC()
	if err := store.CreateSession(ctx, direct); domain.CodeOf(err) != domain.CodeInvalidTransition {
		t.Fatalf("non-terminal resume got %v", err)
	}
	if _, err := store.TransitionSession(ctx, fixture.session.ID, domain.SessionStarting, domain.SessionRunning, time.Unix(110, 0), nil, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.TransitionSession(ctx, fixture.session.ID, domain.SessionRunning, domain.SessionStopping, time.Unix(115, 0), nil, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.TransitionSession(ctx, fixture.session.ID, domain.SessionStopping, domain.SessionExited, time.Unix(120, 0), nil, ""); err != nil {
		t.Fatal(err)
	}
	source, err := store.Session(ctx, fixture.session.ID)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := source.Resume(newID, time.Unix(121, 0))
	if err != nil {
		t.Fatal(err)
	}
	expanded := resumed
	expanded.ID = storageID("session", "11111111111111111111111111111111")
	expanded.CapabilitySnapshot = append(expanded.CapabilitySnapshot, domain.CapabilitySessionStart)
	if err := store.CreateSession(ctx, expanded); domain.CodeOf(err) != domain.CodeConflict {
		t.Fatalf("expanded resume snapshot got %v", err)
	}
	removed := resumed
	removed.ID = storageID("session", "22222222222222222222222222222222")
	removed.CapabilitySnapshot = []domain.Capability{domain.CapabilitySessionResume}
	if err := store.CreateSession(ctx, removed); domain.CodeOf(err) != domain.CodeConflict {
		t.Fatalf("reduced resume snapshot got %v", err)
	}
	if err := store.CreateSession(ctx, resumed); err != nil {
		t.Fatal(err)
	}
	persistedSource, err := store.Session(ctx, fixture.session.ID)
	if err != nil {
		t.Fatal(err)
	}
	persistedResume, err := store.Session(ctx, newID)
	if err != nil {
		t.Fatal(err)
	}
	if persistedSource.Status != domain.SessionExited || persistedSource.EndedAt == nil {
		t.Fatalf("source changed after resume: %+v", persistedSource)
	}
	if persistedResume.Status != domain.SessionStarting || persistedResume.ResumedFromSessionID != fixture.session.ID {
		t.Fatalf("unexpected resumed record: %+v", persistedResume)
	}
}

func TestResumeRepositoryRequiresCapabilityOnSource(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	fixture := createStorageFixtureWithCapabilities(t, store, []domain.Capability{domain.CapabilityMetadataRead})
	if _, err := store.TransitionSession(ctx, fixture.session.ID, domain.SessionStarting, domain.SessionRunning, time.Unix(110, 0), nil, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.TransitionSession(ctx, fixture.session.ID, domain.SessionRunning, domain.SessionStopping, time.Unix(115, 0), nil, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.TransitionSession(ctx, fixture.session.ID, domain.SessionStopping, domain.SessionExited, time.Unix(120, 0), nil, ""); err != nil {
		t.Fatal(err)
	}
	newID := storageID("session", storageHexB)
	manufactured := fixture.session
	manufactured.ID = newID
	manufactured.ResumedFromSessionID = fixture.session.ID
	manufactured.StartedAt = time.Unix(121, 0).UTC()
	manufactured.CapabilitySnapshot = []domain.Capability{domain.CapabilityMetadataRead, domain.CapabilitySessionResume}
	if err := store.CreateSession(ctx, manufactured); domain.CodeOf(err) != domain.CodePermissionDenied {
		t.Fatalf("manufactured resume capability got %v", err)
	}
	if _, err := store.Session(ctx, newID); domain.CodeOf(err) != domain.CodeNotFound {
		t.Fatalf("rejected resume persisted a row: %v", err)
	}
}

func TestForeignKeyFailureRollsBackRepositoryTransaction(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	now := time.Unix(100, 0).UTC()
	workspace := domain.Workspace{
		ID: storageID("workspace", storageHexA), DeviceID: storageID("device", storageHexA),
		Path: "/missing-device", Tags: []string{}, CreatedAt: now, UpdatedAt: now,
	}
	if err := store.CreateWorkspace(ctx, workspace); domain.CodeOf(err) != domain.CodeConflict {
		t.Fatalf("got %v, want conflict", err)
	}
	var count int
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM workspaces").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed repository transaction persisted a workspace")
	}
}

func TestNotFoundErrorsAreStable(t *testing.T) {
	store, _ := openTestStore(t)
	_, err := store.Session(context.Background(), storageID("session", storageHexA))
	if domain.CodeOf(err) != domain.CodeNotFound {
		t.Fatalf("got %v, want not found", err)
	}
}
