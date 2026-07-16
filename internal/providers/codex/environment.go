package codex

import (
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	maxNoProxyBytes   = 4096
	maxNoProxyEntries = 64
	maxNoProxyEntry   = 255
)

// NetworkEnvironment returns the narrowly allowlisted inherited settings that
// an official Codex child may need for network egress. Proxy URLs containing
// credentials or non-HTTP schemes are rejected so secret-bearing environment
// variables remain outside the Provider trust boundary.
func NetworkEnvironment(lookup func(string) string) []string {
	if lookup == nil {
		lookup = os.Getenv
	}
	groups := [][]string{
		{"http_proxy", "HTTP_PROXY"},
		{"https_proxy", "HTTPS_PROXY"},
		{"all_proxy", "ALL_PROXY"},
		{"no_proxy", "NO_PROXY"},
	}
	result := make([]string, 0, len(groups))
	for _, group := range groups {
		for _, name := range group {
			value := lookup(name)
			if value == "" {
				continue
			}
			if validNetworkEnvironment(name, value) {
				result = append(result, name+"="+value)
			}
			break
		}
	}
	return result
}

func validNetworkEnvironment(name, value string) bool {
	if len(value) > 4096 || strings.IndexFunc(value, func(r rune) bool { return r == 0 || r == '\r' || r == '\n' }) >= 0 {
		return false
	}
	if strings.EqualFold(name, "no_proxy") {
		return validNoProxy(value)
	}
	if len(value) > 2048 {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return false
	}
	return true
}

func validNoProxy(value string) bool {
	if len(value) == 0 || len(value) > maxNoProxyBytes {
		return false
	}
	entries := strings.Split(value, ",")
	if len(entries) == 0 || len(entries) > maxNoProxyEntries {
		return false
	}
	for _, entry := range entries {
		if !validNoProxyEntry(entry) {
			return false
		}
	}
	return true
}

func validNoProxyEntry(entry string) bool {
	if len(entry) == 0 || len(entry) > maxNoProxyEntry {
		return false
	}
	for index := 0; index < len(entry); index++ {
		if entry[index] <= 0x20 || entry[index] >= 0x7f {
			return false
		}
	}
	if entry == "*" {
		return true
	}
	if strings.ContainsAny(entry, "@=?#\\") {
		return false
	}
	if net.ParseIP(entry) != nil {
		return true
	}
	if _, _, err := net.ParseCIDR(entry); err == nil {
		return true
	}
	if strings.HasPrefix(entry, "[") {
		host, port, err := net.SplitHostPort(entry)
		return err == nil && strings.Contains(host, ":") && net.ParseIP(host) != nil && validNoProxyPort(port)
	}
	host := entry
	if strings.Count(entry, ":") == 1 {
		var port string
		var err error
		host, port, err = net.SplitHostPort(entry)
		if err != nil || !validNoProxyPort(port) {
			return false
		}
	} else if strings.Contains(entry, ":") {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	return validNoProxyDNSName(host)
}

func validNoProxyPort(port string) bool {
	if port == "" {
		return false
	}
	for index := 0; index < len(port); index++ {
		if port[index] < '0' || port[index] > '9' {
			return false
		}
	}
	value, err := strconv.Atoi(port)
	return err == nil && value >= 1 && value <= 65535
}

func validNoProxyDNSName(name string) bool {
	if strings.HasPrefix(name, "*.") {
		name = strings.TrimPrefix(name, "*.")
	} else if strings.HasPrefix(name, ".") {
		name = strings.TrimPrefix(name, ".")
	}
	if len(name) == 0 || len(name) > 253 {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || !asciiAlphaNumeric(label[0]) || !asciiAlphaNumeric(label[len(label)-1]) {
			return false
		}
		for index := 1; index+1 < len(label); index++ {
			if !asciiAlphaNumeric(label[index]) && label[index] != '-' {
				return false
			}
		}
	}
	return true
}

func asciiAlphaNumeric(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}
