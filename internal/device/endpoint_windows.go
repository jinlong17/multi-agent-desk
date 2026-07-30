//go:build windows

package device

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

const (
	pipeAccessDuplex           = 0x00000003
	fileFlagFirstPipeInstance  = 0x00080000
	fileFlagOverlapped         = 0x40000000
	pipeTypeMessage            = 0x00000004
	pipeReadmodeMessage        = 0x00000002
	pipeWait                   = 0x00000000
	pipeRejectRemoteClients    = 0x00000008
	pipeUnlimitedInstances     = 255
	genericRead                = 0x80000000
	genericWrite               = 0x40000000
	openExisting               = 3
	securitySQOSPresent        = 0x00100000
	securityIdentification     = 0x00010000
	securityDescriptorRevision = 1
	daclSecurityInformation    = 0x00000004
	seKernelObject             = 6
	errPipeBusy                = syscall.Errno(231)
	errMoreData                = syscall.Errno(234)
	errPipeConnected           = syscall.Errno(535)
	errOperationAborted        = syscall.Errno(995)
	errIOPending               = syscall.Errno(997)
)

var (
	kernel32                              = syscall.NewLazyDLL("kernel32.dll")
	advapi32                              = syscall.NewLazyDLL("advapi32.dll")
	procCreateNamedPipeW                  = kernel32.NewProc("CreateNamedPipeW")
	procConnectNamedPipe                  = kernel32.NewProc("ConnectNamedPipe")
	procCreateEventW                      = kernel32.NewProc("CreateEventW")
	procDisconnectNamedPipe               = kernel32.NewProc("DisconnectNamedPipe")
	procGetOverlappedResult               = kernel32.NewProc("GetOverlappedResult")
	procSetNamedPipeHandleState           = kernel32.NewProc("SetNamedPipeHandleState")
	procWaitNamedPipeW                    = kernel32.NewProc("WaitNamedPipeW")
	procCancelIoEx                        = kernel32.NewProc("CancelIoEx")
	procGetNamedPipeClientSessionID       = kernel32.NewProc("GetNamedPipeClientSessionId")
	procProcessIdToSessionId              = kernel32.NewProc("ProcessIdToSessionId")
	procConvertStringSecurityDescriptor   = advapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
	procConvertSecurityDescriptorToString = advapi32.NewProc("ConvertSecurityDescriptorToStringSecurityDescriptorW")
	procGetSecurityInfo                   = advapi32.NewProc("GetSecurityInfo")
)

type windowsListener struct {
	mu            sync.Mutex
	name          string
	pending       syscall.Handle
	accepting     syscall.Handle
	acceptStarted chan struct{}
	acceptDone    chan struct{}
	closed        bool
}

type pipeConn struct {
	handle     syscall.Handle
	mu         sync.Mutex
	operations sync.WaitGroup
	active     int
	closed     bool
	closeDone  chan struct{}
	closeErr   error
	deadline   time.Time
}

func Listen(root string) (Listener, error) {
	if err := verifyPrivateDirectory(root); err != nil {
		return nil, err
	}
	name := `\\.\pipe\` + endpointName(root)
	current, err := createNamedPipe(name, true)
	if err != nil {
		return nil, err
	}
	if err := verifyPipeDACL(current); err != nil {
		_ = syscall.CloseHandle(current)
		return nil, err
	}
	return &windowsListener{name: name, pending: current}, nil
}

func Dial(root string, timeout time.Duration) (io.ReadWriteCloser, error) {
	if err := verifyPrivateDirectory(root); err != nil {
		return nil, err
	}
	name := `\\.\pipe\` + endpointName(root)
	deadline := time.Now().Add(timeout)
	for {
		handle, err := openNamedPipe(name)
		if err == nil {
			return newPipeConn(handle), nil
		}
		if !errors.Is(err, errPipeBusy) || time.Now().After(deadline) {
			return nil, domain.WrapError(domain.CodeDaemonUnavailable, "daemon endpoint could not be reached", err)
		}
		path, _ := syscall.UTF16PtrFromString(name)
		procWaitNamedPipeW.Call(uintptr(unsafe.Pointer(path)), 250)
	}
}

func (l *windowsListener) Accept() (io.ReadWriteCloser, error) {
	l.mu.Lock()
	if l.closed || l.pending == 0 || l.accepting != 0 {
		l.mu.Unlock()
		return nil, os.ErrClosed
	}
	pending := l.pending
	l.pending = 0
	l.accepting = pending
	started := make(chan struct{})
	l.acceptStarted = started
	done := make(chan struct{})
	l.acceptDone = done
	l.mu.Unlock()

	connectErr := connectNamedPipe(pending, started)
	if connectErr == nil {
		connectErr = sameWindowsSession(pending)
	}
	var next syscall.Handle
	if connectErr == nil {
		next, connectErr = createNamedPipe(l.name, false)
	}

	l.mu.Lock()
	closed := l.closed
	if !closed && connectErr == nil {
		l.pending = next
		next = 0
	}
	l.accepting = 0
	l.acceptStarted = nil
	l.acceptDone = nil
	l.mu.Unlock()

	if next != 0 {
		_ = syscall.CloseHandle(next)
	}
	if closed || connectErr != nil {
		_ = disconnectAndClosePipe(pending)
		close(done)
		if closed || errors.Is(connectErr, errOperationAborted) {
			return nil, os.ErrClosed
		}
		return nil, connectErr
	}
	connection := newPipeConn(pending)
	close(done)
	return connection, nil
}

func (l *windowsListener) Close() error {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	pending := l.pending
	l.pending = 0
	accepting := l.accepting
	started := l.acceptStarted
	done := l.acceptDone
	l.mu.Unlock()
	if pending != 0 {
		_ = disconnectAndClosePipe(pending)
	}
	if accepting != 0 {
		if started != nil {
			<-started
		}
		_ = cancelPipeIO(accepting)
		if done != nil {
			<-done
		}
	}
	return nil
}

func (l *windowsListener) Address() string { return l.name }

func localEndpointPath(root string) (string, error) { return `\\.\pipe\` + endpointName(root), nil }

func (c *pipeConn) Read(data []byte) (int, error) {
	return c.doIO(data, false)
}

func (c *pipeConn) Write(data []byte) (int, error) {
	return c.doIO(data, true)
}

func (c *pipeConn) Close() error {
	c.mu.Lock()
	if c.closed {
		done := c.closeDone
		c.mu.Unlock()
		if done != nil {
			<-done
		}
		c.mu.Lock()
		err := c.closeErr
		c.mu.Unlock()
		return err
	}
	c.closed = true
	handle := c.handle
	c.mu.Unlock()

	_ = cancelPipeIO(handle)
	c.operations.Wait()
	err := syscall.CloseHandle(handle)

	c.mu.Lock()
	c.handle = 0
	c.closeErr = err
	close(c.closeDone)
	c.mu.Unlock()
	return err
}

func (c *pipeConn) SetDeadline(deadline time.Time) error {
	c.mu.Lock()
	c.deadline = deadline
	c.mu.Unlock()
	return nil
}

func newPipeConn(handle syscall.Handle) *pipeConn {
	return &pipeConn{handle: handle, closeDone: make(chan struct{})}
}

func (c *pipeConn) doIO(data []byte, write bool) (int, error) {
	overlapped, cleanup, err := newOverlapped()
	if err != nil {
		return 0, err
	}
	defer cleanup()

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0, os.ErrClosed
	}
	deadline := c.deadline
	if !deadline.IsZero() && time.Now().After(deadline) {
		c.mu.Unlock()
		return 0, os.ErrDeadlineExceeded
	}
	handle := c.handle
	c.operations.Add(1)
	c.active++
	var transferred uint32
	if write {
		err = syscall.WriteFile(handle, data, &transferred, overlapped)
	} else {
		err = syscall.ReadFile(handle, data, &transferred, overlapped)
	}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.active--
		c.mu.Unlock()
		c.operations.Done()
	}()

	var timer *time.Timer
	if !deadline.IsZero() {
		timerDone := make(chan struct{})
		timer = time.AfterFunc(time.Until(deadline), func() {
			_ = cancelOverlappedIO(handle, overlapped)
			close(timerDone)
		})
		defer func() {
			if !timer.Stop() {
				<-timerDone
			}
		}()
	}
	if errors.Is(err, errIOPending) {
		err = waitOverlapped(handle, overlapped, &transferred)
	}
	runtime.KeepAlive(data)
	runtime.KeepAlive(overlapped)
	if err != nil {
		c.mu.Lock()
		closed := c.closed
		c.mu.Unlock()
		if closed && errors.Is(err, errOperationAborted) {
			return int(transferred), os.ErrClosed
		}
		if !deadline.IsZero() && !time.Now().Before(deadline) && errors.Is(err, errOperationAborted) {
			return int(transferred), os.ErrDeadlineExceeded
		}
		if !write && errors.Is(err, errMoreData) {
			return int(transferred), nil
		}
	}
	return int(transferred), err
}

func createNamedPipe(name string, first bool) (syscall.Handle, error) {
	path, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	sa, cleanup, err := currentLogonSecurityAttributes()
	if err != nil {
		return 0, err
	}
	defer cleanup()
	flags := uint32(fileFlagOverlapped)
	if first {
		flags |= fileFlagFirstPipeInstance
	}
	value, _, callErr := procCreateNamedPipeW.Call(uintptr(unsafe.Pointer(path)), uintptr(pipeAccessDuplex|flags),
		uintptr(pipeTypeMessage|pipeReadmodeMessage|pipeWait|pipeRejectRemoteClients), uintptr(pipeUnlimitedInstances),
		uintptr(64*1024), uintptr(64*1024), uintptr(5000), uintptr(unsafe.Pointer(sa)))
	handle := syscall.Handle(value)
	if handle == syscall.InvalidHandle {
		return 0, fmt.Errorf("named pipe could not be created: %w", callErr)
	}
	return handle, nil
}

func connectNamedPipe(handle syscall.Handle, started chan<- struct{}) error {
	overlapped, cleanup, err := newOverlapped()
	if err != nil {
		close(started)
		return err
	}
	defer cleanup()
	ok, _, err := procConnectNamedPipe.Call(uintptr(handle), uintptr(unsafe.Pointer(overlapped)))
	close(started)
	if ok != 0 || errors.Is(err, errPipeConnected) {
		return nil
	}
	if errors.Is(err, errIOPending) {
		var transferred uint32
		return waitOverlapped(handle, overlapped, &transferred)
	}
	return fmt.Errorf("named pipe connection failed: %w", err)
}

func openNamedPipe(name string) (syscall.Handle, error) {
	path, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	handle, err := syscall.CreateFile(path, genericRead|genericWrite, 0, nil, openExisting,
		securitySQOSPresent|securityIdentification|fileFlagOverlapped, 0)
	if err != nil {
		return 0, err
	}
	mode := uint32(pipeReadmodeMessage)
	ok, _, callErr := procSetNamedPipeHandleState.Call(uintptr(handle), uintptr(unsafe.Pointer(&mode)), 0, 0)
	if ok == 0 {
		_ = syscall.CloseHandle(handle)
		return 0, fmt.Errorf("pipe read mode could not be set: %w", callErr)
	}
	return handle, nil
}

func disconnectAndClosePipe(handle syscall.Handle) error {
	procDisconnectNamedPipe.Call(uintptr(handle))
	return syscall.CloseHandle(handle)
}

func cancelPipeIO(handle syscall.Handle) error {
	return cancelOverlappedIO(handle, nil)
}

func cancelOverlappedIO(handle syscall.Handle, overlapped *syscall.Overlapped) error {
	ok, _, err := procCancelIoEx.Call(uintptr(handle), uintptr(unsafe.Pointer(overlapped)))
	if ok == 0 && !errors.Is(err, syscall.ERROR_NOT_FOUND) {
		return err
	}
	return nil
}

func newOverlapped() (*syscall.Overlapped, func(), error) {
	value, _, callErr := procCreateEventW.Call(0, 1, 0, 0)
	handle := syscall.Handle(value)
	if handle == 0 || handle == syscall.InvalidHandle {
		return nil, nil, fmt.Errorf("overlapped event could not be created: %w", callErr)
	}
	overlapped := &syscall.Overlapped{HEvent: handle}
	return overlapped, func() { _ = syscall.CloseHandle(handle) }, nil
}

func waitOverlapped(handle syscall.Handle, overlapped *syscall.Overlapped, transferred *uint32) error {
	ok, _, callErr := procGetOverlappedResult.Call(uintptr(handle), uintptr(unsafe.Pointer(overlapped)),
		uintptr(unsafe.Pointer(transferred)), 1)
	if ok == 0 {
		return callErr
	}
	return nil
}

func sameWindowsSession(handle syscall.Handle) error {
	var peer, self uint32
	ok, _, _ := procGetNamedPipeClientSessionID.Call(uintptr(handle), uintptr(unsafe.Pointer(&peer)))
	if ok == 0 {
		return domain.NewError(domain.CodePermissionDenied, "peer session could not be verified")
	}
	ok, _, _ = procProcessIdToSessionId.Call(uintptr(syscall.Getpid()), uintptr(unsafe.Pointer(&self)))
	if ok == 0 || peer != self {
		return domain.NewError(domain.CodePermissionDenied, "peer session is not local")
	}
	return nil
}

func currentLogonSecurityAttributes() (*syscall.SecurityAttributes, func(), error) {
	sid, err := currentLogonSID()
	if err != nil {
		return nil, nil, err
	}
	sddl := fmt.Sprintf("D:P(D;;GA;;;NU)(A;;GA;;;%s)", sid)
	value, err := syscall.UTF16PtrFromString(sddl)
	if err != nil {
		return nil, nil, err
	}
	var descriptor uintptr
	var size uint32
	ok, _, callErr := procConvertStringSecurityDescriptor.Call(uintptr(unsafe.Pointer(value)), securityDescriptorRevision,
		uintptr(unsafe.Pointer(&descriptor)), uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return nil, nil, fmt.Errorf("security descriptor could not be created: %w", callErr)
	}
	sa := &syscall.SecurityAttributes{Length: uint32(unsafe.Sizeof(syscall.SecurityAttributes{})), SecurityDescriptor: descriptor}
	return sa, func() { _, _ = syscall.LocalFree(syscall.Handle(descriptor)) }, nil
}

func verifyPipeDACL(handle syscall.Handle) error {
	var descriptor uintptr
	status, _, _ := procGetSecurityInfo.Call(uintptr(handle), seKernelObject, daclSecurityInformation, 0, 0, 0, 0, uintptr(unsafe.Pointer(&descriptor)))
	if status != 0 {
		return domain.NewError(domain.CodePermissionDenied, "named pipe policy could not be read back")
	}
	defer func() { _, _ = syscall.LocalFree(syscall.Handle(descriptor)) }()
	var text *uint16
	var length uint32
	ok, _, _ := procConvertSecurityDescriptorToString.Call(descriptor, securityDescriptorRevision, daclSecurityInformation,
		uintptr(unsafe.Pointer(&text)), uintptr(unsafe.Pointer(&length)))
	if ok == 0 {
		return domain.NewError(domain.CodePermissionDenied, "named pipe policy could not be read back")
	}
	defer func() { _, _ = syscall.LocalFree(syscall.Handle(uintptr(unsafe.Pointer(text)))) }()
	policy := utf16PointerString(text)
	sid, err := currentLogonSID()
	if err != nil {
		return err
	}
	if !contains(policy, sid) || !contains(policy, ";;;NU") || contains(policy, ";;;WD") || contains(policy, ";;;AN") {
		return domain.NewError(domain.CodePermissionDenied, "named pipe policy is too broad")
	}
	return nil
}

func contains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}

func currentLogonSID() (string, error) {
	token, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return "", err
	}
	defer token.Close()
	var size uint32
	if err := syscall.GetTokenInformation(token, syscall.TokenLogonSid, nil, 0, &size); !errors.Is(err, syscall.ERROR_INSUFFICIENT_BUFFER) || size == 0 {
		return "", err
	}
	buffer := make([]byte, size)
	if err := syscall.GetTokenInformation(token, syscall.TokenLogonSid, &buffer[0], size, &size); err != nil {
		return "", err
	}
	type groups struct {
		Count uint32
		Group [1]syscall.SIDAndAttributes
	}
	value := (*groups)(unsafe.Pointer(&buffer[0]))
	if value.Count != 1 || value.Group[0].Sid == nil {
		return "", errors.New("current logon SID is unavailable")
	}
	return value.Group[0].Sid.String()
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
	runtime.KeepAlive(pointer)
	return string(utf16.Decode(values))
}
