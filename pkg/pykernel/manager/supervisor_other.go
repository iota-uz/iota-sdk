//go:build !linux

package manager

import (
	"errors"
	"net"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
)

var errNoSupervisor = errors.New("the workdir quota supervisor requires Linux")

func recvListener(net.Conn, time.Duration) (int, error) { return -1, errNoSupervisor }

func startSupervisor(*kernel, int, *isolation.Credential) error { return errNoSupervisor }
