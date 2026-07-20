package codex

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

// CapabilityDigest binds a selector preview to the exact ordered method set
// accepted by the compatibility row. It contains no Provider identity or
// credential material.
func CapabilityDigest(capabilities CapabilitySet) string {
	encoded, _ := json.Marshal(capabilities.Methods)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

// RequireSelectorPlatform enforces the narrower platform acceptance boundary
// for the explicit multi-account selector. General app-server schema evidence
// does not by itself accept multi-account identity binding on that platform.
func RequireSelectorPlatform(descriptor BinaryDescriptor) error {
	switch descriptor.Platform {
	case "linux":
		if descriptor.Architecture == "amd64" {
			return nil
		}
	case "darwin":
		return domain.NewError(domain.CodeProviderIdentityPending,
			"Codex multi-account selector identity acceptance is pending on macOS")
	case "windows":
		return domain.NewError(domain.CodeProviderPlatformUnsupported,
			"Codex multi-account selector is unsupported on Windows")
	}
	return domain.NewError(domain.CodeProviderPlatformUnsupported,
		"Codex multi-account selector is supported only on Linux amd64")
}

var compatibilityRows = []CompatibilityRow{
	{
		Version:           "0.142.5",
		SchemaFingerprint: "e29e1e39ef9a45a003170856bb95ac665e3ab06a0ae6c346fbaec854767d7c61",
		Methods:           []string{MethodAccountRead, MethodAccountRateLimits, MethodAccountUsage},
	},
	{
		Version:           "0.143.0",
		SchemaFingerprint: "289e8c92a09b65a11cbaa32b879e43bf2e07bbab84511bdaabaa93cbd658c287",
		Methods:           []string{MethodAccountRead, MethodAccountRateLimits, MethodAccountUsage},
	},
	{
		Version:           "0.144.2",
		SchemaFingerprint: "a1a35476587fe9bbfbe9e291b5200b8bc541df8c00241fe578d285ff26996e1c",
		Methods: []string{MethodAccountRead, MethodAccountRateLimits, MethodAccountUsage, MethodRefreshAuthTokens,
			MethodApprovalCommand, MethodApprovalFileChange, MethodApprovalPermissions,
			MethodThreadStart, MethodTurnStart, MethodTurnInterrupt},
	},
}

func CompatibilityRows() []CompatibilityRow {
	rows := make([]CompatibilityRow, len(compatibilityRows))
	for i, row := range compatibilityRows {
		rows[i] = CompatibilityRow{Version: row.Version, SchemaFingerprint: row.SchemaFingerprint,
			Methods: slices.Clone(row.Methods), Experimental: slices.Clone(row.Experimental)}
	}
	return rows
}

func CapabilitiesFor(descriptor BinaryDescriptor) (CapabilitySet, error) {
	if descriptor.Provider != "" && descriptor.Provider != ProviderName {
		return CapabilitySet{}, domain.NewError(domain.CodeInvalidArgument, "provider descriptor is not codex")
	}
	if descriptor.Version == "" || descriptor.Path == "" {
		return CapabilitySet{}, domain.NewError(domain.CodeInvalidArgument, "provider descriptor is incomplete")
	}
	for _, row := range compatibilityRows {
		if row.Version != descriptor.Version {
			continue
		}
		if descriptor.SchemaFingerprint == "" || descriptor.SchemaFingerprint != row.SchemaFingerprint {
			return CapabilitySet{Provider: ProviderName, BinaryPath: descriptor.Path, Version: descriptor.Version,
					Platform: descriptor.Platform, Architecture: descriptor.Architecture,
					SchemaFingerprint: descriptor.SchemaFingerprint, Status: CapabilityUnsupported},
				domain.NewError(domain.CodeProviderVersionUnsupported, "codex schema fingerprint is unsupported")
		}
		return CapabilitySet{Provider: ProviderName, BinaryPath: descriptor.Path, Version: descriptor.Version,
			Platform: descriptor.Platform, Architecture: descriptor.Architecture,
			SchemaFingerprint: descriptor.SchemaFingerprint, Methods: slices.Clone(row.Methods),
			Experimental: slices.Clone(row.Experimental), Status: CapabilitySupported}, nil
	}
	return CapabilitySet{Provider: ProviderName, BinaryPath: descriptor.Path, Version: descriptor.Version,
			Platform: descriptor.Platform, Architecture: descriptor.Architecture,
			SchemaFingerprint: descriptor.SchemaFingerprint, Status: CapabilityUnsupported},
		domain.NewError(domain.CodeProviderVersionUnsupported, "codex version is unsupported")
}

func (c CapabilitySet) Allows(method string) bool {
	return c.Status == CapabilitySupported && slices.Contains(c.Methods, method)
}
