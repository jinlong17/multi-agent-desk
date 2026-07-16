package vault

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
	"golang.org/x/crypto/argon2"
)

type State string

const (
	StateUninitialized State = "uninitialized"
	StateLocked        State = "locked"
	StateUnlocked      State = "unlocked"
	StateCorrupt       State = "corrupt"

	formatVersion   = 1
	argonTime       = 3
	argonMemoryKiB  = 64 * 1024
	maxPasswordSize = 1024
	maxPayloadSize  = 64 * 1024
)

var keyCheckPlaintext = []byte("MultiAgentDesk Vault v1 key check")
var keyCheckAAD = []byte("vault_config:v1")

type Manager struct {
	mu          sync.RWMutex
	state       State
	unlockEpoch uint64
	store       *storage.Store
	kek         []byte
}

// NewManager retains the Phase 1 in-memory boundary for Fake Provider tests.
func NewManager() *Manager { return &Manager{state: StateLocked} }

// NewPersistentManager binds the portable Vault to the Device Store.
func NewPersistentManager(ctx context.Context, store *storage.Store) (*Manager, error) {
	if store == nil {
		return nil, domain.NewError(domain.CodeInvalidArgument, "vault store is required")
	}
	m := &Manager{store: store, state: StateUninitialized}
	_, err := store.VaultConfig(ctx)
	switch domain.CodeOf(err) {
	case domain.CodeNotFound:
		return m, nil
	case "":
		if err == nil {
			m.state = StateLocked
			return m, nil
		}
	default:
		m.state = StateCorrupt
	}
	return nil, err
}

func (m *Manager) Status() State {
	if m == nil {
		return StateLocked
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.state == "" {
		return StateLocked
	}
	return m.state
}

func parallelism() uint8 {
	count := runtime.NumCPU()
	if count < 1 {
		return 1
	}
	if count > 4 {
		return 4
	}
	return uint8(count)
}

func deriveKEK(password, salt []byte, timeCost, memory uint32, lanes uint8) []byte {
	return argon2.IDKey(password, salt, timeCost, memory, lanes, 32)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func randomBytes(size int) ([]byte, error) {
	value := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, value); err != nil {
		return nil, err
	}
	return value, nil
}

func initRequestDigest(clientID domain.ID, requestKey string) string {
	encoded := encodeFields(string(clientID), requestKey)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func (m *Manager) Initialize(ctx context.Context, clientID domain.ID, requestKey string, password []byte, at time.Time) (State, error) {
	if m == nil || m.store == nil || ctx == nil || domain.ValidateID(clientID) != nil ||
		requestKey == "" || len(requestKey) > 128 || len(password) < 1 || len(password) > maxPasswordSize || at.IsZero() {
		return "", domain.NewError(domain.CodeInvalidArgument, "vault initialization input is invalid")
	}
	salt, err := randomBytes(16)
	if err != nil {
		return "", domain.WrapError(domain.CodeConflict, "vault randomness failed", err)
	}
	nonce, err := randomBytes(12)
	if err != nil {
		return "", domain.WrapError(domain.CodeConflict, "vault randomness failed", err)
	}
	key := deriveKEK(password, salt, argonTime, argonMemoryKiB, parallelism())
	defer zero(key)
	gcm, err := newGCM(key)
	if err != nil {
		return "", domain.WrapError(domain.CodeConflict, "vault cipher failed", err)
	}
	check := gcm.Seal(nil, nonce, keyCheckPlaintext, keyCheckAAD)
	config := storage.VaultConfig{FormatVersion: formatVersion, KDFName: "argon2id-v19",
		KDFSalt: salt, ArgonTime: argonTime, ArgonMemoryKiB: argonMemoryKiB,
		ArgonParallelism: parallelism(), KeyCheckNonce: nonce, KeyCheckCiphertext: check,
		InitializedAt: at.UTC(), InitializedByDevice: clientID,
		InitRequestDigest: initRequestDigest(clientID, requestKey)}
	_, err = m.store.InitializeVault(ctx, config)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	zero(m.kek)
	m.kek = nil
	m.state = StateLocked
	m.mu.Unlock()
	return StateLocked, nil
}

func (m *Manager) Unlock(secret []byte) error {
	if m == nil || len(secret) < 1 || len(secret) > maxPasswordSize {
		return domain.NewError(domain.CodeInvalidArgument, "vault unlock input is invalid")
	}
	if m.store == nil {
		m.mu.Lock()
		m.state = StateUnlocked
		m.unlockEpoch++
		m.mu.Unlock()
		return nil
	}
	config, err := m.store.VaultConfig(context.Background())
	if err != nil {
		if domain.CodeOf(err) == domain.CodeNotFound {
			return domain.NewError(domain.CodeVaultLocked, "vault is uninitialized")
		}
		return err
	}
	key := deriveKEK(secret, config.KDFSalt, config.ArgonTime, config.ArgonMemoryKiB, config.ArgonParallelism)
	gcm, err := newGCM(key)
	if err != nil {
		zero(key)
		return domain.NewError(domain.CodeVaultUnlockFailed, "vault unlock failed")
	}
	plain, err := gcm.Open(nil, config.KeyCheckNonce, config.KeyCheckCiphertext, keyCheckAAD)
	if err != nil || !bytes.Equal(plain, keyCheckPlaintext) {
		zero(key)
		zero(plain)
		return domain.NewError(domain.CodeVaultUnlockFailed, "vault unlock failed")
	}
	zero(plain)
	m.mu.Lock()
	zero(m.kek)
	m.kek = key
	m.state = StateUnlocked
	m.unlockEpoch++
	m.mu.Unlock()
	return nil
}

func (m *Manager) Lock() error {
	if m == nil {
		return domain.NewError(domain.CodeInvalidArgument, "vault is unavailable")
	}
	m.mu.Lock()
	zero(m.kek)
	m.kek = nil
	if m.store != nil {
		if m.state != StateUninitialized && m.state != StateCorrupt {
			m.state = StateLocked
		}
	} else {
		m.state = StateLocked
	}
	m.mu.Unlock()
	return nil
}

func (m *Manager) RequireUnlocked() error {
	if m == nil || m.Status() != StateUnlocked {
		return domain.NewError(domain.CodeVaultLocked, "vault is locked")
	}
	return nil
}

func (m *Manager) Epoch() uint64 {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.unlockEpoch
}

type CredentialMetadata struct {
	CredentialInstanceID domain.ID
	AccountID            domain.ID
	DeviceID             domain.ID
	Provider             string
	ExpectedRevision     int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (m *Manager) currentKEK() ([]byte, error) {
	if m == nil {
		return nil, domain.NewError(domain.CodeVaultLocked, "vault is locked")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.state != StateUnlocked || len(m.kek) != 32 {
		return nil, domain.NewError(domain.CodeVaultLocked, "vault is locked")
	}
	return append([]byte(nil), m.kek...), nil
}

func (m *Manager) SealCredential(ctx context.Context, metadata CredentialMetadata, plaintext []byte) (int64, error) {
	return m.sealCredential(ctx, "", "", "", metadata, plaintext)
}

func (m *Manager) SealEnrollmentCredential(ctx context.Context, enrollmentID, clientID domain.ID, completionDigest string, metadata CredentialMetadata, plaintext []byte) (int64, error) {
	if enrollmentID == "" || clientID == "" || len(completionDigest) != 64 {
		return 0, domain.NewError(domain.CodeInvalidArgument, "enrollment identity is required")
	}
	if _, err := hex.DecodeString(completionDigest); err != nil {
		return 0, domain.NewError(domain.CodeInvalidArgument, "enrollment completion digest is invalid")
	}
	return m.sealCredential(ctx, enrollmentID, clientID, completionDigest, metadata, plaintext)
}

func (m *Manager) sealCredential(ctx context.Context, enrollmentID, clientID domain.ID, completionDigest string, metadata CredentialMetadata, plaintext []byte) (int64, error) {
	if m == nil || m.store == nil || metadata.Provider != domain.ProviderCodex || metadata.ExpectedRevision < 1 ||
		len(plaintext) < 2 || len(plaintext) > maxPayloadSize || metadata.CreatedAt.IsZero() || metadata.UpdatedAt.IsZero() {
		return 0, domain.NewError(domain.CodeInvalidArgument, "vault credential is invalid")
	}
	if err := validateJSONObject(plaintext); err != nil {
		return 0, domain.NewError(domain.CodeInvalidArgument, "vault credential JSON is invalid")
	}
	kek, err := m.currentKEK()
	if err != nil {
		return 0, err
	}
	defer zero(kek)
	dek, err := randomBytes(32)
	if err != nil {
		return 0, err
	}
	defer zero(dek)
	payloadNonce, err := randomBytes(12)
	if err != nil {
		return 0, err
	}
	wrapNonce, err := randomBytes(12)
	if err != nil {
		return 0, err
	}
	nextRevision := metadata.ExpectedRevision + 1
	aad := credentialAAD(metadata, nextRevision)
	payloadGCM, err := newGCM(dek)
	if err != nil {
		return 0, domain.WrapError(domain.CodeConflict, "vault payload cipher failed", err)
	}
	wrapGCM, err := newGCM(kek)
	if err != nil {
		return 0, domain.WrapError(domain.CodeConflict, "vault wrapping cipher failed", err)
	}
	payloadCiphertext := payloadGCM.Seal(nil, payloadNonce, plaintext, aad)
	wrappedDEK := wrapGCM.Seal(nil, wrapNonce, dek, aad)
	aadHash := sha256.Sum256(aad)
	secretHash := sha256.Sum256(plaintext)
	item := storage.VaultItem{CredentialInstanceID: metadata.CredentialInstanceID,
		AccountID: metadata.AccountID, DeviceID: metadata.DeviceID, Provider: domain.ProviderCodex,
		EnvelopeVersion: 1, CredentialRevision: nextRevision, CipherName: "aes-256-gcm",
		PayloadNonce: payloadNonce, PayloadCiphertext: payloadCiphertext, WrapName: "aes-256-gcm",
		WrapNonce: wrapNonce, WrappedDEK: wrappedDEK, AADDigest: hex.EncodeToString(aadHash[:]),
		SecretDigest: hex.EncodeToString(secretHash[:]), CreatedAt: metadata.CreatedAt.UTC(), UpdatedAt: metadata.UpdatedAt.UTC()}
	var saveErr error
	if enrollmentID != "" {
		saveErr = m.store.ReplaceVaultItemEnrollmentCAS(ctx, enrollmentID, clientID, completionDigest, metadata.ExpectedRevision, item, domain.CredentialHealthy)
	} else {
		saveErr = m.store.ReplaceVaultItemCAS(ctx, metadata.ExpectedRevision, item, domain.CredentialHealthy)
	}
	if saveErr != nil {
		return 0, saveErr
	}
	return nextRevision, nil
}

func (m *Manager) ReadCredential(ctx context.Context, credentialID domain.ID) ([]byte, int64, error) {
	kek, err := m.currentKEK()
	if err != nil {
		return nil, 0, err
	}
	defer zero(kek)
	item, err := m.store.VaultItem(ctx, credentialID)
	if err != nil {
		return nil, 0, err
	}
	metadata := CredentialMetadata{CredentialInstanceID: item.CredentialInstanceID, AccountID: item.AccountID,
		DeviceID: item.DeviceID, Provider: item.Provider, ExpectedRevision: item.CredentialRevision - 1}
	aad := credentialAAD(metadata, item.CredentialRevision)
	aadHash := sha256.Sum256(aad)
	if hex.EncodeToString(aadHash[:]) != item.AADDigest {
		return nil, 0, domain.NewError(domain.CodeVaultCorrupt, "vault item authentication failed")
	}
	wrapGCM, err := newGCM(kek)
	if err != nil {
		return nil, 0, domain.NewError(domain.CodeVaultCorrupt, "vault item cipher failed")
	}
	dek, err := wrapGCM.Open(nil, item.WrapNonce, item.WrappedDEK, aad)
	if err != nil {
		return nil, 0, domain.NewError(domain.CodeVaultCorrupt, "vault item authentication failed")
	}
	defer zero(dek)
	payloadGCM, err := newGCM(dek)
	if err != nil {
		return nil, 0, domain.NewError(domain.CodeVaultCorrupt, "vault item cipher failed")
	}
	plain, err := payloadGCM.Open(nil, item.PayloadNonce, item.PayloadCiphertext, aad)
	if err != nil || validateJSONObject(plain) != nil {
		zero(plain)
		return nil, 0, domain.NewError(domain.CodeVaultCorrupt, "vault item authentication failed")
	}
	secretHash := sha256.Sum256(plain)
	if hex.EncodeToString(secretHash[:]) != item.SecretDigest {
		zero(plain)
		return nil, 0, domain.NewError(domain.CodeVaultCorrupt, "vault item digest failed")
	}
	return plain, item.CredentialRevision, nil
}

func credentialAAD(metadata CredentialMetadata, revision int64) []byte {
	return encodeFields(strconv.Itoa(formatVersion), string(metadata.DeviceID), metadata.Provider,
		string(metadata.CredentialInstanceID), string(metadata.AccountID), strconv.FormatInt(revision, 10))
}

func encodeFields(fields ...string) []byte {
	var out bytes.Buffer
	for _, field := range fields {
		_ = binary.Write(&out, binary.BigEndian, uint32(len([]byte(field))))
		out.WriteString(field)
	}
	return out.Bytes()
}

func validateJSONObject(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return errors.New("not an object")
	}
	if err := validateObject(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	return nil
}

func validateObject(decoder *json.Decoder) error {
	seen := map[string]struct{}{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return errors.New("invalid key")
		}
		if _, exists := seen[key]; exists {
			return errors.New("duplicate key")
		}
		seen[key] = struct{}{}
		if err := validateJSONValue(decoder); err != nil {
			return err
		}
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return errors.New("invalid object end")
	}
	return nil
}

func validateJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		return validateObject(decoder)
	case '[':
		for decoder.More() {
			if err := validateJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("invalid array end")
		}
		return nil
	default:
		return errors.New("invalid JSON delimiter")
	}
}

func zero(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
