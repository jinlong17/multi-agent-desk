//go:build windows && named_pipe_spike

// Command named-pipe-probe validates the Windows local daemon IPC mechanism.
// It is Spike-only evidence, not production transport code.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	pipeAccessDuplex          = 0x00000003
	fileFlagFirstPipeInstance = 0x00080000
	pipeTypeMessage           = 0x00000004
	pipeReadmodeMessage       = 0x00000002
	pipeWait                  = 0x00000000
	pipeRejectRemoteClients   = 0x00000008
	pipeUnlimitedInstances    = 255
	genericRead               = 0x80000000
	genericWrite              = 0x40000000
	openExisting              = 3
	securitySQOSPresent       = 0x00100000
	securityIdentification    = 0x00010000
	sddlRevision1             = 1
	daclSecurityInformation   = 0x00000004
	seKernelObject            = 6
	maxMessageBytes           = 1 << 20
)

var (
	kernel32                              = syscall.NewLazyDLL("kernel32.dll")
	advapi32                              = syscall.NewLazyDLL("advapi32.dll")
	procCreateNamedPipeW                  = kernel32.NewProc("CreateNamedPipeW")
	procConnectNamedPipe                  = kernel32.NewProc("ConnectNamedPipe")
	procDisconnectNamedPipe               = kernel32.NewProc("DisconnectNamedPipe")
	procFlushFileBuffers                  = kernel32.NewProc("FlushFileBuffers")
	procWaitNamedPipeW                    = kernel32.NewProc("WaitNamedPipeW")
	procSetNamedPipeHandleState           = kernel32.NewProc("SetNamedPipeHandleState")
	procGetNamedPipeClientProcessID       = kernel32.NewProc("GetNamedPipeClientProcessId")
	procGetNamedPipeClientSessionID       = kernel32.NewProc("GetNamedPipeClientSessionId")
	procProcessIDToSessionID              = kernel32.NewProc("ProcessIdToSessionId")
	procGetProcessHandleCount             = kernel32.NewProc("GetProcessHandleCount")
	procGetCurrentThread                  = kernel32.NewProc("GetCurrentThread")
	procConvertStringSecurityDescriptor   = advapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
	procConvertSecurityDescriptorToString = advapi32.NewProc("ConvertSecurityDescriptorToStringSecurityDescriptorW")
	procGetSecurityInfo                   = advapi32.NewProc("GetSecurityInfo")
	procImpersonateAnonymousToken         = advapi32.NewProc("ImpersonateAnonymousToken")
	procRevertToSelf                      = advapi32.NewProc("RevertToSelf")
)

var (
	errPipeBusy      = syscall.Errno(231)
	errMoreData      = syscall.Errno(234)
	errPipeConnected = syscall.Errno(535)
)

type tokenGroups struct {
	GroupCount uint32
	Groups     [1]syscall.SIDAndAttributes
}

type request struct {
	Version int    `json:"version"`
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Payload string `json:"payload"`
}

type response struct {
	Version       int    `json:"version"`
	ID            string `json:"id"`
	AcceptedBytes int    `json:"accepted_bytes"`
	ServerPID     int    `json:"server_pid"`
}

type connectionPlan struct {
	Sequence int
	Abrupt   bool
}

type serverStats struct {
	RoundTrips             int
	AbruptRecovered        int
	UniqueClientPIDs       map[uint32]struct{}
	ClientSessionMatches   int
	ActualSecurityDesc     string
	Transcript             []byte
	LargestMessageBytes    int
	FirstPipeInstance      bool
	RejectRemoteClientFlag bool
}

type serverOutcome struct {
	Stats serverStats
	Err   error
}

type result struct {
	SchemaVersion          int      `json:"schema_version"`
	Supported              bool     `json:"supported"`
	ExecutedAtUTC          string   `json:"executed_at_utc"`
	GOOS                   string   `json:"goos"`
	GOARCH                 string   `json:"goarch"`
	GoVersion              string   `json:"go_version"`
	RunnerImageOS          string   `json:"runner_image_os"`
	RunnerImageVersion     string   `json:"runner_image_version"`
	WindowsVersion         string   `json:"windows_version"`
	LogonSID               string   `json:"logon_sid"`
	RequestedSecurityDesc  string   `json:"requested_security_descriptor"`
	ActualSecurityDesc     string   `json:"actual_security_descriptor"`
	DefaultDACLUsed        bool     `json:"default_dacl_used"`
	FirstPipeInstance      bool     `json:"first_pipe_instance"`
	RejectRemoteClients    bool     `json:"reject_remote_clients"`
	AnonymousDenied        bool     `json:"anonymous_denied"`
	AnonymousDenial        string   `json:"anonymous_denial"`
	RemotePathDenied       bool     `json:"remote_path_denied"`
	RemotePathDenial       string   `json:"remote_path_denial"`
	RoundTrips             int      `json:"round_trips"`
	IndependentClientPIDs  int      `json:"independent_client_pids"`
	ClientSessionMatches   int      `json:"client_session_matches"`
	AbruptDisconnects      int      `json:"abrupt_disconnects_recovered"`
	LargestMessageBytes    int      `json:"largest_message_bytes"`
	MessageBoundaries      bool     `json:"message_boundaries_preserved"`
	TranscriptSHA256       string   `json:"transcript_sha256"`
	HandlesBefore          uint32   `json:"handles_before"`
	HandlesMid             uint32   `json:"handles_mid"`
	HandlesAfter           uint32   `json:"handles_after"`
	FirstHalfHandleGrowth  int64    `json:"first_half_handle_growth"`
	SecondHalfHandleGrowth int64    `json:"second_half_handle_growth"`
	ShutdownDurationMS     int64    `json:"shutdown_duration_ms"`
	SecurityReviewRequired bool     `json:"security_review_required"`
	Limitations            []string `json:"limitations"`
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--client":
			mustRun(clientMain(os.Args[2:]))
			return
		case "--abrupt-client":
			mustRun(abruptClientMain(os.Args[2:]))
			return
		case "--anonymous-deny":
			mustRun(anonymousDenyMain(os.Args[2:]))
			return
		case "--remote-deny":
			mustRun(remoteDenyMain(os.Args[2:]))
			return
		}
	}

	resultPath := flag.String("result", "named-pipe-result.json", "sanitized JSON result path")
	roundTrips := flag.Int("round-trips", 100, "successful independent client processes")
	flag.Parse()
	if err := parentMain(*resultPath, *roundTrips); err != nil {
		fmt.Fprintln(os.Stderr, "named pipe probe failed:", err)
		os.Exit(1)
	}
}

func mustRun(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parentMain(resultPath string, roundTrips int) error {
	if runtime.GOOS != "windows" {
		return errors.New("this probe requires Windows")
	}
	if roundTrips < 100 {
		return errors.New("round-trips must be at least 100")
	}

	logonSID, err := currentLogonSID()
	if err != nil {
		return err
	}
	requestedSDDL := fmt.Sprintf("D:P(D;;GA;;;NU)(A;;GA;;;%s)", logonSID)
	sa, freeDescriptor, err := securityAttributes(requestedSDDL)
	if err != nil {
		return err
	}
	defer freeDescriptor()

	nameSuffix := make([]byte, 8)
	if _, err := rand.Read(nameSuffix); err != nil {
		return fmt.Errorf("random pipe suffix: %w", err)
	}
	shortName := fmt.Sprintf("multidesk-spike-%d-%s", os.Getpid(), hex.EncodeToString(nameSuffix))
	localPath := `\\.\pipe\` + shortName
	computerName, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("computer name: %w", err)
	}
	remotePath := `\\` + computerName + `\pipe\` + shortName

	plans := make([]connectionPlan, 0, roundTrips+1)
	for sequence := 1; sequence <= roundTrips; sequence++ {
		if sequence == 51 {
			plans = append(plans, connectionPlan{Abrupt: true})
		}
		plans = append(plans, connectionPlan{Sequence: sequence})
	}

	ready := make(chan connectionPlan)
	serverDone := make(chan serverOutcome, 1)

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve probe executable: %w", err)
	}
	// Warm os/exec and the Go Windows poller before taking a baseline so
	// process-lifetime runtime handles are not mistaken for connection leaks.
	if err := exec.Command("cmd.exe", "/d", "/c", "exit 0").Run(); err != nil {
		return fmt.Errorf("warm process launcher: %w", err)
	}
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	handlesBefore, err := processHandleCount()
	if err != nil {
		return err
	}
	go runServer(localPath, sa, plans, ready, serverDone)

	completed := 0
	var handlesMid uint32
	for _, planned := range plans {
		select {
		case announced := <-ready:
			if announced != planned {
				return fmt.Errorf("server plan mismatch: got %+v want %+v", announced, planned)
			}
		case outcome := <-serverDone:
			return fmt.Errorf("server exited before connection %+v: %w", planned, outcome.Err)
		case <-time.After(5 * time.Second):
			return fmt.Errorf("server did not publish connection %+v", planned)
		}

		if completed == 0 {
			anonymousOutput, err := runChild(executable, 8*time.Second, "--anonymous-deny", "-pipe", localPath)
			if err != nil {
				return fmt.Errorf("anonymous denial probe: %w; output=%s", err, anonymousOutput)
			}
			remoteOutput, err := runChild(executable, 8*time.Second, "--remote-deny", "-pipe", remotePath)
			if err != nil {
				return fmt.Errorf("remote denial probe: %w; output=%s", err, remoteOutput)
			}
		}

		if planned.Abrupt {
			output, err := runChild(executable, 8*time.Second, "--abrupt-client", "-pipe", localPath)
			if err != nil {
				return fmt.Errorf("abrupt client: %w; output=%s", err, output)
			}
			continue
		}

		payloadBytes := 128
		if planned.Sequence == roundTrips {
			payloadBytes = 70 * 1024
		}
		output, err := runChild(
			executable,
			12*time.Second,
			"--client",
			"-pipe", localPath,
			"-sequence", strconv.Itoa(planned.Sequence),
			"-payload-bytes", strconv.Itoa(payloadBytes),
		)
		if err != nil {
			return fmt.Errorf("client %d: %w; output=%s", planned.Sequence, err, output)
		}
		completed++
		if completed == roundTrips/2 {
			runtime.GC()
			time.Sleep(200 * time.Millisecond)
			handlesMid, err = processHandleCount()
			if err != nil {
				return err
			}
		}
	}

	shutdownStarted := time.Now()
	var outcome serverOutcome
	select {
	case outcome = <-serverDone:
	case <-time.After(5 * time.Second):
		return errors.New("server did not shut down within five seconds")
	}
	shutdownDuration := time.Since(shutdownStarted)
	if outcome.Err != nil {
		return outcome.Err
	}
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	handlesAfter, err := processHandleCount()
	if err != nil {
		return err
	}
	firstHalfGrowth := int64(handlesMid) - int64(handlesBefore)
	secondHalfGrowth := int64(handlesAfter) - int64(handlesMid)
	if secondHalfGrowth > 4 {
		return fmt.Errorf("handles continued growing with client count: before=%d mid=%d after=%d", handlesBefore, handlesMid, handlesAfter)
	}

	stats := outcome.Stats
	if stats.RoundTrips != roundTrips || stats.AbruptRecovered != 1 || len(stats.UniqueClientPIDs) != roundTrips || stats.ClientSessionMatches != len(plans) || stats.LargestMessageBytes < 70*1024 {
		return fmt.Errorf("server assertions failed: %+v", stats)
	}
	if !strings.Contains(stats.ActualSecurityDesc, logonSID) || strings.Contains(stats.ActualSecurityDesc, ";;;WD)") || strings.Contains(stats.ActualSecurityDesc, ";;;AN)") {
		return fmt.Errorf("actual DACL is not current-logon-only: %s", stats.ActualSecurityDesc)
	}

	digest := sha256.Sum256(stats.Transcript)
	probeResult := result{
		SchemaVersion:          1,
		Supported:              true,
		ExecutedAtUTC:          time.Now().UTC().Format(time.RFC3339),
		GOOS:                   runtime.GOOS,
		GOARCH:                 runtime.GOARCH,
		GoVersion:              runtime.Version(),
		RunnerImageOS:          os.Getenv("ImageOS"),
		RunnerImageVersion:     os.Getenv("ImageVersion"),
		WindowsVersion:         windowsVersion(),
		LogonSID:               logonSID,
		RequestedSecurityDesc:  requestedSDDL,
		ActualSecurityDesc:     stats.ActualSecurityDesc,
		DefaultDACLUsed:        false,
		FirstPipeInstance:      stats.FirstPipeInstance,
		RejectRemoteClients:    stats.RejectRemoteClientFlag,
		AnonymousDenied:        true,
		AnonymousDenial:        "CreateFile denied while impersonating the anonymous token",
		RemotePathDenied:       true,
		RemotePathDenial:       "remote-style machine path could not connect to the PIPE_REJECT_REMOTE_CLIENTS endpoint",
		RoundTrips:             stats.RoundTrips,
		IndependentClientPIDs:  len(stats.UniqueClientPIDs),
		ClientSessionMatches:   stats.ClientSessionMatches,
		AbruptDisconnects:      stats.AbruptRecovered,
		LargestMessageBytes:    stats.LargestMessageBytes,
		MessageBoundaries:      true,
		TranscriptSHA256:       hex.EncodeToString(digest[:]),
		HandlesBefore:          handlesBefore,
		HandlesMid:             handlesMid,
		HandlesAfter:           handlesAfter,
		FirstHalfHandleGrowth:  firstHalfGrowth,
		SecondHalfHandleGrowth: secondHalfGrowth,
		ShutdownDurationMS:     shutdownDuration.Milliseconds(),
		SecurityReviewRequired: true,
		Limitations: []string{
			"The GitHub-hosted x64 runner is Windows Server, not a physical Windows 11 workstation.",
			"The probe validates one current-logon session and does not create a second interactive Windows user session.",
			"Production IPC still requires protocol authentication, request authorization, deadlines, rate limits, payload bounds, and ControllerLease enforcement above the OS transport.",
		},
	}
	encoded, err := json.MarshalIndent(probeResult, "", "  ")
	if err != nil {
		return fmt.Errorf("encode result: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(resultPath, encoded, 0o600); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	fmt.Print(string(encoded))
	return nil
}

func runServer(path string, sa *syscall.SecurityAttributes, plans []connectionPlan, ready chan<- connectionPlan, done chan<- serverOutcome) {
	stats := serverStats{
		UniqueClientPIDs:       map[uint32]struct{}{},
		FirstPipeInstance:      true,
		RejectRemoteClientFlag: true,
	}
	fail := func(err error) {
		done <- serverOutcome{Stats: stats, Err: err}
	}

	serverSession, err := currentProcessSessionID()
	if err != nil {
		fail(err)
		return
	}
	for index, planned := range plans {
		pipe, err := createServerPipe(path, sa)
		if err != nil {
			fail(err)
			return
		}
		if index == 0 {
			stats.ActualSecurityDesc, err = objectDACLString(pipe)
			if err != nil {
				syscall.CloseHandle(pipe)
				fail(err)
				return
			}
		}
		ready <- planned
		if err := connectPipe(pipe); err != nil {
			syscall.CloseHandle(pipe)
			fail(err)
			return
		}

		clientPID, err := namedPipeClientProcessID(pipe)
		if err != nil {
			disconnectAndClose(pipe, false)
			fail(err)
			return
		}
		clientSession, err := namedPipeClientSessionID(pipe)
		if err != nil {
			disconnectAndClose(pipe, false)
			fail(err)
			return
		}
		if clientSession != serverSession {
			disconnectAndClose(pipe, false)
			fail(fmt.Errorf("client session %d differs from server session %d", clientSession, serverSession))
			return
		}
		stats.ClientSessionMatches++

		message, err := readPipeMessage(pipe)
		if err != nil {
			disconnectAndClose(pipe, false)
			fail(err)
			return
		}
		if len(message) > stats.LargestMessageBytes {
			stats.LargestMessageBytes = len(message)
		}
		if planned.Abrupt {
			var invalid request
			if json.Unmarshal(message, &invalid) == nil {
				disconnectAndClose(pipe, false)
				fail(errors.New("abrupt client unexpectedly sent valid JSON"))
				return
			}
			stats.AbruptRecovered++
			disconnectAndClose(pipe, false)
			continue
		}

		var incoming request
		if err := json.Unmarshal(message, &incoming); err != nil {
			disconnectAndClose(pipe, false)
			fail(fmt.Errorf("decode request: %w", err))
			return
		}
		if incoming.Version != 1 || incoming.ID != fmt.Sprintf("req-%03d", planned.Sequence) || incoming.Kind != "probe.echo" {
			disconnectAndClose(pipe, false)
			fail(fmt.Errorf("unexpected request: %+v", incoming))
			return
		}
		outgoing := response{Version: 1, ID: incoming.ID, AcceptedBytes: len(incoming.Payload), ServerPID: os.Getpid()}
		encoded, err := json.Marshal(outgoing)
		if err != nil {
			disconnectAndClose(pipe, false)
			fail(err)
			return
		}
		if err := writePipeMessage(pipe, encoded); err != nil {
			disconnectAndClose(pipe, false)
			fail(err)
			return
		}
		stats.Transcript = append(stats.Transcript, message...)
		stats.Transcript = append(stats.Transcript, encoded...)
		stats.RoundTrips++
		stats.UniqueClientPIDs[clientPID] = struct{}{}
		disconnectAndClose(pipe, true)
	}
	done <- serverOutcome{Stats: stats}
}

func createServerPipe(path string, sa *syscall.SecurityAttributes) (syscall.Handle, error) {
	pathUTF16, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	handle, _, callErr := procCreateNamedPipeW.Call(
		uintptr(unsafe.Pointer(pathUTF16)),
		pipeAccessDuplex|fileFlagFirstPipeInstance,
		pipeTypeMessage|pipeReadmodeMessage|pipeWait|pipeRejectRemoteClients,
		pipeUnlimitedInstances,
		64*1024,
		64*1024,
		5000,
		uintptr(unsafe.Pointer(sa)),
	)
	pipe := syscall.Handle(handle)
	if pipe == syscall.InvalidHandle {
		return 0, fmt.Errorf("CreateNamedPipeW: %w", callErr)
	}
	return pipe, nil
}

func connectPipe(pipe syscall.Handle) error {
	ok, _, callErr := procConnectNamedPipe.Call(uintptr(pipe), 0)
	if ok != 0 || errors.Is(callErr, errPipeConnected) {
		return nil
	}
	return fmt.Errorf("ConnectNamedPipe: %w", callErr)
}

func disconnectAndClose(pipe syscall.Handle, flush bool) {
	if flush {
		procFlushFileBuffers.Call(uintptr(pipe))
	}
	procDisconnectNamedPipe.Call(uintptr(pipe))
	_ = syscall.CloseHandle(pipe)
}

func clientMain(args []string) error {
	flags := flag.NewFlagSet("client", flag.ContinueOnError)
	pipePath := flags.String("pipe", "", "pipe path")
	sequence := flags.Int("sequence", 0, "request sequence")
	payloadBytes := flags.Int("payload-bytes", 0, "payload size")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *pipePath == "" || *sequence < 1 || *payloadBytes < 1 {
		return errors.New("client arguments are incomplete")
	}
	pipe, err := openClientPipe(*pipePath)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(pipe)
	incoming := request{Version: 1, ID: fmt.Sprintf("req-%03d", *sequence), Kind: "probe.echo", Payload: strings.Repeat("x", *payloadBytes)}
	encoded, err := json.Marshal(incoming)
	if err != nil {
		return err
	}
	if err := writePipeMessage(pipe, encoded); err != nil {
		return err
	}
	replyBytes, err := readPipeMessage(pipe)
	if err != nil {
		return err
	}
	var reply response
	if err := json.Unmarshal(replyBytes, &reply); err != nil {
		return err
	}
	if reply.Version != 1 || reply.ID != incoming.ID || reply.AcceptedBytes != len(incoming.Payload) || reply.ServerPID == os.Getpid() {
		return fmt.Errorf("unexpected response: %+v", reply)
	}
	fmt.Printf("client-ok|%s|%d\n", reply.ID, reply.AcceptedBytes)
	return nil
}

func abruptClientMain(args []string) error {
	flags := flag.NewFlagSet("abrupt-client", flag.ContinueOnError)
	pipePath := flags.String("pipe", "", "pipe path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	pipe, err := openClientPipe(*pipePath)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(pipe)
	if err := writePipeMessage(pipe, []byte(`{"version":1,"id":"truncated"`)); err != nil {
		return err
	}
	fmt.Println("abrupt-client-sent")
	return nil
}

func anonymousDenyMain(args []string) error {
	flags := flag.NewFlagSet("anonymous-deny", flag.ContinueOnError)
	pipePath := flags.String("pipe", "", "pipe path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	thread, _, _ := procGetCurrentThread.Call()
	ok, _, impersonateErr := procImpersonateAnonymousToken.Call(thread)
	if ok == 0 {
		return fmt.Errorf("ImpersonateAnonymousToken: %w", impersonateErr)
	}
	defer procRevertToSelf.Call()
	pipe, err := openClientPipeOnce(*pipePath)
	if err == nil {
		syscall.CloseHandle(pipe)
		return errors.New("anonymous token unexpectedly connected")
	}
	fmt.Println("anonymous-denied|" + err.Error())
	return nil
}

func remoteDenyMain(args []string) error {
	flags := flag.NewFlagSet("remote-deny", flag.ContinueOnError)
	pipePath := flags.String("pipe", "", "pipe path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	pipe, err := openClientPipeOnce(*pipePath)
	if err == nil {
		syscall.CloseHandle(pipe)
		return errors.New("remote-style path unexpectedly connected")
	}
	fmt.Println("remote-denied|" + err.Error())
	return nil
}

func openClientPipe(path string) (syscall.Handle, error) {
	deadline := time.Now().Add(5 * time.Second)
	for {
		pipe, err := openClientPipeOnce(path)
		if err == nil {
			return pipe, nil
		}
		if !errors.Is(err, errPipeBusy) || time.Now().After(deadline) {
			return 0, err
		}
		pathUTF16, _ := syscall.UTF16PtrFromString(path)
		procWaitNamedPipeW.Call(uintptr(unsafe.Pointer(pathUTF16)), 250)
	}
}

func openClientPipeOnce(path string) (syscall.Handle, error) {
	pathUTF16, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	pipe, err := syscall.CreateFile(
		pathUTF16,
		genericRead|genericWrite,
		0,
		nil,
		openExisting,
		securitySQOSPresent|securityIdentification,
		0,
	)
	if err != nil {
		return 0, fmt.Errorf("CreateFile(%s): %w", path, err)
	}
	mode := uint32(pipeReadmodeMessage)
	ok, _, modeErr := procSetNamedPipeHandleState.Call(uintptr(pipe), uintptr(unsafe.Pointer(&mode)), 0, 0)
	if ok == 0 {
		syscall.CloseHandle(pipe)
		return 0, fmt.Errorf("SetNamedPipeHandleState: %w", modeErr)
	}
	return pipe, nil
}

func readPipeMessage(pipe syscall.Handle) ([]byte, error) {
	message := make([]byte, 0, 4096)
	buffer := make([]byte, 4096)
	for {
		var read uint32
		err := syscall.ReadFile(pipe, buffer, &read, nil)
		message = append(message, buffer[:read]...)
		if len(message) > maxMessageBytes {
			return nil, errors.New("message exceeds one MiB probe bound")
		}
		if err == nil {
			return message, nil
		}
		if errors.Is(err, errMoreData) {
			continue
		}
		return nil, fmt.Errorf("ReadFile: %w", err)
	}
}

func writePipeMessage(pipe syscall.Handle, message []byte) error {
	var written uint32
	if err := syscall.WriteFile(pipe, message, &written, nil); err != nil {
		return fmt.Errorf("WriteFile: %w", err)
	}
	if int(written) != len(message) {
		return fmt.Errorf("short write: %d/%d", written, len(message))
	}
	return nil
}

func currentLogonSID() (string, error) {
	token, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return "", fmt.Errorf("open process token: %w", err)
	}
	defer token.Close()
	var size uint32
	err = syscall.GetTokenInformation(token, syscall.TokenLogonSid, nil, 0, &size)
	if !errors.Is(err, syscall.ERROR_INSUFFICIENT_BUFFER) || size == 0 {
		return "", fmt.Errorf("size TokenLogonSid: %w", err)
	}
	buffer := make([]byte, size)
	if err := syscall.GetTokenInformation(token, syscall.TokenLogonSid, &buffer[0], uint32(len(buffer)), &size); err != nil {
		return "", fmt.Errorf("read TokenLogonSid: %w", err)
	}
	groups := (*tokenGroups)(unsafe.Pointer(&buffer[0]))
	if groups.GroupCount != 1 || groups.Groups[0].Sid == nil {
		return "", fmt.Errorf("unexpected TokenLogonSid group count %d", groups.GroupCount)
	}
	value, err := groups.Groups[0].Sid.String()
	if err != nil {
		return "", fmt.Errorf("format logon SID: %w", err)
	}
	return value, nil
}

func securityAttributes(sddl string) (*syscall.SecurityAttributes, func(), error) {
	sddlUTF16, err := syscall.UTF16PtrFromString(sddl)
	if err != nil {
		return nil, nil, err
	}
	var descriptor uintptr
	var descriptorBytes uint32
	ok, _, callErr := procConvertStringSecurityDescriptor.Call(
		uintptr(unsafe.Pointer(sddlUTF16)),
		sddlRevision1,
		uintptr(unsafe.Pointer(&descriptor)),
		uintptr(unsafe.Pointer(&descriptorBytes)),
	)
	if ok == 0 {
		return nil, nil, fmt.Errorf("ConvertStringSecurityDescriptorToSecurityDescriptorW: %w", callErr)
	}
	sa := &syscall.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(syscall.SecurityAttributes{})),
		SecurityDescriptor: descriptor,
		InheritHandle:      0,
	}
	cleanup := func() {
		_, _ = syscall.LocalFree(syscall.Handle(descriptor))
	}
	return sa, cleanup, nil
}

func objectDACLString(handle syscall.Handle) (string, error) {
	var descriptor uintptr
	status, _, _ := procGetSecurityInfo.Call(
		uintptr(handle),
		seKernelObject,
		daclSecurityInformation,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&descriptor)),
	)
	if status != 0 {
		return "", fmt.Errorf("GetSecurityInfo status %d", status)
	}
	defer syscall.LocalFree(syscall.Handle(descriptor))
	var text *uint16
	var textLen uint32
	ok, _, callErr := procConvertSecurityDescriptorToString.Call(
		descriptor,
		sddlRevision1,
		daclSecurityInformation,
		uintptr(unsafe.Pointer(&text)),
		uintptr(unsafe.Pointer(&textLen)),
	)
	if ok == 0 {
		return "", fmt.Errorf("ConvertSecurityDescriptorToStringSecurityDescriptorW: %w", callErr)
	}
	defer syscall.LocalFree(syscall.Handle(uintptr(unsafe.Pointer(text))))
	return utf16PointerString(text), nil
}

func utf16PointerString(pointer *uint16) string {
	if pointer == nil {
		return ""
	}
	values := make([]uint16, 0, 128)
	for offset := uintptr(0); ; offset += 2 {
		value := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(pointer)) + offset))
		if value == 0 {
			break
		}
		values = append(values, value)
	}
	return string(utf16.Decode(values))
}

func namedPipeClientProcessID(pipe syscall.Handle) (uint32, error) {
	var value uint32
	ok, _, callErr := procGetNamedPipeClientProcessID.Call(uintptr(pipe), uintptr(unsafe.Pointer(&value)))
	if ok == 0 {
		return 0, fmt.Errorf("GetNamedPipeClientProcessId: %w", callErr)
	}
	return value, nil
}

func namedPipeClientSessionID(pipe syscall.Handle) (uint32, error) {
	var value uint32
	ok, _, callErr := procGetNamedPipeClientSessionID.Call(uintptr(pipe), uintptr(unsafe.Pointer(&value)))
	if ok == 0 {
		return 0, fmt.Errorf("GetNamedPipeClientSessionId: %w", callErr)
	}
	return value, nil
}

func currentProcessSessionID() (uint32, error) {
	var value uint32
	ok, _, callErr := procProcessIDToSessionID.Call(uintptr(os.Getpid()), uintptr(unsafe.Pointer(&value)))
	if ok == 0 {
		return 0, fmt.Errorf("ProcessIdToSessionId: %w", callErr)
	}
	return value, nil
}

func processHandleCount() (uint32, error) {
	process, err := syscall.GetCurrentProcess()
	if err != nil {
		return 0, err
	}
	var value uint32
	ok, _, callErr := procGetProcessHandleCount.Call(uintptr(process), uintptr(unsafe.Pointer(&value)))
	if ok == 0 {
		return 0, fmt.Errorf("GetProcessHandleCount: %w", callErr)
	}
	return value, nil
}

func runChild(executable string, timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, executable, args...)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		return string(output), ctx.Err()
	}
	return string(output), err
}

func windowsVersion() string {
	output, err := exec.Command("cmd.exe", "/d", "/c", "ver").CombinedOutput()
	if err != nil {
		return "unavailable: " + err.Error()
	}
	return strings.TrimSpace(string(output))
}
