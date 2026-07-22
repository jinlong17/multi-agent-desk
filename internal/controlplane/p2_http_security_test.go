package controlplane

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	generatedapi "github.com/jinlong17/multi-agent-desk/internal/controlplane/api/generated"
)

func p2MutationHeaders() http.Header {
	return http.Header{
		"Origin":          {"https://control.example.test"},
		"Sec-Fetch-Site":  {"same-origin"},
		"Sec-Fetch-Mode":  {"cors"},
		"Sec-Fetch-Dest":  {"empty"},
		"Content-Type":    {"application/json"},
		"Idempotency-Key": {"0123456789abcdef"},
	}
}

func p2ReadHeaders() http.Header {
	return http.Header{
		"Sec-Fetch-Site": {"same-origin"},
		"Sec-Fetch-Mode": {"cors"},
		"Sec-Fetch-Dest": {"empty"},
	}
}

func structurallyValidRegistrationCredential() generatedapi.WebAuthnRegistrationCredentialV1 {
	binary := base64.RawURLEncoding.EncodeToString([]byte{1})
	return generatedapi.WebAuthnRegistrationCredentialV1{
		ClientExtensionResults: generatedapi.WebAuthnExtensionResultsV1{},
		Id:                     binary,
		RawId:                  binary,
		Response: generatedapi.WebAuthnRegistrationResponseV1{
			AttestationObject: binary,
			ClientDataJSON:    binary,
		},
		Type: generatedapi.WebAuthnRegistrationCredentialV1TypePublicKey,
	}
}

func structurallyValidAssertionCredential() generatedapi.WebAuthnAssertionCredentialV1 {
	binary := base64.RawURLEncoding.EncodeToString([]byte{1})
	return generatedapi.WebAuthnAssertionCredentialV1{
		ClientExtensionResults: generatedapi.WebAuthnExtensionResultsV1{},
		Id:                     binary,
		RawId:                  binary,
		Response: generatedapi.WebAuthnAssertionResponseV1{
			AuthenticatorData: base64.RawURLEncoding.EncodeToString(make([]byte, 37)),
			ClientDataJSON:    binary,
			Signature:         binary,
		},
		Type: generatedapi.WebAuthnAssertionCredentialV1TypePublicKey,
	}
}

func jsonWithNestedNull(t *testing.T, value any, path ...string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	cursor := object
	for _, member := range path[:len(path)-1] {
		next, ok := cursor[member].(map[string]any)
		if !ok {
			t.Fatalf("JSON path %v is not an object at %q", path, member)
		}
		cursor = next
	}
	cursor[path[len(path)-1]] = nil
	encoded, err = json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestP2MutationSecurityRejectsBeforeSideEffects(t *testing.T) {
	server, store := testServer(t)
	now := time.Now().UTC()
	token, created, err := store.EnsureBootstrapToken(t.Context(), now)
	if err != nil || !created || token == "" {
		t.Fatalf("token created=%v err=%v", created, err)
	}
	_, _, _, descriptor := remoteBootstrapFixture(t, now)
	validBody, err := json.Marshal(generatedapi.BootstrapOptionsRequestV1{DisplayName: "Owner", Anchor: descriptor.Anchor})
	if err != nil {
		t.Fatal(err)
	}
	var before int
	if err := store.db.QueryRow("SELECT count(*) FROM users").Scan(&before); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		headers http.Header
		body    string
		status  int
	}{
		{"missing-origin", http.Header{"Content-Type": {"application/json"}, "Idempotency-Key": {"0123456789abcdef"}}, `{}`, http.StatusForbidden},
		{"wrong-content", http.Header{"Origin": {"https://control.example.test"}, "Sec-Fetch-Site": {"same-origin"}, "Sec-Fetch-Mode": {"cors"}, "Sec-Fetch-Dest": {"empty"}, "Content-Type": {"text/plain"}, "Idempotency-Key": {"0123456789abcdef"}}, `{}`, http.StatusUnsupportedMediaType},
		{"missing-token", p2MutationHeaders(), string(validBody), http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := request(t, server, http.MethodPost, "/v1/bootstrap/options", test.body, test.headers)
			if response.Code != test.status || !strings.Contains(response.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("code=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	var after int
	if err := store.db.QueryRow("SELECT count(*) FROM users").Scan(&after); err != nil || after != before {
		t.Fatalf("users before=%d after=%d err=%v", before, after, err)
	}
}

func TestP2NamedSchemaHostileBodiesRejectBeforeIdempotencyLookup(t *testing.T) {
	server, _ := testServer(t)
	now := time.Now().UTC()
	_, _, _, descriptor := remoteBootstrapFixture(t, now)
	descriptor.Anchor.KeyEnvelopeAssertion.RecordRevision = 9007199254740992
	bootstrapOptions, err := json.Marshal(generatedapi.BootstrapOptionsRequestV1{DisplayName: "Owner", Anchor: descriptor.Anchor})
	if err != nil {
		t.Fatal(err)
	}

	registration := structurallyValidRegistrationCredential()
	bootstrapVerify := generatedapi.BootstrapVerifyRequestV1{
		CeremonyId:    "018f47a0-7b1c-7cc2-8000-000000000001",
		Credential:    registration,
		ExchangeProof: base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
		SigningProof:  base64.RawURLEncoding.EncodeToString(make([]byte, 64)),
	}
	assertion := generatedapi.WebAuthnAssertionVerifyRequestV1{
		CeremonyId: "018f47a0-7b1c-7cc2-8000-000000000002",
		Credential: structurallyValidAssertionCredential(),
	}
	registrationVerify := generatedapi.WebAuthnRegistrationVerifyRequestV1{
		CeremonyId: "018f47a0-7b1c-7cc2-8000-000000000003",
		Credential: registration,
	}

	for index, test := range []struct {
		name, path, body string
	}{
		{"bootstrap-options-range", "/v1/bootstrap/options", string(bootstrapOptions)},
		{"bootstrap-verify-null", "/v1/bootstrap/verify", jsonWithNestedNull(t, bootstrapVerify, "credential", "authenticatorAttachment")},
		{"login-verify-null", "/v1/auth/passkeys/verify", jsonWithNestedNull(t, assertion, "credential", "response", "userHandle")},
		{"registration-verify-null", "/v1/auth/passkeys/registration/verify", jsonWithNestedNull(t, registrationVerify, "credential", "response", "transports")},
		{"uv-verify-null", "/v1/auth/uv/verify", jsonWithNestedNull(t, assertion, "credential", "authenticatorAttachment")},
		{"recovery-verify-pattern", "/v1/auth/recovery/verify", `{"code":"invalid"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			headers := p2MutationHeaders()
			headers.Set("Idempotency-Key", "hostile-schema-request-"+string(rune('a'+index)))
			response := request(t, server, http.MethodPost, test.path, test.body, headers)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"invalid_argument"`) {
				t.Fatalf("schema hostile code=%d body=%s", response.Code, response.Body.String())
			}
			var count int
			if err := server.store.db.QueryRow("SELECT count(*) FROM auth_idempotency_operations").Scan(&count); err != nil || count != 0 {
				t.Fatalf("idempotency rows=%d err=%v", count, err)
			}
		})
	}
}

func TestP2ExactHeadersBodylessJSONAndStableErrors(t *testing.T) {
	server, store := testServer(t)
	_, _, session := seedPasskeySession(t, store, 0)

	missingIdempotency := p2MutationHeaders()
	missingIdempotency.Del("Idempotency-Key")
	response := request(t, server, http.MethodPost, "/v1/auth/passkeys/options", `{}`, missingIdempotency)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"idempotency_key_required"`) {
		t.Fatalf("missing idempotency=%d %s", response.Code, response.Body.String())
	}

	duplicateOrigin := p2MutationHeaders()
	duplicateOrigin["Origin"] = []string{"https://control.example.test", "https://control.example.test"}
	response = request(t, server, http.MethodPost, "/v1/auth/passkeys/options", `{}`, duplicateOrigin)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), `"code":"origin_mismatch"`) {
		t.Fatalf("duplicate origin=%d %s", response.Code, response.Body.String())
	}

	response = request(t, server, http.MethodPost, "/v1/auth/passkeys/options", `{"unexpected":true}`, p2MutationHeaders())
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"invalid_argument"`) {
		t.Fatalf("bodyless mutation accepted=%d %s", response.Code, response.Body.String())
	}
	response = request(t, server, http.MethodPost, "/v1/auth/passkeys/options", `{}`, p2MutationHeaders())
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"ceremonyId"`) {
		t.Fatalf("empty JSON mutation=%d %s", response.Code, response.Body.String())
	}

	verifyHeaders := p2MutationHeaders()
	verifyHeaders.Set("Idempotency-Key", "bootstrap-verify-no-token")
	verifyBody, err := json.Marshal(generatedapi.BootstrapVerifyRequestV1{
		CeremonyId:    "018f47a0-7b1c-7cc2-8000-000000000001",
		Credential:    structurallyValidRegistrationCredential(),
		ExchangeProof: base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
		SigningProof:  base64.RawURLEncoding.EncodeToString(make([]byte, 64)),
	})
	if err != nil {
		t.Fatal(err)
	}
	response = request(t, server, http.MethodPost, "/v1/bootstrap/verify", string(verifyBody), verifyHeaders)
	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), `"code":"bootstrap_unavailable"`) {
		t.Fatalf("bootstrap verify without token=%d %s", response.Code, response.Body.String())
	}

	readHeaders := p2ReadHeaders()
	readHeaders.Set("Cookie", browserSessionCookieName+"="+base64.RawURLEncoding.EncodeToString(session.RawToken))
	response = request(t, server, http.MethodGet, "/v1/auth/current", "", readHeaders)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"csrfToken"`) {
		t.Fatalf("same-origin auth read=%d %s", response.Code, response.Body.String())
	}

	missingFetch := http.Header{"Cookie": readHeaders.Values("Cookie")}
	response = request(t, server, http.MethodGet, "/v1/auth/current", "", missingFetch)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), `"code":"origin_mismatch"`) {
		t.Fatalf("cross-origin auth read=%d %s", response.Code, response.Body.String())
	}

	duplicateCookie := p2ReadHeaders()
	duplicateCookie["Cookie"] = []string{readHeaders.Get("Cookie"), readHeaders.Get("Cookie")}
	response = request(t, server, http.MethodGet, "/v1/auth/current", "", duplicateCookie)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("duplicate cookie=%d %s", response.Code, response.Body.String())
	}
}

func TestP2TrustedProxyParsingIsCanonical(t *testing.T) {
	server, _ := testServer(t)
	server.config.TrustedProxyCIDRs = []string{"192.0.2.0/24"}
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.RemoteAddr = "192.0.2.10:443"
	request.Header.Set("X-Forwarded-For", "198.51.100.7")
	if source := server.requestSource(request); source != "198.51.100.7" {
		t.Fatalf("trusted proxy source=%q", source)
	}
	request.Header.Add("X-Forwarded-For", "203.0.113.8")
	if source := server.requestSource(request); source != "unknown" {
		t.Fatalf("duplicate forwarded source=%q", source)
	}
}

func TestP2DeleteCanonicalPathsRejectAliasesBeforeIdempotencyLookup(t *testing.T) {
	server, store := testServer(t)
	user, passkey, session := seedPasskeySession(t, store, 0)
	secondSession, err := NewBrowserSessionCreate(user.ID, "passkey", passkey.ID, server.config.PublicOrigin, session.AuthenticatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateBrowserSession(t.Context(), secondSession, secondSession.AuthenticatedAt); err != nil {
		t.Fatal(err)
	}
	cookie := browserSessionCookieName + "=" + base64.RawURLEncoding.EncodeToString(session.RawToken)
	csrf := base64.RawURLEncoding.EncodeToString(session.RawCSRF)
	basePaths := []string{
		"/v1/auth/passkeys/" + passkey.ID,
		"/v1/auth/sessions/" + secondSession.ID,
	}
	var targets []string
	for _, path := range basePaths {
		lastSlash := strings.LastIndex(path, "/")
		if lastSlash < 0 || lastSlash == len(path)-1 {
			t.Fatal("test path is invalid")
		}
		prefix, id := path[:lastSlash], path[lastSlash+1:]
		targets = append(targets,
			prefix+"/"+strings.Replace(id, "0", "%30", 1),
			prefix+"/"+strings.Replace(id, "-", "%2F", 1),
			prefix+"/"+strings.ToUpper(id),
			path+"/",
			path+"?",
			path+"?alias=1",
		)
	}
	for index, target := range targets {
		headers := p2MutationHeaders()
		headers.Set("Idempotency-Key", "canonical-path-hostile-"+string(rune('a'+index)))
		headers.Set("If-Match", `"rev-1"`)
		headers.Set("Cookie", cookie)
		headers.Set("X-CSRF-Token", csrf)
		response := request(t, server, http.MethodDelete, target, `{}`, headers)
		if response.Code < http.StatusBadRequest {
			t.Fatalf("alias target=%q code=%d body=%s", target, response.Code, response.Body.String())
		}
		var count int
		if err := store.db.QueryRow("SELECT count(*) FROM auth_idempotency_operations").Scan(&count); err != nil || count != 0 {
			t.Fatalf("target=%q idempotency rows=%d err=%v", target, count, err)
		}
		var passkeyActive int
		if err := store.db.QueryRow("SELECT active FROM passkeys WHERE id=?", passkey.ID).Scan(&passkeyActive); err != nil || passkeyActive != 1 {
			t.Fatalf("target=%q passkey active=%d err=%v", target, passkeyActive, err)
		}
		var revokedAt any
		var revision int64
		if err := store.db.QueryRow("SELECT revoked_at,revision FROM browser_sessions WHERE id=?", secondSession.ID).Scan(&revokedAt, &revision); err != nil || revokedAt != nil || revision != 1 {
			t.Fatalf("target=%q session revoked_at=%v revision=%d err=%v", target, revokedAt, revision, err)
		}
	}
}

func TestRecoverySessionCanOnlyReadCurrentAuthRegisterReplacementOrLogout(t *testing.T) {
	server, store := testServer(t)
	user, passkey, _ := seedPasskeySession(t, store, 0)
	now := time.Now().UTC()
	recovery, err := NewBrowserSessionCreate(user.ID, "recovery", "", server.config.PublicOrigin, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateBrowserSession(t.Context(), recovery, now); err != nil {
		t.Fatal(err)
	}
	cookie := browserSessionCookieName + "=" + base64.RawURLEncoding.EncodeToString(recovery.RawToken)
	csrf := base64.RawURLEncoding.EncodeToString(recovery.RawCSRF)

	readHeaders := p2ReadHeaders()
	readHeaders.Set("Cookie", cookie)
	current := request(t, server, http.MethodGet, "/v1/auth/current", "", readHeaders)
	if current.Code != http.StatusOK || !strings.Contains(current.Body.String(), `"authenticationMethod":"recovery"`) || !strings.Contains(current.Body.String(), `"capabilities":["mad.v1.passkey.manage"]`) {
		t.Fatalf("recovery current=%d %s", current.Code, current.Body.String())
	}

	registrationHeaders := p2MutationHeaders()
	registrationHeaders.Set("Cookie", cookie)
	registrationHeaders.Set("X-CSRF-Token", csrf)
	registration := request(t, server, http.MethodPost, "/v1/auth/passkeys/registration/options", `{}`, registrationHeaders)
	if registration.Code != http.StatusOK || !strings.Contains(registration.Body.String(), `"ceremonyId"`) {
		t.Fatalf("replacement registration=%d %s", registration.Code, registration.Body.String())
	}

	for index, test := range []struct {
		name, method, path string
		body               string
		read               bool
		ifMatch            string
	}{
		{"passkey-list", http.MethodGet, "/v1/auth/passkeys", "", true, ""},
		{"session-list", http.MethodGet, "/v1/auth/sessions", "", true, ""},
		{"uv", http.MethodPost, "/v1/auth/uv/options", `{}`, false, ""},
		{"rotate-codes", http.MethodPost, "/v1/auth/recovery-codes/rotate", `{}`, false, ""},
		{"delete-passkey", http.MethodDelete, "/v1/auth/passkeys/" + passkey.ID, `{}`, false, `"rev-1"`},
		{"delete-session", http.MethodDelete, "/v1/auth/sessions/" + recovery.ID, `{}`, false, `"rev-1"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			headers := p2MutationHeaders()
			if test.read {
				headers = p2ReadHeaders()
			}
			headers.Set("Cookie", cookie)
			if !test.read {
				headers.Set("Idempotency-Key", "recovery-scope-"+string(rune('a'+index))+"-request")
				headers.Set("X-CSRF-Token", csrf)
				if test.ifMatch != "" {
					headers.Set("If-Match", test.ifMatch)
				}
			}
			response := request(t, server, test.method, test.path, test.body, headers)
			if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), `"code":"permission_denied"`) {
				t.Fatalf("recovery scope=%d path=%s key=%s body=%s", response.Code, test.path, headers.Get("Idempotency-Key"), response.Body.String())
			}
		})
	}
}
