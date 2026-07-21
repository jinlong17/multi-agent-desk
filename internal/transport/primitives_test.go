package transport

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestUUIDv7Strict(t *testing.T) {
	value, err := NewUUIDv7()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseUUIDv7(value); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{strings.ToUpper(value), "550e8400-e29b-41d4-a716-446655440000", "not-a-uuid"} {
		if _, err := ParseUUIDv7(invalid); err == nil {
			t.Fatalf("accepted invalid UUID %q", invalid)
		}
	}
}

func TestBase64URLAndUTCDateTimeStrict(t *testing.T) {
	raw := bytes.Repeat([]byte{0xa5}, 32)
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	decoded, err := DecodeBase64URLFixed(encoded, 32)
	if err != nil || !bytes.Equal(decoded, raw) {
		t.Fatalf("decoded=%x err=%v", decoded, err)
	}
	for _, invalid := range []string{encoded + "=", "+" + encoded[1:], " " + encoded, encoded[:len(encoded)-1]} {
		if _, err := DecodeBase64URLFixed(invalid, 32); err == nil {
			t.Fatalf("accepted invalid Base64url %q", invalid)
		}
	}
	for _, valid := range []string{"2030-02-28T23:59:59Z", "2032-02-29T00:00:00.123456Z"} {
		if _, err := ParseUTCDateTime(valid); err != nil {
			t.Fatalf("rejected time %q: %v", valid, err)
		}
	}
	for _, invalid := range []string{"2030-02-29T00:00:00Z", "2030-01-01T24:00:00Z", "2030-01-01T00:00:00+00:00", "2030-01-01T00:00:00.1234567Z", "2030-01-01T00:00:60Z"} {
		if _, err := ParseUTCDateTime(invalid); err == nil {
			t.Fatalf("accepted invalid time %q", invalid)
		}
	}
}

func TestIfMatchStrict(t *testing.T) {
	if revision, err := ParseIfMatch(`"rev-42"`); err != nil || revision != 42 {
		t.Fatalf("revision=%d err=%v", revision, err)
	}
	for _, invalid := range []string{"", "*", `W/"rev-1"`, `"rev-0"`, `"rev-01"`, `"rev--1"`, `"rev-9223372036854775808"`, `rev-1`} {
		if _, err := ParseIfMatch(invalid); err == nil {
			t.Fatalf("accepted invalid If-Match %q", invalid)
		}
	}
}

func TestCursorBindingTamperExpiry(t *testing.T) {
	now := time.Unix(1_900_000_000, 0)
	codec, err := NewCursorCodec(bytes.Repeat([]byte{7}, 32), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := codec.Encode(CursorState{Endpoint: "accounts", Binding: "provider=codex", SortValue: "a", TieBreaker: "id", ExpiresAt: now.Add(time.Minute).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := codec.Decode(encoded, "accounts", "provider=codex")
	if err != nil || decoded.TieBreaker != "id" {
		t.Fatalf("decoded=%+v err=%v", decoded, err)
	}
	tamperedPrefix := "A"
	if encoded[0] == 'A' {
		tamperedPrefix = "B"
	}
	for _, changed := range []struct{ value, endpoint, binding string }{
		{tamperedPrefix + encoded[1:], "accounts", "provider=codex"},
		{encoded, "usage", "provider=codex"}, {encoded, "accounts", "provider=claude"},
	} {
		if _, err := codec.Decode(changed.value, changed.endpoint, changed.binding); err == nil {
			t.Fatal("accepted tampered or cross-bound cursor")
		}
	}
	now = now.Add(2 * time.Minute)
	if _, err := codec.Decode(encoded, "accounts", "provider=codex"); err == nil {
		t.Fatal("accepted expired cursor")
	}
}

func TestIdempotencyScopeAndStrictJSON(t *testing.T) {
	one, err := IdempotencyScopeDigest("user", "v1", "POST", "/v1/profiles", "0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	two, _ := IdempotencyScopeDigest("user", "v1", "POST", "/v1/profiles", "0123456789abcdeg")
	if one == two || one == sha256.Sum256(nil) {
		t.Fatal("idempotency digest did not bind key")
	}
	for _, invalid := range []string{"short", "0123456789abcde\n", strings.Repeat("x", 129)} {
		if _, err := IdempotencyScopeDigest("u", "v1", "POST", "/", invalid); err == nil {
			t.Fatalf("accepted key %q", invalid)
		}
	}
	type payload struct {
		Name string `json:"name"`
	}
	var got payload
	if err := DecodeStrictJSON(strings.NewReader(`{"name":"ok"}`), 128, &got); err != nil || got.Name != "ok" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	for _, invalid := range []string{`{"name":"ok","unknown":1}`, `{"name":"a"}{"name":"b"}`, `{"name":"` + strings.Repeat("x", 200) + `"}`} {
		if err := DecodeStrictJSON(strings.NewReader(invalid), 128, &got); err == nil {
			t.Fatalf("accepted hostile JSON %q", invalid[:min(len(invalid), 40)])
		}
	}
	for _, invalid := range []string{
		`{"name":"first","name":"second"}`,
		`{"name":` + strings.Repeat(`[`, 65) + `"deep"` + strings.Repeat(`]`, 65) + `}`,
		`{"name":` + strings.Repeat("9", 65) + `}`,
		string([]byte{'{', '"', 'n', 'a', 'm', 'e', '"', ':', '"', 0xff, '"', '}'}),
	} {
		if err := DecodeStrictJSON(strings.NewReader(invalid), 1<<20, &got); err == nil {
			t.Fatalf("accepted hostile JSON %q", invalid[:min(len(invalid), 40)])
		}
	}
}
