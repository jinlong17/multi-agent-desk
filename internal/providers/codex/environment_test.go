package codex

import (
	"reflect"
	"strings"
	"testing"
)

func TestNetworkEnvironmentAllowsOnlyCredentialFreeHTTPProxySettings(t *testing.T) {
	values := map[string]string{
		"http_proxy":  "http://proxy.internal:8080",
		"HTTP_PROXY":  "http://ignored.internal:8080",
		"https_proxy": "https://proxy.internal:8443/",
		"all_proxy":   "socks5://proxy.internal:1080",
		"NO_PROXY":    "localhost,127.0.0.1",
	}
	got := NetworkEnvironment(func(name string) string { return values[name] })
	want := []string{"http_proxy=http://proxy.internal:8080", "https_proxy=https://proxy.internal:8443/", "NO_PROXY=localhost,127.0.0.1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("network environment=%v want=%v", got, want)
	}
}

func TestNetworkEnvironmentRejectsCredentialsControlsAndPaths(t *testing.T) {
	for _, value := range []string{
		"http://user:synthetic-secret@proxy.invalid:8080",
		"http://proxy.internal:8080/path",
		"http://proxy.internal:8080?token=secret",
		"http://proxy.internal:8080\nOPENAI_API_KEY=secret",
	} {
		if got := NetworkEnvironment(func(name string) string {
			if name == "https_proxy" {
				return value
			}
			return ""
		}); len(got) != 0 {
			t.Fatalf("unsafe proxy %q was inherited as %v", value, got)
		}
	}
}

func TestNoProxyAcceptsOnlyExactNetworkEntryGrammar(t *testing.T) {
	positive := []string{
		"*",
		"localhost",
		".example.com",
		"*.example.com",
		"127.0.0.1",
		"2001:db8::1",
		"2001:db8::1:443", // a valid unbracketed IPv6 literal, not a host/port pair
		"10.0.0.0/8",
		"2001:db8::/32",
		"example.com:1",
		"example.com:65535",
		"127.0.0.1:8080",
		"[2001:db8::1]:8443",
		"localhost,127.0.0.1,.example.com,2001:db8::/32",
	}
	for _, value := range positive {
		if !validNetworkEnvironment("NO_PROXY", value) {
			t.Errorf("safe NO_PROXY value rejected: %q", value)
		}
	}

	sixtyFiveEntries := strings.Repeat("a,", 64) + "a"
	oversizedDNSLabel := strings.Repeat("a", 64) + ".example"
	oversizedDNSName := strings.Repeat("a.", 126) + "aa"
	negative := []string{
		"",
		",localhost",
		"localhost,",
		"localhost,,example.com",
		strings.Repeat("a", 256),
		strings.Repeat("a", 4097),
		sixtyFiveEntries,
		"local host",
		"localhost\t",
		"localhost\nOPENAI_API_KEY=synthetic",
		"éxample.com",
		oversizedDNSLabel,
		oversizedDNSName,
		"-bad.example",
		"bad-.example",
		"bad..example",
		"http://proxy.invalid",
		"user:synthetic-secret@proxy.invalid",
		"token=synthetic",
		"example.com/path",
		"example.com?token=synthetic",
		"example.com#fragment",
		"fe80::1%lo0",
		"10.0.0.0/99",
		"2001:db8::1:65536",
		"example.com:0",
		"example.com:65536",
		"example.com:https",
		"[127.0.0.1]:8080",
	}
	for _, value := range negative {
		if validNetworkEnvironment("NO_PROXY", value) {
			t.Errorf("unsafe NO_PROXY value accepted: %q", value)
		}
	}
}

func TestNetworkEnvironmentOmitsWholeNoProxyValueWhenAnyEntryIsUnsafe(t *testing.T) {
	values := map[string]string{
		"https_proxy": "https://proxy.invalid:8443",
		"NO_PROXY":    "localhost,token=synthetic",
	}
	got := NetworkEnvironment(func(name string) string { return values[name] })
	want := []string{"https_proxy=https://proxy.invalid:8443"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("network environment=%v want=%v", got, want)
	}
}
