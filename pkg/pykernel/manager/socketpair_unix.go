//go:build unix

package manager

import (
	"net"
	"os"
	"syscall"
)

// socketPair returns a connected AF_UNIX stream pair: the host end as a net.Conn
// and the child end as an *os.File to hand the subprocess as fd 3. Passing the
// socket by inherited fd avoids the sun_path length limit on filesystem socket
// paths and keeps no socket file on disk.
func socketPair() (net.Conn, *os.File, error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, nil, err
	}
	hostFile := os.NewFile(uintptr(fds[0]), "pykernel-host")
	childFile := os.NewFile(uintptr(fds[1]), "pykernel-child")

	conn, err := net.FileConn(hostFile) // dups the host fd into a net.Conn
	_ = hostFile.Close()
	if err != nil {
		_ = childFile.Close()
		return nil, nil, err
	}
	return conn, childFile, nil
}
