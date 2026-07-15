package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/device"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
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
	secret := flags.String("secret", "", "unlock input; use a secret manager for production secrets")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args[1:]); err != nil || *root == "" {
		return domain.NewError(domain.CodeInvalidArgument, "vault command requires --root")
	}
	switch args[0] {
	case "status":
		return runRPCCommand(*root, "vault.status", domain.CapabilityMetadataRead, nil, nil, false, *jsonOutput, stdout)
	case "unlock":
		if *secret == "" {
			return domain.NewError(domain.CodeInvalidArgument, "vault unlock requires --secret")
		}
		return runRPCCommand(*root, "vault.unlock", domain.CapabilityVaultControl, map[string]string{"secret": *secret}, nil, true, *jsonOutput, stdout)
	case "lock":
		return runRPCCommand(*root, "vault.lock", domain.CapabilityVaultControl, nil, nil, true, *jsonOutput, stdout)
	default:
		return domain.NewError(domain.CodeMethodNotFound, "vault command is not available")
	}
}

func runSessionStart(args []string, stdout, stderr *os.File) error {
	if len(args) == 0 || args[0] != "fake" {
		return domain.NewError(domain.CodeInvalidArgument, "run currently supports only run fake")
	}
	flags := flag.NewFlagSet("run fake", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	workspace := flags.String("workspace", "", "workspace ID")
	deviceID := flags.String("device-id", "", "device ID")
	credentialID := flags.String("credential-id", "", "credential instance ID")
	profileID := flags.String("profile-id", "", "runtime profile ID")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args[1:]); err != nil || *root == "" || *workspace == "" || *deviceID == "" || *credentialID == "" || *profileID == "" {
		return domain.NewError(domain.CodeInvalidArgument, "run fake requires --root, --workspace, --device-id, --credential-id, and --profile-id")
	}
	body := map[string]string{"workspace_id": *workspace, "device_id": *deviceID,
		"credential_instance_id": *credentialID, "runtime_profile_id": *profileID}
	return runRPCCommand(*root, "sessions.start", domain.CapabilitySessionStart, body, nil, true, *jsonOutput, stdout)
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
	if err := flags.Parse(args); err != nil || *root == "" {
		return domain.NewError(domain.CodeInvalidArgument, "tui requires --root")
	}
	if _, err := fmt.Fprintln(stdout, "MultiAgentDesk TUI (minimal Phase 1 view)"); err != nil {
		return err
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
	owner, err := device.LoadOwnerIdentity(root)
	if err != nil {
		return err
	}
	connection, err := device.Dial(root, 5*time.Second)
	if err != nil {
		return err
	}
	defer connection.Close()
	if _, err := (device.ClientAuthenticator{Identity: owner, RequestedCapabilities: []domain.Capability{capability}}).Handshake(context.Background(), connection); err != nil {
		return err
	}
	rawBody, err := device.JSONBody(body)
	if err != nil {
		return err
	}
	requestID := "cli-" + strings.NewReplacer(".", "-", "_", "-").Replace(method)
	request := device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: requestID, Method: method, Body: rawBody, LeaseRevision: revision}
	if idempotent {
		request.IdempotencyKey = requestID
	}
	response, err := (&device.Client{Connection: connection}).Call(context.Background(), request)
	if err != nil {
		return err
	}
	var result any
	if len(response.Result) > 0 {
		if err := json.Unmarshal(response.Result, &result); err != nil {
			return domain.NewError(domain.CodeConflict, "daemon response could not be decoded")
		}
	}
	return writeCLI(stdout, jsonOutput, requestID, result, nil)
}
