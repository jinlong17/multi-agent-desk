package device

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
)

func TestClientAdministrationRotationAndRevocation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "device")
	now := time.Now().UTC()
	if _, err := Bootstrap(context.Background(), root, "admin-test", now); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(context.Background(), DeviceDatabasePath(root))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	private, record, err := CreateClient(context.Background(), store, "secondary", []domain.Capability{domain.CapabilityMetadataRead}, now)
	if err != nil {
		t.Fatal(err)
	}
	if private.ClientID != record.ID || len(private.PrivateKey) != 64 || record.Revision != 1 {
		t.Fatalf("unexpected client: %+v %+v", private, record)
	}
	rotated, record, err := RotateClient(context.Background(), store, record.ID, record.Revision, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if rotated.ClientID != record.ID || record.Revision != 2 || record.Status != domain.ClientIdentityActive {
		t.Fatalf("unexpected rotation: %+v", record)
	}
	if _, _, err := RotateClient(context.Background(), store, record.ID, 1, now.Add(2*time.Second)); domain.CodeOf(err) != domain.CodeConflict {
		t.Fatalf("stale rotation code = %v", domain.CodeOf(err))
	}
	revoked, err := RevokeClient(context.Background(), store, record.ID, record.Revision, now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if revoked.Status != domain.ClientIdentityRevoked || revoked.Revision != 3 {
		t.Fatalf("unexpected revoke: %+v", revoked)
	}
}
