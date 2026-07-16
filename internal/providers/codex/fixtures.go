package codex

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

// Fixtures are synthetic and contain no account, authorization, or credential
// values. They exercise only the response-key shapes recorded by the Spike.
//
//go:embed testdata/*.jsonl
var fixtureFiles embed.FS

type FixtureResult struct {
	Version   string
	Account   AccountSnapshot
	Usage     UsageProjection
	Methods   []string
	Approval  ApprovalRequest
	Sanitized bool
}

func ReplayFixture(version string) (FixtureResult, error) {
	if version == "" {
		return FixtureResult{}, domain.NewError(domain.CodeInvalidArgument, "fixture version is required")
	}
	path := "testdata/" + version + ".jsonl"
	data, err := fs.ReadFile(fixtureFiles, path)
	if err != nil {
		return FixtureResult{}, domain.NewError(domain.CodeNotFound, "codex fixture was not found")
	}
	rows, err := fixtureLines(data)
	if err != nil {
		return FixtureResult{}, err
	}
	result := FixtureResult{Version: version, Sanitized: true}
	observedAt := time.Unix(0, 0).UTC()
	for _, row := range rows {
		var envelope struct {
			Kind   string          `json:"kind"`
			Method string          `json:"method"`
			Body   json.RawMessage `json:"body"`
		}
		if err := DecodeObject(row, &envelope); err != nil {
			return FixtureResult{}, err
		}
		switch envelope.Kind {
		case "initialize":
			result.Methods = append(result.Methods, MethodAccountRead, MethodAccountRateLimits, MethodAccountUsage)
		case "account":
			account, err := DecodeAccountResponse(envelope.Body, observedAt)
			if err != nil {
				return FixtureResult{}, err
			}
			result.Account = account
		case "usage":
			usage, err := DecodeUsageResponse(envelope.Body, version, observedAt)
			if err != nil {
				return FixtureResult{}, err
			}
			result.Usage = usage
		case "approval":
			approval, err := DecodeApprovalServerRequest(envelope.Method, envelope.Body)
			if err != nil {
				return FixtureResult{}, err
			}
			result.Approval = approval
		default:
			return FixtureResult{}, domain.NewError(domain.CodeProviderProtocolError, "fixture kind is unmapped")
		}
	}
	if len(result.Methods) == 0 || result.Usage.SourceVersion != version || result.Approval.ProviderApprovalID == "" {
		return FixtureResult{}, domain.NewError(domain.CodeProviderProtocolError, "fixture is incomplete")
	}
	return result, nil
}

func fixtureLines(data []byte) ([][]byte, error) {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return nil, domain.NewError(domain.CodeProviderProtocolError, "fixture is empty")
	}
	result := make([][]byte, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) > MaxFrameBytes {
			return nil, domain.NewError(domain.CodeFrameTooLarge, "fixture frame is too large")
		}
		if err := validateJSON([]byte(line)); err != nil {
			return nil, fmt.Errorf("fixture frame invalid: %w", err)
		}
		result = append(result, []byte(line))
	}
	return result, nil
}
