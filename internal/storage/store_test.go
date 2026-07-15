package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
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
	if version != 3 {
		t.Fatalf("got schema version %d, want 3", version)
	}
	var migrationCount int
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&migrationCount); err != nil {
		t.Fatal(err)
	}
	if migrationCount != 3 {
		t.Fatalf("got %d applied migrations, want 3", migrationCount)
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
	if migrationCount != 3 {
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
	if _, err := raw.ExecContext(ctx, "PRAGMA user_version=3"); err != nil {
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

func TestMigrationFailureRollsBackDDLAndLedger(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	contents := []byte("CREATE TABLE should_rollback(id INTEGER); THIS IS NOT SQL;")
	migration := devicemigrations.Migration{
		Version:  4,
		Name:     "0004_invalid.sql",
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
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations WHERE version=4").Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 0 {
		t.Fatal("failed migration was recorded")
	}
	version, err := store.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != 3 {
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
		CapabilitySnapshot: []domain.Capability{domain.CapabilityMetadataRead, domain.CapabilitySessionResume},
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
