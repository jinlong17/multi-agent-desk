package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/device"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
	"github.com/jinlong17/multi-agent-desk/internal/providers/codex"
)

func runClient(args []string, stdout, stderr *os.File) error {
	if len(args) == 0 {
		return domain.NewError(domain.CodeInvalidArgument, "client command is required")
	}
	if args[0] != "list" {
		return domain.NewError(domain.CodeUnsupportedPlatform, "client provisioning and rotation remain offline-only in Phase 1")
	}
	flags := flag.NewFlagSet("client list", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args[1:]); err != nil || *root == "" {
		return domain.NewError(domain.CodeInvalidArgument, "client list requires --root")
	}
	return runRPCCommand(*root, "client.list", domain.CapabilityClientAdmin, nil, nil, false, *jsonOutput, stdout)
}

func runAccounts(args []string, stdout, stderr *os.File) error {
	if len(args) == 0 {
		return domain.NewError(domain.CodeInvalidArgument, "accounts command is required")
	}
	action := args[0]
	flags := flag.NewFlagSet("accounts "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	name := flags.String("name", "", "operator display name")
	provider := flags.String("provider", domain.ProviderCodex, "Provider name")
	subjectDigest := flags.String("subject-digest", "", "one-way Provider subject digest")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args[1:]); err != nil || *root == "" {
		return domain.NewError(domain.CodeInvalidArgument, "accounts command requires --root")
	}
	positionals := flags.Args()
	switch action {
	case "list":
		return runRPCCommand(*root, "accounts.list", domain.CapabilityMetadataRead, nil, nil, false, *jsonOutput, stdout)
	case "show":
		if len(positionals) != 1 {
			return domain.NewError(domain.CodeInvalidArgument, "account ID is required")
		}
		return runRPCCommand(*root, "accounts.show", domain.CapabilityMetadataRead, map[string]string{"account_id": positionals[0]}, nil, false, *jsonOutput, stdout)
	case "create":
		if *name == "" {
			return domain.NewError(domain.CodeInvalidArgument, "accounts create requires --name")
		}
		return runRPCCommand(*root, "accounts.create", domain.CapabilityMetadataRead,
			map[string]string{"provider": *provider, "display_name": *name, "provider_subject_digest": *subjectDigest}, nil, true, *jsonOutput, stdout)
	case "disable":
		if len(positionals) != 1 {
			return domain.NewError(domain.CodeInvalidArgument, "account ID is required")
		}
		return runRPCCommand(*root, "accounts.disable", domain.CapabilityMetadataRead, map[string]string{"account_id": positionals[0]}, nil, true, *jsonOutput, stdout)
	default:
		return domain.NewError(domain.CodeMethodNotFound, "accounts command is not available")
	}
}

func runProfiles(args []string, stdout, stderr *os.File) error {
	if len(args) == 0 {
		return domain.NewError(domain.CodeInvalidArgument, "profiles command is required")
	}
	action := args[0]
	flags := flag.NewFlagSet("profiles "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	deviceID := flags.String("device-id", "", "Device ID")
	accountID := flags.String("account-id", "", "Account ID")
	name := flags.String("name", "", "Profile name")
	provider := flags.String("provider", domain.ProviderCodex, "Provider name")
	settings := flags.String("settings", "{}", "non-secret JSON settings")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args[1:]); err != nil || *root == "" {
		return domain.NewError(domain.CodeInvalidArgument, "profiles command requires --root")
	}
	positionals := flags.Args()
	switch action {
	case "list":
		return runRPCCommand(*root, "profiles.list", domain.CapabilityMetadataRead, nil, nil, false, *jsonOutput, stdout)
	case "create":
		if *deviceID == "" || *name == "" || *provider == "" {
			return domain.NewError(domain.CodeInvalidArgument, "profiles create requires --device-id, --name, and --provider")
		}
		var raw json.RawMessage
		if !json.Valid([]byte(*settings)) {
			return domain.NewError(domain.CodeInvalidArgument, "--settings must be valid JSON")
		}
		raw = json.RawMessage(*settings)
		return runRPCCommand(*root, "profiles.create", domain.CapabilityMetadataRead,
			map[string]any{"device_id": *deviceID, "account_id": *accountID, "name": *name, "provider": *provider, "settings": raw}, nil, true, *jsonOutput, stdout)
	case "edit":
		if len(positionals) != 1 {
			return domain.NewError(domain.CodeInvalidArgument, "profile ID is required")
		}
		body := map[string]any{"profile_id": positionals[0]}
		if *name != "" {
			body["name"] = *name
		}
		if *provider != domain.ProviderCodex {
			body["provider"] = *provider
		}
		if *accountID != "" {
			body["account_id"] = *accountID
		}
		if *settings != "{}" {
			if !json.Valid([]byte(*settings)) {
				return domain.NewError(domain.CodeInvalidArgument, "--settings must be valid JSON")
			}
			body["settings"] = json.RawMessage(*settings)
		}
		return runRPCCommand(*root, "profiles.edit", domain.CapabilityMetadataRead, body, nil, true, *jsonOutput, stdout)
	case "delete":
		if len(positionals) != 1 {
			return domain.NewError(domain.CodeInvalidArgument, "profile ID is required")
		}
		return runRPCCommand(*root, "profiles.delete", domain.CapabilityMetadataRead, map[string]string{"profile_id": positionals[0]}, nil, true, *jsonOutput, stdout)
	case "validate":
		if len(positionals) != 1 {
			return domain.NewError(domain.CodeInvalidArgument, "profile ID is required")
		}
		return runRPCCommand(*root, "profile.validate", domain.CapabilityProviderMetadataRead, map[string]string{"profile_id": positionals[0]}, nil, false, *jsonOutput, stdout)
	default:
		return domain.NewError(domain.CodeMethodNotFound, "profiles command is not available")
	}
}

func runUsage(args []string, stdout, stderr *os.File) error {
	flags := flag.NewFlagSet("usage", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	provider := flags.String("provider", domain.ProviderCodex, "Provider name")
	accountID := flags.String("account", "", "Account ID")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil || *root == "" {
		return domain.NewError(domain.CodeInvalidArgument, "usage requires --root")
	}
	if *provider != domain.ProviderCodex {
		return domain.NewError(domain.CodeUnsupportedVersion, "usage is only available for codex in this phase")
	}
	body := map[string]string{}
	if *accountID != "" {
		body["account_id"] = *accountID
	}
	return runRPCCommand(*root, "usage.read", domain.CapabilityProviderUsageRead, body, nil, false, *jsonOutput, stdout)
}

func runProvider(args []string, stdout, stderr *os.File) error {
	if len(args) == 0 {
		return domain.NewError(domain.CodeInvalidArgument, "provider command is required")
	}
	action := args[0]
	flags := flag.NewFlagSet("provider "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args[1:]); err != nil || *root == "" {
		return domain.NewError(domain.CodeInvalidArgument, "provider command requires --root")
	}
	switch action {
	case "describe", "health":
		return runRPCCommand(*root, "provider."+action, domain.CapabilityProviderMetadataRead, nil, nil, false, *jsonOutput, stdout)
	default:
		return domain.NewError(domain.CodeMethodNotFound, "provider command is not available")
	}
}

func runApprovals(args []string, stdout, stderr *os.File) error {
	if len(args) == 0 {
		return domain.NewError(domain.CodeInvalidArgument, "approvals command is required")
	}
	action := args[0]
	flags := flag.NewFlagSet("approvals "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	approvalID := flags.String("approval-id", "", "local Approval ID")
	providerApprovalID := flags.String("provider-approval-id", "", "Provider Approval ID")
	decision := flags.String("decision", "", "approve, deny, or cancel")
	revision := flags.Int64("revision", 0, "controller lease revision")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args[1:]); err != nil || *root == "" {
		return domain.NewError(domain.CodeInvalidArgument, "approvals command requires --root")
	}
	positionals := flags.Args()
	switch action {
	case "list", "observe":
		if len(positionals) != 1 {
			return domain.NewError(domain.CodeInvalidArgument, "session ID is required")
		}
		method := "approval." + action
		return runRPCCommand(*root, method, domain.CapabilityApprovalRead, map[string]string{"session_id": positionals[0]}, nil, false, *jsonOutput, stdout)
	case "respond":
		if len(positionals) != 1 || *approvalID == "" || *providerApprovalID == "" || *decision == "" || *revision < 1 {
			return domain.NewError(domain.CodeInvalidArgument, "approvals respond requires session ID, --approval-id, --provider-approval-id, --decision, and --revision")
		}
		body := map[string]string{"session_id": positionals[0], "approval_id": *approvalID, "provider_approval_id": *providerApprovalID, "decision": *decision}
		return runRPCCommand(*root, "approval.respond", domain.CapabilityApprovalRespond, body, revision, true, *jsonOutput, stdout)
	default:
		return domain.NewError(domain.CodeMethodNotFound, "approvals command is not available")
	}
}

func runServiceSpec(action string, args []string, stdout, stderr *os.File) error {
	flags := flag.NewFlagSet("daemon "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	executable := flags.String("executable", "", "absolute multidesk executable")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return domain.NewError(domain.CodeInvalidArgument, "service specification arguments are invalid")
	}
	if *root == "" {
		return domain.NewError(domain.CodeInvalidArgument, "--root is required")
	}
	if *executable == "" {
		value, err := os.Executable()
		if err != nil {
			return domain.WrapError(domain.CodeConflict, "multidesk executable could not be resolved", err)
		}
		*executable = value
	}
	rootPath, err := filepath.Abs(*root)
	if err != nil {
		return domain.WrapError(domain.CodeInvalidArgument, "device root is invalid", err)
	}
	executablePath, err := filepath.Abs(*executable)
	if err != nil {
		return domain.WrapError(domain.CodeInvalidArgument, "multidesk executable is invalid", err)
	}
	spec, err := device.RenderServiceSpec(runtime.GOOS, rootPath, executablePath)
	if err != nil {
		return err
	}
	result := map[string]any{"action": action, "goos": spec.GOOS, "name": spec.Name,
		"executable": spec.Executable, "root": spec.Root, "endpoint": spec.Endpoint, "contents": spec.Contents}
	return writeCLI(stdout, *jsonOutput, "daemon-"+action, result, nil)
}

func runVault(args []string, stdout, stderr *os.File) error {
	if len(args) == 0 {
		return domain.NewError(domain.CodeInvalidArgument, "vault command is required")
	}
	flags := flag.NewFlagSet("vault "+args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	secretStdin := flags.Bool("secret-stdin", false, "read unlock input from stdin")
	passwordStdin := flags.Bool("password-stdin", false, "read two matching initialization passwords from stdin")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args[1:]); err != nil {
		return domain.NewError(domain.CodeInvalidArgument, "vault command arguments are invalid")
	}
	if *root == "" {
		return domain.NewError(domain.CodeInvalidArgument, "vault command requires --root")
	}
	if len(flags.Args()) != 0 {
		return domain.NewError(domain.CodeInvalidArgument, "vault command accepts no positional arguments")
	}
	switch args[0] {
	case "status":
		return runRPCCommand(*root, "vault.status", domain.CapabilityMetadataRead, nil, nil, false, *jsonOutput, stdout)
	case "initialize":
		if !*passwordStdin || *secretStdin {
			return domain.NewError(domain.CodeInvalidArgument, "vault initialize requires --password-stdin")
		}
		secret, err := readVaultInitialization(os.Stdin)
		if err != nil {
			return err
		}
		return runRPCCommand(*root, "vault.initialize", domain.CapabilityVaultControl, map[string]string{"secret": secret}, nil, true, *jsonOutput, stdout)
	case "unlock":
		if !*secretStdin {
			return domain.NewError(domain.CodeInvalidArgument, "vault unlock requires --secret-stdin")
		}
		secret, err := readVaultSecret(os.Stdin)
		if err != nil {
			return err
		}
		return runRPCCommand(*root, "vault.unlock", domain.CapabilityVaultControl, map[string]string{"secret": secret}, nil, true, *jsonOutput, stdout)
	case "lock":
		return runRPCCommand(*root, "vault.lock", domain.CapabilityVaultControl, nil, nil, true, *jsonOutput, stdout)
	default:
		return domain.NewError(domain.CodeMethodNotFound, "vault command is not available")
	}
}

func runAuth(args []string, stdout, stderr *os.File) error {
	if len(args) == 0 {
		return domain.NewError(domain.CodeInvalidArgument, "auth command is required")
	}
	action := args[0]
	flags := flag.NewFlagSet("auth "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	profileID := flags.String("profile-id", "", "RuntimeProfile ID")
	profileSelector := flags.String("profile", "", "public profile selector such as @A")
	credentialID := flags.String("credential-id", "", "CredentialInstance ID")
	enrollmentID := flags.String("enrollment-id", "", "Auth enrollment ID")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args[1:]); err != nil || *root == "" || len(flags.Args()) != 0 {
		return domain.NewError(domain.CodeInvalidArgument, "auth command requires --root and no positional arguments")
	}
	switch action {
	case "begin":
		if (*profileID == "") == (*profileSelector == "") {
			return domain.NewError(domain.CodeInvalidArgument, "auth begin requires exactly one --profile or --profile-id")
		}
		body := map[string]string{}
		if *profileID != "" {
			body["profile_id"] = *profileID
		}
		if *profileSelector != "" {
			body["profile_selector"] = *profileSelector
		}
		if *credentialID != "" {
			body["credential_id"] = *credentialID
		}
		requestID, result, err := callRPC(*root, "auth.begin", domain.CapabilityProviderAuth, body, nil, true)
		if err != nil {
			return err
		}
		data, _ := json.Marshal(result)
		var enrollment cliAuthEnrollment
		if json.Unmarshal(data, &enrollment) != nil || enrollment.EnrollmentID == "" || enrollment.BinaryPath == "" || len(enrollment.Argv) != 1 || enrollment.Argv[0] != "login" || enrollment.StagingPath == "" {
			return domain.NewError(domain.CodeConflict, "auth enrollment response is invalid")
		}
		if err := runOfficialCodexLogin(enrollment, os.Stdin, stdout, stderr); err != nil {
			_, _, _ = callRPC(*root, "auth.cancel", domain.CapabilityProviderAuth, map[string]string{"enrollment_id": enrollment.EnrollmentID}, nil, true)
			return err
		}
		completeID, completed, err := callRPC(*root, "auth.complete", domain.CapabilityProviderAuth, map[string]string{"enrollment_id": enrollment.EnrollmentID}, nil, true)
		if err != nil {
			return err
		}
		if completeID == "" {
			completeID = requestID
		}
		if *jsonOutput {
			return writeCLI(stdout, true, completeID, completed, nil)
		}
		encoded, _ := json.Marshal(completed)
		var pending struct {
			ProfileSelector string `json:"profile_selector"`
			State           string `json:"state"`
		}
		if json.Unmarshal(encoded, &pending) != nil || pending.ProfileSelector == "" || pending.State != "awaiting_confirmation" {
			return domain.NewError(domain.CodeConflict, "auth confirmation response is invalid")
		}
		if _, err := fmt.Fprintf(stderr, "Type %s to confirm this internal account: ", pending.ProfileSelector); err != nil {
			return err
		}
		var typed string
		if _, err := fmt.Fscanln(os.Stdin, &typed); err != nil || typed != pending.ProfileSelector {
			return domain.NewError(domain.CodeIdentityConfirmationMismatch, "typed profile selector did not match")
		}
		_, confirmed, err := callRPC(*root, "auth.confirm", domain.CapabilityProviderAuth,
			map[string]any{"enrollment_id": enrollment.EnrollmentID, "profile_selector": typed, "confirmed": true}, nil, true)
		if err != nil {
			return err
		}
		return writeCLI(stdout, false, completeID, confirmed, nil)
	case "confirm":
		if *enrollmentID == "" || *profileSelector == "" {
			return domain.NewError(domain.CodeInvalidArgument, "auth confirm requires --enrollment-id and --profile")
		}
		return runRPCCommand(*root, "auth.confirm", domain.CapabilityProviderAuth,
			map[string]any{"enrollment_id": *enrollmentID, "profile_selector": *profileSelector, "confirmed": true}, nil, true, *jsonOutput, stdout)
	case "status":
		body := map[string]string{}
		if *enrollmentID != "" {
			body["enrollment_id"] = *enrollmentID
		} else if *profileSelector != "" {
			body["profile_selector"] = *profileSelector
		} else if *credentialID != "" {
			body["credential_id"] = *credentialID
		} else {
			return domain.NewError(domain.CodeInvalidArgument, "auth status requires --enrollment-id, --profile, or --credential-id")
		}
		return runRPCCommand(*root, "auth.status", domain.CapabilityProviderAuth, body, nil, false, *jsonOutput, stdout)
	case "cancel":
		if *enrollmentID == "" {
			return domain.NewError(domain.CodeInvalidArgument, "auth cancel requires --enrollment-id")
		}
		return runRPCCommand(*root, "auth.cancel", domain.CapabilityProviderAuth, map[string]string{"enrollment_id": *enrollmentID}, nil, true, *jsonOutput, stdout)
	case "logout":
		if (*credentialID == "") == (*profileSelector == "") {
			return domain.NewError(domain.CodeInvalidArgument, "auth logout requires exactly one --profile or --credential-id")
		}
		body := map[string]string{}
		if *profileSelector != "" {
			body["profile_selector"] = *profileSelector
		} else {
			body["credential_id"] = *credentialID
		}
		return runRPCCommand(*root, "auth.logout", domain.CapabilityProviderAuth, body, nil, true, *jsonOutput, stdout)
	default:
		return domain.NewError(domain.CodeMethodNotFound, "auth command is not available")
	}
}

type cliAuthEnrollment struct {
	EnrollmentID string    `json:"enrollment_id"`
	BinaryPath   string    `json:"binary_path"`
	Argv         []string  `json:"argv"`
	StagingPath  string    `json:"staging_path"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func runOfficialCodexLogin(enrollment cliAuthEnrollment, stdin io.Reader, stdout, stderr io.Writer) error {
	if enrollment.BinaryPath == "" || len(enrollment.Argv) != 1 || enrollment.Argv[0] != "login" || enrollment.StagingPath == "" || enrollment.ExpiresAt.IsZero() || !time.Now().Before(enrollment.ExpiresAt) {
		return domain.NewError(domain.CodeDeadlineExceeded, "Codex login enrollment expired")
	}
	ctx, cancel := context.WithDeadline(context.Background(), enrollment.ExpiresAt)
	defer cancel()
	command := exec.CommandContext(ctx, enrollment.BinaryPath, enrollment.Argv...)
	command.Env = loginEnvironment(enrollment.StagingPath)
	command.Stdin, command.Stdout, command.Stderr = stdin, stdout, stderr
	if err := command.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return domain.NewError(domain.CodeDeadlineExceeded, "Codex login enrollment expired")
		}
		return domain.NewError(domain.CodeProviderFailed, "official codex login failed")
	}
	return nil
}

func loginEnvironment(staging string) []string {
	result := []string{"CODEX_HOME=" + staging}
	for _, name := range []string{"HOME", "PATH", "TERM", "DISPLAY", "BROWSER", "XDG_RUNTIME_DIR"} {
		if value := os.Getenv(name); value != "" {
			result = append(result, name+"="+value)
		}
	}
	result = append(result, codex.NetworkEnvironment(os.Getenv)...)
	return result
}

func runSessionStart(args []string, stdout, stderr *os.File) error {
	if len(args) == 0 || (args[0] != "fake" && args[0] != "codex") {
		return domain.NewError(domain.CodeInvalidArgument, "run supports run fake and run codex")
	}
	provider := args[0]
	flags := flag.NewFlagSet("run "+provider, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	workspace := flags.String("workspace", "", "workspace ID")
	deviceID := flags.String("device-id", "", "device ID")
	credentialID := flags.String("credential-id", "", "credential instance ID")
	profileID := flags.String("profile-id", "", "runtime profile ID")
	accountID := flags.String("account-id", "", "Account ID")
	profileSelector := flags.String("profile", "", "public profile selector such as @A")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args[1:]); err != nil || *root == "" || *workspace == "" {
		return domain.NewError(domain.CodeInvalidArgument, "run requires --root and --workspace")
	}
	if provider == domain.ProviderCodex {
		if *profileSelector == "" || *deviceID != "" || *credentialID != "" || *profileID != "" || *accountID != "" {
			return domain.NewError(domain.CodeIdentityConfirmationRequired, "run codex requires --profile and rejects raw identity flags")
		}
		if *jsonOutput {
			return domain.NewError(domain.CodeIdentityConfirmationRequired, "JSON callers must use sessions preview and the confirmed session.start RPC")
		}
		_, rawPreview, err := callRPC(*root, "sessions.preview", domain.CapabilityMetadataRead,
			map[string]any{"provider": domain.ProviderCodex, "profile_selector": *profileSelector, "workspace_id": *workspace}, nil, false)
		if err != nil {
			return err
		}
		encoded, _ := json.Marshal(rawPreview)
		var preview cliSessionPreview
		if json.Unmarshal(encoded, &preview) != nil || preview.PreviewID == "" || preview.ProfileAlias == "" {
			return domain.NewError(domain.CodeConflict, "session preview response is invalid")
		}
		canonical := "@" + preview.ProfileAlias
		if _, err := fmt.Fprintf(stderr, "Start %s (%s) with credential revision %d? Type %s: ",
			canonical, preview.AccountLabel, preview.CredentialRevision, canonical); err != nil {
			return err
		}
		var typed string
		if _, err := fmt.Fscanln(os.Stdin, &typed); err != nil || typed != canonical {
			return domain.NewError(domain.CodeIdentityConfirmationMismatch, "typed profile selector did not match")
		}
		confirmation := map[string]any{"confirmed": true, "account_id": preview.AccountID,
			"account_revision": preview.AccountRevision, "runtime_profile_id": preview.RuntimeProfileID,
			"profile_revision": preview.ProfileRevision, "credential_instance_id": preview.CredentialInstanceID,
			"credential_revision": preview.CredentialRevision, "device_id": preview.DeviceID,
			"workspace_id": preview.WorkspaceID, "usage_snapshot_id": preview.UsageSnapshotID,
			"provider_version": preview.ProviderVersion}
		return runRPCCommand(*root, "session.start", domain.CapabilitySessionStart,
			map[string]any{"provider": domain.ProviderCodex, "profile_selector": *profileSelector,
				"workspace_id": *workspace, "preview_id": preview.PreviewID, "confirmation": confirmation}, nil, true, false, stdout)
	}
	if *deviceID == "" || *credentialID == "" || *profileID == "" {
		return domain.NewError(domain.CodeInvalidArgument, "run fake requires --device-id, --credential-id, and --profile-id")
	}
	body := map[string]string{"provider": provider, "workspace_id": *workspace, "device_id": *deviceID,
		"credential_instance_id": *credentialID, "runtime_profile_id": *profileID, "account_id": *accountID}
	return runRPCCommand(*root, "session.start", domain.CapabilitySessionStart, body, nil, true, *jsonOutput, stdout)
}

type cliSessionPreview struct {
	PreviewID            domain.ID `json:"preview_id"`
	AccountID            domain.ID `json:"account_id"`
	AccountRevision      int64     `json:"account_revision"`
	AccountLabel         string    `json:"account_label"`
	RuntimeProfileID     domain.ID `json:"runtime_profile_id"`
	ProfileRevision      int64     `json:"profile_revision"`
	ProfileAlias         string    `json:"profile_alias"`
	CredentialInstanceID domain.ID `json:"credential_instance_id"`
	CredentialRevision   int64     `json:"credential_revision"`
	DeviceID             domain.ID `json:"device_id"`
	WorkspaceID          domain.ID `json:"workspace_id"`
	UsageSnapshotID      domain.ID `json:"usage_snapshot_id"`
	ProviderVersion      string    `json:"provider_version"`
}

func runSessions(args []string, stdout, stderr *os.File) error {
	if len(args) == 0 {
		return domain.NewError(domain.CodeInvalidArgument, "sessions command is required")
	}
	action := args[0]
	flags := flag.NewFlagSet("sessions "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	mode := flags.String("mode", "observer", "attachment mode")
	from := flags.Int64("from-sequence", 0, "replay start sequence")
	revision := flags.Int64("revision", 0, "controller lease revision")
	jsonOutput := flags.Bool("json", false, "JSON output")
	profileSelector := flags.String("profile", "", "public profile selector such as @A")
	workspaceID := flags.String("workspace", "", "workspace ID")
	if err := flags.Parse(args[1:]); err != nil || *root == "" {
		return domain.NewError(domain.CodeInvalidArgument, "sessions command requires --root")
	}
	positionals := flags.Args()
	method := "sessions." + action
	var body any
	capability := domain.CapabilityMetadataRead
	idempotent := false
	switch action {
	case "list":
	case "preview":
		if len(positionals) != 0 || *profileSelector == "" || *workspaceID == "" {
			return domain.NewError(domain.CodeInvalidArgument, "sessions preview requires --profile and --workspace")
		}
		body = map[string]any{"provider": domain.ProviderCodex, "profile_selector": *profileSelector, "workspace_id": *workspaceID}
	case "show", "detach", "stop", "kill", "resume":
		if len(positionals) != 1 {
			return domain.NewError(domain.CodeInvalidArgument, "session ID is required")
		}
		body = map[string]string{"session_id": positionals[0]}
		if action == "detach" {
			capability, idempotent = domain.CapabilitySessionObserve, true
		} else if action == "stop" || action == "kill" {
			if *revision < 1 {
				return domain.NewError(domain.CodeInvalidArgument, "--revision is required")
			}
			capability, idempotent = domain.CapabilitySessionControl, true
		} else if action == "resume" {
			capability, idempotent = domain.CapabilitySessionResume, true
		}
	case "observe":
		if len(positionals) != 1 {
			return domain.NewError(domain.CodeInvalidArgument, "session ID is required")
		}
		body = map[string]any{"session_id": positionals[0], "from_sequence": *from}
	case "attach":
		if len(positionals) != 1 {
			return domain.NewError(domain.CodeInvalidArgument, "session ID is required")
		}
		body = map[string]string{"session_id": positionals[0], "mode": *mode}
		capability, idempotent = domain.CapabilitySessionObserve, true
	default:
		return domain.NewError(domain.CodeMethodNotFound, "sessions command is not available")
	}
	var leaseRevision *int64
	if action == "stop" || action == "kill" {
		leaseRevision = revision
	}
	return runRPCCommand(*root, method, capability, body, leaseRevision, idempotent, *jsonOutput, stdout)
}

func runControl(args []string, stdout, stderr *os.File) error {
	if len(args) == 0 {
		return domain.NewError(domain.CodeInvalidArgument, "control command is required")
	}
	action := args[0]
	flags := flag.NewFlagSet("control "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	revision := flags.Int64("revision", 0, "controller lease revision")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args[1:]); err != nil || *root == "" || len(flags.Args()) != 1 {
		return domain.NewError(domain.CodeInvalidArgument, "control command requires --root and a session ID")
	}
	if action != "acquire" && *revision < 1 {
		return domain.NewError(domain.CodeInvalidArgument, "--revision is required")
	}
	request := runRPCRequest{Root: *root, Method: "control." + action, Capability: domain.CapabilitySessionControl, Body: map[string]string{"session_id": flags.Args()[0]}, Revision: revision, Idempotent: true, JSON: *jsonOutput, Stdout: stdout}
	if action == "acquire" {
		request.Capability = domain.CapabilitySessionControlAcquire
		request.Revision = nil
	}
	if action != "acquire" && action != "heartbeat" && action != "release" {
		return domain.NewError(domain.CodeMethodNotFound, "control command is not available")
	}
	return request.call()
}

func runTerminal(args []string, stdout, stderr *os.File) error {
	if len(args) == 0 {
		return domain.NewError(domain.CodeInvalidArgument, "terminal command is required")
	}
	action := args[0]
	flags := flag.NewFlagSet("terminal "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	revision := flags.Int64("revision", 0, "controller lease revision")
	sequence := flags.Int64("sequence", 0, "input sequence")
	payload := flags.String("payload", "", "terminal input payload")
	rows := flags.Int("rows", 0, "terminal rows")
	cols := flags.Int("cols", 0, "terminal columns")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args[1:]); err != nil || *root == "" || len(flags.Args()) != 1 || *revision < 1 {
		return domain.NewError(domain.CodeInvalidArgument, "terminal command requires --root, --revision, and a session ID")
	}
	if action == "input" {
		if *sequence < 1 {
			return domain.NewError(domain.CodeInvalidArgument, "--sequence is required")
		}
		return (&runRPCRequest{Root: *root, Method: "terminal.input", Capability: domain.CapabilityTerminalControl,
			Body: map[string]any{"session_id": flags.Args()[0], "sequence": *sequence, "payload": *payload}, Revision: revision, Idempotent: true, JSON: *jsonOutput, Stdout: stdout}).call()
	}
	if action == "resize" {
		return (&runRPCRequest{Root: *root, Method: "terminal.resize", Capability: domain.CapabilityTerminalControl,
			Body: map[string]any{"session_id": flags.Args()[0], "rows": *rows, "cols": *cols}, Revision: revision, Idempotent: true, JSON: *jsonOutput, Stdout: stdout}).call()
	}
	return domain.NewError(domain.CodeMethodNotFound, "terminal command is not available")
}

func runTUI(args []string, stdout, stderr *os.File) error {
	flags := flag.NewFlagSet("tui", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	profileSelector := flags.String("profile", "", "public Codex profile selector such as @A")
	workspaceID := flags.String("workspace", "", "workspace ID for a confirmed Codex start")
	if err := flags.Parse(args); err != nil || *root == "" {
		return domain.NewError(domain.CodeInvalidArgument, "tui requires --root")
	}
	if (*profileSelector == "") != (*workspaceID == "") {
		return domain.NewError(domain.CodeInvalidArgument, "tui Codex start requires both --profile and --workspace")
	}
	if _, err := fmt.Fprintln(stdout, "MultiAgentDesk TUI"); err != nil {
		return err
	}
	if *profileSelector != "" {
		if _, err := fmt.Fprintln(stderr, "Codex selector confirmation"); err != nil {
			return err
		}
		return runSessionStart([]string{"codex", "--root", *root, "--profile", *profileSelector, "--workspace", *workspaceID}, stdout, stderr)
	}
	return runRPCCommand(*root, "sessions.list", domain.CapabilityMetadataRead, nil, nil, false, false, stdout)
}

type runRPCRequest struct {
	Root       string
	Method     string
	Capability domain.Capability
	Body       any
	Revision   *int64
	Idempotent bool
	JSON       bool
	Stdout     *os.File
}

func (r *runRPCRequest) call() error {
	return runRPCCommand(r.Root, r.Method, r.Capability, r.Body, r.Revision, r.Idempotent, r.JSON, r.Stdout)
}

func runRPCCommand(root, method string, capability domain.Capability, body any, revision *int64, idempotent, jsonOutput bool, stdout *os.File) error {
	requestID, result, err := callRPC(root, method, capability, body, revision, idempotent)
	if err != nil {
		if jsonOutput {
			_ = writeCLI(stdout, true, requestID, nil, err)
		}
		return err
	}
	return writeCLI(stdout, jsonOutput, requestID, result, nil)
}

func callRPC(root, method string, capability domain.Capability, body any, revision *int64, idempotent bool) (string, any, error) {
	owner, err := device.LoadOwnerIdentity(root)
	if err != nil {
		return "", nil, err
	}
	rawBody, err := device.JSONBody(body)
	if err != nil {
		return "", nil, err
	}
	if method == "vault.initialize" || method == "vault.unlock" {
		defer func() {
			for index := range rawBody {
				rawBody[index] = 0
			}
		}()
	}
	var requestID, idempotencyKey string
	if idempotent {
		randomID := make([]byte, 24)
		if _, err := io.ReadFull(rand.Reader, randomID); err != nil {
			return "", nil, domain.NewError(domain.CodeConflict, "operation identity could not be generated")
		}
		encoded := hex.EncodeToString(randomID)
		slug := strings.NewReplacer(".", "-", "_", "-").Replace(method)
		if len(slug) > 48 {
			slug = slug[:48]
		}
		requestID, idempotencyKey = "cli-"+slug+"-"+encoded[:20], "cli-"+encoded
	} else {
		requestID, idempotencyKey = cliRequestIdentity(method, rawBody, revision)
	}
	request := device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: requestID, Method: method, Body: rawBody, LeaseRevision: revision}
	if idempotent {
		request.IdempotencyKey = idempotencyKey
	}
	attempt := func() (device.Response, error) {
		connection, err := device.Dial(root, 5*time.Second)
		if err != nil {
			return device.Response{}, err
		}
		defer connection.Close()
		if _, err := (device.ClientAuthenticator{Identity: owner, RequestedCapabilities: []domain.Capability{capability}}).Handshake(context.Background(), connection); err != nil {
			return device.Response{}, err
		}
		return (&device.Client{Connection: connection}).Call(context.Background(), request)
	}
	response, err := callWithLostResponseRetry(idempotent, attempt)
	if err != nil {
		return requestID, nil, err
	}
	var result any
	if len(response.Result) > 0 {
		if err := json.Unmarshal(response.Result, &result); err != nil {
			return requestID, nil, domain.NewError(domain.CodeConflict, "daemon response could not be decoded")
		}
	}
	return requestID, result, nil
}

func callWithLostResponseRetry(idempotent bool, attempt func() (device.Response, error)) (device.Response, error) {
	response, err := attempt()
	if idempotent && domain.CodeOf(err) == domain.CodeDaemonUnavailable {
		return attempt()
	}
	return response, err
}

func cliRequestIdentity(method string, body json.RawMessage, revision *int64) (string, string) {
	hasher := sha256.New()
	_, _ = hasher.Write([]byte(method))
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write(body)
	_, _ = hasher.Write([]byte{0})
	if revision != nil {
		_, _ = hasher.Write([]byte(strconv.FormatInt(*revision, 10)))
	}
	digest := hex.EncodeToString(hasher.Sum(nil))
	slug := strings.NewReplacer(".", "-", "_", "-").Replace(method)
	if len(slug) > 48 {
		slug = slug[:48]
	}
	return "cli-" + slug + "-" + digest[:20], "cli-" + digest
}

const maxVaultUnlockInput = 4096

func readVaultSecret(reader io.Reader) (string, error) {
	if reader == nil {
		return "", domain.NewError(domain.CodeInvalidArgument, "vault unlock input is unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxVaultUnlockInput+1))
	if err != nil {
		return "", domain.NewError(domain.CodeInvalidArgument, "vault unlock input could not be read")
	}
	if len(data) > maxVaultUnlockInput {
		return "", domain.NewError(domain.CodeInvalidArgument, "vault unlock input is too large")
	}
	secret := strings.TrimRight(string(data), "\r\n")
	if secret == "" {
		return "", domain.NewError(domain.CodeInvalidArgument, "vault unlock input is empty")
	}
	return secret, nil
}

func readVaultInitialization(reader io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(reader, 2*maxVaultUnlockInput+3))
	if err != nil {
		return "", domain.NewError(domain.CodeInvalidArgument, "vault initialization input could not be read")
	}
	separator := []byte{'\n'}
	if bytes.Contains(data, []byte{'\r'}) {
		separator = []byte{'\r', '\n'}
	}
	parts := bytes.Split(data, separator)
	for _, part := range parts {
		if bytes.ContainsAny(part, "\r\n") {
			return "", domain.NewError(domain.CodeInvalidArgument, "vault initialization input is invalid")
		}
	}
	if len(parts) != 3 || len(parts[2]) != 0 || len(parts[0]) == 0 || len(parts[0]) > 1024 || !bytes.Equal(parts[0], parts[1]) {
		return "", domain.NewError(domain.CodeInvalidArgument, "vault initialization passwords do not match")
	}
	return string(parts[0]), nil
}
