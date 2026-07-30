package vault

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
	"golang.org/x/crypto/curve25519"
)

const maxDeviceKeyEnvelopeSize = 4096

type DeviceKeyEnvelopeStatus string

const (
	DeviceKeyEnvelopePending DeviceKeyEnvelopeStatus = "pending"
	DeviceKeyEnvelopeActive  DeviceKeyEnvelopeStatus = "active"
	DeviceKeyEnvelopeRetired DeviceKeyEnvelopeStatus = "retired"
)

// DeviceKeyEnvelopeV1 is the only plaintext representation of a remote Device
// private identity. It is never stored outside the nested Vault-v1 envelope.
type DeviceKeyEnvelopeV1 struct {
	Version           int                     `json:"version"`
	ServerOrigin      string                  `json:"serverOrigin"`
	ServerDeviceID    string                  `json:"serverDeviceId"`
	Ed25519Seed       string                  `json:"ed25519Seed"`
	X25519PrivateKey  string                  `json:"x25519PrivateKey"`
	SigningPublicKey  string                  `json:"signingPublicKey"`
	ExchangePublicKey string                  `json:"exchangePublicKey"`
	SigningKeyDigest  string                  `json:"signingKeyDigest"`
	ExchangeKeyDigest string                  `json:"exchangeKeyDigest"`
	KeyRevision       int                     `json:"keyRevision"`
	Status            DeviceKeyEnvelopeStatus `json:"status"`
	CreatedAt         time.Time               `json:"createdAt"`
	UpdatedAt         time.Time               `json:"updatedAt"`
}

type RemoteIdentityOptions struct {
	AllowDevelopmentLocalhost bool
}

type OpenedRemoteIdentity struct {
	Record            storage.RemoteDeviceIdentityRecord
	Envelope          DeviceKeyEnvelopeV1
	Ed25519Seed       []byte
	X25519PrivateKey  []byte
	SigningPublicKey  ed25519.PublicKey
	ExchangePublicKey []byte
}

func (o *OpenedRemoteIdentity) ZeroPrivateMaterial() {
	if o == nil {
		return
	}
	zero(o.Ed25519Seed)
	zero(o.X25519PrivateKey)
	o.Ed25519Seed = nil
	o.X25519PrivateKey = nil
}

// PrepareRemoteIdentity reuses the exact pending identity for one canonical
// origin, or creates a new server-bound identity and mapping atomically. A
// different origin always takes the create path and therefore receives new
// local/server IDs and key material.
func (m *Manager) PrepareRemoteIdentity(ctx context.Context, serverOrigin string, options RemoteIdentityOptions, at time.Time) (OpenedRemoteIdentity, error) {
	if m == nil || m.store == nil || ctx == nil || at.IsZero() {
		return OpenedRemoteIdentity{}, domain.NewError(domain.CodeInvalidArgument, "remote identity preparation input is invalid")
	}
	if err := m.RequireUnlocked(); err != nil {
		return OpenedRemoteIdentity{}, err
	}
	canonical, err := transport.ParseCanonicalServerOriginV1(serverOrigin, transport.CanonicalServerOriginOptions{AllowDevelopmentLocalhost: options.AllowDevelopmentLocalhost})
	if err != nil {
		return OpenedRemoteIdentity{}, domain.NewError(domain.CodeInvalidArgument, "server origin is invalid")
	}
	existing, err := m.store.PendingRemoteDeviceIdentityForOrigin(ctx, string(canonical))
	if err == nil {
		return m.openRemoteIdentityRecord(ctx, existing, string(canonical))
	}
	if domain.CodeOf(err) != domain.CodeNotFound {
		return OpenedRemoteIdentity{}, err
	}
	return m.createRemoteIdentity(ctx, string(canonical), at.UTC())
}

func (m *Manager) createRemoteIdentity(ctx context.Context, serverOrigin string, at time.Time) (OpenedRemoteIdentity, error) {
	seed, err := randomBytes(ed25519.SeedSize)
	if err != nil {
		return OpenedRemoteIdentity{}, domain.WrapError(domain.CodeConflict, "remote signing key randomness failed", err)
	}
	defer zero(seed)
	xPrivate, err := randomBytes(curve25519.ScalarSize)
	if err != nil {
		return OpenedRemoteIdentity{}, domain.WrapError(domain.CodeConflict, "remote exchange key randomness failed", err)
	}
	defer zero(xPrivate)
	xPublic, err := curve25519.X25519(xPrivate, curve25519.Basepoint)
	if err != nil || allZero(xPublic) {
		return OpenedRemoteIdentity{}, domain.NewError(domain.CodeConflict, "remote exchange key generation failed")
	}
	signingPrivate := ed25519.NewKeyFromSeed(seed)
	signingPublic := append(ed25519.PublicKey(nil), signingPrivate.Public().(ed25519.PublicKey)...)
	zero(signingPrivate)
	signingDigest := sha256.Sum256(signingPublic)
	exchangeDigest := sha256.Sum256(xPublic)
	serverDeviceID, err := transport.NewUUIDv7()
	if err != nil {
		return OpenedRemoteIdentity{}, domain.WrapError(domain.CodeConflict, "remote server Device ID generation failed", err)
	}
	localRandom, err := randomBytes(16)
	if err != nil {
		return OpenedRemoteIdentity{}, domain.WrapError(domain.CodeConflict, "remote local identity ID generation failed", err)
	}
	localID := "remote_identity_" + hex.EncodeToString(localRandom)
	zero(localRandom)
	envelope := DeviceKeyEnvelopeV1{
		Version:           1,
		ServerOrigin:      serverOrigin,
		ServerDeviceID:    serverDeviceID,
		Ed25519Seed:       base64.RawURLEncoding.EncodeToString(seed),
		X25519PrivateKey:  base64.RawURLEncoding.EncodeToString(xPrivate),
		SigningPublicKey:  base64.RawURLEncoding.EncodeToString(signingPublic),
		ExchangePublicKey: base64.RawURLEncoding.EncodeToString(xPublic),
		SigningKeyDigest:  base64.RawURLEncoding.EncodeToString(signingDigest[:]),
		ExchangeKeyDigest: base64.RawURLEncoding.EncodeToString(exchangeDigest[:]),
		KeyRevision:       1,
		Status:            DeviceKeyEnvelopePending,
		CreatedAt:         at,
		UpdatedAt:         at,
	}
	sealed, err := m.sealDeviceKeyEnvelope(localID, envelope, signingDigest[:], exchangeDigest[:])
	if err != nil {
		return OpenedRemoteIdentity{}, err
	}
	record := storage.RemoteDeviceIdentityRecord{
		ID:                localID,
		ServerOrigin:      serverOrigin,
		ServerDeviceID:    serverDeviceID,
		SigningPublicKey:  signingPublic,
		ExchangePublicKey: xPublic,
		SigningKeyDigest:  append([]byte(nil), signingDigest[:]...),
		ExchangeKeyDigest: append([]byte(nil), exchangeDigest[:]...),
		KeyRevision:       1,
		RecordRevision:    1,
		Lifecycle:         storage.RemoteIdentityPending,
		PayloadAlgorithm:  "aes-256-gcm",
		PayloadNonce:      sealed.payloadNonce,
		PayloadCiphertext: sealed.payloadCiphertext,
		WrapAlgorithm:     "aes-256-gcm",
		WrapNonce:         sealed.wrapNonce,
		WrappedDEK:        sealed.wrappedDEK,
		AADDigest:         sealed.aadDigest,
		PlaintextDigest:   sealed.plaintextDigest,
		CreatedAt:         at,
		UpdatedAt:         at,
	}
	mapping := storage.ControlPlaneIDMapping{EntityType: "device", LocalID: localID, ServerID: serverDeviceID, CreatedAt: at, UpdatedAt: at}
	if err := m.store.CreateRemoteDeviceIdentity(ctx, record, mapping); err != nil {
		return OpenedRemoteIdentity{}, err
	}
	return m.OpenRemoteIdentity(ctx, localID, serverOrigin, RemoteIdentityOptions{AllowDevelopmentLocalhost: true})
}

func (m *Manager) OpenRemoteIdentity(ctx context.Context, localID, serverOrigin string, options RemoteIdentityOptions) (OpenedRemoteIdentity, error) {
	if m == nil || m.store == nil || ctx == nil {
		return OpenedRemoteIdentity{}, domain.NewError(domain.CodeInvalidArgument, "remote identity open input is invalid")
	}
	if err := m.RequireUnlocked(); err != nil {
		return OpenedRemoteIdentity{}, err
	}
	canonical, err := transport.ParseCanonicalServerOriginV1(serverOrigin, transport.CanonicalServerOriginOptions{AllowDevelopmentLocalhost: options.AllowDevelopmentLocalhost})
	if err != nil {
		return OpenedRemoteIdentity{}, domain.NewError(domain.CodeInvalidArgument, "server origin is invalid")
	}
	record, err := m.store.RemoteDeviceIdentity(ctx, localID)
	if err != nil {
		return OpenedRemoteIdentity{}, err
	}
	return m.openRemoteIdentityRecord(ctx, record, string(canonical))
}

func (m *Manager) openRemoteIdentityRecord(ctx context.Context, record storage.RemoteDeviceIdentityRecord, serverOrigin string) (OpenedRemoteIdentity, error) {
	fail := func(reason string) (OpenedRemoteIdentity, error) {
		_ = m.store.QuarantineRemoteDeviceIdentity(ctx, record.ID, reason)
		return OpenedRemoteIdentity{}, domain.NewError(domain.CodeVaultCorrupt, "remote identity authentication failed")
	}
	if record.ServerOrigin != serverOrigin || record.QuarantineReason != "" {
		return fail("origin_or_state_mismatch")
	}
	mapping, err := m.store.ControlPlaneMapping(ctx, "device", record.ID)
	if err != nil || mapping.ServerID != record.ServerDeviceID {
		return fail("mapping_mismatch")
	}
	aad, err := deviceEnvelopeAAD(record.ID, record.ServerOrigin, record.ServerDeviceID, record.SigningKeyDigest, record.ExchangeKeyDigest, record.KeyRevision)
	if err != nil {
		return fail("aad_invalid")
	}
	aadDigest := sha256.Sum256(aad)
	if !bytes.Equal(aadDigest[:], record.AADDigest) {
		return fail("aad_digest_mismatch")
	}
	kek, err := m.currentKEK()
	if err != nil {
		return OpenedRemoteIdentity{}, err
	}
	defer zero(kek)
	wrapGCM, err := newGCM(kek)
	if err != nil {
		return fail("wrap_cipher_invalid")
	}
	dek, err := wrapGCM.Open(nil, record.WrapNonce, record.WrappedDEK, aad)
	if err != nil || len(dek) != 32 {
		zero(dek)
		return fail("wrapped_dek_invalid")
	}
	defer zero(dek)
	payloadGCM, err := newGCM(dek)
	if err != nil {
		return fail("payload_cipher_invalid")
	}
	plaintext, err := payloadGCM.Open(nil, record.PayloadNonce, record.PayloadCiphertext, aad)
	if err != nil || len(plaintext) < 2 || len(plaintext) > maxDeviceKeyEnvelopeSize {
		zero(plaintext)
		return fail("payload_invalid")
	}
	defer zero(plaintext)
	plaintextDigest := sha256.Sum256(plaintext)
	if !bytes.Equal(plaintextDigest[:], record.PlaintextDigest) {
		return fail("plaintext_digest_mismatch")
	}
	var envelope DeviceKeyEnvelopeV1
	if err := transport.DecodeStrictJSON(bytes.NewReader(plaintext), maxDeviceKeyEnvelopeSize, &envelope); err != nil {
		return fail("payload_json_invalid")
	}
	seed, xPrivate, signingPublic, xPublic, err := validateDeviceKeyEnvelope(envelope)
	if err != nil {
		zero(seed)
		zero(xPrivate)
		return fail("payload_fields_invalid")
	}
	rowStatus := DeviceKeyEnvelopeStatus(record.Lifecycle)
	if envelope.ServerOrigin != record.ServerOrigin || envelope.ServerDeviceID != record.ServerDeviceID || envelope.KeyRevision != int(record.KeyRevision) || envelope.Status != rowStatus ||
		!envelope.CreatedAt.Equal(record.CreatedAt) || !envelope.UpdatedAt.Equal(record.UpdatedAt) ||
		!bytes.Equal(signingPublic, record.SigningPublicKey) || !bytes.Equal(xPublic, record.ExchangePublicKey) ||
		!bytes.Equal(hashBytes(signingPublic), record.SigningKeyDigest) || !bytes.Equal(hashBytes(xPublic), record.ExchangeKeyDigest) {
		zero(seed)
		zero(xPrivate)
		return fail("payload_row_mismatch")
	}
	return OpenedRemoteIdentity{
		Record:            record,
		Envelope:          envelope,
		Ed25519Seed:       seed,
		X25519PrivateKey:  xPrivate,
		SigningPublicKey:  ed25519.PublicKey(signingPublic),
		ExchangePublicKey: xPublic,
	}, nil
}

// ActivateRemoteIdentityCAS stores the public server receipt digest and
// reseals unchanged private key material with fresh DEK/nonces in the same
// record-revision CAS transaction.
func (m *Manager) ActivateRemoteIdentityCAS(ctx context.Context, localID, serverOrigin string, options RemoteIdentityOptions, expectedRecordRevision int64, receiptJSON []byte, at time.Time) (OpenedRemoteIdentity, error) {
	if expectedRecordRevision < 1 || len(receiptJSON) < 2 || len(receiptJSON) > 4096 || at.IsZero() {
		return OpenedRemoteIdentity{}, domain.NewError(domain.CodeInvalidArgument, "remote identity activation input is invalid")
	}
	var receipt map[string]any
	if err := transport.DecodeStrictJSON(bytes.NewReader(receiptJSON), 4096, &receipt); err != nil || len(receipt) == 0 {
		return OpenedRemoteIdentity{}, domain.NewError(domain.CodeInvalidArgument, "bootstrap receipt JSON is invalid")
	}
	opened, err := m.OpenRemoteIdentity(ctx, localID, serverOrigin, options)
	if err != nil {
		return OpenedRemoteIdentity{}, err
	}
	defer opened.ZeroPrivateMaterial()
	if opened.Record.RecordRevision != expectedRecordRevision || opened.Envelope.Status != DeviceKeyEnvelopePending {
		return OpenedRemoteIdentity{}, domain.NewError(domain.CodeCredentialRevisionConflict, "remote identity record revision changed")
	}
	envelope := opened.Envelope
	envelope.Status = DeviceKeyEnvelopeActive
	envelope.UpdatedAt = at.UTC()
	sealed, err := m.sealDeviceKeyEnvelope(opened.Record.ID, envelope, opened.Record.SigningKeyDigest, opened.Record.ExchangeKeyDigest)
	if err != nil {
		return OpenedRemoteIdentity{}, err
	}
	receiptDigest := sha256.Sum256(receiptJSON)
	reseal := storage.RemoteIdentityReseal{
		ExpectedRecordRevision: expectedRecordRevision,
		NextRecordRevision:     expectedRecordRevision + 1,
		Lifecycle:              storage.RemoteIdentityActive,
		PayloadNonce:           sealed.payloadNonce,
		PayloadCiphertext:      sealed.payloadCiphertext,
		WrapNonce:              sealed.wrapNonce,
		WrappedDEK:             sealed.wrappedDEK,
		AADDigest:              sealed.aadDigest,
		PlaintextDigest:        sealed.plaintextDigest,
		BootstrapReceiptJSON:   append([]byte(nil), receiptJSON...),
		BootstrapReceiptDigest: append([]byte(nil), receiptDigest[:]...),
		UpdatedAt:              at.UTC(),
	}
	if err := m.store.ActivateRemoteDeviceIdentityCAS(ctx, localID, reseal); err != nil {
		return OpenedRemoteIdentity{}, err
	}
	return m.OpenRemoteIdentity(ctx, localID, serverOrigin, options)
}

type sealedDeviceKeyEnvelope struct {
	payloadNonce      []byte
	payloadCiphertext []byte
	wrapNonce         []byte
	wrappedDEK        []byte
	aadDigest         []byte
	plaintextDigest   []byte
}

func (m *Manager) sealDeviceKeyEnvelope(localID string, envelope DeviceKeyEnvelopeV1, signingDigest, exchangeDigest []byte) (sealedDeviceKeyEnvelope, error) {
	var result sealedDeviceKeyEnvelope
	if _, _, _, _, err := validateDeviceKeyEnvelope(envelope); err != nil {
		return result, domain.NewError(domain.CodeInvalidArgument, "remote identity envelope is invalid")
	}
	plaintext, err := json.Marshal(envelope)
	if err != nil || len(plaintext) > maxDeviceKeyEnvelopeSize {
		return result, domain.NewError(domain.CodeInvalidArgument, "remote identity envelope is too large")
	}
	aad, err := deviceEnvelopeAAD(localID, envelope.ServerOrigin, envelope.ServerDeviceID, signingDigest, exchangeDigest, int64(envelope.KeyRevision))
	if err != nil {
		return result, domain.NewError(domain.CodeInvalidArgument, "remote identity AAD is invalid")
	}
	kek, err := m.currentKEK()
	if err != nil {
		return result, err
	}
	defer zero(kek)
	dek, err := randomBytes(32)
	if err != nil {
		return result, domain.WrapError(domain.CodeConflict, "remote identity DEK randomness failed", err)
	}
	defer zero(dek)
	result.payloadNonce, err = randomBytes(12)
	if err != nil {
		return sealedDeviceKeyEnvelope{}, err
	}
	result.wrapNonce, err = randomBytes(12)
	if err != nil {
		return sealedDeviceKeyEnvelope{}, err
	}
	if bytes.Equal(result.payloadNonce, result.wrapNonce) {
		return sealedDeviceKeyEnvelope{}, domain.NewError(domain.CodeConflict, "remote identity nonces were not independent")
	}
	payloadGCM, err := newGCM(dek)
	if err != nil {
		return sealedDeviceKeyEnvelope{}, domain.WrapError(domain.CodeConflict, "remote identity payload cipher failed", err)
	}
	wrapGCM, err := newGCM(kek)
	if err != nil {
		return sealedDeviceKeyEnvelope{}, domain.WrapError(domain.CodeConflict, "remote identity wrap cipher failed", err)
	}
	result.payloadCiphertext = payloadGCM.Seal(nil, result.payloadNonce, plaintext, aad)
	result.wrappedDEK = wrapGCM.Seal(nil, result.wrapNonce, dek, aad)
	aadDigest := sha256.Sum256(aad)
	plaintextDigest := sha256.Sum256(plaintext)
	result.aadDigest = append([]byte(nil), aadDigest[:]...)
	result.plaintextDigest = append([]byte(nil), plaintextDigest[:]...)
	zero(plaintext)
	return result, nil
}

func deviceEnvelopeAAD(localID, serverOrigin, serverDeviceID string, signingDigest, exchangeDigest []byte, keyRevision int64) ([]byte, error) {
	if !remoteIdentityIDPattern.MatchString(localID) || len(signingDigest) != 32 || len(exchangeDigest) != 32 || keyRevision != 1 {
		return nil, errors.New("invalid envelope AAD input")
	}
	if _, err := transport.ParseCanonicalServerOriginV1(serverOrigin, transport.CanonicalServerOriginOptions{AllowDevelopmentLocalhost: true}); err != nil {
		return nil, err
	}
	if _, err := transport.ParseUUIDv7(serverDeviceID); err != nil {
		return nil, err
	}
	return transport.Frame(
		[]byte("multidesk-device-key-envelope-v1"),
		[]byte("1"),
		[]byte(serverOrigin),
		[]byte(localID),
		[]byte(serverDeviceID),
		signingDigest,
		exchangeDigest,
		[]byte(strconv.FormatInt(keyRevision, 10)),
	)
}

func validateDeviceKeyEnvelope(envelope DeviceKeyEnvelopeV1) (seed, xPrivate, signingPublic, xPublic []byte, err error) {
	fail := func() ([]byte, []byte, []byte, []byte, error) {
		zero(seed)
		zero(xPrivate)
		return nil, nil, nil, nil, errors.New("invalid DeviceKeyEnvelopeV1")
	}
	if envelope.Version != 1 || envelope.KeyRevision != 1 ||
		(envelope.Status != DeviceKeyEnvelopePending && envelope.Status != DeviceKeyEnvelopeActive && envelope.Status != DeviceKeyEnvelopeRetired) ||
		envelope.CreatedAt.IsZero() || envelope.UpdatedAt.IsZero() || envelope.UpdatedAt.Before(envelope.CreatedAt) || envelope.CreatedAt.Location() != time.UTC || envelope.UpdatedAt.Location() != time.UTC {
		return fail()
	}
	if _, parseErr := transport.ParseCanonicalServerOriginV1(envelope.ServerOrigin, transport.CanonicalServerOriginOptions{AllowDevelopmentLocalhost: true}); parseErr != nil {
		return fail()
	}
	if _, parseErr := transport.ParseUUIDv7(envelope.ServerDeviceID); parseErr != nil {
		return fail()
	}
	seed, err = transport.DecodeBase64URLFixed(envelope.Ed25519Seed, 32)
	if err != nil {
		return fail()
	}
	xPrivate, err = transport.DecodeBase64URLFixed(envelope.X25519PrivateKey, 32)
	if err != nil {
		return fail()
	}
	encodedSigning, err := transport.DecodeBase64URLFixed(envelope.SigningPublicKey, 32)
	if err != nil {
		return fail()
	}
	encodedExchange, err := transport.DecodeBase64URLFixed(envelope.ExchangePublicKey, 32)
	if err != nil {
		return fail()
	}
	encodedSigningDigest, err := transport.DecodeBase64URLFixed(envelope.SigningKeyDigest, 32)
	if err != nil {
		return fail()
	}
	encodedExchangeDigest, err := transport.DecodeBase64URLFixed(envelope.ExchangeKeyDigest, 32)
	if err != nil {
		return fail()
	}
	private := ed25519.NewKeyFromSeed(seed)
	signingPublic = append([]byte(nil), private.Public().(ed25519.PublicKey)...)
	zero(private)
	xPublic, err = curve25519.X25519(xPrivate, curve25519.Basepoint)
	if err != nil || allZero(xPublic) || !bytes.Equal(signingPublic, encodedSigning) || !bytes.Equal(xPublic, encodedExchange) ||
		!bytes.Equal(hashBytes(signingPublic), encodedSigningDigest) || !bytes.Equal(hashBytes(xPublic), encodedExchangeDigest) {
		return fail()
	}
	return seed, xPrivate, signingPublic, xPublic, nil
}

func hashBytes(value []byte) []byte {
	digest := sha256.Sum256(value)
	return digest[:]
}

func allZero(value []byte) bool {
	var combined byte
	for _, item := range value {
		combined |= item
	}
	return combined == 0
}

var remoteIdentityIDPattern = regexp.MustCompile(`^remote_identity_[0-9a-f]{32}$`)
