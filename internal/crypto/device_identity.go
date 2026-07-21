package crypto

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/transport"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

type DevicePoPContextV1 struct {
	Purpose                          string
	CeremonyID                       string
	DeviceID                         string
	SigningPublicKey                 []byte
	ExchangePublicKey                []byte
	StorageMode                      string
	StorageAssertionDigest           []byte
	ServerEphemeralExchangePublicKey []byte
	Challenge                        []byte
	ExpiresAt                        time.Time
}

func DevicePinDigestV1(deviceID string, signingPublicKey, exchangePublicKey []byte) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	if _, err := transport.ParseUUIDv7(deviceID); err != nil || len(signingPublicKey) != ed25519.PublicKeySize || len(exchangePublicKey) != curve25519.PointSize {
		return digest, errors.New("device pin input is invalid")
	}
	framed, err := transport.Frame([]byte("multidesk-device-pin-v1"), []byte(deviceID), signingPublicKey, exchangePublicKey)
	if err != nil {
		return digest, err
	}
	return sha256.Sum256(framed), nil
}

func DeviceFingerprintV1(pinDigest []byte) (string, error) {
	if len(pinDigest) != sha256.Size {
		return "", errors.New("device pin digest is invalid")
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(pinDigest[:15])
	if len(encoded) != 24 {
		return "", errors.New("device fingerprint encoding failed")
	}
	parts := make([]string, 0, 6)
	for offset := 0; offset < len(encoded); offset += 4 {
		parts = append(parts, encoded[offset:offset+4])
	}
	return strings.Join(parts, "-"), nil
}

func BuildDevicePoPContextV1(input DevicePoPContextV1) ([]byte, error) {
	if input.Purpose != "bootstrap" && input.Purpose != "enrollment" {
		return nil, errors.New("device proof purpose is invalid")
	}
	if _, err := transport.ParseUUIDv7(input.CeremonyID); err != nil {
		return nil, errors.New("device proof ceremony is invalid")
	}
	if _, err := transport.ParseUUIDv7(input.DeviceID); err != nil {
		return nil, errors.New("device proof subject is invalid")
	}
	if len(input.SigningPublicKey) != 32 || len(input.ExchangePublicKey) != 32 || len(input.StorageAssertionDigest) != 32 || len(input.ServerEphemeralExchangePublicKey) != 32 || len(input.Challenge) != 32 || input.ExpiresAt.IsZero() || input.ExpiresAt.Location() != time.UTC {
		return nil, errors.New("device proof fields are invalid")
	}
	if input.StorageMode != "portable_vault_v1" && input.StorageMode != "native" && input.StorageMode != "software_wrapped" && input.StorageMode != "metadata_only" && input.StorageMode != "desktop_key_store_deferred" {
		return nil, errors.New("device proof storage mode is invalid")
	}
	return transport.Frame(
		[]byte("multidesk-x25519-pop-context-v1"),
		[]byte("v1"),
		[]byte(input.Purpose),
		[]byte(input.CeremonyID),
		[]byte(input.DeviceID),
		input.SigningPublicKey,
		input.ExchangePublicKey,
		[]byte(input.StorageMode),
		input.StorageAssertionDigest,
		input.ServerEphemeralExchangePublicKey,
		input.Challenge,
		[]byte(input.ExpiresAt.Format(time.RFC3339Nano)),
	)
}

func CreateDevicePoPV1(ed25519Seed, x25519PrivateKey []byte, input DevicePoPContextV1) (signingProof, exchangeProof []byte, err error) {
	if len(ed25519Seed) != ed25519.SeedSize || len(x25519PrivateKey) != curve25519.ScalarSize {
		return nil, nil, errors.New("device proof private material is invalid")
	}
	private := ed25519.NewKeyFromSeed(ed25519Seed)
	defer zero(private)
	if !hmac.Equal(private.Public().(ed25519.PublicKey), input.SigningPublicKey) {
		return nil, nil, errors.New("device signing proof key does not match transcript")
	}
	xPublic, err := curve25519.X25519(x25519PrivateKey, curve25519.Basepoint)
	if err != nil || !hmac.Equal(xPublic, input.ExchangePublicKey) {
		return nil, nil, errors.New("device exchange proof key does not match transcript")
	}
	contextBytes, err := BuildDevicePoPContextV1(input)
	if err != nil {
		return nil, nil, err
	}
	shared, err := curve25519.X25519(x25519PrivateKey, input.ServerEphemeralExchangePublicKey)
	if err != nil || allZero(shared) {
		zero(shared)
		return nil, nil, errors.New("device exchange proof shared secret is invalid")
	}
	defer zero(shared)
	popKey, err := deriveDevicePoPKey(input.CeremonyID, input.Challenge, shared, contextBytes)
	if err != nil {
		return nil, nil, err
	}
	defer zero(popKey)
	exchangeMessage, err := transport.Frame([]byte("multidesk-x25519-pop-proof-v1"), contextBytes)
	if err != nil {
		return nil, nil, err
	}
	exchangeMAC := hmac.New(sha256.New, popKey)
	_, _ = exchangeMAC.Write(exchangeMessage)
	exchangeProof = exchangeMAC.Sum(nil)
	signingMessage, err := transport.Frame([]byte("multidesk-ed25519-pop-proof-v1"), contextBytes)
	if err != nil {
		return nil, nil, err
	}
	signingProof = ed25519.Sign(private, signingMessage)
	return signingProof, exchangeProof, nil
}

func VerifyDevicePoPV1(serverEphemeralPrivateKey, signingProof, exchangeProof []byte, input DevicePoPContextV1) error {
	if len(serverEphemeralPrivateKey) != curve25519.ScalarSize || len(signingProof) != ed25519.SignatureSize || len(exchangeProof) != sha256.Size {
		return errors.New("device proof is invalid")
	}
	contextBytes, err := BuildDevicePoPContextV1(input)
	if err != nil {
		return err
	}
	signingMessage, err := transport.Frame([]byte("multidesk-ed25519-pop-proof-v1"), contextBytes)
	if err != nil || !ed25519.Verify(ed25519.PublicKey(input.SigningPublicKey), signingMessage, signingProof) {
		return errors.New("device signing proof is invalid")
	}
	shared, err := curve25519.X25519(serverEphemeralPrivateKey, input.ExchangePublicKey)
	if err != nil || allZero(shared) {
		zero(shared)
		return errors.New("device exchange proof shared secret is invalid")
	}
	defer zero(shared)
	popKey, err := deriveDevicePoPKey(input.CeremonyID, input.Challenge, shared, contextBytes)
	if err != nil {
		return err
	}
	defer zero(popKey)
	exchangeMessage, err := transport.Frame([]byte("multidesk-x25519-pop-proof-v1"), contextBytes)
	if err != nil {
		return err
	}
	want := hmac.New(sha256.New, popKey)
	_, _ = want.Write(exchangeMessage)
	if !hmac.Equal(want.Sum(nil), exchangeProof) {
		return errors.New("device exchange proof is invalid")
	}
	return nil
}

func deriveDevicePoPKey(ceremonyID string, challenge, shared, contextBytes []byte) ([]byte, error) {
	saltFrame, err := transport.Frame([]byte("multidesk-x25519-pop-salt-v1"), []byte(ceremonyID), challenge)
	if err != nil {
		return nil, err
	}
	salt := sha256.Sum256(saltFrame)
	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.New(sha256.New, shared, salt[:], contextBytes), key); err != nil {
		zero(key)
		return nil, err
	}
	return key, nil
}

func allZero(value []byte) bool {
	var combined byte
	for _, item := range value {
		combined |= item
	}
	return combined == 0
}

func zero(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
