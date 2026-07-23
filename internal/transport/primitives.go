package transport

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var rawBase64 = base64.RawURLEncoding
var utcDateTimePattern = regexp.MustCompile(`^[0-9]{4}-(?:0[1-9]|1[0-2])-(?:0[1-9]|[12][0-9]|3[01])T(?:[01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9](?:\.[0-9]{1,6})?Z$`)

func NewUUIDv7() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate UUIDv7: %w", err)
	}
	return id.String(), nil
}

func ParseUUIDv7(value string) (uuid.UUID, error) {
	if value != strings.ToLower(value) {
		return uuid.Nil, fmt.Errorf("UUIDv7 must be lowercase canonical text")
	}
	id, err := uuid.Parse(value)
	if err != nil || id.Version() != 7 || id.String() != value {
		return uuid.Nil, fmt.Errorf("invalid UUIDv7")
	}
	return id, nil
}

func DecodeBase64URLFixed(value string, size int) ([]byte, error) {
	if size <= 0 || size > 1<<20 || value == "" || strings.Contains(value, "=") {
		return nil, fmt.Errorf("invalid unpadded Base64url value")
	}
	decoded, err := rawBase64.DecodeString(value)
	if err != nil || len(decoded) != size || rawBase64.EncodeToString(decoded) != value {
		return nil, fmt.Errorf("invalid unpadded Base64url value")
	}
	return decoded, nil
}

func ParseUTCDateTime(value string) (time.Time, error) {
	if !utcDateTimePattern.MatchString(value) {
		return time.Time{}, fmt.Errorf("timestamp must be UTC RFC3339 with microsecond-or-less precision")
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.Location() != time.UTC || parsed.Nanosecond()%1_000 != 0 {
		return time.Time{}, fmt.Errorf("timestamp must be a valid UTC RFC3339 value")
	}
	return parsed, nil
}

func ParseIfMatch(value string) (uint64, error) {
	if len(value) < 7 || !strings.HasPrefix(value, `"rev-`) || !strings.HasSuffix(value, `"`) {
		return 0, fmt.Errorf("If-Match must be a strong revision tag")
	}
	revisionText := value[5 : len(value)-1]
	if revisionText == "" || revisionText[0] == '0' || strings.ContainsAny(revisionText, "+- ") {
		return 0, fmt.Errorf("If-Match revision is invalid")
	}
	revision, err := strconv.ParseUint(revisionText, 10, 63)
	if err != nil || revision == 0 {
		return 0, fmt.Errorf("If-Match revision is invalid")
	}
	return revision, nil
}

type CursorCodec struct {
	key [32]byte
	now func() time.Time
}

type CursorState struct {
	Version    int    `json:"version"`
	Endpoint   string `json:"endpoint"`
	Binding    string `json:"binding"`
	SortValue  string `json:"sortValue"`
	TieBreaker string `json:"tieBreaker"`
	ExpiresAt  int64  `json:"expiresAt"`
}

func NewCursorCodec(key []byte, now func() time.Time) (*CursorCodec, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("cursor HMAC key must be exactly 32 bytes")
	}
	if now == nil {
		now = time.Now
	}
	codec := &CursorCodec{now: now}
	copy(codec.key[:], key)
	return codec, nil
}

func (c *CursorCodec) Encode(state CursorState) (string, error) {
	state.Version = 1
	if state.Endpoint == "" || state.Binding == "" || state.ExpiresAt <= c.now().Unix() {
		return "", fmt.Errorf("cursor state is incomplete or expired")
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("encode cursor state: %w", err)
	}
	if len(payload) > 1024 {
		return "", fmt.Errorf("cursor state is too large")
	}
	mac := hmac.New(sha256.New, c.key[:])
	_, _ = mac.Write(payload)
	frame := append(append([]byte{}, payload...), mac.Sum(nil)...)
	return rawBase64.EncodeToString(frame), nil
}

func (c *CursorCodec) Decode(value, endpoint, binding string) (CursorState, error) {
	var state CursorState
	if len(value) == 0 || len(value) > 2048 {
		return state, fmt.Errorf("invalid_cursor")
	}
	frame, err := rawBase64.DecodeString(value)
	if err != nil || len(frame) <= sha256.Size {
		return state, fmt.Errorf("invalid_cursor")
	}
	payload, signature := frame[:len(frame)-sha256.Size], frame[len(frame)-sha256.Size:]
	mac := hmac.New(sha256.New, c.key[:])
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return state, fmt.Errorf("invalid_cursor")
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return state, fmt.Errorf("invalid_cursor")
	}
	if state.Version != 1 || state.Endpoint != endpoint || state.Binding != binding || state.ExpiresAt <= c.now().Unix() {
		return CursorState{}, fmt.Errorf("invalid_cursor")
	}
	return state, nil
}

func IdempotencyScopeDigest(principal, version, method, canonicalPath, key string) ([32]byte, error) {
	var result [32]byte
	if len(key) < 16 || len(key) > 128 {
		return result, fmt.Errorf("idempotency key length is invalid")
	}
	for _, char := range []byte(key) {
		if char < 0x21 || char > 0x7e {
			return result, fmt.Errorf("idempotency key must be visible ASCII")
		}
	}
	hash := sha256.New()
	for _, part := range []string{"multidesk-idempotency-scope-v1", principal, version, method, canonicalPath, key} {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(part)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(part))
	}
	copy(result[:], hash.Sum(nil))
	return result, nil
}

func DecodeStrictJSON(reader io.Reader, limit int64, destination any) error {
	if limit <= 0 || limit > 1<<20 {
		return fmt.Errorf("invalid JSON size limit")
	}
	contents, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return fmt.Errorf("read JSON: %w", err)
	}
	if int64(len(contents)) > limit {
		return fmt.Errorf("request_too_large")
	}
	if !utf8.Valid(contents) {
		return fmt.Errorf("invalid JSON: input is not valid UTF-8")
	}
	if err := validateJSONShape(contents); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("JSON must contain exactly one value")
	}
	return nil
}

type jsonFrame struct {
	object       bool
	expectingKey bool
	keys         map[string]struct{}
	items        int
}

func validateJSONShape(contents []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()
	frames := make([]jsonFrame, 0, 16)
	topLevelValues := 0
	tokens := 0
	markValue := func() error {
		if len(frames) == 0 {
			topLevelValues++
			if topLevelValues > 1 {
				return fmt.Errorf("JSON must contain exactly one value")
			}
			return nil
		}
		frame := &frames[len(frames)-1]
		if frame.object {
			if frame.expectingKey {
				return fmt.Errorf("invalid JSON object member")
			}
			frame.expectingKey = true
			return nil
		}
		frame.items++
		if frame.items > 10_000 {
			return fmt.Errorf("JSON array exceeds the item limit")
		}
		return nil
	}
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
		tokens++
		if tokens > 100_000 {
			return fmt.Errorf("JSON token count exceeds the limit")
		}
		if delimiter, ok := token.(json.Delim); ok {
			switch delimiter {
			case '{', '[':
				if err := markValue(); err != nil {
					return err
				}
				if len(frames) >= 64 {
					return fmt.Errorf("JSON nesting exceeds the depth limit")
				}
				frames = append(frames, jsonFrame{object: delimiter == '{', expectingKey: delimiter == '{', keys: map[string]struct{}{}})
			case '}', ']':
				if len(frames) == 0 {
					return fmt.Errorf("invalid JSON container close")
				}
				frame := frames[len(frames)-1]
				if (delimiter == '}' && !frame.object) || (delimiter == ']' && frame.object) || (frame.object && !frame.expectingKey) {
					return fmt.Errorf("invalid JSON container close")
				}
				frames = frames[:len(frames)-1]
			}
			continue
		}
		if len(frames) != 0 && frames[len(frames)-1].object && frames[len(frames)-1].expectingKey {
			key, ok := token.(string)
			if !ok || len(key) > 256 {
				return fmt.Errorf("invalid JSON object key")
			}
			frame := &frames[len(frames)-1]
			if _, exists := frame.keys[key]; exists {
				return fmt.Errorf("duplicate JSON object key %q", key)
			}
			if len(frame.keys) >= 1_024 {
				return fmt.Errorf("JSON object exceeds the member limit")
			}
			frame.keys[key] = struct{}{}
			frame.expectingKey = false
			continue
		}
		if number, ok := token.(json.Number); ok && len(number.String()) > 64 {
			return fmt.Errorf("JSON number exceeds the representation limit")
		}
		if text, ok := token.(string); ok && len(text) > 256<<10 {
			return fmt.Errorf("JSON string exceeds the representation limit")
		}
		if err := markValue(); err != nil {
			return err
		}
	}
	if len(frames) != 0 || topLevelValues != 1 {
		return fmt.Errorf("JSON must contain exactly one complete value")
	}
	return nil
}
