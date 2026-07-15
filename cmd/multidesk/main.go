package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/app"
	"github.com/jinlong17/multi-agent-desk/internal/device"
	"github.com/jinlong17/multi-agent-desk/internal/domain"
	runtimepkg "github.com/jinlong17/multi-agent-desk/internal/runtime"
	"github.com/jinlong17/multi-agent-desk/internal/storage"
	"github.com/jinlong17/multi-agent-desk/internal/vault"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr *os.File) error {
	if len(args) == 0 {
		return domain.NewError(domain.CodeInvalidArgument, "a command is required")
	}
	switch args[0] {
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "daemon":
		if len(args) < 2 {
			return domain.NewError(domain.CodeInvalidArgument, "daemon command is required")
		}
		switch args[1] {
		case "serve":
			return runServe(args[2:], stderr)
		case "status":
			return runStatus(args[2:], stdout, stderr)
		default:
			return domain.NewError(domain.CodeMethodNotFound, "daemon command is not available")
		}
	case "internal":
		if len(args) == 2 && args[1] == "fake-provider" {
			return runtimepkg.RunFakeProvider(os.Stdin, os.Stdout)
		}
		return domain.NewError(domain.CodeMethodNotFound, "internal command is not available")
	default:
		return domain.NewError(domain.CodeMethodNotFound, "command is not available")
	}
}

func runInit(args []string, stdout, stderr *os.File) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	name := flags.String("name", "MultiAgentDesk Device", "display name")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return domain.NewError(domain.CodeInvalidArgument, "init arguments are invalid")
	}
	if *root == "" {
		return domain.NewError(domain.CodeInvalidArgument, "--root is required")
	}
	result, err := device.Bootstrap(context.Background(), *root, *name, time.Now().UTC())
	if err != nil {
		return err
	}
	return writeCLI(stdout, *jsonOutput, "init-1", map[string]any{"root": result.Root, "device_id": result.DeviceID, "client_id": result.ClientID}, nil)
}

func runServe(args []string, stderr *os.File) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	if err := flags.Parse(args); err != nil {
		return domain.NewError(domain.CodeInvalidArgument, "serve arguments are invalid")
	}
	if *root == "" {
		return domain.NewError(domain.CodeInvalidArgument, "--root is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := storage.Open(ctx, device.DeviceDatabasePath(*root))
	if err != nil {
		return err
	}
	defer store.Close()
	if err := device.VerifyIdentityStore(ctx, *root, store); err != nil {
		return err
	}
	daemon, err := device.LoadDaemonIdentity(*root)
	if err != nil {
		return err
	}
	instance, err := device.NewEndpointInstance()
	if err != nil {
		return err
	}
	listener, err := device.Listen(*root)
	if err != nil {
		return err
	}
	defer listener.Close()
	authenticator, err := device.NewServerAuthenticator(daemon, instance, store)
	if err != nil {
		return err
	}
	authorizer := app.Authorizer{Clients: store}
	manager := runtimepkg.NewManager(store, os.Args[0])
	vaultManager := vault.NewManager()
	manager.Vault = vaultManager
	defer manager.Close()
	service := app.NewSessionService(store, manager)
	service.Vault = vaultManager
	server := &device.Server{Listener: listener, Authenticator: authenticator, Authorizer: authorizer.Authorize, Handler: service}
	return server.Serve(ctx)
}

func handleRequest(_ context.Context, _ device.AuthContext, request device.Request) (any, error) {
	switch request.Method {
	case "daemon.status":
		return map[string]any{"status": "ok", "schema_version": 1}, nil
	default:
		return nil, domain.NewError(domain.CodeMethodNotFound, "method is not available")
	}
}

func runStatus(args []string, stdout, stderr *os.File) error {
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return domain.NewError(domain.CodeInvalidArgument, "status arguments are invalid")
	}
	if *root == "" {
		return domain.NewError(domain.CodeInvalidArgument, "--root is required")
	}
	owner, err := device.LoadOwnerIdentity(*root)
	if err != nil {
		return err
	}
	connection, err := device.Dial(*root, 5*time.Second)
	if err != nil {
		return err
	}
	defer connection.Close()
	if _, err := (device.ClientAuthenticator{Identity: owner, RequestedCapabilities: []domain.Capability{domain.CapabilityMetadataRead}}).Handshake(context.Background(), connection); err != nil {
		return err
	}
	client := &device.Client{Connection: connection}
	response, err := client.Call(context.Background(), device.Request{ProtocolMajor: device.ProtocolMajor, RequestID: "status-1", Method: "daemon.status"})
	if err != nil {
		return err
	}
	var result any
	if len(response.Result) > 0 {
		_ = json.Unmarshal(response.Result, &result)
	}
	return writeCLI(stdout, *jsonOutput, response.RequestID, result, nil)
}

func writeCLI(stdout *os.File, jsonOutput bool, requestID string, result any, err error) error {
	if jsonOutput {
		value := map[string]any{"schema_version": 1, "request_id": requestID}
		if err == nil {
			value["ok"] = true
			value["result"] = result
		} else {
			value["ok"] = false
			value["error"] = map[string]any{"code": domain.CodeOf(err), "message": safeMessage(err)}
		}
		encoded, encodeErr := json.Marshal(value)
		if encodeErr != nil {
			return encodeErr
		}
		_, writeErr := fmt.Fprintln(stdout, string(encoded))
		return writeErr
	}
	if err != nil {
		return err
	}
	_, writeErr := fmt.Fprintln(stdout, result)
	return writeErr
}

func safeMessage(err error) string {
	var safe *domain.Error
	if errors.As(err, &safe) {
		return safe.Message
	}
	return "command failed"
}
