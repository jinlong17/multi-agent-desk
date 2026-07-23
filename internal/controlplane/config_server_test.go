package controlplane

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	generatedapi "github.com/jinlong17/multi-agent-desk/internal/controlplane/api/generated"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

func privateFile(t *testing.T, directory, name string, contents []byte) string {
	t.Helper()
	return privateTestFile(t, directory, name, contents)
}

func validConfigMap(t *testing.T, directory string) map[string]any {
	t.Helper()
	return map[string]any{
		"listen": "127.0.0.1:8443", "publicOrigin": "https://control.example.test", "rpId": "example.test",
		"databasePath":       filepath.Join(directory, "server.sqlite"),
		"tlsCertificateFile": privateFile(t, directory, "cert.pem", []byte("certificate")),
		"tlsPrivateKeyFile":  privateFile(t, directory, "key.pem", []byte("private-key")),
		"cursorHmacKeyFile":  privateFile(t, directory, "cursor.key", []byte(base64.RawURLEncoding.EncodeToString(make([]byte, 32)))),
		"trustedProxyCidrs":  []string{"127.0.0.1/32"}, "shutdownTimeout": "5s", "databaseBusyTimeout": "500ms",
	}
}

func writeConfig(t *testing.T, directory string, value any) string {
	t.Helper()
	contents, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return privateFile(t, directory, "config.json", contents)
}

func TestLoadConfigStrictAndPrivate(t *testing.T) {
	directory := privateTestDirectory(t)
	path := writeConfig(t, directory, validConfigMap(t, directory))
	config, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.BusyTimeout() != 500*time.Millisecond || config.shutdownTimeout != 5*time.Second {
		t.Fatalf("durations not loaded: %+v", config)
	}
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"unknown", func(value map[string]any) { value["surprise"] = true }},
		{"listen", func(value map[string]any) { value["listen"] = "localhost" }},
		{"http", func(value map[string]any) { value["publicOrigin"] = "http://control.example.test" }},
		{"wildcard", func(value map[string]any) { value["publicOrigin"] = "https://*.example.test" }},
		{"origin-path", func(value map[string]any) { value["publicOrigin"] = "https://control.example.test/path" }},
		{"origin-default-port", func(value map[string]any) { value["publicOrigin"] = "https://control.example.test:443" }},
		{"origin-ip", func(value map[string]any) { value["publicOrigin"] = "https://127.0.0.1"; value["rpId"] = "127.0.0.1" }},
		{"origin-case", func(value map[string]any) { value["publicOrigin"] = "https://Control.example.test" }},
		{"proxy", func(value map[string]any) { value["trustedProxyCidrs"] = []string{"0.0.0.0/0"} }},
		{"rp", func(value map[string]any) { value["rpId"] = "attacker.test" }},
		{"duration", func(value map[string]any) { value["shutdownTimeout"] = "1s" }},
		{"missing-secret", func(value map[string]any) { value["cursorHmacKeyFile"] = filepath.Join(directory, "missing") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := validConfigMap(t, directory)
			test.mutate(value)
			if _, err := LoadConfig(writeConfig(t, directory, value)); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
	if err := makeTestFileUnsafe(path); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("world-readable config accepted")
	}
}

func testServer(t *testing.T) (*Server, *Store) {
	t.Helper()
	directory := privateTestDirectory(t)
	store := openTestStore(t, filepath.Join(directory, "server.sqlite"))
	server, err := NewServer(Config{Listen: "127.0.0.1:0", PublicOrigin: "https://control.example.test", RPID: "example.test", shutdownTimeout: time.Second}, store)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.bootstrap.clearEphemeral)
	return server, store
}

func request(t *testing.T, server *Server, method, target string, body string, headers http.Header) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header = headers
	server.http.Handler.ServeHTTP(recorder, req)
	return recorder
}

func TestP2HandlersExposeFoundationBootstrapAndAuthenticationOnly(t *testing.T) {
	server, store := testServer(t)
	for _, endpoint := range []struct{ path, status string }{{"/v1/healthz", "ok"}, {"/v1/readyz", "ready"}} {
		response := request(t, server, http.MethodGet, endpoint.path, "", nil)
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("%s code=%d headers=%v", endpoint.path, response.Code, response.Header())
		}
		if _, err := transport.ParseUUIDv7(response.Header().Get("X-Request-ID")); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(response.Body.String(), `"status":"`+endpoint.status+`"`) {
			t.Fatalf("unexpected response %s", response.Body.String())
		}
	}
	version := request(t, server, http.MethodGet, "/v1/version", "", nil)
	if version.Code != http.StatusOK || !strings.Contains(version.Body.String(), `"enabledFeatures":["foundation","bootstrap","passkey","recovery","browser-session"]`) {
		t.Fatalf("version=%s", version.Body.String())
	}
	bootstrap := request(t, server, http.MethodGet, "/v1/bootstrap/status", "", nil)
	if bootstrap.Code != http.StatusOK || !strings.Contains(bootstrap.Body.String(), `"state":"uninitialized"`) {
		t.Fatalf("bootstrap status=%d %s", bootstrap.Code, bootstrap.Body.String())
	}
	current := request(t, server, http.MethodGet, "/v1/auth/current", "", nil)
	if current.Code != http.StatusUnauthorized || !strings.Contains(current.Body.String(), `"code":"unauthenticated"`) {
		t.Fatalf("auth current=%d %s", current.Code, current.Body.String())
	}
	for _, path := range []string{"/v1/devices", "/v1/accounts", "/v1/overview"} {
		response := request(t, server, http.MethodGet, path, "", nil)
		if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
			t.Fatalf("P3+ endpoint %s exposed: %d %s", path, response.Code, response.Body.String())
		}
	}
	_ = store.Close()
	response := request(t, server, http.MethodGet, "/v1/readyz", "", nil)
	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), "server.sqlite") {
		t.Fatalf("unsafe readiness response: %d %s", response.Code, response.Body.String())
	}
}

func TestCompleteContractInventoryMountsOnlyP2AndLeavesLaterPhasesSideEffectFree(t *testing.T) {
	server, store := testServer(t)
	contract, err := generatedapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	tables := make([]string, 0, 4)
	rows, err := store.db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	wantTables := []string{"anchor_devices", "auth_audit_events", "auth_idempotency_operations", "bootstrap_receipts", "bootstrap_state", "browser_sessions", "idempotency_records", "passkeys", "pre_user_audit_events", "recovery_batches", "recovery_codes", "schema_migrations", "server_metadata", "users", "webauthn_ceremonies"}
	if !slices.Equal(tables, wantTables) {
		t.Fatalf("P2 schema inventory mismatch: %v", tables)
	}
	var beforeChanges int64
	if err := store.db.QueryRow("SELECT total_changes()").Scan(&beforeChanges); err != nil {
		t.Fatal(err)
	}

	operationCount := 0
	foundationCount := 0
	p2Paths := map[string]bool{
		"/v1/bootstrap/status": true, "/v1/bootstrap/options": true, "/v1/bootstrap/verify": true, "/v1/bootstrap/ceremonies/{ceremonyId}": true,
		"/v1/auth/current": true, "/v1/auth/logout": true, "/v1/auth/passkeys/options": true, "/v1/auth/passkeys/verify": true,
		"/v1/auth/passkeys/registration/options": true, "/v1/auth/passkeys/registration/verify": true, "/v1/auth/passkeys": true,
		"/v1/auth/passkeys/{passkeyId}": true, "/v1/auth/uv/options": true, "/v1/auth/uv/verify": true, "/v1/auth/recovery/verify": true,
		"/v1/auth/recovery-codes/rotate": true, "/v1/auth/sessions": true, "/v1/auth/sessions/{sessionId}": true,
	}
	for path, item := range contract.Paths.Map() {
		for method := range item.Operations() {
			operationCount++
			if path == "/v1/healthz" || path == "/v1/readyz" || path == "/v1/version" {
				foundationCount++
				continue
			}
			if p2Paths[path] {
				continue
			}
			target := path
			for _, parameter := range []string{"passkeyId", "sessionId", "ceremonyId", "deviceId", "enrollmentId", "id", "commandId"} {
				target = strings.ReplaceAll(target, "{"+parameter+"}", "018f47a0-7b1c-7cc2-8000-000000000001")
			}
			response := request(t, server, strings.ToUpper(method), target, "", nil)
			if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
				t.Fatalf("P3+ operation mounted: %s %s -> %d %s", method, target, response.Code, response.Body.String())
			}
			if len(response.Header().Values("Set-Cookie")) != 0 || response.Header().Get("Location") != "" {
				t.Fatalf("P3+ operation emitted authority: %s %s headers=%v", method, target, response.Header())
			}
		}
	}
	if operationCount != 65 || foundationCount != 3 {
		t.Fatalf("contract inventory operations=%d foundation=%d", operationCount, foundationCount)
	}
	var afterChanges, idempotencyRows, auditRows int64
	if err := store.db.QueryRow("SELECT total_changes()").Scan(&afterChanges); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow("SELECT count(*) FROM idempotency_records").Scan(&idempotencyRows); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow("SELECT count(*) FROM pre_user_audit_events").Scan(&auditRows); err != nil {
		t.Fatal(err)
	}
	if afterChanges != beforeChanges || idempotencyRows != 0 || auditRows != 0 {
		t.Fatalf("unmounted contract caused side effects: changes=%d->%d idempotency=%d audit=%d", beforeChanges, afterChanges, idempotencyRows, auditRows)
	}
}

func TestFoundationMiddlewareRejectsHostileRequests(t *testing.T) {
	server, _ := testServer(t)
	tests := []struct {
		name, target, body string
		headers            http.Header
		status             int
	}{
		{"duplicate-query", "/v1/healthz?a=1&a=2", "", nil, 400},
		{"compressed", "/v1/healthz", "", http.Header{"Content-Encoding": {"gzip"}}, 415},
		{"body", "/v1/healthz", `{}`, http.Header{"Content-Type": {"application/json"}}, 400},
	}
	tooMany := http.Header{}
	for index := range 65 {
		tooMany.Add("X-Test-"+string(rune('A'+index%26)), string(rune(index)))
	}
	tests = append(tests, struct {
		name, target, body string
		headers            http.Header
		status             int
	}{"headers", "/v1/healthz", "", tooMany, 431})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := request(t, server, http.MethodGet, test.target, test.body, test.headers)
			if response.Code != test.status || !strings.Contains(response.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("code=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestRunHonorsCancelledContextWithoutListening(t *testing.T) {
	server, _ := testServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() { done <- server.Run(ctx) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancelled server run did not return within shutdown bound")
	}
}
