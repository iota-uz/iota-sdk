//go:build !unix

package manager

import (
	"errors"
	"net"
	"os"
)

func socketPair() (net.Conn, *os.File, error) {
	return nil, nil, errors.New("pykernel/manager: socket pair not supported on this platform")
}
