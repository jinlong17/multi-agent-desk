package controlplane

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

type Config struct {
	Listen                    string   `json:"listen"`
	PublicOrigin              string   `json:"publicOrigin"`
	RPID                      string   `json:"rpId"`
	DatabasePath              string   `json:"databasePath"`
	TLSCertificateFile        string   `json:"tlsCertificateFile"`
	TLSPrivateKeyFile         string   `json:"tlsPrivateKeyFile"`
	CursorHMACKeyFile         string   `json:"cursorHmacKeyFile"`
	TrustedProxyCIDRs         []string `json:"trustedProxyCidrs"`
	ShutdownTimeout           string   `json:"shutdownTimeout"`
	DatabaseBusyTimeout       string   `json:"databaseBusyTimeout"`
	DevelopmentAllowLocalhost bool     `json:"developmentAllowLocalhost,omitempty"`
	shutdownTimeout           time.Duration
	busyTimeout               time.Duration
	cursorHMACKey             [32]byte
}

func (c Config) BusyTimeout() time.Duration { return c.busyTimeout }

func LoadConfig(path string) (Config, error) {
	var config Config
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return config, fmt.Errorf("config path must be absolute and clean")
	}
	if err := verifyPrivateFile(path); err != nil {
		return config, err
	}
	file, err := os.Open(path)
	if err != nil {
		return config, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()
	if err := transport.DecodeStrictJSON(file, 64<<10, &config); err != nil {
		return config, fmt.Errorf("decode config: %w", err)
	}
	if err := config.validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (c *Config) validate() error {
	host, portText, err := net.SplitHostPort(c.Listen)
	port, portErr := strconv.Atoi(portText)
	if err != nil || portErr != nil || host == "" || host == "*" || port < 1 || port > 65535 {
		return fmt.Errorf("listen must be an explicit host:port")
	}
	if _, err := transport.ParseCanonicalServerOriginV1(c.PublicOrigin, transport.CanonicalServerOriginOptions{AllowDevelopmentLocalhost: c.DevelopmentAllowLocalhost}); err != nil {
		return fmt.Errorf("publicOrigin must be one canonical fixed HTTPS origin: %w", err)
	}
	origin, _ := url.Parse(c.PublicOrigin)
	originHost := origin.Hostname()
	if c.RPID == "" || c.RPID != strings.ToLower(c.RPID) || strings.HasSuffix(c.RPID, ".") || strings.ContainsAny(c.RPID, "*/:") || !(originHost == c.RPID || strings.HasSuffix(originHost, "."+c.RPID)) {
		return fmt.Errorf("rpId must be a fixed registrable suffix of publicOrigin")
	}
	for _, path := range []string{c.DatabasePath, c.TLSCertificateFile, c.TLSPrivateKeyFile, c.CursorHMACKeyFile} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return fmt.Errorf("all configured file paths must be absolute and clean")
		}
	}
	for _, path := range []string{c.TLSCertificateFile, c.TLSPrivateKeyFile, c.CursorHMACKeyFile} {
		if err := verifyPrivateFile(path); err != nil {
			return err
		}
	}
	for _, network := range c.TrustedProxyCIDRs {
		_, parsed, err := net.ParseCIDR(network)
		if err != nil || parsed.String() != network || parsed.String() == "0.0.0.0/0" || parsed.String() == "::/0" {
			return fmt.Errorf("trusted proxy CIDR is invalid or overly broad")
		}
	}
	c.shutdownTimeout, err = parseBoundedDuration(c.ShutdownTimeout, 5*time.Second, 2*time.Minute, 20*time.Second)
	if err != nil {
		return fmt.Errorf("shutdownTimeout: %w", err)
	}
	c.busyTimeout, err = parseBoundedDuration(c.DatabaseBusyTimeout, 100*time.Millisecond, 30*time.Second, defaultBusyTimeout)
	if err != nil {
		return fmt.Errorf("databaseBusyTimeout: %w", err)
	}
	secret, err := os.ReadFile(c.CursorHMACKeyFile)
	if err != nil {
		return fmt.Errorf("read cursor HMAC key: %w", err)
	}
	decoded := secret
	if candidate, decodeErr := base64.RawURLEncoding.DecodeString(strings.TrimSpace(string(secret))); decodeErr == nil {
		decoded = candidate
	}
	if len(decoded) != 32 {
		return fmt.Errorf("cursor HMAC key must contain exactly 32 bytes")
	}
	copy(c.cursorHMACKey[:], decoded)
	return nil
}

func parseBoundedDuration(value string, minimum, maximum, fallback time.Duration) (time.Duration, error) {
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration < minimum || duration > maximum {
		return 0, fmt.Errorf("duration must be between %s and %s", minimum, maximum)
	}
	return duration, nil
}
