package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	stdruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

const childLineLimit = 64 * 1024

type childRequest struct {
	Version int    `json:"version"`
	Kind    string `json:"kind"`
	Payload string `json:"payload,omitempty"`
	Rows    int    `json:"rows,omitempty"`
	Cols    int    `json:"cols,omitempty"`
}

type Process struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	events chan ChildEvent
	ready  chan struct{}
	done   chan struct{}

	writeMu   sync.Mutex
	stateMu   sync.Mutex
	readyOnce sync.Once
	exitSent  bool
	exitCode  int
	waitErr   error
}

func StartProcess(executable string) (*Process, error) {
	if executable == "" {
		return nil, domain.NewError(domain.CodeInvalidArgument, "fake provider executable is required")
	}
	if stdruntime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(executable), ".exe") {
		// The Go tool may append .exe to an extensionless -o path. Prefer
		// that concrete artifact even when the extensionless placeholder also
		// exists, because CreateProcess does not execute the latter reliably.
		if _, exeErr := os.Stat(executable + ".exe"); exeErr == nil {
			executable += ".exe"
		}
	}
	command := exec.Command(executable, "internal", "fake-provider")
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, domain.WrapError(domain.CodeProviderFailed, "fake provider stdin could not be opened", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, domain.WrapError(domain.CodeProviderFailed, "fake provider stdout could not be opened", err)
	}
	if err := command.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, domain.WrapError(domain.CodeProviderFailed, "fake provider could not be started", err)
	}
	process := &Process{cmd: command, stdin: stdin, events: make(chan ChildEvent, 64), ready: make(chan struct{}), done: make(chan struct{})}
	go process.collect(stdout)
	return process, nil
}

func (p *Process) collect(stdout io.ReadCloser) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024), childLineLimit)
	for scanner.Scan() {
		var event ChildEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil || event.Version != fakeProviderProtocolVersion || event.Kind == "" {
			_ = p.cmd.Process.Kill()
			_ = stdout.Close()
			waitErr := p.cmd.Wait()
			p.finish(domain.NewError(domain.CodeProviderFailed, "fake provider emitted an invalid event"), -1)
			if waitErr != nil {
				p.stateMu.Lock()
				p.waitErr = domain.NewError(domain.CodeProviderFailed, "fake provider emitted an invalid event")
				p.stateMu.Unlock()
			}
			return
		}
		if event.Kind == "ready" {
			p.readyOnce.Do(func() { close(p.ready) })
		}
		if event.Kind == "exit" {
			p.stateMu.Lock()
			p.exitSent = true
			p.exitCode = event.Code
			p.stateMu.Unlock()
		}
		p.events <- event
	}
	err := scanner.Err()
	_ = stdout.Close()
	waitErr := p.cmd.Wait()
	p.stateMu.Lock()
	exitSent := p.exitSent
	exitCode := p.exitCode
	p.stateMu.Unlock()
	if !exitSent {
		if waitErr != nil {
			exitCode = -1
		} else {
			exitCode = 0
		}
		p.events <- ChildEvent{Version: fakeProviderProtocolVersion, Kind: "exit", Code: exitCode}
	}
	if err != nil && !errors.Is(err, os.ErrClosed) {
		waitErr = err
	}
	p.finish(waitErr, exitCode)
}

func (p *Process) finish(err error, exitCode int) {
	p.stateMu.Lock()
	p.waitErr = err
	p.exitCode = exitCode
	p.stateMu.Unlock()
	close(p.events)
	close(p.done)
}

func (p *Process) Events() <-chan ChildEvent { return p.events }
func (p *Process) Ready() <-chan struct{}    { return p.ready }
func (p *Process) Done() <-chan struct{}     { return p.done }

func (p *Process) send(request childRequest) error {
	if p == nil || p.stdin == nil {
		return domain.NewError(domain.CodeProviderFailed, "fake provider is unavailable")
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return domain.WrapError(domain.CodeInvalidArgument, "fake provider request is invalid", err)
	}
	if len(encoded) > childLineLimit {
		return domain.NewError(domain.CodeFrameTooLarge, "fake provider request exceeds limit")
	}
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	if _, err := p.stdin.Write(append(encoded, '\n')); err != nil {
		return domain.WrapError(domain.CodeProviderFailed, "fake provider request could not be written", err)
	}
	return nil
}

func (p *Process) Input(payload string) error {
	return p.send(childRequest{Version: fakeProviderProtocolVersion, Kind: "input", Payload: payload})
}

func (p *Process) Resize(rows, cols int) error {
	return p.send(childRequest{Version: fakeProviderProtocolVersion, Kind: "resize", Rows: rows, Cols: cols})
}

func (p *Process) Stop(ctx context.Context) error {
	if err := p.send(childRequest{Version: fakeProviderProtocolVersion, Kind: "stop"}); err != nil {
		return err
	}
	select {
	case <-p.done:
		return p.Err()
	case <-ctx.Done():
		return domain.WrapError(domain.CodeDeadlineExceeded, "fake provider graceful stop timed out", ctx.Err())
	}
}

func (p *Process) Kill() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return domain.WrapError(domain.CodeProviderFailed, "fake provider could not be killed", err)
	}
	return nil
}

func (p *Process) Err() error {
	if p == nil {
		return domain.NewError(domain.CodeProviderFailed, "fake provider is unavailable")
	}
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	return p.waitErr
}

func (p *Process) ExitCode() int {
	if p == nil {
		return -1
	}
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	return p.exitCode
}

func waitReady(p *Process, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-p.Ready():
		return nil
	case <-p.Done():
		return domain.NewError(domain.CodeProviderFailed, "fake provider exited before ready")
	case <-timer.C:
		return domain.NewError(domain.CodeDeadlineExceeded, "fake provider ready deadline exceeded")
	}
}
