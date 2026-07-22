package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
	"github.com/jinlong17/multi-agent-desk/internal/vault"
)

func TestRemoteBootstrapPrepareNormalizesHighPrecisionTimeBeforeVaultPersistence(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "device", "device.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	setupAt := time.Date(2030, 3, 1, 0, 0, 0, 100_000_000, time.UTC)
	deviceID, clientID := appTestID(t, "device"), appTestID(t, "client")
	if err := store.CreateDevice(ctx, domain.Device{ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: "bootstrap", SigningPublicKey: make([]byte, 32), CreatedAt: setupAt, UpdatedAt: setupAt}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{ID: clientID, Name: "owner", PublicKey: make([]byte, 32), Revision: 1, Status: domain.ClientIdentityActive, Caps: []domain.Capability{domain.CapabilityVaultControl}, CreatedAt: setupAt, UpdatedAt: setupAt}); err != nil {
		t.Fatal(err)
	}
	manager, err := vault.NewPersistentManager(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	password := []byte("remote-bootstrap-time-test")
	if _, err := manager.Initialize(ctx, clientID, "remote-bootstrap-time", password, setupAt); err != nil {
		t.Fatal(err)
	}
	if err := manager.Unlock(password); err != nil {
		t.Fatal(err)
	}
	highPrecision := time.Date(2030, 3, 1, 2, 3, 4, 123_456_789, time.FixedZone("plus-one", 3600))
	want := highPrecision.UTC().Truncate(time.Microsecond)
	service := &RemoteBootstrapService{
		Store: store, Vault: manager, Now: func() time.Time { return highPrecision },
		ClientVersion: "0.1.0-test", Platform: "darwin", Architecture: "arm64",
	}
	origin := "https://control.example.test"
	descriptor, err := service.Prepare(ctx, BootstrapPrepareInput{ServerOrigin: origin, Name: "Test Daemon"})
	if err != nil {
		t.Fatal(err)
	}
	sealedAt := descriptor.Anchor.KeyEnvelopeAssertion.SealedAt
	if !sealedAt.Equal(want) || sealedAt.Location() != time.UTC || sealedAt.Nanosecond()%1_000 != 0 {
		t.Fatalf("sealedAt=%s want=%s", sealedAt.Format(time.RFC3339Nano), want.Format(time.RFC3339Nano))
	}
	record, err := store.PendingRemoteDeviceIdentityForOrigin(ctx, origin)
	if err != nil {
		t.Fatal(err)
	}
	if !record.CreatedAt.Equal(want) || !record.UpdatedAt.Equal(want) || record.UpdatedAt.Nanosecond()%1_000 != 0 {
		t.Fatalf("record created=%s updated=%s want=%s", record.CreatedAt.Format(time.RFC3339Nano), record.UpdatedAt.Format(time.RFC3339Nano), want.Format(time.RFC3339Nano))
	}
	opened, err := manager.OpenRemoteIdentity(ctx, record.ID, origin, vault.RemoteIdentityOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.ZeroPrivateMaterial()
	if !opened.Envelope.CreatedAt.Equal(want) || !opened.Envelope.UpdatedAt.Equal(want) {
		t.Fatalf("envelope created=%s updated=%s want=%s", opened.Envelope.CreatedAt.Format(time.RFC3339Nano), opened.Envelope.UpdatedAt.Format(time.RFC3339Nano), want.Format(time.RFC3339Nano))
	}
	canonical, err := transport.BootstrapKeyEnvelopeAssertionJCSV1(
		int(descriptor.Anchor.KeyEnvelopeAssertion.FormatVersion),
		int(descriptor.Anchor.KeyEnvelopeAssertion.KeyRevision),
		descriptor.Anchor.KeyEnvelopeAssertion.RecordRevision,
		sealedAt,
		string(descriptor.Anchor.KeyEnvelopeAssertion.Status),
	)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		SealedAt string `json:"sealedAt"`
	}
	if err := json.Unmarshal(canonical, &wire); err != nil {
		t.Fatal(err)
	}
	parsed, err := transport.ParseUTCDateTime(wire.SealedAt)
	if err != nil || !parsed.Equal(want) || wire.SealedAt != "2030-03-01T01:03:04.123456Z" {
		t.Fatalf("wire sealedAt=%q parsed=%s err=%v", wire.SealedAt, parsed.Format(time.RFC3339Nano), err)
	}
}
