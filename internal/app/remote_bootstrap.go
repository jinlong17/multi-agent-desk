package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime"
	"sort"
	"time"

	controlplanev1 "github.com/jinlong17/multi-agent-desk/internal/controlplane/api/generated"
	identitycrypto "github.com/jinlong17/multi-agent-desk/internal/crypto"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
	"github.com/jinlong17/multi-agent-desk/internal/vault"
)

type RemoteBootstrapService struct {
	Store         *storage.Store
	Vault         *vault.Manager
	Now           func() time.Time
	ClientVersion string
	Platform      string
	Architecture  string
}

type BootstrapPrepareInput struct {
	ServerOrigin              string
	Name                      string
	AllowDevelopmentLocalhost bool
}

// BootstrapAnchorProofV1 is the public Daemon proof transfer object imported
// by the same-origin Web page. It contains no private key, token, cookie, CSRF,
// connection credential, or activation secret.
type BootstrapAnchorProofV1 struct {
	Version        int    `json:"version"`
	CeremonyID     string `json:"ceremonyId"`
	ServerOrigin   string `json:"serverOrigin"`
	AnchorDeviceID string `json:"anchorDeviceId"`
	SigningProof   string `json:"signingProof"`
	ExchangeProof  string `json:"exchangeProof"`
}

func (s *RemoteBootstrapService) defaults() {
	if s.Now == nil {
		s.Now = time.Now
	}
	if s.ClientVersion == "" {
		s.ClientVersion = "devel"
	}
	if s.Platform == "" {
		s.Platform = runtime.GOOS
	}
	if s.Architecture == "" {
		s.Architecture = runtime.GOARCH
	}
}

func (s *RemoteBootstrapService) Prepare(ctx context.Context, input BootstrapPrepareInput) (controlplanev1.BootstrapAnchorDescriptorV1, error) {
	s.defaults()
	if s.Store == nil || s.Vault == nil || ctx == nil || input.Name == "" || len(input.Name) > 128 || len(s.ClientVersion) > 64 || len(s.Architecture) > 32 {
		return controlplanev1.BootstrapAnchorDescriptorV1{}, domain.NewError(domain.CodeInvalidArgument, "bootstrap prepare input is invalid")
	}
	platform := controlplanev1.BootstrapAnchorV1Platform(s.Platform)
	if !platform.Valid() {
		return controlplanev1.BootstrapAnchorDescriptorV1{}, domain.NewError(domain.CodeUnsupportedPlatform, "bootstrap platform is unsupported")
	}
	opened, err := s.Vault.PrepareRemoteIdentity(ctx, input.ServerOrigin, vault.RemoteIdentityOptions{AllowDevelopmentLocalhost: input.AllowDevelopmentLocalhost}, s.Now().UTC())
	if err != nil {
		return controlplanev1.BootstrapAnchorDescriptorV1{}, err
	}
	defer opened.ZeroPrivateMaterial()
	assertion := controlplanev1.BootstrapKeyEnvelopeAssertionV1{
		FormatVersion:  controlplanev1.BootstrapKeyEnvelopeAssertionV1FormatVersionN1,
		KeyRevision:    controlplanev1.BootstrapKeyEnvelopeAssertionV1KeyRevisionN1,
		RecordRevision: int(opened.Record.RecordRevision),
		SealedAt:       opened.Record.UpdatedAt,
		Status:         controlplanev1.BootstrapKeyEnvelopeAssertionV1StatusPending,
	}
	pinDigest, err := identitycrypto.DevicePinDigestV1(opened.Record.ServerDeviceID, opened.Record.SigningPublicKey, opened.Record.ExchangePublicKey)
	if err != nil {
		return controlplanev1.BootstrapAnchorDescriptorV1{}, domain.NewError(domain.CodeVaultCorrupt, "bootstrap pin digest could not be derived")
	}
	capabilities := daemonBootstrapCapabilities()
	anchor := controlplanev1.BootstrapAnchorV1{
		Architecture:         s.Architecture,
		Capabilities:         capabilities,
		ClientVersion:        s.ClientVersion,
		DeviceId:             opened.Record.ServerDeviceID,
		ExchangeKeyDigest:    base64.RawURLEncoding.EncodeToString(opened.Record.ExchangeKeyDigest),
		ExchangePublicKey:    base64.RawURLEncoding.EncodeToString(opened.Record.ExchangePublicKey),
		KeyEnvelopeAssertion: assertion,
		Kind:                 controlplanev1.BootstrapAnchorV1KindDaemon,
		Name:                 input.Name,
		PinDigest:            base64.RawURLEncoding.EncodeToString(pinDigest[:]),
		Platform:             platform,
		SigningKeyDigest:     base64.RawURLEncoding.EncodeToString(opened.Record.SigningKeyDigest),
		SigningPublicKey:     base64.RawURLEncoding.EncodeToString(opened.Record.SigningPublicKey),
		StorageMode:          controlplanev1.BootstrapAnchorV1StorageModePortableVaultV1,
	}
	return controlplanev1.BootstrapAnchorDescriptorV1{
		Anchor:       anchor,
		ServerOrigin: opened.Record.ServerOrigin,
		Version:      controlplanev1.BootstrapAnchorDescriptorV1VersionN1,
	}, nil
}

// Prove requires the file-imported challenge and the freshly refetched HTTPS
// challenge to marshal byte-identically before opening private material. It
// performs no database state transition.
func (s *RemoteBootstrapService) Prove(ctx context.Context, imported, refetched controlplanev1.BootstrapAnchorChallengeV1) (BootstrapAnchorProofV1, error) {
	s.defaults()
	if s.Store == nil || s.Vault == nil || ctx == nil {
		return BootstrapAnchorProofV1{}, domain.NewError(domain.CodeInvalidArgument, "bootstrap prove service is unavailable")
	}
	importedBytes, err := json.Marshal(imported)
	if err != nil {
		return BootstrapAnchorProofV1{}, domain.NewError(domain.CodeInvalidArgument, "bootstrap challenge is invalid")
	}
	refetchedBytes, err := json.Marshal(refetched)
	if err != nil || subtle.ConstantTimeCompare(importedBytes, refetchedBytes) != 1 {
		return BootstrapAnchorProofV1{}, domain.NewError(domain.CodeIdentityConfirmationMismatch, "bootstrap challenge did not match the HTTPS ceremony")
	}
	if err := validateBootstrapChallenge(imported, s.Now().UTC()); err != nil {
		return BootstrapAnchorProofV1{}, err
	}
	record, err := s.Store.PendingRemoteDeviceIdentityForOrigin(ctx, imported.ServerOrigin)
	if err != nil {
		return BootstrapAnchorProofV1{}, err
	}
	opened, err := s.Vault.OpenRemoteIdentity(ctx, record.ID, imported.ServerOrigin, vault.RemoteIdentityOptions{AllowDevelopmentLocalhost: true})
	if err != nil {
		return BootstrapAnchorProofV1{}, err
	}
	defer opened.ZeroPrivateMaterial()
	if err := matchAnchorToRecord(imported.Anchor, opened.Record); err != nil {
		_ = s.Store.QuarantineRemoteDeviceIdentity(ctx, opened.Record.ID, "bootstrap_anchor_mismatch")
		return BootstrapAnchorProofV1{}, err
	}
	assertionDigest, err := bootstrapStorageAssertionDigest(imported.Anchor.KeyEnvelopeAssertion)
	if err != nil {
		return BootstrapAnchorProofV1{}, err
	}
	wireAssertionDigest, err := transport.DecodeBase64URLFixed(imported.StorageAssertionDigest, 32)
	if err != nil || subtle.ConstantTimeCompare(assertionDigest[:], wireAssertionDigest) != 1 {
		return BootstrapAnchorProofV1{}, domain.NewError(domain.CodeIdentityConfirmationMismatch, "bootstrap storage assertion digest did not match")
	}
	challenge, err := transport.DecodeBase64URLFixed(imported.Challenge, 32)
	if err != nil {
		return BootstrapAnchorProofV1{}, domain.NewError(domain.CodeInvalidArgument, "bootstrap challenge bytes are invalid")
	}
	serverEphemeral, err := transport.DecodeBase64URLFixed(imported.ServerEphemeralExchangePublicKey, 32)
	if err != nil {
		return BootstrapAnchorProofV1{}, domain.NewError(domain.CodeInvalidArgument, "bootstrap server exchange key is invalid")
	}
	popInput := identitycrypto.DevicePoPContextV1{
		Purpose:                          "bootstrap",
		CeremonyID:                       imported.CeremonyId,
		DeviceID:                         imported.Anchor.DeviceId,
		SigningPublicKey:                 opened.Record.SigningPublicKey,
		ExchangePublicKey:                opened.Record.ExchangePublicKey,
		StorageMode:                      string(imported.Anchor.StorageMode),
		StorageAssertionDigest:           assertionDigest[:],
		ServerEphemeralExchangePublicKey: serverEphemeral,
		Challenge:                        challenge,
		ExpiresAt:                        imported.ExpiresAt,
	}
	signingProof, exchangeProof, err := identitycrypto.CreateDevicePoPV1(opened.Ed25519Seed, opened.X25519PrivateKey, popInput)
	if err != nil {
		return BootstrapAnchorProofV1{}, domain.NewError(domain.CodeIdentityConfirmationMismatch, "bootstrap proof could not be created")
	}
	return BootstrapAnchorProofV1{
		Version:        1,
		CeremonyID:     imported.CeremonyId,
		ServerOrigin:   imported.ServerOrigin,
		AnchorDeviceID: imported.Anchor.DeviceId,
		SigningProof:   base64.RawURLEncoding.EncodeToString(signingProof),
		ExchangeProof:  base64.RawURLEncoding.EncodeToString(exchangeProof),
	}, nil
}

func (s *RemoteBootstrapService) Activate(ctx context.Context, imported, refetched controlplanev1.BootstrapCommitReceiptV1) (controlplanev1.BootstrapCommitReceiptV1, error) {
	s.defaults()
	if s.Store == nil || s.Vault == nil || ctx == nil {
		return controlplanev1.BootstrapCommitReceiptV1{}, domain.NewError(domain.CodeInvalidArgument, "bootstrap activate service is unavailable")
	}
	importedBytes, err := json.Marshal(imported)
	if err != nil {
		return controlplanev1.BootstrapCommitReceiptV1{}, domain.NewError(domain.CodeInvalidArgument, "bootstrap receipt is invalid")
	}
	refetchedBytes, err := json.Marshal(refetched)
	if err != nil || subtle.ConstantTimeCompare(importedBytes, refetchedBytes) != 1 {
		return controlplanev1.BootstrapCommitReceiptV1{}, domain.NewError(domain.CodeIdentityConfirmationMismatch, "bootstrap receipt did not match the HTTPS ceremony")
	}
	if err := validateBootstrapReceipt(imported, s.Now().UTC()); err != nil {
		return controlplanev1.BootstrapCommitReceiptV1{}, err
	}
	record, err := s.Store.PendingRemoteDeviceIdentityForOrigin(ctx, imported.ServerOrigin)
	if err != nil {
		// Idempotent crash recovery: an already-active exact receipt succeeds.
		active, activeErr := s.Store.ActiveRemoteDeviceIdentityForOrigin(ctx, imported.ServerOrigin)
		if activeErr == nil && active.ServerDeviceID == imported.AnchorDeviceId && bytes.Equal(active.BootstrapReceiptJSON, importedBytes) {
			return imported, nil
		}
		return controlplanev1.BootstrapCommitReceiptV1{}, err
	}
	if record.ServerDeviceID != imported.AnchorDeviceId || base64.RawURLEncoding.EncodeToString(record.SigningKeyDigest) != imported.SigningKeyDigest || base64.RawURLEncoding.EncodeToString(record.ExchangeKeyDigest) != imported.ExchangeKeyDigest {
		_ = s.Store.QuarantineRemoteDeviceIdentity(ctx, record.ID, "bootstrap_receipt_mismatch")
		return controlplanev1.BootstrapCommitReceiptV1{}, domain.NewError(domain.CodeIdentityConfirmationMismatch, "bootstrap receipt did not match the pending identity")
	}
	opened, err := s.Vault.ActivateRemoteIdentityCAS(ctx, record.ID, imported.ServerOrigin, vault.RemoteIdentityOptions{AllowDevelopmentLocalhost: true}, record.RecordRevision, importedBytes, s.Now().UTC())
	if err != nil {
		return controlplanev1.BootstrapCommitReceiptV1{}, err
	}
	opened.ZeroPrivateMaterial()
	return imported, nil
}

func daemonBootstrapCapabilities() controlplanev1.DeviceCapabilityListV1 {
	values := controlplanev1.DeviceCapabilityListV1{
		controlplanev1.DeviceCapabilityV1MadV1DeviceEnrollApprove,
		controlplanev1.DeviceCapabilityV1MadV1DeviceEnrollRequest,
		controlplanev1.DeviceCapabilityV1MadV1DeviceRevoke,
		controlplanev1.DeviceCapabilityV1MadV1MetadataRead,
		controlplanev1.DeviceCapabilityV1MadV1MetadataWrite,
		controlplanev1.DeviceCapabilityV1MadV1PresenceWrite,
		controlplanev1.DeviceCapabilityV1MadV1SessionCommandAck,
		controlplanev1.DeviceCapabilityV1MadV1SessionCommandClaim,
		controlplanev1.DeviceCapabilityV1MadV1SessionCommandResult,
		controlplanev1.DeviceCapabilityV1MadV1SyncPull,
		controlplanev1.DeviceCapabilityV1MadV1SyncPush,
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values
}

func bootstrapStorageAssertionDigest(assertion controlplanev1.BootstrapKeyEnvelopeAssertionV1) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	if assertion.FormatVersion != controlplanev1.BootstrapKeyEnvelopeAssertionV1FormatVersionN1 || assertion.KeyRevision != controlplanev1.BootstrapKeyEnvelopeAssertionV1KeyRevisionN1 || assertion.RecordRevision < 1 || assertion.Status != controlplanev1.BootstrapKeyEnvelopeAssertionV1StatusPending || assertion.SealedAt.IsZero() || assertion.SealedAt.Location() != time.UTC {
		return digest, domain.NewError(domain.CodeInvalidArgument, "bootstrap key-envelope assertion is invalid")
	}
	canonical, err := transport.BootstrapKeyEnvelopeAssertionJCSV1(int(assertion.FormatVersion), int(assertion.KeyRevision), assertion.RecordRevision, assertion.SealedAt, string(assertion.Status))
	if err != nil {
		return digest, err
	}
	return sha256.Sum256(canonical), nil
}

func validateBootstrapChallenge(value controlplanev1.BootstrapAnchorChallengeV1, now time.Time) error {
	if value.Version != controlplanev1.BootstrapAnchorChallengeV1VersionN1 || value.ExpiresAt.IsZero() || !value.ExpiresAt.After(now) || value.ExpiresAt.After(now.Add(5*time.Minute+time.Second)) {
		return domain.NewError(domain.CodeConfirmationExpired, "bootstrap challenge is expired or invalid")
	}
	if _, err := transport.ParseUUIDv7(value.CeremonyId); err != nil {
		return domain.NewError(domain.CodeInvalidArgument, "bootstrap ceremony ID is invalid")
	}
	if _, err := transport.ParseCanonicalServerOriginV1(value.ServerOrigin, transport.CanonicalServerOriginOptions{AllowDevelopmentLocalhost: true}); err != nil {
		return domain.NewError(domain.CodeInvalidArgument, "bootstrap server origin is invalid")
	}
	if _, err := transport.DecodeBase64URLFixed(value.Challenge, 32); err != nil {
		return domain.NewError(domain.CodeInvalidArgument, "bootstrap challenge is invalid")
	}
	if _, err := transport.DecodeBase64URLFixed(value.ServerEphemeralExchangePublicKey, 32); err != nil {
		return domain.NewError(domain.CodeInvalidArgument, "bootstrap server exchange key is invalid")
	}
	if _, err := transport.DecodeBase64URLFixed(value.StorageAssertionDigest, 32); err != nil {
		return domain.NewError(domain.CodeInvalidArgument, "bootstrap storage assertion digest is invalid")
	}
	return nil
}

func validateBootstrapReceipt(value controlplanev1.BootstrapCommitReceiptV1, now time.Time) error {
	if value.Version != controlplanev1.BootstrapCommitReceiptV1VersionN1 || value.Type != controlplanev1.BootstrapCommitReceipt || value.StorageMode != controlplanev1.BootstrapCommitReceiptV1StorageModePortableVaultV1 || value.ActivatedAt.IsZero() || value.ActivatedAt.After(now.Add(time.Minute)) {
		return domain.NewError(domain.CodeInvalidArgument, "bootstrap receipt is invalid")
	}
	for _, id := range []string{value.CeremonyId, value.UserId, value.AnchorDeviceId} {
		if _, err := transport.ParseUUIDv7(id); err != nil {
			return domain.NewError(domain.CodeInvalidArgument, "bootstrap receipt identity is invalid")
		}
	}
	if _, err := transport.ParseCanonicalServerOriginV1(value.ServerOrigin, transport.CanonicalServerOriginOptions{AllowDevelopmentLocalhost: true}); err != nil {
		return domain.NewError(domain.CodeInvalidArgument, "bootstrap receipt origin is invalid")
	}
	for _, digest := range []string{value.SigningKeyDigest, value.ExchangeKeyDigest, value.StorageAssertionDigest, value.SigningProofDigest, value.ExchangeProofDigest} {
		if _, err := transport.DecodeBase64URLFixed(digest, 32); err != nil {
			return domain.NewError(domain.CodeInvalidArgument, "bootstrap receipt digest is invalid")
		}
	}
	return nil
}

func matchAnchorToRecord(anchor controlplanev1.BootstrapAnchorV1, record storage.RemoteDeviceIdentityRecord) error {
	if anchor.Kind != controlplanev1.BootstrapAnchorV1KindDaemon || anchor.StorageMode != controlplanev1.BootstrapAnchorV1StorageModePortableVaultV1 || anchor.DeviceId != record.ServerDeviceID ||
		anchor.SigningPublicKey != base64.RawURLEncoding.EncodeToString(record.SigningPublicKey) || anchor.ExchangePublicKey != base64.RawURLEncoding.EncodeToString(record.ExchangePublicKey) ||
		anchor.SigningKeyDigest != base64.RawURLEncoding.EncodeToString(record.SigningKeyDigest) || anchor.ExchangeKeyDigest != base64.RawURLEncoding.EncodeToString(record.ExchangeKeyDigest) ||
		anchor.KeyEnvelopeAssertion.RecordRevision != int(record.RecordRevision) || anchor.KeyEnvelopeAssertion.Status != controlplanev1.BootstrapKeyEnvelopeAssertionV1StatusPending {
		return domain.NewError(domain.CodeIdentityConfirmationMismatch, "bootstrap anchor did not match the pending Vault envelope")
	}
	pin, err := identitycrypto.DevicePinDigestV1(record.ServerDeviceID, record.SigningPublicKey, record.ExchangePublicKey)
	if err != nil || anchor.PinDigest != base64.RawURLEncoding.EncodeToString(pin[:]) {
		return domain.NewError(domain.CodeIdentityConfirmationMismatch, "bootstrap anchor pin did not match")
	}
	return nil
}

func DecodeBootstrapDescriptor(data []byte) (controlplanev1.BootstrapAnchorDescriptorV1, error) {
	var result controlplanev1.BootstrapAnchorDescriptorV1
	if err := transport.DecodeStrictJSON(bytes.NewReader(data), 64<<10, &result); err != nil {
		return result, domain.NewError(domain.CodeInvalidArgument, "bootstrap descriptor JSON is invalid")
	}
	if result.Version != controlplanev1.BootstrapAnchorDescriptorV1VersionN1 || result.ServerOrigin == "" {
		return result, domain.NewError(domain.CodeInvalidArgument, "bootstrap descriptor is invalid")
	}
	return result, nil
}

func DecodeBootstrapChallenge(data []byte) (controlplanev1.BootstrapAnchorChallengeV1, error) {
	var result controlplanev1.BootstrapAnchorChallengeV1
	if err := transport.DecodeStrictJSON(bytes.NewReader(data), 256<<10, &result); err != nil {
		return result, domain.NewError(domain.CodeInvalidArgument, "bootstrap challenge JSON is invalid")
	}
	return result, nil
}

func DecodeBootstrapReceipt(data []byte) (controlplanev1.BootstrapCommitReceiptV1, error) {
	var result controlplanev1.BootstrapCommitReceiptV1
	if err := transport.DecodeStrictJSON(bytes.NewReader(data), 4096, &result); err != nil {
		return result, domain.NewError(domain.CodeInvalidArgument, "bootstrap receipt JSON is invalid")
	}
	return result, nil
}

func EncodePublicBootstrapTransfer(value any, limit int) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil || len(data) < 2 || len(data) > limit {
		return nil, fmt.Errorf("public bootstrap transfer object is invalid")
	}
	return data, nil
}
