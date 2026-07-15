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
	seFileObject               = 1
	seKernelObject             = 6
	protectedDACLInformation   = 0x80000000
	errPipeBusy                = syscall.Errno(231)
	errMoreData                = syscall.Errno(234)
	errPipeConnected           = syscall.Errno(535)
)

var (
	kernel32                              = syscall.NewLazyDLL("kernel32.dll")
	advapi32                              = syscall.NewLazyDLL("advapi32.dll")
	procCreateNamedPipeW                  = kernel32.NewProc("CreateNamedPipeW")
	procConnectNamedPipe                  = kernel32.NewProc("ConnectNamedPipe")
	procDisconnectNamedPipe               = kernel32.NewProc("DisconnectNamedPipe")
	procSetNamedPipeHandleState           = kernel32.NewProc("SetNamedPipeHandleState")
	procWaitNamedPipeW                    = kernel32.NewProc("WaitNamedPipeW")
	procCancelIoEx                        = kernel32.NewProc("CancelIoEx")
	procGetNamedPipeClientSessionID       = kernel32.NewProc("GetNamedPipeClientSessionId")
	procProcessIdToSessionId              = kernel32.NewProc("ProcessIdToSessionId")
	procConvertStringSecurityDescriptor   = advapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
	procConvertSecurityDescriptorToString = advapi32.NewProc("ConvertSecurityDescriptorToStringSecurityDescriptorW")
	procGetSecurityInfo                   = advapi32.NewProc("GetSecurityInfo")
	procGetNamedSecurityInfo              = advapi32.NewProc("GetNamedSecurityInfoW")
	procSetFileSecurity                   = advapi32.NewProc("SetFileSecurityW")
)

type windowsListener struct {
	mu      sync.Mutex
	name    string
	pending syscall.Handle
	closed  bool
}

type pipeConn struct {
	handle   syscall.Handle
	mu       sync.Mutex
	closed   bool
	deadline time.Time
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
			return &pipeConn{handle: handle}, nil
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
	if l.closed {
		l.mu.Unlock()
		return nil, os.ErrClosed
	}
	pending := l.pending
	l.pending = 0
	l.mu.Unlock()
	if pending == 0 {
		return nil, os.ErrClosed
	}
	if err := connectNamedPipe(pending); err != nil {
		_ = syscall.CloseHandle(pending)
		return nil, err
	}
	if err := sameWindowsSession(pending); err != nil {
		_ = disconnectAndClosePipe(pending)
		return nil, err
	}
	connection := &pipeConn{handle: pending}
	next, err := createNamedPipe(l.name, false)
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		_ = syscall.CloseHandle(next)
		_ = connection.Close()
		return nil, os.ErrClosed
	}
	l.pending = next
	l.mu.Unlock()
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
	l.mu.Unlock()
	if pending != 0 {
		_ = disconnectAndClosePipe(pending)
	}
	return nil
}

func (l *windowsListener) Address() string { return l.name }

func localEndpointPath(root string) (string, error) { return `\\.\pipe\` + endpointName(root), nil }

func (c *pipeConn) Read(data []byte) (int, error) {
	return c.withDeadline(func() (int, error) {
		var read uint32
		err := syscall.ReadFile(c.handle, data, &read, nil)
		if err != nil && !errors.Is(err, errMoreData) {
			return int(read), err
		}
		return int(read), nil
	})
}

func (c *pipeConn) Write(data []byte) (int, error) {
	return c.withDeadline(func() (int, error) {
		var written uint32
		err := syscall.WriteFile(c.handle, data, &written, nil)
		return int(written), err
	})
}

func (c *pipeConn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	_ = cancelPipeIO(c.handle)
	handle := c.handle
	c.mu.Unlock()
	return syscall.CloseHandle(handle)
}

func (c *pipeConn) SetDeadline(deadline time.Time) error {
	c.mu.Lock()
	c.deadline = deadline
	c.mu.Unlock()
	return nil
}

func (c *pipeConn) withDeadline(fn func() (int, error)) (int, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0, os.ErrClosed
	}
	deadline := c.deadline
	c.mu.Unlock()
	if !deadline.IsZero() && time.Now().After(deadline) {
		return 0, os.ErrDeadlineExceeded
	}
	var timer *time.Timer
	if !deadline.IsZero() {
		timer = time.AfterFunc(time.Until(deadline), func() { _ = cancelPipeIO(c.handle) })
		defer timer.Stop()
	}
	n, err := fn()
	if err != nil && !deadline.IsZero() && time.Now().After(deadline) {
		return n, os.ErrDeadlineExceeded
	}
	return n, err
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
	flags := uint32(0)
	if first {
		flags = fileFlagFirstPipeInstance
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

func connectNamedPipe(handle syscall.Handle) error {
	ok, _, err := procConnectNamedPipe.Call(uintptr(handle), 0)
	if ok != 0 || errors.Is(err, errPipeConnected) {
		return nil
	}
	return fmt.Errorf("named pipe connection failed: %w", err)
}

func openNamedPipe(name string) (syscall.Handle, error) {
	path, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	handle, err := syscall.CreateFile(path, genericRead|genericWrite, 0, nil, openExisting,
		securitySQOSPresent|securityIdentification, 0)
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
	ok, _, err := procCancelIoEx.Call(uintptr(handle), 0)
	if ok == 0 && !errors.Is(err, syscall.ERROR_NOT_FOUND) {
		return err
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
