package controlplane

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func TestP2MutationSecurityRejectsBeforeSideEffects(t *testing.T) {
	server, store := testServer(t)
	token, created, err := store.EnsureBootstrapToken(t.Context(), time.Now().UTC())
	if err != nil || !created || token == "" {
		t.Fatalf("token created=%v err=%v", created, err)
	}
	var before int
	if err := store.db.QueryRow("SELECT count(*) FROM users").Scan(&before); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		headers http.Header
		status  int
	}{
		{"missing-origin", http.Header{"Content-Type": {"application/json"}, "Idempotency-Key": {"0123456789abcdef"}}, http.StatusForbidden},
		{"wrong-content", http.Header{"Origin": {"https://control.example.test"}, "Sec-Fetch-Site": {"same-origin"}, "Sec-Fetch-Mode": {"cors"}, "Sec-Fetch-Dest": {"empty"}, "Content-Type": {"text/plain"}, "Idempotency-Key": {"0123456789abcdef"}}, http.StatusUnsupportedMediaType},
		{"missing-token", p2MutationHeaders(), http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := request(t, server, http.MethodPost, "/v1/bootstrap/options", `{}`, test.headers)
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
	response = request(t, server, http.MethodPost, "/v1/bootstrap/verify", `{}`, verifyHeaders)
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

func TestP2IfMatchAndTrustedProxyParsingAreCanonical(t *testing.T) {
	for _, test := range []struct {
		value string
		ok    bool
	}{
		{`"rev-1"`, true}, {`"rev-01"`, false}, {`"rev-+1"`, false}, {`W/"rev-1"`, false}, {"", false},
	} {
		request := httptest.NewRequest(http.MethodDelete, "/", strings.NewReader(`{}`))
		if test.value != "" {
			request.Header.Set("If-Match", test.value)
		}
		_, err := parseIfMatch(request)
		if (err == nil) != test.ok {
			t.Fatalf("If-Match %q err=%v", test.value, err)
		}
	}

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

func TestRecoverySessionCanOnlyReadCurrentAuthRegisterReplacementOrLogout(t *testing.T) {
	server, store := testServer(t)
	user, passkey, _ := seedPasskeySession(t, store, 0)
	now := time.Now().UTC()
	recovery, err := NewBrowserSessionCreate(user.ID, "recovery", "", now)
	if err != nil {
		t.Fatal(err)
	}
	recovery.RawCSRF = deriveSessionCSRF(recovery.RawToken, recovery.ID)
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

	for _, test := range []struct {
		name, method, path string
		body               string
		read               bool
	}{
		{"passkey-list", http.MethodGet, "/v1/auth/passkeys", "", true},
		{"session-list", http.MethodGet, "/v1/auth/sessions", "", true},
		{"uv", http.MethodPost, "/v1/auth/uv/options", `{}`, false},
		{"rotate-codes", http.MethodPost, "/v1/auth/recovery-codes/rotate", `{}`, false},
		{"delete-passkey", http.MethodDelete, "/v1/auth/passkeys/" + passkey.ID, `{}`, false},
		{"delete-session", http.MethodDelete, "/v1/auth/sessions/" + recovery.ID, `{}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			headers := p2MutationHeaders()
			if test.read {
				headers = p2ReadHeaders()
			}
			headers.Set("Cookie", cookie)
			if !test.read {
				headers.Set("X-CSRF-Token", csrf)
			}
			response := request(t, server, test.method, test.path, test.body, headers)
			if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), `"code":"permission_denied"`) {
				t.Fatalf("recovery scope=%d %s", response.Code, response.Body.String())
			}
		})
	}
}
