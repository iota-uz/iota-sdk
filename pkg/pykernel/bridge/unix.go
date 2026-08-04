package bridge

import (
	"fmt"
	"net"
	"os"
)

// ListenUnix creates a unix-domain socket listener at path with 0700
// permissions, clearing any stale socket first. The kernel subprocess connects
// to this socket (it learns the path from its sandbox workdir). The caller owns
// and closes the returned listener.
func ListenUnix(path string) (net.Listener, error) {
	if err := os.RemoveAll(path); err != nil {
		return nil, fmt.Errorf("pykernel/bridge: remove stale socket %q: %w", path, err)
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("pykernel/bridge: listen unix %q: %w", path, err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		_ = l.Close()
		return nil, fmt.Errorf("pykernel/bridge: chmod socket %q: %w", path, err)
	}
	return l, nil
}

// Accept waits for the kernel to connect and returns a Bridge over that
// connection. At most one connection is expected per kernel.
func Accept(l net.Listener) (Bridge, error) {
	conn, err := l.Accept()
	if err != nil {
		return nil, fmt.Errorf("pykernel/bridge: accept: %w", err)
	}
	return New(conn), nil
}
