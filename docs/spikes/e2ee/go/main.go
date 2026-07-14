package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/cloudflare/circl/hpke"
	"golang.org/x/crypto/chacha20poly1305"
)

const hpkeSuiteName = "DHKEM(X25519,HKDF-SHA256)/HKDF-SHA256/ChaCha20Poly1305/Auth"

type vectorInput struct {
	SchemaVersion int `json:"schemaVersion"`
	Seeds         struct {
		ApproverEd25519    string `json:"approverEd25519"`
		SubjectEd25519     string `json:"subjectEd25519"`
		SourceX25519IKM    string `json:"sourceX25519Ikm"`
		TargetX25519IKM    string `json:"targetX25519Ikm"`
		EphemeralX25519IKM string `json:"ephemeralX25519Ikm"`
		SessionKeyEpoch1   string `json:"sessionKeyEpoch1"`
		SessionKeyEpoch2   string `json:"sessionKeyEpoch2"`
	} `json:"seeds"`
	Attestation struct {
		ApproverDeviceID string   `json:"approverDeviceId"`
		AttestationID    string   `json:"attestationId"`
		Capabilities     []string `json:"capabilities"`
		ExpiresAt        string   `json:"expiresAt"`
		IssuedAt         string   `json:"issuedAt"`
		SubjectDeviceID  string   `json:"subjectDeviceId"`
	} `json:"attestation"`
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
	Rotation struct {
		KeyEpoch  string `json:"keyEpoch"`
		MessageID string `json:"messageId"`
		Plaintext string `json:"plaintextHex"`
		SentAt    string `json:"sentAt"`
		Sequence  string `json:"sequence"`
	} `json:"rotation"`
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
	sourcePublicRaw := rawPublic(sourcePublic)
	targetPublicRaw := rawPublic(targetPublic)

	attestation := map[string]any{
		"approverDeviceId":   input.Attestation.ApproverDeviceID,
		"attestationId":      input.Attestation.AttestationID,
		"capabilities":       input.Attestation.Capabilities,
		"expiresAt":          input.Attestation.ExpiresAt,
		"issuedAt":           input.Attestation.IssuedAt,
		"subjectDeviceId":    input.Attestation.SubjectDeviceID,
		"subjectExchangeKey": b64url(targetPublicRaw),
		"subjectSigningKey":  b64url(subjectPublic),
		"type":               "device_attestation",
		"version":            1,
	}
	attestationCanonical := canonical(attestation)
	attestationMessage := framed([]byte("multidesk-device-attestation-v1"), attestationCanonical)
	attestationSignature := ed25519.Sign(approverPrivate, attestationMessage)
	attestationMutation := append([]byte(nil), attestationMessage...)
	attestationMutation[len(attestationMutation)-2] ^= 1
	attestationMutationRejected := !ed25519.Verify(approverPublic, attestationMutation, attestationSignature)

	pinDigest := digest(
		[]byte("multidesk-device-pin-v1"),
		[]byte(input.Attestation.SubjectDeviceID),
		subjectPublic,
		targetPublicRaw,
	)

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
	sessionKey1 := mustHex(input.Seeds.SessionKeyEpoch1)
	wrappedKey, err := sealer.Seal(sessionKey1, wrapAAD)
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
	if !bytes.Equal(recoveredKey, sessionKey1) {
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
		sessionKey1,
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

	sessionKey2 := mustHex(input.Seeds.SessionKeyEpoch2)
	trafficKey2, noncePrefix2, trafficContext2 := deriveTraffic(
		sessionKey2,
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
			"approverPublicKey":       b64url(approverPublic),
			"canonical":               string(attestationCanonical),
			"mutationRejected":        attestationMutationRejected,
			"signature":               b64url(attestationSignature),
			"subjectSigningPublicKey": b64url(subjectPublic),
			"verifies":                ed25519.Verify(approverPublic, attestationMessage, attestationSignature),
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
			"aad":                 string(payloadAAD),
			"aadMutationRejected": payloadAADMutationRejected,
			"ciphertext":          b64url(payloadCiphertext),
			"nonce":               b64url(nonce1),
			"replaySequence":      []string{"100", "98", "99", "100", "36"},
			"replayVerdicts":      replayVerdicts,
			"roundTrip":           true,
			"trafficContext":      string(trafficContext1),
			"trafficKey":          b64url(trafficKey1),
		},
		"pin": map[string]any{
			"digest":      b64url(pinDigest),
			"fingerprint": fmt.Sprintf("%x-%x-%x-%x-%x-%x-%x-%x", pinDigest[0:4], pinDigest[4:8], pinDigest[8:12], pinDigest[12:16], pinDigest[16:20], pinDigest[20:24], pinDigest[24:28], pinDigest[28:32]),
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
