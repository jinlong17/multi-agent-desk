package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/app"
	generatedapi "github.com/jinlong17/multi-agent-desk/internal/controlplane/api/generated"
	"github.com/jinlong17/multi-agent-desk/internal/device"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

func runDevices(args []string, stdout, stderr *os.File) error {
	if len(args) < 2 || args[0] != "bootstrap" {
		return domain.NewError(domain.CodeMethodNotFound, "devices command is not available")
	}
	switch args[1] {
	case "prepare":
		return runDeviceBootstrapPrepare(args[2:], stdout, stderr)
	case "prove":
		return runDeviceBootstrapProve(args[2:], stdout, stderr)
	case "activate":
		return runDeviceBootstrapActivate(args[2:], stdout, stderr)
	default:
		return domain.NewError(domain.CodeMethodNotFound, "devices bootstrap command is not available")
	}
}

func runDeviceBootstrapPrepare(args []string, stdout, stderr *os.File) error {
	flags := flag.NewFlagSet("devices bootstrap prepare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	serverOrigin := flags.String("server", "", "canonical HTTPS Control Plane origin")
	out := flags.String("out", "", "absolute owner-only descriptor file")
	name := flags.String("name", "MultiAgentDesk Daemon", "anchor display name")
	caFile := flags.String("ca-file", "", "absolute optional CA certificate file")
	developmentLocalhost := flags.Bool("development-localhost", false, "allow canonical localhost HTTPS origin")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *root == "" || *serverOrigin == "" || *out == "" {
		return domain.NewError(domain.CodeInvalidArgument, "bootstrap prepare requires --root, --server, and --out")
	}
	if err := validateAbsoluteOutput(*out); err != nil {
		return err
	}
	if *caFile != "" {
		if _, err := loadRootCAs(*caFile); err != nil {
			return err
		}
	}
	canonical, err := transport.ParseCanonicalServerOriginV1(*serverOrigin, transport.CanonicalServerOriginOptions{AllowDevelopmentLocalhost: *developmentLocalhost})
	if err != nil {
		return domain.NewError(domain.CodeInvalidArgument, "--server must be one canonical HTTPS origin")
	}
	_, result, err := callRPC(*root, "remote.bootstrap.prepare", domain.CapabilityVaultControl, map[string]any{"server_origin": string(canonical), "name": *name, "allow_development_localhost": *developmentLocalhost}, nil, true)
	if err != nil {
		return err
	}
	var descriptor generatedapi.BootstrapAnchorDescriptorV1
	if err := decodeRPCResult(result, &descriptor); err != nil {
		return err
	}
	encoded, err := app.EncodePublicBootstrapTransfer(descriptor, 64<<10)
	if err != nil {
		return domain.NewError(domain.CodeConflict, "bootstrap descriptor could not be encoded")
	}
	encoded = append(encoded, '\n')
	if err := device.WritePrivateFileAtomic(*out, encoded); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Bootstrap descriptor written to %s\n", *out)
	return err
}

func runDeviceBootstrapProve(args []string, stdout, stderr *os.File) error {
	flags := flag.NewFlagSet("devices bootstrap prove", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	challengePath := flags.String("challenge", "", "absolute owner-only challenge file")
	out := flags.String("out", "", "absolute owner-only proof file")
	caFile := flags.String("ca-file", "", "absolute optional CA certificate file")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *root == "" || *challengePath == "" || *out == "" {
		return domain.NewError(domain.CodeInvalidArgument, "bootstrap prove requires --root, --challenge, and --out")
	}
	if err := validateAbsoluteOutput(*out); err != nil {
		return err
	}
	contents, err := readPrivateTransfer(*challengePath, 256<<10)
	if err != nil {
		return err
	}
	imported, err := app.DecodeBootstrapChallenge(contents)
	if err != nil {
		return err
	}
	refetchedRaw, err := fetchBootstrapCeremony(imported.ServerOrigin, imported.CeremonyId, *caFile, 256<<10)
	if err != nil {
		return err
	}
	refetched, err := app.DecodeBootstrapChallenge(refetchedRaw)
	if err != nil {
		return err
	}
	_, result, err := callRPC(*root, "remote.bootstrap.prove", domain.CapabilityVaultControl, map[string]any{"imported": imported, "refetched": refetched}, nil, false)
	if err != nil {
		return err
	}
	var proof app.BootstrapAnchorProofV1
	if err := decodeRPCResult(result, &proof); err != nil {
		return err
	}
	encoded, err := app.EncodePublicBootstrapTransfer(proof, 4096)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err := device.WritePrivateFileAtomic(*out, encoded); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Bootstrap proof written to %s\n", *out)
	return err
}

func runDeviceBootstrapActivate(args []string, stdout, stderr *os.File) error {
	flags := flag.NewFlagSet("devices bootstrap activate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	receiptPath := flags.String("receipt", "", "absolute owner-only receipt file")
	caFile := flags.String("ca-file", "", "absolute optional CA certificate file")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *root == "" || *receiptPath == "" {
		return domain.NewError(domain.CodeInvalidArgument, "bootstrap activate requires --root and --receipt")
	}
	contents, err := readPrivateTransfer(*receiptPath, 4096)
	if err != nil {
		return err
	}
	imported, err := app.DecodeBootstrapReceipt(contents)
	if err != nil {
		return err
	}
	refetchedRaw, err := fetchBootstrapCeremony(imported.ServerOrigin, imported.CeremonyId, *caFile, 4096)
	if err != nil {
		return err
	}
	refetched, err := app.DecodeBootstrapReceipt(refetchedRaw)
	if err != nil {
		return err
	}
	requestID, result, err := callRPC(*root, "remote.bootstrap.activate", domain.CapabilityVaultControl, map[string]any{"imported": imported, "refetched": refetched}, nil, true)
	if err != nil {
		return err
	}
	return writeCLI(stdout, *jsonOutput, requestID, result, nil)
}

func validateAbsoluteOutput(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Base(path) == "." || filepath.Base(path) == string(filepath.Separator) {
		return domain.NewError(domain.CodeInvalidArgument, "output path must be absolute and clean")
	}
	return device.VerifyPrivateDirectory(filepath.Dir(path))
}

func readPrivateTransfer(path string, limit int64) ([]byte, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, domain.NewError(domain.CodeInvalidArgument, "transfer path must be absolute and clean")
	}
	if err := device.VerifyPrivateFile(path); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "transfer file could not be opened", err)
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(contents)) > limit {
		return nil, domain.NewError(domain.CodeFrameTooLarge, "transfer file exceeds its limit")
	}
	return contents, nil
}

func decodeRPCResult(value any, target any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return domain.NewError(domain.CodeConflict, "daemon response could not be encoded")
	}
	if err := transport.DecodeStrictJSON(strings.NewReader(string(encoded)), int64(len(encoded))+1, target); err != nil {
		return domain.NewError(domain.CodeConflict, "daemon response did not match the bootstrap contract")
	}
	return nil
}

func fetchBootstrapCeremony(serverOrigin, ceremonyID, caFile string, limit int64) ([]byte, error) {
	if _, err := transport.ParseCanonicalServerOriginV1(serverOrigin, transport.CanonicalServerOriginOptions{AllowDevelopmentLocalhost: true}); err != nil {
		return nil, domain.NewError(domain.CodeInvalidArgument, "bootstrap server origin is invalid")
	}
	if _, err := transport.ParseUUIDv7(ceremonyID); err != nil {
		return nil, domain.NewError(domain.CodeInvalidArgument, "bootstrap ceremony ID is invalid")
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile != "" {
		roots, err := loadRootCAs(caFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.RootCAs = roots
	}
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsConfig, Proxy: http.ProxyFromEnvironment, ForceAttemptHTTP2: true,
			ResponseHeaderTimeout: 5 * time.Second, TLSHandshakeTimeout: 5 * time.Second},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return errors.New("redirects are forbidden") },
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, serverOrigin+"/v1/bootstrap/ceremonies/"+ceremonyID, nil)
	if err != nil {
		return nil, domain.NewError(domain.CodeInvalidArgument, "bootstrap ceremony URL is invalid")
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, domain.WrapError(domain.CodeDaemonUnavailable, "bootstrap ceremony could not be fetched over HTTPS", err)
	}
	defer response.Body.Close()
	contents, err := io.ReadAll(io.LimitReader(response.Body, limit+4097))
	if err != nil || int64(len(contents)) > limit+4096 || response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/json" {
		return nil, domain.NewError(domain.CodeDaemonUnavailable, "bootstrap ceremony HTTPS response was invalid")
	}
	var envelope struct {
		APIVersion string                    `json:"apiVersion"`
		Data       json.RawMessage           `json:"data"`
		Meta       generatedapi.ResponseMeta `json:"meta"`
	}
	if err := transport.DecodeStrictJSON(strings.NewReader(string(contents)), int64(len(contents))+1, &envelope); err != nil || envelope.APIVersion != "v1" || len(envelope.Data) > int(limit) {
		return nil, domain.NewError(domain.CodeDaemonUnavailable, "bootstrap ceremony response did not match the contract")
	}
	return append([]byte(nil), envelope.Data...), nil
}

func loadRootCAs(path string) (*x509.CertPool, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, domain.NewError(domain.CodeInvalidArgument, "CA file path must be absolute and clean")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 1 || info.Size() > 1<<20 {
		return nil, domain.NewError(domain.CodeInvalidArgument, "CA file must be a bounded regular file")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, domain.WrapError(domain.CodePermissionDenied, "CA file could not be read", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(contents) {
		return nil, domain.NewError(domain.CodeInvalidArgument, "CA file does not contain a certificate")
	}
	return pool, nil
}
