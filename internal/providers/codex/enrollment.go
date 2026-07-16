package codex

import (
	"context"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

// ValidateEnrollment proves that staged auth can initialize the exact supported
// app-server and answer account/read. It returns no Provider identity.
func ValidateEnrollment(ctx context.Context, descriptor BinaryDescriptor, home string) error {
	if ctx == nil || descriptor.Path == "" || home == "" {
		return domain.NewError(domain.CodeInvalidArgument, "enrollment validation is incomplete")
	}
	capabilities, err := Probe(ctx, descriptor, ProbeOptions{Timeout: 15 * time.Second})
	if err != nil {
		return err
	}
	if capabilities.Status != CapabilitySupported || !capabilities.Allows(MethodAccountRead) {
		return domain.NewError(domain.CodeProviderVersionUnsupported, "codex enrollment version is unsupported")
	}
	validateCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(validateCtx, descriptor.Path, "app-server")
	cmd.Env = append([]string{"CODEX_HOME=" + home, "HOME=" + os.Getenv("HOME"), "PATH=" + os.Getenv("PATH")}, NetworkEnvironment(os.Getenv)...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return domain.NewError(domain.CodeProviderFailed, "codex validation stdin failed")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return domain.NewError(domain.CodeProviderFailed, "codex validation stdout failed")
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return domain.NewError(domain.CodeProviderFailed, "codex validation start failed")
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()
	client := NewClient(stdout, stdin)
	if err := client.ConfigureMethods(capabilities.Methods); err != nil {
		return err
	}
	if _, err := client.Handshake(validateCtx, InitializeParams{ClientInfo: ClientInfo{Name: "multi-agent-desk", Version: "phase2-p2b"}, Capabilities: &InitializeCapabilities{ExperimentalAPI: false}}); err != nil {
		return err
	}
	var raw any
	if err := client.Call(validateCtx, MethodAccountRead, map[string]any{}, &raw); err != nil {
		return err
	}
	return nil
}
