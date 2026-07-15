package device

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
)

const (
	identitySchemaVersion = 1
	deviceDatabaseName    = "device.db"
	daemonIdentityName    = "daemon.identity.json"
	ownerIdentityName     = "owner.identity.json"
)

type DaemonIdentity struct {
	SchemaVersion int                `json:"schema_version"`
	DeviceID      domain.ID          `json:"device_id"`
	PrivateKey    ed25519.PrivateKey `json:"private_key"`
}

func (i DaemonIdentity) PublicKey() ed25519.PublicKey {
	if len(i.PrivateKey) != ed25519.PrivateKeySize {
		return nil
	}
	return append(ed25519.PublicKey(nil), i.PrivateKey.Public().(ed25519.PublicKey)...)
}

type ClientIdentity struct {
	SchemaVersion   int                `json:"schema_version"`
	ClientID        domain.ID          `json:"client_id"`
	PrivateKey      ed25519.PrivateKey `json:"private_key"`
	DaemonID        domain.ID          `json:"daemon_id"`
	DaemonPublicKey ed25519.PublicKey  `json:"daemon_public_key"`
}

func (i ClientIdentity) PublicKey() ed25519.PublicKey {
	if len(i.PrivateKey) != ed25519.PrivateKeySize {
		return nil
	}
	return append(ed25519.PublicKey(nil), i.PrivateKey.Public().(ed25519.PublicKey)...)
}

type BootstrapResult struct {
	Root     string
	DeviceID domain.ID
	ClientID domain.ID
}

var ownerCapabilities = []domain.Capability{
	domain.CapabilityClientAdmin,
	domain.CapabilityMetadataRead,
	domain.CapabilitySessionControl,
	domain.CapabilitySessionControlAcquire,
	domain.CapabilitySessionObserve,
	domain.CapabilitySessionResume,
	domain.CapabilitySessionStart,
	domain.CapabilityTerminalControl,
	domain.CapabilityVaultControl,
}

// Bootstrap atomically creates a new Device root. Existing roots are never
// adopted or reset, because doing so could replace an identity pin.
func Bootstrap(ctx context.Context, root, displayName string, at time.Time) (BootstrapResult, error) {
	if ctx == nil || root == "" || displayName == "" || len(displayName) > 128 || at.IsZero() {
		return BootstrapResult{}, domain.NewError(domain.CodeInvalidArgument, "invalid bootstrap request")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return BootstrapResult{}, domain.WrapError(domain.CodeInvalidArgument, "device root is invalid", err)
	}
	if _, err := os.Lstat(absolute); err == nil {
		return BootstrapResult{}, domain.NewError(domain.CodeAlreadyExists, "device root already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return BootstrapResult{}, domain.WrapError(domain.CodeConflict, "device root could not be inspected", err)
	}
	parent := filepath.Dir(absolute)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return BootstrapResult{}, domain.WrapError(domain.CodeConflict, "device root parent could not be created", err)
	}
	staging, err := randomSibling(absolute)
	if err != nil {
		return BootstrapResult{}, err
	}
	if err := createPrivateDirectory(staging); err != nil {
		return BootstrapResult{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(staging)
		}
	}()

	deviceID, err := domain.NewID("device")
	if err != nil {
		return BootstrapResult{}, err
	}
	clientID, err := domain.NewID("client")
	if err != nil {
		return BootstrapResult{}, err
	}
	daemonPublic, daemonPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return BootstrapResult{}, domain.WrapError(domain.CodeConflict, "daemon identity generation failed", err)
	}
	clientPublic, clientPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return BootstrapResult{}, domain.WrapError(domain.CodeConflict, "client identity generation failed", err)
	}

	store, err := storage.Open(ctx, filepath.Join(staging, deviceDatabaseName))
	if err != nil {
		return BootstrapResult{}, err
	}
	closeStore := true
	defer func() {
		if closeStore {
			_ = store.Close()
		}
	}()
	if err := store.CreateDevice(ctx, domain.Device{
		ID: deviceID, Kind: domain.DeviceKindDaemon, DisplayName: displayName,
		SigningPublicKey: daemonPublic, CreatedAt: at, UpdatedAt: at,
	}); err != nil {
		return BootstrapResult{}, err
	}
	if err := store.CreateClientIdentity(ctx, domain.ClientIdentity{
		ID: clientID, Name: "local-owner", PublicKey: clientPublic, Revision: 1,
		Status: domain.ClientIdentityActive, Caps: ownerCapabilities, CreatedAt: at, UpdatedAt: at,
	}); err != nil {
		return BootstrapResult{}, err
	}
	if err := writePrivateJSON(filepath.Join(staging, daemonIdentityName), DaemonIdentity{
		SchemaVersion: identitySchemaVersion, DeviceID: deviceID, PrivateKey: daemonPrivate,
	}); err != nil {
		return BootstrapResult{}, err
	}
	if err := writePrivateJSON(filepath.Join(staging, ownerIdentityName), ClientIdentity{
		SchemaVersion: identitySchemaVersion, ClientID: clientID, PrivateKey: clientPrivate,
		DaemonID: deviceID, DaemonPublicKey: daemonPublic,
	}); err != nil {
		return BootstrapResult{}, err
	}
	if err := store.Close(); err != nil {
		return BootstrapResult{}, domain.WrapError(domain.CodeConflict, "device database could not be closed", err)
	}
	closeStore = false
	if err := os.Rename(staging, absolute); err != nil {
		return BootstrapResult{}, domain.WrapError(domain.CodeConflict, "device root could not be committed", err)
	}
	committed = true
	return BootstrapResult{Root: absolute, DeviceID: deviceID, ClientID: clientID}, nil
}

func LoadDaemonIdentity(root string) (DaemonIdentity, error) {
	var identity DaemonIdentity
	if err := readPrivateJSON(filepath.Join(root, daemonIdentityName), &identity); err != nil {
		return DaemonIdentity{}, err
	}
	if identity.SchemaVersion != identitySchemaVersion || domain.ValidateID(identity.DeviceID) != nil || len(identity.PrivateKey) != ed25519.PrivateKeySize {
		return DaemonIdentity{}, domain.NewError(domain.CodeSchemaIncompatible, "daemon identity file is invalid")
	}
	return identity, nil
}

func LoadOwnerIdentity(root string) (ClientIdentity, error) {
	var identity ClientIdentity
	if err := readPrivateJSON(filepath.Join(root, ownerIdentityName), &identity); err != nil {
		return ClientIdentity{}, err
	}
	if identity.SchemaVersion != identitySchemaVersion || domain.ValidateID(identity.ClientID) != nil ||
		domain.ValidateID(identity.DaemonID) != nil || len(identity.PrivateKey) != ed25519.PrivateKeySize ||
		len(identity.DaemonPublicKey) != ed25519.PublicKeySize {
		return ClientIdentity{}, domain.NewError(domain.CodeSchemaIncompatible, "client identity file is invalid")
	}
	return identity, nil
}

func VerifyIdentityStore(ctx context.Context, root string, store interface {
	Device(context.Context) (domain.Device, error)
	ClientIdentity(context.Context, domain.ID) (domain.ClientIdentity, error)
}) error {
	daemon, err := LoadDaemonIdentity(root)
	if err != nil {
		return err
	}
	owner, err := LoadOwnerIdentity(root)
	if err != nil {
		return err
	}
	deviceRecord, err := store.Device(ctx)
	if err != nil {
		return err
	}
	clientRecord, err := store.ClientIdentity(ctx, owner.ClientID)
	if err != nil {
		return err
	}
	if deviceRecord.ID != daemon.DeviceID || owner.DaemonID != daemon.DeviceID ||
		!ed25519.PublicKey(deviceRecord.SigningPublicKey).Equal(daemon.PublicKey()) ||
		!owner.DaemonPublicKey.Equal(daemon.PublicKey()) ||
		clientRecord.Status != domain.ClientIdentityActive ||
		!ed25519.PublicKey(clientRecord.PublicKey).Equal(owner.PublicKey()) {
		return domain.NewError(domain.CodeUnauthenticated, "identity store verification failed")
	}
	return nil
}

func DeviceDatabasePath(root string) string { return filepath.Join(root, deviceDatabaseName) }

func randomSibling(target string) (string, error) {
	var suffix [12]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", domain.WrapError(domain.CodeConflict, "bootstrap staging name failed", err)
	}
	return fmt.Sprintf("%s.init-%x", target, suffix[:]), nil
}

func writePrivateJSON(path string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "identity file could not be encoded", err)
	}
	encoded = append(encoded, '\n')
	return writePrivateFileAtomic(path, encoded)
}

func readPrivateJSON(path string, target any) error {
	if err := verifyPrivateFile(path); err != nil {
		return err
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return domain.WrapError(domain.CodeConflict, "identity file could not be read", err)
	}
	if len(encoded) > 8*1024 {
		return domain.NewError(domain.CodeSchemaIncompatible, "identity file is too large")
	}
	if err := validateJSON(encoded); err != nil {
		return domain.NewError(domain.CodeSchemaIncompatible, "identity file is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return domain.WrapError(domain.CodeSchemaIncompatible, "identity file is invalid", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return domain.NewError(domain.CodeSchemaIncompatible, "identity file has trailing data")
	}
	return nil
}
