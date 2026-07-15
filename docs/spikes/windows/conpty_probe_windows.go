//go:build windows && !named_pipe_spike

// Command conpty-probe exercises the Windows pseudoconsole contract without
// introducing a production dependency. It launches a full-screen fixture in a
// real ConPTY, drives input and resize events, captures the UTF-8/VT stream,
// replays semantic markers, and verifies bounded teardown.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	procThreadAttributePseudoConsole = 0x00020016
	extendedStartupInfoPresent       = 0x00080000
	createUnicodeEnvironment         = 0x00000400
	waitObject0                      = 0x00000000
	waitTimeout                      = 0x00000102
)

var (
	kernel32                         = syscall.NewLazyDLL("kernel32.dll")
	procCreatePseudoConsole          = kernel32.NewProc("CreatePseudoConsole")
	procResizePseudoConsole          = kernel32.NewProc("ResizePseudoConsole")
	procClosePseudoConsole           = kernel32.NewProc("ClosePseudoConsole")
	procInitializeProcThreadAttrList = kernel32.NewProc("InitializeProcThreadAttributeList")
	procUpdateProcThreadAttribute    = kernel32.NewProc("UpdateProcThreadAttribute")
	procDeleteProcThreadAttrList     = kernel32.NewProc("DeleteProcThreadAttributeList")
	procCreateProcessW               = kernel32.NewProc("CreateProcessW")
	procWaitForSingleObject          = kernel32.NewProc("WaitForSingleObject")
	procGetExitCodeProcess           = kernel32.NewProc("GetExitCodeProcess")
	procTerminateProcess             = kernel32.NewProc("TerminateProcess")
	procGetConsoleScreenBufferInfo   = kernel32.NewProc("GetConsoleScreenBufferInfo")
)

type coord struct {
	X int16
	Y int16
}

type smallRect struct {
	Left   int16
	Top    int16
	Right  int16
	Bottom int16
}

type consoleScreenBufferInfo struct {
	Size              coord
	CursorPosition    coord
	Attributes        uint16
	Window            smallRect
	MaximumWindowSize coord
}

type startupInfoEx struct {
	StartupInfo   syscall.StartupInfo
	AttributeList *byte
}

type synchronizedBuffer struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.Write(p)
}

func (b *synchronizedBuffer) snapshot() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return bytes.Clone(b.data.Bytes())
}

type resizeResult struct {
	Tag      string `json:"tag"`
	Expected string `json:"expected"`
	Observed string `json:"observed"`
}

type replaySummary struct {
	ReadyMarkers   int      `json:"ready_markers"`
	ResizeMarkers  []string `json:"resize_markers"`
	HistoryMarkers int      `json:"history_markers"`
	FrameMarkers   int      `json:"frame_markers"`
	ByeMarkers     int      `json:"bye_markers"`
}

type result struct {
	SchemaVersion       int            `json:"schema_version"`
	Supported           bool           `json:"supported"`
	ExecutedAtUTC       string         `json:"executed_at_utc"`
	GOOS                string         `json:"goos"`
	GOARCH              string         `json:"goarch"`
	GoVersion           string         `json:"go_version"`
	RunnerImageOS       string         `json:"runner_image_os"`
	RunnerImageVersion  string         `json:"runner_image_version"`
	WindowsVersion      string         `json:"windows_version"`
	InitialSize         string         `json:"initial_size"`
	Resizes             []resizeResult `json:"resizes"`
	StressDurationMS    int64          `json:"stress_duration_ms"`
	FramesSent          int            `json:"frames_sent"`
	HistoryLines        int            `json:"history_lines"`
	CapturedBytes       int            `json:"captured_bytes"`
	TranscriptSHA256    string         `json:"transcript_sha256"`
	AlternateScreenOpen bool           `json:"alternate_screen_open"`
	AlternateScreenExit bool           `json:"alternate_screen_exit"`
	WholeReplay         replaySummary  `json:"whole_replay"`
	ChunkedReplay       replaySummary  `json:"chunked_replay"`
	ReplayEquivalent    bool           `json:"replay_equivalent"`
	ProcessExitCode     uint32         `json:"process_exit_code"`
	ReaderReachedEOF    bool           `json:"reader_reached_eof"`
	TeardownDurationMS  int64          `json:"teardown_duration_ms"`
	Limitations         []string       `json:"limitations"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--child" {
		childMain()
		return
	}

	duration := flag.Duration("duration", 15*time.Second, "interactive stress duration")
	resultPath := flag.String("result", "conpty-result.json", "sanitized JSON result path")
	flag.Parse()

	if err := parentMain(*duration, *resultPath); err != nil {
		fmt.Fprintln(os.Stderr, "conpty probe failed:", err)
		os.Exit(1)
	}
}

func parentMain(duration time.Duration, resultPath string) (retErr error) {
	if runtime.GOOS != "windows" {
		return errors.New("this probe requires Windows")
	}
	if duration < time.Second {
		return errors.New("duration must be at least one second")
	}

	const historyLines = 512
	initial := coord{X: 120, Y: 40}
	resizes := []struct {
		tag  string
		size coord
	}{
		{tag: "compact", size: coord{X: 80, Y: 24}},
		{tag: "wide", size: coord{X: 132, Y: 43}},
		{tag: "restore", size: coord{X: 100, Y: 30}},
	}

	var inputRead, inputWrite syscall.Handle
	var outputRead, outputWrite syscall.Handle
	if err := syscall.CreatePipe(&inputRead, &inputWrite, nil, 0); err != nil {
		return fmt.Errorf("create input pipe: %w", err)
	}
	defer closeHandle(&inputRead)
	defer closeHandle(&inputWrite)
	if err := syscall.CreatePipe(&outputRead, &outputWrite, nil, 0); err != nil {
		return fmt.Errorf("create output pipe: %w", err)
	}
	defer closeHandle(&outputRead)
	defer closeHandle(&outputWrite)

	var pseudoConsole uintptr
	if hr, _, _ := procCreatePseudoConsole.Call(
		packCoord(initial),
		uintptr(inputRead),
		uintptr(outputWrite),
		0,
		uintptr(unsafe.Pointer(&pseudoConsole)),
	); uint32(hr) != 0 {
		return fmt.Errorf("CreatePseudoConsole HRESULT 0x%08x", uint32(hr))
	}
	pseudoConsoleOpen := true
	defer func() {
		if pseudoConsoleOpen {
			procClosePseudoConsole.Call(pseudoConsole)
		}
	}()

	attributeList, err := makePseudoConsoleAttributeList(pseudoConsole)
	if err != nil {
		return err
	}
	defer procDeleteProcThreadAttrList.Call(uintptr(unsafe.Pointer(&attributeList[0])))

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve probe executable: %w", err)
	}
	commandLine, err := syscall.UTF16FromString(quoteWindowsArgument(executable) + " --child")
	if err != nil {
		return fmt.Errorf("encode child command line: %w", err)
	}
	si := startupInfoEx{AttributeList: &attributeList[0]}
	si.StartupInfo.Cb = uint32(unsafe.Sizeof(si))
	var pi syscall.ProcessInformation
	created, _, createErr := procCreateProcessW.Call(
		0,
		uintptr(unsafe.Pointer(&commandLine[0])),
		0,
		0,
		0,
		createUnicodeEnvironment|extendedStartupInfoPresent,
		0,
		0,
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	if created == 0 {
		return fmt.Errorf("CreateProcessW: %w", createErr)
	}
	defer syscall.CloseHandle(pi.Thread)
	defer syscall.CloseHandle(pi.Process)

	// Once the child owns the ConPTY attachment, the host keeps only its pipe
	// ends. The reader remains active on a separate goroutine through teardown.
	closeHandle(&inputRead)
	closeHandle(&outputWrite)
	input := os.NewFile(uintptr(inputWrite), "conpty-input")
	output := os.NewFile(uintptr(outputRead), "conpty-output")
	if input == nil || output == nil {
		return errors.New("convert ConPTY pipe handles")
	}
	inputWrite = 0
	outputRead = 0
	defer input.Close()
	defer output.Close()

	var captured synchronizedBuffer
	readerDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(&captured, output)
		readerDone <- copyErr
	}()

	if err := waitForMarker(&captured, "READY|120x40", 10*time.Second); err != nil {
		terminateProcess(pi.Process)
		return err
	}

	resizeEvidence := make([]resizeResult, 0, len(resizes))
	for _, resize := range resizes {
		if hr, _, _ := procResizePseudoConsole.Call(pseudoConsole, packCoord(resize.size)); uint32(hr) != 0 {
			terminateProcess(pi.Process)
			return fmt.Errorf("ResizePseudoConsole(%s) HRESULT 0x%08x", resize.tag, uint32(hr))
		}
		if err := writeCommand(input, "SIZE|"+resize.tag); err != nil {
			terminateProcess(pi.Process)
			return err
		}
		expected := formatCoord(resize.size)
		marker := "SIZE-ACK|" + resize.tag + "|" + expected
		if err := waitForMarker(&captured, marker, 5*time.Second); err != nil {
			terminateProcess(pi.Process)
			return err
		}
		resizeEvidence = append(resizeEvidence, resizeResult{Tag: resize.tag, Expected: expected, Observed: expected})
	}

	if err := writeCommand(input, fmt.Sprintf("FLOOD|%d", historyLines)); err != nil {
		terminateProcess(pi.Process)
		return err
	}
	if err := waitForMarker(&captured, fmt.Sprintf("HISTORY|%04d|scrollback", historyLines-1), 10*time.Second); err != nil {
		terminateProcess(pi.Process)
		return err
	}

	stressStarted := time.Now()
	deadline := stressStarted.Add(duration)
	framesSent := 0
	for time.Now().Before(deadline) {
		framesSent++
		if err := writeCommand(input, fmt.Sprintf("FRAME|%06d", framesSent)); err != nil {
			terminateProcess(pi.Process)
			return err
		}
		if framesSent%64 == 0 {
			marker := fmt.Sprintf("FRAME-ACK|%06d", framesSent)
			if err := waitForMarker(&captured, marker, 5*time.Second); err != nil {
				terminateProcess(pi.Process)
				return err
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	stressDuration := time.Since(stressStarted)
	if err := waitForMarker(&captured, fmt.Sprintf("FRAME-ACK|%06d", framesSent), 10*time.Second); err != nil {
		terminateProcess(pi.Process)
		return err
	}

	if err := writeCommand(input, "EXIT"); err != nil {
		terminateProcess(pi.Process)
		return err
	}
	if err := waitForMarker(&captured, "BYE|clean", 5*time.Second); err != nil {
		terminateProcess(pi.Process)
		return err
	}

	teardownStarted := time.Now()
	waitResult, _, waitErr := procWaitForSingleObject.Call(uintptr(pi.Process), uintptr((10 * time.Second).Milliseconds()))
	if uint32(waitResult) == waitTimeout {
		terminateProcess(pi.Process)
		return errors.New("child did not exit within 10 seconds")
	}
	if uint32(waitResult) != waitObject0 {
		return fmt.Errorf("WaitForSingleObject result 0x%08x: %w", uint32(waitResult), waitErr)
	}
	var exitCode uint32
	if ok, _, getErr := procGetExitCodeProcess.Call(uintptr(pi.Process), uintptr(unsafe.Pointer(&exitCode))); ok == 0 {
		return fmt.Errorf("GetExitCodeProcess: %w", getErr)
	}
	if exitCode != 0 {
		return fmt.Errorf("child exit code %d", exitCode)
	}

	input.Close()
	procClosePseudoConsole.Call(pseudoConsole)
	pseudoConsoleOpen = false
	readerReachedEOF := false
	select {
	case readErr := <-readerDone:
		if readErr != nil && !errors.Is(readErr, os.ErrClosed) {
			return fmt.Errorf("read ConPTY output: %w", readErr)
		}
		readerReachedEOF = true
	case <-time.After(5 * time.Second):
		output.Close()
		return errors.New("ConPTY output reader did not reach EOF within five seconds")
	}
	teardownDuration := time.Since(teardownStarted)

	transcript := captured.snapshot()
	wholeReplay := replay(transcript, len(transcript))
	chunkedReplay := replay(transcript, 17)
	replayEquivalent := equalReplay(wholeReplay, chunkedReplay)
	altOpen := bytes.Contains(transcript, []byte("\x1b[?1049h"))
	altExit := bytes.Contains(transcript, []byte("\x1b[?1049l"))
	if wholeReplay.ReadyMarkers < 1 || len(wholeReplay.ResizeMarkers) != len(resizes) || wholeReplay.HistoryMarkers != historyLines || wholeReplay.FrameMarkers != framesSent || wholeReplay.ByeMarkers != 1 || !replayEquivalent || !altOpen || !altExit {
		return fmt.Errorf("replay assertions failed: whole=%+v chunked=%+v frames=%d alt=%t/%t", wholeReplay, chunkedReplay, framesSent, altOpen, altExit)
	}

	digest := sha256.Sum256(transcript)
	probeResult := result{
		SchemaVersion:       1,
		Supported:           true,
		ExecutedAtUTC:       time.Now().UTC().Format(time.RFC3339),
		GOOS:                runtime.GOOS,
		GOARCH:              runtime.GOARCH,
		GoVersion:           runtime.Version(),
		RunnerImageOS:       os.Getenv("ImageOS"),
		RunnerImageVersion:  os.Getenv("ImageVersion"),
		WindowsVersion:      windowsVersion(),
		InitialSize:         formatCoord(initial),
		Resizes:             resizeEvidence,
		StressDurationMS:    stressDuration.Milliseconds(),
		FramesSent:          framesSent,
		HistoryLines:        historyLines,
		CapturedBytes:       len(transcript),
		TranscriptSHA256:    hex.EncodeToString(digest[:]),
		AlternateScreenOpen: altOpen,
		AlternateScreenExit: altExit,
		WholeReplay:         wholeReplay,
		ChunkedReplay:       chunkedReplay,
		ReplayEquivalent:    replayEquivalent,
		ProcessExitCode:     exitCode,
		ReaderReachedEOF:    readerReachedEOF,
		TeardownDurationMS:  teardownDuration.Milliseconds(),
		Limitations: []string{
			"The GitHub-hosted x64 runner is Windows Server, not a physical Windows 11 workstation.",
			"The probe validates ConPTY transport and VT capture; the host application remains responsible for terminal rendering and scrollback storage.",
			"Mouse, IME, accessibility, GPU rendering, and a real Codex or Claude full-screen binary are outside this minimal protocol probe.",
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

func childMain() {
	closeConsole, err := attachChildConsole()
	if err != nil {
		fmt.Fprintln(os.Stderr, "attach ConPTY console:", err)
		os.Exit(2)
	}
	defer closeConsole()

	out := bufio.NewWriterSize(os.Stdout, 64*1024)
	defer out.Flush()
	writeChild(out, "\x1b[?1049h\x1b[2J\x1b[H")
	width, height, err := consoleSize()
	if err != nil {
		writeChild(out, "ERROR|console-size|"+err.Error()+"\r\n")
		os.Exit(3)
	}
	writeChild(out, fmt.Sprintf("READY|%dx%d\r\n", width, height))

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		switch {
		case strings.HasPrefix(line, "SIZE|"):
			tag := strings.TrimPrefix(line, "SIZE|")
			width, height, err := consoleSize()
			if err != nil {
				writeChild(out, "ERROR|console-size|"+err.Error()+"\r\n")
				os.Exit(4)
			}
			writeChild(out, fmt.Sprintf("SIZE-ACK|%s|%dx%d\r\n", tag, width, height))
		case strings.HasPrefix(line, "FLOOD|"):
			count, err := strconv.Atoi(strings.TrimPrefix(line, "FLOOD|"))
			if err != nil || count < 1 || count > 4096 {
				writeChild(out, "ERROR|bad-flood\r\n")
				os.Exit(5)
			}
			for i := 0; i < count; i++ {
				writeChild(out, fmt.Sprintf("HISTORY|%04d|scrollback\r\n", i))
			}
		case strings.HasPrefix(line, "FRAME|"):
			sequence := strings.TrimPrefix(line, "FRAME|")
			writeChild(out, "\x1b[H\x1b[2J\x1b[1;1H")
			writeChild(out, "MultiAgentDesk provider TUI fixture\r\n")
			writeChild(out, "session: interactive\r\n")
			writeChild(out, "frame: "+sequence+"\r\n")
			writeChild(out, "FRAME-ACK|"+sequence+"\r\n")
		case line == "EXIT":
			writeChild(out, "\x1b[?1049lBYE|clean\r\n")
			return
		default:
			writeChild(out, "ERROR|unknown-command\r\n")
			os.Exit(6)
		}
	}
	if err := scanner.Err(); err != nil {
		writeChild(out, "ERROR|stdin|"+err.Error()+"\r\n")
		os.Exit(7)
	}
}

// Go preserves the process standard-handle values supplied by CreateProcess.
// Opening the console device names after the pseudoconsole attachment makes
// the fixture's streams explicit and avoids accidentally using the host
// runner's inherited PowerShell handles.
func attachChildConsole() (func(), error) {
	input, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open CONIN$: %w", err)
	}
	output, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		input.Close()
		return nil, fmt.Errorf("open CONOUT$: %w", err)
	}
	os.Stdin = input
	os.Stdout = output
	os.Stderr = output
	return func() {
		_ = input.Close()
		_ = output.Close()
	}, nil
}

func writeChild(out *bufio.Writer, value string) {
	_, _ = out.WriteString(value)
	_ = out.Flush()
}

func consoleSize() (int, int, error) {
	var info consoleScreenBufferInfo
	ok, _, callErr := procGetConsoleScreenBufferInfo.Call(os.Stdout.Fd(), uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return 0, 0, callErr
	}
	return int(info.Size.X), int(info.Size.Y), nil
}

func makePseudoConsoleAttributeList(pseudoConsole uintptr) ([]byte, error) {
	var size uintptr
	procInitializeProcThreadAttrList.Call(0, 1, 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 {
		return nil, errors.New("InitializeProcThreadAttributeList returned zero size")
	}
	list := make([]byte, size)
	ok, _, initErr := procInitializeProcThreadAttrList.Call(uintptr(unsafe.Pointer(&list[0])), 1, 0, uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return nil, fmt.Errorf("InitializeProcThreadAttributeList: %w", initErr)
	}
	ok, _, updateErr := procUpdateProcThreadAttribute.Call(
		uintptr(unsafe.Pointer(&list[0])),
		0,
		procThreadAttributePseudoConsole,
		pseudoConsole,
		unsafe.Sizeof(pseudoConsole),
		0,
		0,
	)
	if ok == 0 {
		procDeleteProcThreadAttrList.Call(uintptr(unsafe.Pointer(&list[0])))
		return nil, fmt.Errorf("UpdateProcThreadAttribute: %w", updateErr)
	}
	return list, nil
}

func packCoord(value coord) uintptr {
	return uintptr(uint32(uint16(value.X)) | uint32(uint16(value.Y))<<16)
}

func formatCoord(value coord) string {
	return fmt.Sprintf("%dx%d", value.X, value.Y)
}

func quoteWindowsArgument(value string) string {
	return "\"" + strings.ReplaceAll(value, "\"", "\\\"") + "\""
}

func writeCommand(input *os.File, command string) error {
	if _, err := io.WriteString(input, command+"\r\n"); err != nil {
		return fmt.Errorf("write command %q: %w", command, err)
	}
	return nil
}

func waitForMarker(captured *synchronizedBuffer, marker string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if bytes.Contains(captured.snapshot(), []byte(marker)) {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %q; captured tail=%q", marker, tail(captured.snapshot(), 800))
}

func tail(value []byte, count int) []byte {
	if len(value) <= count {
		return value
	}
	return value[len(value)-count:]
}

func replay(transcript []byte, chunkSize int) replaySummary {
	if chunkSize < 1 {
		chunkSize = len(transcript)
	}
	var accumulated bytes.Buffer
	for start := 0; start < len(transcript); start += chunkSize {
		end := start + chunkSize
		if end > len(transcript) {
			end = len(transcript)
		}
		_, _ = accumulated.Write(transcript[start:end])
	}
	text := accumulated.String()
	summary := replaySummary{
		ReadyMarkers:   strings.Count(text, "READY|120x40"),
		HistoryMarkers: strings.Count(text, "HISTORY|"),
		FrameMarkers:   strings.Count(text, "FRAME-ACK|"),
		ByeMarkers:     strings.Count(text, "BYE|clean"),
	}
	for _, marker := range []string{
		"SIZE-ACK|compact|80x24",
		"SIZE-ACK|restore|100x30",
		"SIZE-ACK|wide|132x43",
	} {
		if strings.Contains(text, marker) {
			summary.ResizeMarkers = append(summary.ResizeMarkers, marker)
		}
	}
	return summary
}

func equalReplay(left, right replaySummary) bool {
	if left.ReadyMarkers != right.ReadyMarkers || left.HistoryMarkers != right.HistoryMarkers || left.FrameMarkers != right.FrameMarkers || left.ByeMarkers != right.ByeMarkers || len(left.ResizeMarkers) != len(right.ResizeMarkers) {
		return false
	}
	for i := range left.ResizeMarkers {
		if left.ResizeMarkers[i] != right.ResizeMarkers[i] {
			return false
		}
	}
	return true
}

func windowsVersion() string {
	output, err := exec.Command("cmd.exe", "/d", "/c", "ver").CombinedOutput()
	if err != nil {
		return "unavailable: " + err.Error()
	}
	return strings.TrimSpace(string(output))
}

func terminateProcess(process syscall.Handle) {
	procTerminateProcess.Call(uintptr(process), 1)
	procWaitForSingleObject.Call(uintptr(process), 5000)
}

func closeHandle(handle *syscall.Handle) {
	if handle == nil || *handle == 0 || *handle == syscall.InvalidHandle {
		return
	}
	_ = syscall.CloseHandle(*handle)
	*handle = 0
}
