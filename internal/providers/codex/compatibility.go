package codex

import (
	"slices"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

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
			MethodApprovalCommand, MethodApprovalFileChange, MethodApprovalPermissions},
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
