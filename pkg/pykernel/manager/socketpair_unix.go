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
	// Hold ForkLock across Socketpair and set FD_CLOEXEC on both ends before a
	// concurrent fork+exec elsewhere in the process can observe (and leak) the
	// raw fds into an unrelated child. The child end is still inherited fine:
	// exec.Cmd.ExtraFiles dups it and clears CLOEXEC on the child's copy.
	syscall.ForkLock.RLock()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		syscall.ForkLock.RUnlock()
		return nil, nil, err
	}
	syscall.CloseOnExec(fds[0])
	syscall.CloseOnExec(fds[1])
	syscall.ForkLock.RUnlock()

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
