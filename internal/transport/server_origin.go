package transport

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/idna"
)

// CanonicalServerOriginV1 is the immutable namespace for every remote Device
// identity. The stored value is deliberately a string so callers cannot later
// reserialize a URL differently and accidentally rebind an identity.
type CanonicalServerOriginV1 string

type CanonicalServerOriginOptions struct {
	AllowDevelopmentLocalhost bool
}

// ParseCanonicalServerOriginV1 accepts only the exact canonical wire spelling.
// Unicode and uppercase hosts can be normalized explicitly with
// NormalizeCanonicalServerOriginV1, but are never accepted as stored identity
// input because parse/serialize drift would create an ambiguous namespace.
func ParseCanonicalServerOriginV1(value string, options CanonicalServerOriginOptions) (CanonicalServerOriginV1, error) {
	canonical, err := NormalizeCanonicalServerOriginV1(value, options)
	if err != nil {
		return "", err
	}
	if string(canonical) != value {
		return "", fmt.Errorf("server origin is not in canonical wire form")
	}
	return canonical, nil
}

// NormalizeCanonicalServerOriginV1 converts an operator-entered HTTPS origin
// to its canonical IDNA-ASCII form. Product state must subsequently use the
// exact returned bytes.
func NormalizeCanonicalServerOriginV1(value string, options CanonicalServerOriginOptions) (CanonicalServerOriginV1, error) {
	if value == "" || len(value) > 2048 || strings.TrimSpace(value) != value || strings.Contains(value, "%") {
		return "", fmt.Errorf("server origin is invalid")
	}
	parsed, err := url.Parse(value)
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Opaque != "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.Path != "" || parsed.RawPath != "" {
		return "", fmt.Errorf("server origin must be one HTTPS origin")
	}
	host := parsed.Hostname()
	if host == "" || strings.HasSuffix(host, ".") || strings.ContainsAny(host, "*[]") {
		return "", fmt.Errorf("server origin host is invalid")
	}
	asciiHost, err := idna.Lookup.ToASCII(strings.ToLower(host))
	if err != nil || asciiHost == "" || len(asciiHost) > 253 || strings.HasSuffix(asciiHost, ".") || strings.ContainsAny(asciiHost, "*[]%") {
		return "", fmt.Errorf("server origin host is invalid")
	}
	isLocalhost := asciiHost == "localhost" || strings.HasSuffix(asciiHost, ".localhost")
	if net.ParseIP(asciiHost) != nil || isLocalhost && !options.AllowDevelopmentLocalhost {
		return "", fmt.Errorf("server origin host is forbidden")
	}
	for _, label := range strings.Split(asciiHost, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("server origin host is invalid")
		}
		for _, char := range label {
			if !(char == '-' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9') {
				return "", fmt.Errorf("server origin host is not IDNA ASCII")
			}
		}
	}
	port := parsed.Port()
	if port == "443" {
		port = ""
	}
	if port != "" {
		parsedPort, err := strconv.Atoi(port)
		if err != nil || parsedPort < 1 || parsedPort > 65535 || strconv.Itoa(parsedPort) != port {
			return "", fmt.Errorf("server origin port is invalid")
		}
	}
	hostPort := asciiHost
	if port != "" {
		hostPort = net.JoinHostPort(asciiHost, port)
	}
	canonical := "https://" + hostPort
	reparsed, err := url.Parse(canonical)
	if err != nil || reparsed.String() != canonical || reparsed.Hostname() != asciiHost {
		return "", fmt.Errorf("server origin serialization is unstable")
	}
	return CanonicalServerOriginV1(canonical), nil
}

// Frame encodes each field as an unsigned 32-bit big-endian byte length
// followed by the raw bytes. It is shared by the P0/P2 signature and AAD
// domains and refuses fields that cannot have a unique representation.
func Frame(fields ...[]byte) ([]byte, error) {
	var framed bytes.Buffer
	for _, field := range fields {
		if uint64(len(field)) > uint64(^uint32(0)) {
			return nil, fmt.Errorf("framed field is too large")
		}
		if err := binary.Write(&framed, binary.BigEndian, uint32(len(field))); err != nil {
			return nil, fmt.Errorf("frame field length: %w", err)
		}
		_, _ = framed.Write(field)
	}
	return framed.Bytes(), nil
}
