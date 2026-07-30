package transport

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
)

func TestNormalizeIdempotencyKeyV1(t *testing.T) {
	const key = "AbCd-._~!$&'()*+;=:@0123456789"
	got, err := NormalizeIdempotencyKeyV1(" \t" + key + "\t ")
	if err != nil || got != key {
		t.Fatalf("normalize key=%q err=%v", got, err)
	}
	for _, invalid := range []string{
		"short", "0123456789abcde,", "0123456789abcde\n", "0123456789abcde£",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdefX",
	} {
		if _, err := NormalizeIdempotencyKeyV1(invalid); err == nil {
			t.Fatalf("accepted invalid key %q", invalid)
		}
	}
}

func TestCanonicalStrictJSONV1RFC8785Vector(t *testing.T) {
	input := []byte(`{"numbers":[333333333.33333329,1E30,4.50,2e-3,0.000000000000000000000000001],"string":"€$\u000f\nA'B\"\\\"/","literals":[null,true,false]}`)
	var schema struct {
		Literals []any         `json:"literals"`
		Numbers  []json.Number `json:"numbers"`
		String   string        `json:"string"`
	}
	got, err := CanonicalStrictJSONV1(input, 4096, &schema)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"literals":[null,true,false],"numbers":[333333333.3333333,1e+30,4.5,0.002,1e-27],"string":"€$\u000f\nA'B\"\\\"/"}`)
	if !bytes.Equal(got, want) {
		t.Fatalf("canonical JSON\n got %s\nwant %s", got, want)
	}
	if _, err := CanonicalStrictJSONV1([]byte(`{"numbers":[],"numbers":[],"string":"x","literals":[]}`), 4096, &schema); err == nil {
		t.Fatal("accepted duplicate endpoint member")
	}
	if _, err := CanonicalStrictJSONV1([]byte(`{"numbers":[],"string":"x","literals":[],"unknown":true}`), 4096, &schema); err == nil {
		t.Fatal("accepted unknown endpoint member")
	}
}

func TestCanonicalStrictJSONV1HostileUnicodeRequiredAndNumbers(t *testing.T) {
	type requiredFixture struct {
		Required string  `json:"required"`
		Optional *string `json:"optional,omitempty"`
	}
	var schema requiredFixture
	if _, err := CanonicalStrictJSONV1([]byte(`{}`), 64, &schema); err == nil {
		t.Fatal("accepted a missing required endpoint member")
	}
	for _, invalid := range []string{`{"required":"\ud800"}`, `{"required":"\udfff"}`, `{"required":"\ud800x"}`} {
		if _, err := CanonicalStrictJSONV1([]byte(invalid), 128, &schema); err == nil {
			t.Fatalf("accepted lone surrogate %s", invalid)
		}
	}
	paired, err := CanonicalStrictJSONV1([]byte(`{"required":"\ud83d\ude00"}`), 128, &schema)
	if err != nil || string(paired) != `{"required":"😀"}` {
		t.Fatalf("paired surrogate canonicalization=%s err=%v", paired, err)
	}

	var numbers []json.Number
	got, err := CanonicalStrictJSONV1([]byte(`[-0,0.000001,0.0000001,100000000000000000000,1e21,333333333.33333329]`), 512, &numbers)
	if err != nil {
		t.Fatal(err)
	}
	if want := `[0,0.000001,1e-7,100000000000000000000,1e+21,333333333.3333333]`; string(got) != want {
		t.Fatalf("number boundaries=%s want=%s", got, want)
	}
	if _, err := CanonicalStrictJSONV1([]byte(`[1e309]`), 64, &numbers); err == nil {
		t.Fatal("accepted a non-finite IEEE-754 result")
	}
}

func TestAuthIdempotencyDigestDomainsV1(t *testing.T) {
	key := "0123456789abcdef"
	keyDigest, err := AuthIdempotencyKeyDigestV1(key)
	if err != nil {
		t.Fatal(err)
	}
	keyFrame, _ := Frame([]byte(authIdempotencyKeyDomain), []byte("1"), []byte(key))
	if want := sha256.Sum256(keyFrame); keyDigest != want {
		t.Fatal("key digest domain drifted")
	}
	if got := fmt.Sprintf("%x", keyDigest); got != "3bbe6124a204cb21542b8142aba3782c3a5484a18a5525ab2eed4673b5a6ca92" {
		t.Fatalf("key cross-language golden=%s", got)
	}
	bodyDigest, err := AuthIdempotencyBodyDigestV1([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", bodyDigest); got != "4c1d668fb3a72160b6883c25379d844eac6ad23dd9794219a92c84546299a6d1" {
		t.Fatalf("body cross-language golden=%s", got)
	}
	actor := [sha256.Size]byte{}
	for index := range actor {
		actor[index] = 0x5a
	}
	identity, err := AuthIdempotencyRequestIdentityDigestV1(
		"https://control.example.test", "browser_session", actor[:],
		"logout", "POST", "/v1/auth/logout", bodyDigest, "",
	)
	if err != nil {
		t.Fatal(err)
	}
	identityFrame, _ := Frame(
		[]byte(authIdempotencyRequestIdentityDomain), []byte("1"), []byte("https://control.example.test"),
		[]byte("browser_session"), actor[:], []byte("logout"), []byte("POST"),
		[]byte("/v1/auth/logout"), bodyDigest[:], []byte(""),
	)
	if want := sha256.Sum256(identityFrame); identity != want {
		t.Fatal("request identity frame drifted")
	}
	changed, _ := AuthIdempotencyRequestIdentityDigestV1(
		"https://control.example.test", "browser_session", actor[:],
		"logout", "POST", "/v1/auth/logout", bodyDigest, `"rev-1"`,
	)
	if identity == changed {
		t.Fatal("If-Match was not identity-bearing")
	}
	if got := fmt.Sprintf("%x", identity); got != "66302fe7c8bb4af27b2da538a00ebc716057ed380d1f351b13155f07b81209ad" {
		t.Fatalf("request identity cross-language golden=%s", got)
	}
}
