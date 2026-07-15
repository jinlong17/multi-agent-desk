//go:build !windows

package device

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

type unixListener struct {
	listener net.Listener
	path     string
}

func Listen(root string) (Listener, error) {
	if err := verifyPrivateDirectory(root); err != nil {
		return nil, err
	}
	path, err := localEndpointPath(root)
	if err != nil {
		return nil, err
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeSocket == 0 {
			return nil, domain.NewError(domain.CodeConflict, "daemon endpoint is occupied")
		}
		// A stale socket is ambiguous ownership. Never unlink it automatically.
		return nil, domain.NewError(domain.CodeConflict, "daemon endpoint ownership is ambiguous")
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, domain.WrapError(domain.CodeConflict, "daemon endpoint could not be inspected", err)
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, domain.WrapError(domain.CodeConflict, "daemon endpoint could not be opened", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		return nil, domain.WrapError(domain.CodePermissionDenied, "daemon endpoint permissions could not be restricted", err)
	}
	return &unixListener{listener: listener, path: path}, nil
}

func Dial(root string, timeout time.Duration) (io.ReadWriteCloser, error) {
	if err := verifyPrivateDirectory(root); err != nil {
		return nil, err
	}
	path, err := localEndpointPath(root)
	if err != nil {
		return nil, err
	}
	connection, err := net.DialTimeout("unix", path, timeout)
	if err != nil {
		return nil, domain.WrapError(domain.CodeDaemonUnavailable, "daemon endpoint could not be reached", err)
	}
	return connection, nil
}

func (l *unixListener) Accept() (io.ReadWriteCloser, error) {
	connection, err := l.listener.Accept()
	if err != nil {
		return nil, err
	}
	return connection, nil
}

func (l *unixListener) Close() error {
	err := l.listener.Close()
	if removeErr := os.Remove(l.path); err == nil && removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		err = removeErr
	}
	return err
}

func (l *unixListener) Address() string { return l.path }

func localEndpointPath(root string) (string, error) {
	path := filepath.Join(root, "daemon.sock")
	// macOS sockaddr_un has a small path field. Keep the endpoint in the
	// private Device root when possible; long temporary/test roots use a
	// private hashed directory while protocol authentication remains mandatory.
	if len(path) <= 100 {
		return path, nil
	}
	directory := filepath.Join(os.TempDir(), "multidesk-ipc")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", domain.WrapError(domain.CodeConflict, "fallback daemon endpoint directory could not be created", err)
	}
	if err := verifyPrivateDirectory(directory); err != nil {
		return "", err
	}
	return filepath.Join(directory, endpointName(root)+".sock"), nil
}
