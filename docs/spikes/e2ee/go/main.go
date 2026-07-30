package main

import (
	"bytes"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cloudflare/circl/hpke"
	"golang.org/x/crypto/chacha20poly1305"
)

const hpkeSuiteName = "DHKEM(X25519,HKDF-SHA256)/HKDF-SHA256/ChaCha20Poly1305/Auth"

type vectorInput struct {
	SchemaVersion int `json:"schemaVersion"`
	Seeds         struct {
		ApproverEd25519         string `json:"approverEd25519"`
		SubjectEd25519          string `json:"subjectEd25519"`
		ServerPopX25519Private  string `json:"serverPopX25519Private"`
		RestartPopX25519Private string `json:"restartPopX25519Private"`
		SourceX25519IKM         string `json:"sourceX25519Ikm"`
		TargetX25519IKM         string `json:"targetX25519Ikm"`
		EphemeralX25519IKM      string `json:"ephemeralX25519Ikm"`
		PairwiseRootPeerAEpoch1 string `json:"pairwiseRootPeerAEpoch1"`
		PairwiseRootPeerAEpoch2 string `json:"pairwiseRootPeerAEpoch2"`
		PeerBX25519IKM          string `json:"peerBX25519Ikm"`
		PairwiseRootPeerBEpoch1 string `json:"pairwiseRootPeerBEpoch1"`
	} `json:"seeds"`
	Attestation struct {
		ApproverDeviceID string   `json:"approverDeviceId"`
		AttestationID    string   `json:"attestationId"`
		Capabilities     []string `json:"capabilities"`
		ExpiresAt        string   `json:"expiresAt"`
		IssuedAt         string   `json:"issuedAt"`
		SubjectDeviceID  string   `json:"subjectDeviceId"`
	} `json:"attestation"`
	KeyPop struct {
		APIVersion           string `json:"apiVersion"`
		Purpose              string `json:"purpose"`
		CeremonyID           string `json:"ceremonyId"`
		SubjectDeviceID      string `json:"subjectDeviceId"`
		StorageMode          string `json:"storageMode"`
		KeyEnvelopeAssertion struct {
			FormatVersion  int    `json:"formatVersion"`
			KeyRevision    int    `json:"keyRevision"`
			RecordRevision int    `json:"recordRevision"`
			SealedAt       string `json:"sealedAt"`
			Status         string `json:"status"`
		} `json:"keyEnvelopeAssertion"`
		Challenge string `json:"challengeHex"`
		ExpiresAt string `json:"expiresAt"`
	} `json:"keyPop"`
	KeyWrap struct {
		ExpiresAt string `json:"expiresAt"`
		KeyEpoch  string `json:"keyEpoch"`
		Purpose   string `json:"purpose"`
		SessionID string `json:"sessionId"`
		WrapID    string `json:"wrapId"`
	} `json:"keyWrap"`
	Payload struct {
		Direction string `json:"direction"`
		KeyEpoch  string `json:"keyEpoch"`
		Kind      string `json:"kind"`
		MessageID string `json:"messageId"`
		Plaintext string `json:"plaintextHex"`
		SentAt    string `json:"sentAt"`
		Sequence  string `json:"sequence"`
		StreamID  string `json:"streamId"`
	} `json:"payload"`
	PeerB struct {
		DeviceID  string `json:"deviceId"`
		MessageID string `json:"messageId"`
		Plaintext string `json:"plaintextHex"`
		SentAt    string `json:"sentAt"`
		Sequence  string `json:"sequence"`
	} `json:"peerB"`
	Rotation struct {
		KeyEpoch  string `json:"keyEpoch"`
		MessageID string `json:"messageId"`
		Plaintext string `json:"plaintextHex"`
		SentAt    string `json:"sentAt"`
		Sequence  string `json:"sequence"`
	} `json:"rotation"`
}

const maxSafeInteger = 9007199254740991

type deviceAttestationV1 struct {
	ApproverDeviceID         string   `json:"approverDeviceId"`
	AttestationID            string   `json:"attestationId"`
	Capabilities             []string `json:"capabilities"`
	ExpiresAt                string   `json:"expiresAt"`
	IssuedAt                 string   `json:"issuedAt"`
	SubjectDeviceID          string   `json:"subjectDeviceId"`
	SubjectExchangeKeyDigest string   `json:"subjectExchangeKeyDigest"`
	SubjectSigningKeyDigest  string   `json:"subjectSigningKeyDigest"`
	Type                     string   `json:"type"`
	Version                  int      `json:"version"`
}

var attestationMembers = map[string]bool{
	"approverDeviceId": true, "attestationId": true, "capabilities": true,
	"expiresAt": true, "issuedAt": true, "subjectDeviceId": true,
	"subjectExchangeKeyDigest": true, "subjectSigningKeyDigest": true,
	"type": true, "version": true,
}

var (
	uuidV7Pattern     = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	capabilityPattern = regexp.MustCompile(`^mad\.v[1-9][0-9]*\.[a-z][a-z0-9]*(?:\.[a-z][a-z0-9_]*)+$`)
	utcTimePattern    = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]{1,6})?Z$`)
)

func canonicalAttestation(raw []byte) ([]byte, deviceAttestationV1, error) {
	if hasUnpairedSurrogateEscape(raw) {
		return nil, deviceAttestationV1{}, errors.New("non-I-JSON surrogate")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, deviceAttestationV1{}, errors.New("attestation must be object")
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, deviceAttestationV1{}, err
		}
		name, ok := token.(string)
		if !ok || !attestationMembers[name] || seen[name] {
			return nil, deviceAttestationV1{}, errors.New("unknown or duplicate attestation member")
		}
		seen[name] = true
		var value any
		if err := decoder.Decode(&value); err != nil {
			return nil, deviceAttestationV1{}, err
		}
		if object, ok := value.(map[string]any); ok && object != nil {
			return nil, deviceAttestationV1{}, errors.New("arbitrary object forbidden")
		}
	}
	if _, err := decoder.Token(); err != nil || len(seen) != len(attestationMembers) {
		return nil, deviceAttestationV1{}, errors.New("incomplete attestation")
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, deviceAttestationV1{}, errors.New("trailing attestation data")
	}
	var attestation deviceAttestationV1
	strict := json.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&attestation); err != nil {
		return nil, deviceAttestationV1{}, err
	}
	if err := strict.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, deviceAttestationV1{}, errors.New("trailing attestation data")
	}
	if attestation.Version != 1 || attestation.Version > maxSafeInteger || attestation.Type != "device_attestation" {
		return nil, deviceAttestationV1{}, errors.New("invalid attestation version/type")
	}
	for _, id := range []string{attestation.ApproverDeviceID, attestation.AttestationID, attestation.SubjectDeviceID} {
		if !uuidV7Pattern.MatchString(id) {
			return nil, deviceAttestationV1{}, errors.New("invalid attestation UUIDv7")
		}
	}
	if !utcTimePattern.MatchString(attestation.IssuedAt) || !utcTimePattern.MatchString(attestation.ExpiresAt) {
		return nil, deviceAttestationV1{}, errors.New("invalid attestation timestamp")
	}
	issuedAt, issuedErr := time.Parse(time.RFC3339Nano, attestation.IssuedAt)
	expiresAt, expiresErr := time.Parse(time.RFC3339Nano, attestation.ExpiresAt)
	if issuedErr != nil || expiresErr != nil || !expiresAt.After(issuedAt) || expiresAt.Sub(issuedAt) > 10*time.Minute {
		return nil, deviceAttestationV1{}, errors.New("invalid attestation lifetime")
	}
	if len(attestation.Capabilities) == 0 || !slices.IsSorted(attestation.Capabilities) {
		return nil, deviceAttestationV1{}, errors.New("capabilities not canonical")
	}
	for index := 1; index < len(attestation.Capabilities); index++ {
		if attestation.Capabilities[index-1] == attestation.Capabilities[index] {
			return nil, deviceAttestationV1{}, errors.New("duplicate capability")
		}
	}
	for _, capability := range attestation.Capabilities {
		if !capabilityPattern.MatchString(capability) {
			return nil, deviceAttestationV1{}, errors.New("invalid capability")
		}
	}
	for _, encoded := range []string{attestation.SubjectExchangeKeyDigest, attestation.SubjectSigningKeyDigest} {
		decoded, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil || len(decoded) != sha256.Size || base64.RawURLEncoding.EncodeToString(decoded) != encoded {
			return nil, deviceAttestationV1{}, errors.New("invalid key digest")
		}
	}
	canonical, err := json.Marshal(attestation)
	return canonical, attestation, err
}

func hasUnpairedSurrogateEscape(raw []byte) bool {
	for index := 0; index+5 < len(raw); index++ {
		if raw[index] != '\\' || raw[index+1] != 'u' {
			continue
		}
		precedingSlashes := 0
		for prior := index - 1; prior >= 0 && raw[prior] == '\\'; prior-- {
			precedingSlashes++
		}
		if precedingSlashes%2 == 1 {
			continue
		}
		codeBytes, err := hex.DecodeString(string(raw[index+2 : index+6]))
		if err != nil || len(codeBytes) != 2 {
			continue
		}
		code := binary.BigEndian.Uint16(codeBytes)
		if code >= 0xdc00 && code <= 0xdfff {
			return true
		}
		if code < 0xd800 || code > 0xdbff {
			continue
		}
		if index+11 >= len(raw) || raw[index+6] != '\\' || raw[index+7] != 'u' {
			return true
		}
		lowBytes, err := hex.DecodeString(string(raw[index+8 : index+12]))
		if err != nil || len(lowBytes) != 2 {
			return true
		}
		low := binary.BigEndian.Uint16(lowBytes)
		if low < 0xdc00 || low > 0xdfff {
			return true
		}
		index += 6
	}
	return false
}

func base32Fingerprint(pinDigest []byte) string {
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(pinDigest[:15])
	groups := make([]string, 0, 6)
	for offset := 0; offset < len(encoded); offset += 4 {
		groups = append(groups, encoded[offset:offset+4])
	}
	return strings.Join(groups, "-")
}

func attestationKeyDigestsMatch(attestation deviceAttestationV1, signingPublicKey, exchangePublicKey []byte) bool {
	signingDigest, signingErr := base64.RawURLEncoding.DecodeString(attestation.SubjectSigningKeyDigest)
	exchangeDigest, exchangeErr := base64.RawURLEncoding.DecodeString(attestation.SubjectExchangeKeyDigest)
	expectedSigningDigest := sha256Bytes(signingPublicKey)
	expectedExchangeDigest := sha256Bytes(exchangePublicKey)
	return signingErr == nil && exchangeErr == nil &&
		subtle.ConstantTimeCompare(signingDigest, expectedSigningDigest) == 1 &&
		subtle.ConstantTimeCompare(exchangeDigest, expectedExchangeDigest) == 1
}

func parseBase32Fingerprint(value string) ([]byte, error) {
	if len(value) == 29 {
		for _, offset := range []int{4, 9, 14, 19, 24} {
			if value[offset] != '-' {
				return nil, errors.New("invalid fingerprint grouping")
			}
		}
		value = strings.ReplaceAll(value, "-", "")
	} else if len(value) != 24 {
		return nil, errors.New("invalid fingerprint length")
	}
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(value))
	if err != nil || len(decoded) != 15 {
		return nil, errors.New("invalid fingerprint encoding")
	}
	return decoded, nil
}

func fingerprintMatches(value string, pinDigest []byte) bool {
	decoded, err := parseBase32Fingerprint(value)
	return err == nil && subtle.ConstantTimeCompare(decoded, pinDigest[:15]) == 1
}

func mustHex(value string) []byte {
	b, err := hex.DecodeString(value)
	if err != nil {
		panic(err)
	}
	return b
}

func b64url(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}

func canonical(value any) []byte {
	b, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return b
}

func framed(parts ...[]byte) []byte {
	var out bytes.Buffer
	for _, part := range parts {
		if len(part) > int(^uint32(0)) {
			panic("frame part too large")
		}
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(len(part)))
		out.Write(size[:])
		out.Write(part)
	}
	return out.Bytes()
}

func digest(parts ...[]byte) []byte {
	sum := sha256.Sum256(framed(parts...))
	return sum[:]
}

func sha256Bytes(value []byte) []byte {
	sum := sha256.Sum256(value)
	return sum[:]
}

type popFields struct {
	APIVersion               string
	Purpose                  string
	CeremonyID               string
	SubjectDeviceID          string
	SubjectSigningPublicKey  []byte
	SubjectExchangePublicKey []byte
	StorageMode              string
	StorageAssertionDigest   []byte
	ServerEphemeralPublicKey []byte
	Challenge                []byte
	ExpiresAt                string
}

func popContext(fields popFields) []byte {
	return framed(
		[]byte("multidesk-x25519-pop-context-v1"), []byte(fields.APIVersion),
		[]byte(fields.Purpose), []byte(fields.CeremonyID), []byte(fields.SubjectDeviceID),
		fields.SubjectSigningPublicKey, fields.SubjectExchangePublicKey,
		[]byte(fields.StorageMode), fields.StorageAssertionDigest,
		fields.ServerEphemeralPublicKey, fields.Challenge, []byte(fields.ExpiresAt),
	)
}

func popProofs(subjectExchangePrivate *ecdh.PrivateKey, subjectSigningPrivate ed25519.PrivateKey, fields popFields) ([]byte, []byte, []byte, []byte, []byte, error) {
	serverPublic, err := ecdh.X25519().NewPublicKey(fields.ServerEphemeralPublicKey)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	sharedSecret, err := subjectExchangePrivate.ECDH(serverPublic)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	context := popContext(fields)
	salt := digest([]byte("multidesk-x25519-pop-salt-v1"), []byte(fields.CeremonyID), fields.Challenge)
	key, err := hkdf.Key(sha256.New, sharedSecret, salt, string(context), 32)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(framed([]byte("multidesk-x25519-pop-proof-v1"), context))
	exchangeProof := mac.Sum(nil)
	signingProof := ed25519.Sign(subjectSigningPrivate,
		framed([]byte("multidesk-ed25519-pop-proof-v1"), context))
	return context, sharedSecret, key, exchangeProof, signingProof, nil
}

func verifyPop(serverPrivate *ecdh.PrivateKey, subjectSigningPublic ed25519.PublicKey, fields popFields, exchangeProof, signingProof []byte) bool {
	subjectPublic, err := ecdh.X25519().NewPublicKey(fields.SubjectExchangePublicKey)
	if err != nil {
		return false
	}
	sharedSecret, err := serverPrivate.ECDH(subjectPublic)
	if err != nil {
		return false
	}
	context := popContext(fields)
	salt := digest([]byte("multidesk-x25519-pop-salt-v1"), []byte(fields.CeremonyID), fields.Challenge)
	key, err := hkdf.Key(sha256.New, sharedSecret, salt, string(context), 32)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(framed([]byte("multidesk-x25519-pop-proof-v1"), context))
	expectedExchangeProof := mac.Sum(nil)
	return hmac.Equal(expectedExchangeProof, exchangeProof) && ed25519.Verify(
		subjectSigningPublic,
		framed([]byte("multidesk-ed25519-pop-proof-v1"), context),
		signingProof,
	)
}

func rawPublic(key interface{ MarshalBinary() ([]byte, error) }) []byte {
	b, err := key.MarshalBinary()
	if err != nil {
		panic(err)
	}
	return b
}

func deriveTraffic(root []byte, sessionID, keyEpoch, sourceID, targetID, direction, streamID string) ([]byte, []byte, []byte) {
	context := map[string]any{
		"direction":      direction,
		"keyEpoch":       keyEpoch,
		"purpose":        "session_traffic",
		"sessionId":      sessionID,
		"sourceDeviceId": sourceID,
		"streamId":       streamID,
		"targetDeviceId": targetID,
		"version":        1,
	}
	contextBytes := canonical(context)
	salt := digest([]byte("multidesk-session-traffic-salt-v1"), []byte(sessionID), []byte(keyEpoch))
	info := framed([]byte("multidesk-session-traffic-info-v1"), contextBytes)
	material, err := hkdf.Key(sha256.New, root, salt, string(info), 48)
	if err != nil {
		panic(err)
	}
	return material[:32], material[32:], contextBytes
}

func makeNonce(prefix []byte, sequence string) []byte {
	seq, err := strconv.ParseUint(sequence, 10, 64)
	if err != nil {
		panic(err)
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	copy(nonce, prefix)
	binary.BigEndian.PutUint64(nonce[16:], seq)
	return nonce
}

type replayWindow struct {
	initialized bool
	high        uint64
	seen        uint64
}

func (w *replayWindow) accept(sequence uint64) bool {
	if !w.initialized {
		w.initialized = true
		w.high = sequence
		w.seen = 1
		return true
	}
	if sequence > w.high {
		delta := sequence - w.high
		if delta >= 64 {
			w.seen = 1
		} else {
			w.seen = (w.seen << delta) | 1
		}
		w.high = sequence
		return true
	}
	delta := w.high - sequence
	if delta >= 64 {
		return false
	}
	mask := uint64(1) << delta
	if w.seen&mask != 0 {
		return false
	}
	w.seen |= mask
	return true
}

func openX(key, nonce, ciphertext, aad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, ciphertext, aad)
}

func run(input vectorInput) (map[string]any, error) {
	if input.SchemaVersion != 1 {
		return nil, errors.New("unsupported vector schema")
	}

	approverPrivate := ed25519.NewKeyFromSeed(mustHex(input.Seeds.ApproverEd25519))
	approverPublic := approverPrivate.Public().(ed25519.PublicKey)
	subjectPrivate := ed25519.NewKeyFromSeed(mustHex(input.Seeds.SubjectEd25519))
	subjectPublic := subjectPrivate.Public().(ed25519.PublicKey)

	scheme := hpke.KEM_X25519_HKDF_SHA256.Scheme()
	sourcePublic, sourcePrivate := scheme.DeriveKeyPair(mustHex(input.Seeds.SourceX25519IKM))
	targetPublic, targetPrivate := scheme.DeriveKeyPair(mustHex(input.Seeds.TargetX25519IKM))
	peerBPublic, _ := scheme.DeriveKeyPair(mustHex(input.Seeds.PeerBX25519IKM))
	sourcePublicRaw := rawPublic(sourcePublic)
	targetPublicRaw := rawPublic(targetPublic)
	peerBPublicRaw := rawPublic(peerBPublic)
	targetPrivateRaw, err := targetPrivate.MarshalBinary()
	if err != nil {
		return nil, err
	}
	curve := ecdh.X25519()
	subjectExchangePrivate, err := curve.NewPrivateKey(targetPrivateRaw)
	if err != nil {
		return nil, err
	}
	serverPopPrivate, err := curve.NewPrivateKey(mustHex(input.Seeds.ServerPopX25519Private))
	if err != nil {
		return nil, err
	}
	restartPopPrivate, err := curve.NewPrivateKey(mustHex(input.Seeds.RestartPopX25519Private))
	if err != nil {
		return nil, err
	}

	subjectSigningDigest := sha256Bytes(subjectPublic)
	subjectExchangeDigest := sha256Bytes(targetPublicRaw)
	attestationValue := deviceAttestationV1{
		ApproverDeviceID:         input.Attestation.ApproverDeviceID,
		AttestationID:            input.Attestation.AttestationID,
		Capabilities:             input.Attestation.Capabilities,
		ExpiresAt:                input.Attestation.ExpiresAt,
		IssuedAt:                 input.Attestation.IssuedAt,
		SubjectDeviceID:          input.Attestation.SubjectDeviceID,
		SubjectExchangeKeyDigest: b64url(subjectExchangeDigest),
		SubjectSigningKeyDigest:  b64url(subjectSigningDigest),
		Type:                     "device_attestation",
		Version:                  1,
	}
	attestationRaw, err := json.Marshal(attestationValue)
	if err != nil {
		return nil, err
	}
	attestationCanonical, _, err := canonicalAttestation(attestationRaw)
	if err != nil {
		return nil, err
	}
	attestationMessage := framed([]byte("multidesk-device-attestation-v1"), attestationCanonical)
	attestationSignature := ed25519.Sign(approverPrivate, attestationMessage)
	attestationMutation := append([]byte(nil), attestationMessage...)
	attestationMutation[len(attestationMutation)-2] ^= 1
	attestationMutationRejected := !ed25519.Verify(approverPublic, attestationMutation, attestationSignature)
	attestationChangeRejected := func(mutate func(*deviceAttestationV1)) bool {
		candidate := attestationValue
		candidate.Capabilities = slices.Clone(attestationValue.Capabilities)
		mutate(&candidate)
		raw, marshalErr := json.Marshal(candidate)
		if marshalErr != nil {
			return true
		}
		canonicalCandidate, _, canonicalErr := canonicalAttestation(raw)
		if canonicalErr != nil {
			return true
		}
		return !ed25519.Verify(approverPublic,
			framed([]byte("multidesk-device-attestation-v1"), canonicalCandidate),
			attestationSignature)
	}
	exchangeDigestMutationRejected := attestationChangeRejected(func(value *deviceAttestationV1) {
		value.SubjectExchangeKeyDigest = b64url(make([]byte, 32))
	})
	signingDigestMutationRejected := attestationChangeRejected(func(value *deviceAttestationV1) {
		value.SubjectSigningKeyDigest = b64url(make([]byte, 32))
	})
	capabilityMutationRejected := attestationChangeRejected(func(value *deviceAttestationV1) {
		value.Capabilities[0] = "mad.v1.device.revoke"
		slices.Sort(value.Capabilities)
	})
	attestationExpiryMutationRejected := attestationChangeRejected(func(value *deviceAttestationV1) {
		value.ExpiresAt = "2026-07-15T00:00:01Z"
	})
	attestationIDMutationRejected := attestationChangeRejected(func(value *deviceAttestationV1) {
		value.AttestationID = input.KeyPop.CeremonyID
	})
	duplicateRaw := append(append([]byte(nil), attestationRaw[:len(attestationRaw)-1]...), []byte(`,"version":1}`)...)
	_, _, duplicateErr := canonicalAttestation(duplicateRaw)
	unknownRaw := append(append([]byte(nil), attestationRaw[:len(attestationRaw)-1]...), []byte(`,"unknown":true}`)...)
	_, _, unknownErr := canonicalAttestation(unknownRaw)
	floatRaw := bytes.Replace(attestationRaw, []byte(`"version":1`), []byte(`"version":1.5`), 1)
	_, _, floatErr := canonicalAttestation(floatRaw)
	mapRaw := bytes.Replace(attestationRaw, []byte(`"capabilities":[`), []byte(`"capabilities":{"value":[`), 1)
	mapRaw = bytes.Replace(mapRaw, []byte(`],"expiresAt"`), []byte(`]},"expiresAt"`), 1)
	_, _, mapErr := canonicalAttestation(mapRaw)
	escapedRaw := bytes.Replace(attestationRaw, []byte(`device_attestation`), []byte(`device\u005fattestation`), 1)
	escapedCanonical, _, escapedErr := canonicalAttestation(escapedRaw)
	unsafeIntegerRaw := bytes.Replace(attestationRaw, []byte(`"version":1`), []byte(`"version":9007199254740992`), 1)
	_, _, unsafeIntegerErr := canonicalAttestation(unsafeIntegerRaw)
	negativeIntegerRaw := bytes.Replace(attestationRaw, []byte(`"version":1`), []byte(`"version":-1`), 1)
	_, _, negativeIntegerErr := canonicalAttestation(negativeIntegerRaw)
	invalidCapabilityRaw := bytes.Replace(attestationRaw, []byte(`mad.v1.metadata.read`), []byte(`MAD.invalid`), 1)
	_, _, invalidCapabilityErr := canonicalAttestation(invalidCapabilityRaw)
	invalidIDRaw := bytes.Replace(attestationRaw, []byte(input.Attestation.AttestationID), []byte(`018f47d2-7c11-6d3f-a9b8-1f6d83de1003`), 1)
	_, _, invalidIDErr := canonicalAttestation(invalidIDRaw)
	invalidLifetimeRaw := bytes.Replace(attestationRaw, []byte(input.Attestation.ExpiresAt), []byte(`2026-07-14T16:10:00.0000001Z`), 1)
	_, _, invalidLifetimeErr := canonicalAttestation(invalidLifetimeRaw)
	invalidCalendarRaw := bytes.Replace(attestationRaw, []byte(input.Attestation.IssuedAt), []byte(`2026-02-30T16:00:00Z`), 1)
	invalidCalendarRaw = bytes.Replace(invalidCalendarRaw, []byte(input.Attestation.ExpiresAt), []byte(`2026-02-30T16:10:00Z`), 1)
	_, _, invalidCalendarErr := canonicalAttestation(invalidCalendarRaw)
	invalidHour24Raw := bytes.Replace(attestationRaw, []byte(input.Attestation.IssuedAt), []byte(`2026-07-14T24:00:00Z`), 1)
	invalidHour24Raw = bytes.Replace(invalidHour24Raw, []byte(input.Attestation.ExpiresAt), []byte(`2026-07-14T24:10:00Z`), 1)
	_, _, invalidHour24Err := canonicalAttestation(invalidHour24Raw)
	leapDayBoundaryRaw := bytes.Replace(attestationRaw, []byte(input.Attestation.IssuedAt), []byte(`2024-02-29T23:59:59.999999Z`), 1)
	leapDayBoundaryRaw = bytes.Replace(leapDayBoundaryRaw, []byte(input.Attestation.ExpiresAt), []byte(`2024-03-01T00:00:00Z`), 1)
	_, _, leapDayBoundaryErr := canonicalAttestation(leapDayBoundaryRaw)
	unicodeRaw := bytes.Replace(attestationRaw, []byte(`device_attestation`), []byte(`\ud800`), 1)
	_, _, unicodeErr := canonicalAttestation(unicodeRaw)
	var reorderedMap map[string]any
	if err := json.Unmarshal(attestationRaw, &reorderedMap); err != nil {
		return nil, err
	}
	reorderedRaw, err := json.Marshal(reorderedMap)
	if err != nil {
		return nil, err
	}
	reorderedCanonical, _, reorderedErr := canonicalAttestation(reorderedRaw)
	attestationKeyDigestsVerify := attestationKeyDigestsMatch(attestationValue, subjectPublic, targetPublicRaw)
	mutatedSigningPublic := slices.Clone(subjectPublic)
	mutatedSigningPublic[0] ^= 1
	mutatedExchangePublic := slices.Clone(targetPublicRaw)
	mutatedExchangePublic[0] ^= 1
	signingKeyDigestMismatchRejected := !attestationKeyDigestsMatch(attestationValue, mutatedSigningPublic, targetPublicRaw)
	exchangeKeyDigestMismatchRejected := !attestationKeyDigestsMatch(attestationValue, subjectPublic, mutatedExchangePublic)

	pinDigest := digest(
		[]byte("multidesk-device-pin-v1"),
		[]byte(input.Attestation.SubjectDeviceID),
		subjectPublic,
		targetPublicRaw,
	)
	fingerprint := base32Fingerprint(pinDigest)
	fingerprintLowercaseAccepted := fingerprintMatches(strings.ToLower(fingerprint), pinDigest)
	fingerprintUnhyphenatedAccepted := fingerprintMatches(strings.ReplaceAll(fingerprint, "-", ""), pinDigest)
	alteredFingerprint := []byte(fingerprint)
	if alteredFingerprint[0] == 'A' {
		alteredFingerprint[0] = 'B'
	} else {
		alteredFingerprint[0] = 'A'
	}
	fingerprintAlteredGroupRejected := !fingerprintMatches(string(alteredFingerprint), pinDigest)
	fingerprintLength23Rejected := !fingerprintMatches(strings.ReplaceAll(fingerprint, "-", "")[:23], pinDigest)
	fingerprintLength25Rejected := !fingerprintMatches(strings.ReplaceAll(fingerprint, "-", "")+"A", pinDigest)
	fingerprintInvalidBase32Rejected := !fingerprintMatches("0000-0000-0000-0000-0000-0000", pinDigest)
	oldHexFingerprint := fmt.Sprintf("%x-%x-%x-%x-%x-%x-%x-%x", pinDigest[0:4], pinDigest[4:8], pinDigest[8:12], pinDigest[12:16], pinDigest[16:20], pinDigest[20:24], pinDigest[24:28], pinDigest[28:32])
	fingerprintOldHexRejected := !fingerprintMatches(oldHexFingerprint, pinDigest)
	fingerprintTruncatedAsFullDigestRejected := len(pinDigest[:15]) != sha256.Size

	keyEnvelopeAssertionCanonical := canonical(input.KeyPop.KeyEnvelopeAssertion)
	storageAssertionDigest := sha256Bytes(keyEnvelopeAssertionCanonical)
	challenge := mustHex(input.KeyPop.Challenge)
	popFieldsValue := popFields{
		APIVersion:               input.KeyPop.APIVersion,
		Purpose:                  input.KeyPop.Purpose,
		CeremonyID:               input.KeyPop.CeremonyID,
		SubjectDeviceID:          input.KeyPop.SubjectDeviceID,
		SubjectSigningPublicKey:  subjectPublic,
		SubjectExchangePublicKey: targetPublicRaw,
		StorageMode:              input.KeyPop.StorageMode,
		StorageAssertionDigest:   storageAssertionDigest,
		ServerEphemeralPublicKey: serverPopPrivate.PublicKey().Bytes(),
		Challenge:                challenge,
		ExpiresAt:                input.KeyPop.ExpiresAt,
	}
	popContextBytes, popSharedSecret, popKey, exchangeProof, signingProof, err := popProofs(
		subjectExchangePrivate, subjectPrivate, popFieldsValue,
	)
	if err != nil {
		return nil, err
	}
	popVerifies := verifyPop(serverPopPrivate, subjectPublic, popFieldsValue, exchangeProof, signingProof)
	mutatedExchangeProof := slices.Clone(exchangeProof)
	mutatedExchangeProof[0] ^= 1
	exchangeProofContentMutationRejected := !verifyPop(serverPopPrivate, subjectPublic, popFieldsValue, mutatedExchangeProof, signingProof)
	exchangeProofShortRejected := !verifyPop(serverPopPrivate, subjectPublic, popFieldsValue, exchangeProof[:len(exchangeProof)-1], signingProof)
	exchangeProofLongRejected := !verifyPop(serverPopPrivate, subjectPublic, popFieldsValue, append(slices.Clone(exchangeProof), 0), signingProof)
	mutatePop := func(mutate func(*popFields)) bool {
		candidate := popFieldsValue
		candidate.SubjectSigningPublicKey = slices.Clone(popFieldsValue.SubjectSigningPublicKey)
		candidate.SubjectExchangePublicKey = slices.Clone(popFieldsValue.SubjectExchangePublicKey)
		candidate.StorageAssertionDigest = slices.Clone(popFieldsValue.StorageAssertionDigest)
		candidate.ServerEphemeralPublicKey = slices.Clone(popFieldsValue.ServerEphemeralPublicKey)
		candidate.Challenge = slices.Clone(popFieldsValue.Challenge)
		mutate(&candidate)
		return !verifyPop(serverPopPrivate, subjectPublic, candidate, exchangeProof, signingProof)
	}
	storageModeMutationRejected := mutatePop(func(fields *popFields) { fields.StorageMode = "native" })
	storageAssertionMutationRejected := mutatePop(func(fields *popFields) { fields.StorageAssertionDigest[0] ^= 1 })
	purposeMutationRejected := mutatePop(func(fields *popFields) { fields.Purpose = "bootstrap" })
	ceremonyMutationRejected := mutatePop(func(fields *popFields) { fields.CeremonyID = input.Attestation.AttestationID })
	deviceMutationRejected := mutatePop(func(fields *popFields) { fields.SubjectDeviceID = input.Attestation.ApproverDeviceID })
	signingKeyMutationRejected := mutatePop(func(fields *popFields) { fields.SubjectSigningPublicKey[0] ^= 1 })
	exchangeKeyMutationRejected := mutatePop(func(fields *popFields) { fields.SubjectExchangePublicKey[0] ^= 1 })
	challengeMutationRejected := mutatePop(func(fields *popFields) { fields.Challenge[0] ^= 1 })
	expiryMutationRejected := mutatePop(func(fields *popFields) { fields.ExpiresAt = "2026-07-14T16:09:59Z" })
	serverEphemeralMutationRejected := mutatePop(func(fields *popFields) {
		fields.ServerEphemeralPublicKey = restartPopPrivate.PublicKey().Bytes()
	})
	zeroPublic, allZeroRejectedErr := curve.NewPublicKey(make([]byte, 32))
	allZeroRejected := allZeroRejectedErr != nil
	if allZeroRejectedErr == nil {
		_, allZeroRejectedErr = serverPopPrivate.ECDH(zeroPublic)
		allZeroRejected = allZeroRejectedErr != nil
	}
	restartFields := popFieldsValue
	restartFields.ServerEphemeralPublicKey = restartPopPrivate.PublicKey().Bytes()
	restartInvalidated := !verifyPop(restartPopPrivate, subjectPublic, restartFields, exchangeProof, signingProof)
	consumed := false
	verifyOnce := func() bool {
		if consumed || !verifyPop(serverPopPrivate, subjectPublic, popFieldsValue, exchangeProof, signingProof) {
			return false
		}
		consumed = true
		return true
	}
	firstConsumeAccepted := verifyOnce()
	replayRejected := !verifyOnce()

	sourceDigest := sha256.Sum256(sourcePublicRaw)
	targetDigest := sha256.Sum256(targetPublicRaw)
	wrapBase := map[string]any{
		"expiresAt":               input.KeyWrap.ExpiresAt,
		"keyEpoch":                input.KeyWrap.KeyEpoch,
		"purpose":                 input.KeyWrap.Purpose,
		"sessionId":               input.KeyWrap.SessionID,
		"sourceDeviceId":          input.Attestation.ApproverDeviceID,
		"sourceExchangeKeyDigest": b64url(sourceDigest[:]),
		"targetDeviceId":          input.Attestation.SubjectDeviceID,
		"targetExchangeKeyDigest": b64url(targetDigest[:]),
		"type":                    "session_key_wrap",
		"version":                 1,
		"wrapId":                  input.KeyWrap.WrapID,
	}
	wrapBaseCanonical := canonical(wrapBase)
	hpkeInfo := digest([]byte("multidesk-hpke-session-wrap-info-v1"), wrapBaseCanonical)
	suite := hpke.NewSuite(
		hpke.KEM_X25519_HKDF_SHA256,
		hpke.KDF_HKDF_SHA256,
		hpke.AEAD_ChaCha20Poly1305,
	)
	sender, err := suite.NewSender(targetPublic, hpkeInfo)
	if err != nil {
		return nil, err
	}
	enc, sealer, err := sender.SetupAuth(bytes.NewReader(mustHex(input.Seeds.EphemeralX25519IKM)), sourcePrivate)
	if err != nil {
		return nil, err
	}
	wrapHeader := make(map[string]any, len(wrapBase)+2)
	for key, value := range wrapBase {
		wrapHeader[key] = value
	}
	wrapHeader["enc"] = b64url(enc)
	wrapHeader["hpkeSuite"] = hpkeSuiteName
	wrapAAD := canonical(wrapHeader)
	pairwiseRootPeerA1 := mustHex(input.Seeds.PairwiseRootPeerAEpoch1)
	wrappedKey, err := sealer.Seal(pairwiseRootPeerA1, wrapAAD)
	if err != nil {
		return nil, err
	}
	receiver, err := suite.NewReceiver(targetPrivate, hpkeInfo)
	if err != nil {
		return nil, err
	}
	opener, err := receiver.SetupAuth(enc, sourcePublic)
	if err != nil {
		return nil, err
	}
	recoveredKey, err := opener.Open(wrappedKey, wrapAAD)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(recoveredKey, pairwiseRootPeerA1) {
		return nil, errors.New("HPKE recovered key mismatch")
	}
	mutatedWrapHeader := make(map[string]any, len(wrapHeader))
	for key, value := range wrapHeader {
		mutatedWrapHeader[key] = value
	}
	mutatedWrapHeader["targetDeviceId"] = input.Attestation.ApproverDeviceID
	mutatedWrapAAD := canonical(mutatedWrapHeader)
	receiverMutated, _ := suite.NewReceiver(targetPrivate, hpkeInfo)
	openerMutated, err := receiverMutated.SetupAuth(enc, sourcePublic)
	if err != nil {
		return nil, err
	}
	_, err = openerMutated.Open(wrappedKey, mutatedWrapAAD)
	wrapAADMutationRejected := err != nil
	receiverWrongSender, _ := suite.NewReceiver(targetPrivate, hpkeInfo)
	openerWrongSender, err := receiverWrongSender.SetupAuth(enc, targetPublic)
	wrongPinnedSenderRejected := err != nil
	if err == nil {
		_, err = openerWrongSender.Open(wrappedKey, wrapAAD)
		wrongPinnedSenderRejected = err != nil
	}

	trafficKey1, noncePrefix1, trafficContext1 := deriveTraffic(
		pairwiseRootPeerA1,
		input.KeyWrap.SessionID,
		input.Payload.KeyEpoch,
		input.Attestation.ApproverDeviceID,
		input.Attestation.SubjectDeviceID,
		input.Payload.Direction,
		input.Payload.StreamID,
	)
	nonce1 := makeNonce(noncePrefix1, input.Payload.Sequence)
	payloadHeader := map[string]any{
		"direction":      input.Payload.Direction,
		"keyEpoch":       input.Payload.KeyEpoch,
		"kind":           input.Payload.Kind,
		"messageId":      input.Payload.MessageID,
		"nonce":          b64url(nonce1),
		"sentAt":         input.Payload.SentAt,
		"sequence":       input.Payload.Sequence,
		"sessionId":      input.KeyWrap.SessionID,
		"sourceDeviceId": input.Attestation.ApproverDeviceID,
		"streamId":       input.Payload.StreamID,
		"targetDeviceId": input.Attestation.SubjectDeviceID,
		"type":           "session_envelope",
		"version":        1,
	}
	payloadAAD := canonical(payloadHeader)
	payloadPlaintext := mustHex(input.Payload.Plaintext)
	payloadAEAD, err := chacha20poly1305.NewX(trafficKey1)
	if err != nil {
		return nil, err
	}
	payloadCiphertext := payloadAEAD.Seal(nil, nonce1, payloadPlaintext, payloadAAD)
	payloadRecovered, err := openX(trafficKey1, nonce1, payloadCiphertext, payloadAAD)
	if err != nil || !bytes.Equal(payloadRecovered, payloadPlaintext) {
		return nil, errors.New("payload round trip failed")
	}
	mutatedPayloadHeader := make(map[string]any, len(payloadHeader))
	for key, value := range payloadHeader {
		mutatedPayloadHeader[key] = value
	}
	mutatedPayloadHeader["kind"] = "approval_request"
	_, err = openX(trafficKey1, nonce1, payloadCiphertext, canonical(mutatedPayloadHeader))
	payloadAADMutationRejected := err != nil
	badNonce := makeNonce(noncePrefix1, "101")
	badNonceHeader := make(map[string]any, len(payloadHeader))
	for key, value := range payloadHeader {
		badNonceHeader[key] = value
	}
	badNonceHeader["nonce"] = b64url(badNonce)
	badNonceAAD := canonical(badNonceHeader)
	badNonceCiphertext := payloadAEAD.Seal(nil, badNonce, payloadPlaintext, badNonceAAD)
	_, err = openX(trafficKey1, nonce1, badNonceCiphertext, badNonceAAD)
	nonceSequenceMismatchRejected := !bytes.Equal(badNonce, nonce1) && err != nil

	pairwiseRootPeerB1 := mustHex(input.Seeds.PairwiseRootPeerBEpoch1)
	peerBTrafficKey, peerBNoncePrefix, peerBTrafficContext := deriveTraffic(
		pairwiseRootPeerB1,
		input.KeyWrap.SessionID,
		input.Payload.KeyEpoch,
		input.Attestation.ApproverDeviceID,
		input.PeerB.DeviceID,
		input.Payload.Direction,
		input.Payload.StreamID,
	)
	peerBNonce := makeNonce(peerBNoncePrefix, input.PeerB.Sequence)
	peerBHeader := map[string]any{
		"direction":      input.Payload.Direction,
		"keyEpoch":       input.Payload.KeyEpoch,
		"kind":           input.Payload.Kind,
		"messageId":      input.PeerB.MessageID,
		"nonce":          b64url(peerBNonce),
		"sentAt":         input.PeerB.SentAt,
		"sequence":       input.PeerB.Sequence,
		"sessionId":      input.KeyWrap.SessionID,
		"sourceDeviceId": input.Attestation.ApproverDeviceID,
		"streamId":       input.Payload.StreamID,
		"targetDeviceId": input.PeerB.DeviceID,
		"type":           "session_envelope",
		"version":        1,
	}
	peerBAAD := canonical(peerBHeader)
	peerBPlaintext := mustHex(input.PeerB.Plaintext)
	peerBAEAD, _ := chacha20poly1305.NewX(peerBTrafficKey)
	peerBCiphertext := peerBAEAD.Seal(nil, peerBNonce, peerBPlaintext, peerBAAD)
	_, err = openX(trafficKey1, peerBNonce, peerBCiphertext, peerBAAD)
	peerAOpenPeerBRejected := err != nil

	forgeDirection := "client_to_device"
	forgeStream := "control"
	forgeSequence := "1"
	attackerKey, attackerNoncePrefix, _ := deriveTraffic(
		pairwiseRootPeerA1,
		input.KeyWrap.SessionID,
		input.Payload.KeyEpoch,
		input.PeerB.DeviceID,
		input.Attestation.ApproverDeviceID,
		forgeDirection,
		forgeStream,
	)
	expectedPeerBKey, expectedPeerBNoncePrefix, _ := deriveTraffic(
		pairwiseRootPeerB1,
		input.KeyWrap.SessionID,
		input.Payload.KeyEpoch,
		input.PeerB.DeviceID,
		input.Attestation.ApproverDeviceID,
		forgeDirection,
		forgeStream,
	)
	attackerNonce := makeNonce(attackerNoncePrefix, forgeSequence)
	expectedPeerBNonce := makeNonce(expectedPeerBNoncePrefix, forgeSequence)
	forgeHeader := map[string]any{
		"direction":      forgeDirection,
		"keyEpoch":       input.Payload.KeyEpoch,
		"kind":           "control_input",
		"messageId":      "018f47d2-7c11-7d3f-a9b8-1f6d83de3004",
		"nonce":          b64url(attackerNonce),
		"sentAt":         input.PeerB.SentAt,
		"sequence":       forgeSequence,
		"sessionId":      input.KeyWrap.SessionID,
		"sourceDeviceId": input.PeerB.DeviceID,
		"streamId":       forgeStream,
		"targetDeviceId": input.Attestation.ApproverDeviceID,
		"type":           "session_envelope",
		"version":        1,
	}
	forgeAAD := canonical(forgeHeader)
	attackerAEAD, _ := chacha20poly1305.NewX(attackerKey)
	forgeCiphertext := attackerAEAD.Seal(nil, attackerNonce, []byte("forged control"), forgeAAD)
	_, err = openX(expectedPeerBKey, expectedPeerBNonce, forgeCiphertext, forgeAAD)
	peerAForgeryRejected := !bytes.Equal(attackerNonce, expectedPeerBNonce) && err != nil

	replay := replayWindow{}
	replaySequence := []uint64{100, 98, 99, 100, 36}
	replayVerdicts := make([]string, 0, len(replaySequence))
	for _, sequence := range replaySequence {
		if replay.accept(sequence) {
			replayVerdicts = append(replayVerdicts, "accept")
		} else {
			replayVerdicts = append(replayVerdicts, "reject")
		}
	}

	pairwiseRootPeerA2 := mustHex(input.Seeds.PairwiseRootPeerAEpoch2)
	trafficKey2, noncePrefix2, trafficContext2 := deriveTraffic(
		pairwiseRootPeerA2,
		input.KeyWrap.SessionID,
		input.Rotation.KeyEpoch,
		input.Attestation.ApproverDeviceID,
		input.Attestation.SubjectDeviceID,
		input.Payload.Direction,
		input.Payload.StreamID,
	)
	nonce2 := makeNonce(noncePrefix2, input.Rotation.Sequence)
	rotationHeader := map[string]any{
		"direction":      input.Payload.Direction,
		"keyEpoch":       input.Rotation.KeyEpoch,
		"kind":           input.Payload.Kind,
		"messageId":      input.Rotation.MessageID,
		"nonce":          b64url(nonce2),
		"sentAt":         input.Rotation.SentAt,
		"sequence":       input.Rotation.Sequence,
		"sessionId":      input.KeyWrap.SessionID,
		"sourceDeviceId": input.Attestation.ApproverDeviceID,
		"streamId":       input.Payload.StreamID,
		"targetDeviceId": input.Attestation.SubjectDeviceID,
		"type":           "session_envelope",
		"version":        1,
	}
	rotationAAD := canonical(rotationHeader)
	rotationPlaintext := mustHex(input.Rotation.Plaintext)
	rotationAEAD, _ := chacha20poly1305.NewX(trafficKey2)
	rotationCiphertext := rotationAEAD.Seal(nil, nonce2, rotationPlaintext, rotationAAD)
	_, err = openX(trafficKey1, nonce2, rotationCiphertext, rotationAAD)
	oldKeyRejected := err != nil
	rotationRecovered, err := openX(trafficKey2, nonce2, rotationCiphertext, rotationAAD)
	newKeyRecovered := err == nil && bytes.Equal(rotationRecovered, rotationPlaintext)

	return map[string]any{
		"attestation": map[string]any{
			"approverPublicKey":                 b64url(approverPublic),
			"arbitraryMapRejected":              mapErr != nil,
			"canonical":                         string(attestationCanonical),
			"capabilityMutationRejected":        capabilityMutationRejected,
			"exchangeDigestMutationRejected":    exchangeDigestMutationRejected,
			"exchangeKeyDigestMismatchRejected": exchangeKeyDigestMismatchRejected,
			"duplicateMemberRejected":           duplicateErr != nil,
			"escapingCanonical":                 escapedErr == nil && bytes.Equal(escapedCanonical, attestationCanonical),
			"expiryMutationRejected":            attestationExpiryMutationRejected,
			"floatRejected":                     floatErr != nil,
			"idMutationRejected":                attestationIDMutationRejected,
			"invalidCapabilityRejected":         invalidCapabilityErr != nil,
			"invalidCalendarDateRejected":       invalidCalendarErr != nil,
			"invalidHour24Rejected":             invalidHour24Err != nil,
			"invalidIDRejected":                 invalidIDErr != nil,
			"invalidLifetimeRejected":           invalidLifetimeErr != nil,
			"leapDayBoundaryAccepted":           leapDayBoundaryErr == nil,
			"mutationRejected":                  attestationMutationRejected,
			"negativeIntegerRejected":           negativeIntegerErr != nil,
			"orderIndependent":                  reorderedErr == nil && bytes.Equal(reorderedCanonical, attestationCanonical),
			"signature":                         b64url(attestationSignature),
			"subjectExchangeKeyDigest":          b64url(subjectExchangeDigest),
			"subjectExchangePublicKey":          b64url(targetPublicRaw),
			"subjectSigningKeyDigest":           b64url(subjectSigningDigest),
			"subjectSigningPublicKey":           b64url(subjectPublic),
			"signingDigestMutationRejected":     signingDigestMutationRejected,
			"signingKeyDigestMismatchRejected":  signingKeyDigestMismatchRejected,
			"subjectKeyDigestsMatch":            attestationKeyDigestsVerify,
			"unicodeSurrogateRejected":          unicodeErr != nil,
			"unknownMemberRejected":             unknownErr != nil,
			"unsafeIntegerRejected":             unsafeIntegerErr != nil,
			"verifies":                          ed25519.Verify(approverPublic, attestationMessage, attestationSignature),
		},
		"keyPop": map[string]any{
			"allZeroSharedSecretRejected":          allZeroRejected,
			"ceremonyMutationRejected":             ceremonyMutationRejected,
			"challengeMutationRejected":            challengeMutationRejected,
			"context":                              b64url(popContextBytes),
			"deviceMutationRejected":               deviceMutationRejected,
			"exchangeKeyMutationRejected":          exchangeKeyMutationRejected,
			"exchangeProof":                        b64url(exchangeProof),
			"exchangeProofContentMutationRejected": exchangeProofContentMutationRejected,
			"exchangeProofLongRejected":            exchangeProofLongRejected,
			"exchangeProofShortRejected":           exchangeProofShortRejected,
			"expiryMutationRejected":               expiryMutationRejected,
			"firstConsumeAccepted":                 firstConsumeAccepted,
			"popKey":                               b64url(popKey),
			"purposeMutationRejected":              purposeMutationRejected,
			"replayRejected":                       replayRejected,
			"restartInvalidated":                   restartInvalidated,
			"serverEphemeralMutationRejected":      serverEphemeralMutationRejected,
			"serverEphemeralPublicKey":             b64url(serverPopPrivate.PublicKey().Bytes()),
			"signingKeyMutationRejected":           signingKeyMutationRejected,
			"signingProof":                         b64url(signingProof),
			"sharedSecret":                         b64url(popSharedSecret),
			"storageAssertionDigest":               b64url(storageAssertionDigest),
			"storageAssertionMutationRejected":     storageAssertionMutationRejected,
			"storageModeMutationRejected":          storageModeMutationRejected,
			"verifies":                             popVerifies,
		},
		"keyWrap": map[string]any{
			"aad":                       string(wrapAAD),
			"aadMutationRejected":       wrapAADMutationRejected,
			"ciphertext":                b64url(wrappedKey),
			"enc":                       b64url(enc),
			"info":                      b64url(hpkeInfo),
			"sourceExchangePublicKey":   b64url(sourcePublicRaw),
			"targetExchangePublicKey":   b64url(targetPublicRaw),
			"unwrapMatches":             true,
			"wrongPinnedSenderRejected": wrongPinnedSenderRejected,
		},
		"payload": map[string]any{
			"aad":                           string(payloadAAD),
			"aadMutationRejected":           payloadAADMutationRejected,
			"ciphertext":                    b64url(payloadCiphertext),
			"nonce":                         b64url(nonce1),
			"nonceSequenceMismatchRejected": nonceSequenceMismatchRejected,
			"replaySequence":                []string{"100", "98", "99", "100", "36"},
			"replayVerdicts":                replayVerdicts,
			"roundTrip":                     true,
			"trafficContext":                string(trafficContext1),
			"trafficKey":                    b64url(trafficKey1),
		},
		"crossPeer": map[string]any{
			"forgeAAD":                   string(forgeAAD),
			"forgeCiphertext":            b64url(forgeCiphertext),
			"peerAForPeerBOpenRejected":  peerAOpenPeerBRejected,
			"peerAForPeerBForgeRejected": peerAForgeryRejected,
			"peerBAAD":                   string(peerBAAD),
			"peerBCiphertext":            b64url(peerBCiphertext),
			"peerBExchangePublicKey":     b64url(peerBPublicRaw),
			"peerBTrafficContext":        string(peerBTrafficContext),
			"peerBTrafficKey":            b64url(peerBTrafficKey),
		},
		"pin": map[string]any{
			"alteredGroupRejected":          fingerprintAlteredGroupRejected,
			"digest":                        b64url(pinDigest),
			"fingerprint":                   fingerprint,
			"invalidBase32Rejected":         fingerprintInvalidBase32Rejected,
			"length23Rejected":              fingerprintLength23Rejected,
			"length25Rejected":              fingerprintLength25Rejected,
			"lowercaseAccepted":             fingerprintLowercaseAccepted,
			"oldFullHexDisplayRejected":     fingerprintOldHexRejected,
			"truncatedAsFullDigestRejected": fingerprintTruncatedAsFullDigestRejected,
			"unhyphenatedAccepted":          fingerprintUnhyphenatedAccepted,
		},
		"rotation": map[string]any{
			"aad":             string(rotationAAD),
			"ciphertext":      b64url(rotationCiphertext),
			"newKeyRecovered": newKeyRecovered,
			"nonce":           b64url(nonce2),
			"oldKeyRejected":  oldKeyRejected,
			"trafficContext":  string(trafficContext2),
			"trafficKey":      b64url(trafficKey2),
		},
	}, nil
}

func main() {
	path := "../vectors.json"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var input vectorInput
	if err := json.Unmarshal(data, &input); err != nil {
		panic(err)
	}
	result, err := run(input)
	if err != nil {
		panic(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(encoded))
}
