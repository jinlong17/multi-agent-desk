package controlplane

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/jinlong17/multi-agent-desk/internal/controlplane/webassets"
)

var staticAssetReference = regexp.MustCompile(`(?:src|href)="/(assets/index-[A-Za-z0-9_-]{8,}\.(?:js|css))"`)

func assertNoCORSHeaders(t *testing.T, header http.Header) {
	t.Helper()
	for name := range header {
		if strings.HasPrefix(strings.ToLower(name), "access-control-") {
			t.Fatalf("response unexpectedly emitted CORS header %s=%q", name, header.Values(name))
		}
	}
}

func TestEmbeddedWebAssetsServeExactRoutesAndSecurityHeaders(t *testing.T) {
	server, _ := testServer(t)
	index, err := webassets.Files.ReadFile("index.html")
	if err != nil {
		t.Fatal(err)
	}
	matches := staticAssetReference.FindAllSubmatch(index, -1)
	if len(matches) != 2 || strings.Contains(string(index), "/src/main.ts") {
		t.Fatalf("embedded index asset references=%q", matches)
	}

	resources := []struct {
		path        string
		contentType string
		contents    []byte
	}{
		{path: "/", contentType: "text/html; charset=utf-8", contents: index},
		{path: "/index.html", contentType: "text/html; charset=utf-8", contents: index},
	}
	for _, match := range matches {
		path := "/" + string(match[1])
		if !embeddedWebAssetName.MatchString(strings.TrimPrefix(path, "/assets/")) {
			t.Fatalf("embedded runtime asset is not content-hashed: %s", path)
		}
		contents, readErr := webassets.Files.ReadFile(strings.TrimPrefix(path, "/"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		contentType := "text/javascript; charset=utf-8"
		if strings.HasSuffix(path, ".css") {
			contentType = "text/css; charset=utf-8"
		}
		resources = append(resources, struct {
			path        string
			contentType string
			contents    []byte
		}{path: path, contentType: contentType, contents: contents})
	}

	for _, resource := range resources {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(method+" "+resource.path, func(t *testing.T) {
				response := request(t, server, method, resource.path, "", nil)
				assertNoCORSHeaders(t, response.Header())
				if response.Code != http.StatusOK {
					t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
				}
				if got := response.Header().Get("Content-Type"); got != resource.contentType {
					t.Fatalf("Content-Type=%q want=%q", got, resource.contentType)
				}
				if got := response.Header().Get("Content-Length"); got != fmt.Sprintf("%d", len(resource.contents)) {
					t.Fatalf("Content-Length=%q want=%d", got, len(resource.contents))
				}
				for name, want := range map[string]string{
					"Cache-Control":          "no-store",
					"X-Content-Type-Options": "nosniff",
					"X-Frame-Options":        "DENY",
					"Referrer-Policy":        "no-referrer",
				} {
					if got := response.Header().Get(name); got != want {
						t.Fatalf("%s=%q want=%q", name, got, want)
					}
				}
				csp := response.Header().Get("Content-Security-Policy")
				for _, directive := range []string{"default-src 'none'", "base-uri 'none'", "frame-ancestors 'none'", "object-src 'none'", "connect-src 'self'", "script-src 'self'", "style-src 'self'", "manifest-src 'none'", "worker-src 'none'"} {
					if !strings.Contains(csp, directive) {
						t.Fatalf("CSP=%q missing %q", csp, directive)
					}
				}
				if method == http.MethodHead {
					if response.Body.Len() != 0 {
						t.Fatalf("HEAD body=%q", response.Body.String())
					}
				} else if got := response.Body.Bytes(); string(got) != string(resource.contents) {
					t.Fatal("GET body differs from embedded asset")
				}
			})
		}
	}
}

func TestEmbeddedWebAssetsRejectAliasesUnknownRoutesAndP6Artifacts(t *testing.T) {
	server, _ := testServer(t)
	for _, target := range []string{
		"/assets/not-found.js",
		"/assets/nested/index-deadbeef.js",
		"/src/main.ts",
		"/manifest.webmanifest",
		"/service-worker.js",
		"/index.html?cache=1",
		"/index.html?",
		"/%69ndex.html",
	} {
		t.Run(target, func(t *testing.T) {
			response := request(t, server, http.MethodGet, target, "", nil)
			assertNoCORSHeaders(t, response.Header())
			if response.Code != http.StatusNotFound || response.Header().Get("Content-Type") != "application/json" || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
				t.Fatalf("status=%d type=%q body=%q", response.Code, response.Header().Get("Content-Type"), response.Body.String())
			}
			if strings.Contains(strings.ToLower(response.Body.String()), "<!doctype html") {
				t.Fatal("unknown static route received an SPA fallback")
			}
		})
	}

	response := request(t, server, http.MethodPost, "/", `{}`, http.Header{"Content-Type": {"application/json"}})
	assertNoCORSHeaders(t, response.Header())
	if response.Code != http.StatusNotFound || response.Header().Get("Content-Type") != "application/json" || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
		t.Fatalf("static POST status=%d type=%q body=%q", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}

	response = request(t, server, http.MethodGet, "/v1/not-mounted", "", nil)
	assertNoCORSHeaders(t, response.Header())
	if response.Code != http.StatusNotFound || response.Header().Get("Content-Type") != "application/json" || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
		t.Fatalf("unknown API status=%d type=%q body=%q", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}
}

func TestEveryMountedP2APIResponseOmitsCORSHeaders(t *testing.T) {
	server, _ := testServer(t)
	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/v1/healthz"},
		{http.MethodGet, "/v1/readyz"},
		{http.MethodGet, "/v1/version"},
		{http.MethodGet, "/v1/bootstrap/status"},
		{http.MethodPost, "/v1/bootstrap/options"},
		{http.MethodPost, "/v1/bootstrap/verify"},
		{http.MethodGet, "/v1/bootstrap/ceremonies/ceremony_fixture"},
		{http.MethodGet, "/v1/auth/current"},
		{http.MethodPost, "/v1/auth/logout"},
		{http.MethodPost, "/v1/auth/passkeys/options"},
		{http.MethodPost, "/v1/auth/passkeys/verify"},
		{http.MethodPost, "/v1/auth/passkeys/registration/options"},
		{http.MethodPost, "/v1/auth/passkeys/registration/verify"},
		{http.MethodGet, "/v1/auth/passkeys"},
		{http.MethodDelete, "/v1/auth/passkeys/passkey_fixture"},
		{http.MethodPost, "/v1/auth/uv/options"},
		{http.MethodPost, "/v1/auth/uv/verify"},
		{http.MethodPost, "/v1/auth/recovery/verify"},
		{http.MethodPost, "/v1/auth/recovery-codes/rotate"},
		{http.MethodGet, "/v1/auth/sessions"},
		{http.MethodDelete, "/v1/auth/sessions/session_fixture"},
		{http.MethodGet, "/v1/not-mounted"},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			headers := http.Header{
				"Content-Type":   {"application/json"},
				"Origin":         {"https://cross-origin.invalid"},
				"Sec-Fetch-Site": {"cross-site"},
			}
			response := request(t, server, test.method, test.path, `{}`, headers)
			assertNoCORSHeaders(t, response.Header())
		})
	}
}
