package codex

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

type DiscoverOptions struct {
	Override     string
	Paths        []string
	VersionArgs  []string
	Timeout      time.Duration
	Platform     string
	Architecture string
}

type ProbeOptions struct {
	SchemaFingerprint string
	SchemaDirectory   string
	Timeout           time.Duration
}

var versionPattern = regexp.MustCompile(`(?:^|[^0-9])v?([0-9]+\.[0-9]+\.[0-9]+)(?:[^0-9]|$)`)

func Discover(ctx context.Context, options DiscoverOptions) (BinaryDescriptor, error) {
	if ctx == nil {
		return BinaryDescriptor{}, domain.NewError(domain.CodeInvalidArgument, "discovery context is required")
	}
	paths := candidatePaths(options)
	if len(paths) == 0 {
		return BinaryDescriptor{}, domain.NewError(domain.CodeNotFound, "codex binary was not configured")
	}
	args := slices.Clone(options.VersionArgs)
	if len(args) == 0 {
		args = []string{"--version"}
	}
	for _, path := range paths {
		if err := validateExecutable(path); err != nil {
			continue
		}
		version, err := probeVersion(ctx, path, args, options.Timeout)
		if err != nil {
			continue
		}
		platform := options.Platform
		if platform == "" {
			platform = runtime.GOOS
		}
		architecture := options.Architecture
		if architecture == "" {
			architecture = runtime.GOARCH
		}
		return BinaryDescriptor{Provider: ProviderName, Path: path, Version: version,
			Platform: platform, Architecture: architecture}, nil
	}
	return BinaryDescriptor{}, domain.NewError(domain.CodeProviderFailed, "no usable codex binary was found")
}

// BinaryFingerprint binds enrollment to the exact executable bytes and
// descriptor, preventing a same-path/same-version replacement.
func BinaryFingerprint(descriptor BinaryDescriptor) (string, error) {
	if err := validateExecutable(descriptor.Path); err != nil {
		return "", domain.NewError(domain.CodeProviderFailed, "codex binary cannot be fingerprinted")
	}
	file, err := os.Open(descriptor.Path)
	if err != nil {
		return "", domain.NewError(domain.CodeProviderFailed, "codex binary cannot be fingerprinted")
	}
	defer file.Close()
	hash := sha256.New()
	_, _ = hash.Write([]byte(descriptor.Path + "\x00" + descriptor.Version + "\x00" + descriptor.Platform + "\x00" + descriptor.Architecture + "\x00"))
	if _, err := io.Copy(hash, file); err != nil {
		return "", domain.NewError(domain.CodeProviderFailed, "codex binary cannot be fingerprinted")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func Probe(ctx context.Context, descriptor BinaryDescriptor, options ProbeOptions) (CapabilitySet, error) {
	if descriptor.Provider == "" {
		descriptor.Provider = ProviderName
	}
	if descriptor.SchemaFingerprint == "" {
		descriptor.SchemaFingerprint = options.SchemaFingerprint
	}
	if descriptor.SchemaFingerprint == "" && options.SchemaDirectory != "" {
		fingerprint, err := FingerprintSchema(options.SchemaDirectory)
		if err != nil {
			return CapabilitySet{}, err
		}
		descriptor.SchemaFingerprint = fingerprint
	}
	if descriptor.SchemaFingerprint == "" && options.SchemaDirectory == "" {
		directory, err := os.MkdirTemp("", "multidesk-codex-schema-")
		if err != nil {
			return CapabilitySet{}, domain.WrapError(domain.CodeProviderFailed, "codex schema workspace could not be created", err)
		}
		defer os.RemoveAll(directory)
		if err := GenerateSchema(ctx, descriptor, directory, options.Timeout); err != nil {
			return CapabilitySet{}, err
		}
		fingerprint, err := FingerprintSchema(directory)
		if err != nil {
			return CapabilitySet{}, err
		}
		descriptor.SchemaFingerprint = fingerprint
	}
	return CapabilitiesFor(descriptor)
}

func GenerateSchema(ctx context.Context, descriptor BinaryDescriptor, outputDirectory string, timeout time.Duration) error {
	if ctx == nil || descriptor.Path == "" || outputDirectory == "" {
		return domain.NewError(domain.CodeInvalidArgument, "codex schema probe is incomplete")
	}
	if err := validateExecutable(descriptor.Path); err != nil {
		return domain.NewError(domain.CodeProviderFailed, "codex binary is not executable")
	}
	if err := os.MkdirAll(outputDirectory, 0o700); err != nil {
		return domain.WrapError(domain.CodeProviderFailed, "codex schema directory could not be created", err)
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, descriptor.Path, "app-server", "generate-json-schema", "--out", outputDirectory)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	var stdout, stderr boundedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(probeCtx.Err(), context.DeadlineExceeded) {
			return domain.NewError(domain.CodeDeadlineExceeded, "codex schema probe timed out")
		}
		return domain.NewError(domain.CodeProviderFailed, "codex schema probe failed")
	}
	if stdout.overflow || stderr.overflow {
		return domain.NewError(domain.CodeFrameTooLarge, "codex schema probe output is too large")
	}
	return nil
}

func candidatePaths(options DiscoverOptions) []string {
	paths := make([]string, 0, len(options.Paths)+6)
	if options.Override != "" {
		paths = append(paths, options.Override)
	}
	if configured := os.Getenv("MULTIDESK_CODEX_BINARY"); configured != "" {
		paths = append(paths, configured)
	}
	paths = append(paths, options.Paths...)
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			paths = append(paths, filepath.Join(local, "Programs", "codex", "codex.exe"))
		}
	} else {
		if home, err := os.UserHomeDir(); err == nil {
			paths = append(paths, filepath.Join(home, ".local", "bin", "codex"))
		}
		paths = append(paths, "/opt/homebrew/bin/codex", "/usr/local/bin/codex", "/usr/bin/codex")
	}
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if _, ok := seen[absolute]; ok {
			continue
		}
		seen[absolute] = struct{}{}
		result = append(result, absolute)
	}
	return result
}

func validateExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("codex path is not a regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return errors.New("codex path is not executable")
	}
	return nil
}

func probeVersion(ctx context.Context, path string, args []string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, path, args...)
	// Version discovery uses an absolute path and no shell. Keep the
	// environment minimal so arbitrary secret-bearing variables are not passed.
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	var stdout, stderr boundedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(probeCtx.Err(), context.DeadlineExceeded) {
			return "", domain.NewError(domain.CodeDeadlineExceeded, "codex version probe timed out")
		}
		return "", domain.NewError(domain.CodeProviderFailed, "codex version probe failed")
	}
	if stdout.overflow || stderr.overflow {
		return "", domain.NewError(domain.CodeFrameTooLarge, "codex version probe output is too large")
	}
	return parseVersion(stdout.String() + "\n" + stderr.String())
}

func parseVersion(output string) (string, error) {
	if !utf8.ValidString(output) {
		return "", domain.NewError(domain.CodeProviderProtocolError, "codex version output is invalid")
	}
	match := versionPattern.FindStringSubmatch(output)
	if len(match) != 2 {
		return "", domain.NewError(domain.CodeProviderVersionUnsupported, "codex version output is not recognized")
	}
	return match[1], nil
}

// FingerprintSchema hashes sorted relative paths and canonical parsed JSON.
// Generated aggregate schemas may reorder object keys across identical runs;
// canonicalization removes only that non-semantic ordering. Invalid JSON,
// duplicate keys, symlinks, oversized files, and traversal fail closed.
func FingerprintSchema(root string) (string, error) {
	if root == "" {
		return "", domain.NewError(domain.CodeInvalidArgument, "schema directory is required")
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", domain.NewError(domain.CodeNotFound, "schema directory was not found")
	}
	files := make([]string, 0)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return domain.NewError(domain.CodeProviderProtocolError, "codex schema contains a symlink")
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return domain.NewError(domain.CodeProviderProtocolError, "codex schema contains a non-regular file")
		}
		if len(files) >= MaxSchemaFiles {
			return domain.NewError(domain.CodeFrameTooLarge, "codex schema has too many files")
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		if _, ok := err.(*domain.Error); ok {
			return "", err
		}
		return "", domain.WrapError(domain.CodeProviderFailed, "codex schema could not be walked", err)
	}
	sort.Strings(files)
	hash := sha256.New()
	for _, path := range files {
		relative, err := filepath.Rel(root, path)
		if err != nil || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", domain.NewError(domain.CodeProviderProtocolError, "codex schema path is invalid")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", domain.WrapError(domain.CodeProviderFailed, "codex schema file could not be read", err)
		}
		if len(data) > MaxSchemaFileBytes {
			return "", domain.NewError(domain.CodeFrameTooLarge, "codex schema file is too large")
		}
		canonical, err := canonicalSchemaJSON(data)
		if err != nil {
			return "", err
		}
		if _, err := hash.Write([]byte(filepath.ToSlash(relative) + "\x00")); err != nil {
			return "", err
		}
		if _, err := hash.Write(canonical); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func canonicalSchemaJSON(data []byte) ([]byte, error) {
	structure := json.NewDecoder(bytes.NewReader(data))
	structure.UseNumber()
	if err := validateValue(structure); err != nil {
		return nil, domain.NewError(domain.CodeProviderProtocolError, "codex schema JSON is invalid or contains duplicate keys")
	}
	var trailing any
	if err := structure.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, domain.NewError(domain.CodeProviderProtocolError, "codex schema JSON contains trailing data")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, domain.NewError(domain.CodeProviderProtocolError, "codex schema JSON is invalid")
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, domain.NewError(domain.CodeProviderProtocolError, "codex schema JSON cannot be canonicalized")
	}
	canonical := bytes.TrimSuffix(output.Bytes(), []byte("\n"))
	return canonical, nil
}

type boundedBuffer struct {
	bytes.Buffer
	overflow bool
}

func (b *boundedBuffer) Write(data []byte) (int, error) {
	if b.Len()+len(data) > MaxProbeOutput {
		b.overflow = true
		remaining := MaxProbeOutput - b.Len()
		if remaining > 0 {
			_, _ = b.Buffer.Write(data[:remaining])
		}
		return len(data), nil
	}
	return b.Buffer.Write(data)
}

func (b *boundedBuffer) String() string { return b.Buffer.String() }
