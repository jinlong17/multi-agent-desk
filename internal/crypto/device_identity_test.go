package crypto

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"strings"
	"testing"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/transport"
	"golang.org/x/crypto/curve25519"
)

func TestDevicePinAndPoPRoundTrip(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	seed := private.Seed()
	xPrivate := make([]byte, 32)
	if _, err := rand.Read(xPrivate); err != nil {
		t.Fatal(err)
	}
	xPublic, err := curve25519.X25519(xPrivate, curve25519.Basepoint)
	if err != nil {
		t.Fatal(err)
	}
	deviceID, _ := transport.NewUUIDv7()
	ceremonyID, _ := transport.NewUUIDv7()
	pin, err := DevicePinDigestV1(deviceID, public, xPublic)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := DeviceFingerprintV1(pin[:])
	if err != nil || len(fingerprint) != 29 || strings.Count(fingerprint, "-") != 5 {
		t.Fatalf("fingerprint=%q err=%v", fingerprint, err)
	}
	serverPrivate := make([]byte, 32)
	if _, err := rand.Read(serverPrivate); err != nil {
		t.Fatal(err)
	}
	serverPublic, err := curve25519.X25519(serverPrivate, curve25519.Basepoint)
	if err != nil {
		t.Fatal(err)
	}
	input := DevicePoPContextV1{
		Purpose: "bootstrap", CeremonyID: ceremonyID, DeviceID: deviceID,
		SigningPublicKey: public, ExchangePublicKey: xPublic,
		StorageMode: "portable_vault_v1", StorageAssertionDigest: bytes.Repeat([]byte{7}, 32),
		ServerEphemeralExchangePublicKey: serverPublic, Challenge: bytes.Repeat([]byte{9}, 32),
		ExpiresAt: time.Unix(1_900_000_000, 0).UTC(),
	}
	signingProof, exchangeProof, err := CreateDevicePoPV1(seed, xPrivate, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyDevicePoPV1(serverPrivate, signingProof, exchangeProof, input); err != nil {
		t.Fatal(err)
	}
	tampered := input
	tampered.StorageAssertionDigest = append([]byte(nil), input.StorageAssertionDigest...)
	tampered.StorageAssertionDigest[0] ^= 1
	if err := VerifyDevicePoPV1(serverPrivate, signingProof, exchangeProof, tampered); err == nil {
		t.Fatal("tampered transcript verified")
	}
	if got := sha256.Sum256(public); bytes.Equal(pin[:], got[:]) {
		t.Fatal("pin digest omitted domain/device/exchange binding")
	}
}
