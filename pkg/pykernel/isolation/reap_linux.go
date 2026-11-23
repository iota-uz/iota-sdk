//go:build linux

package isolation

import (
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// ReapGroup reaps processes of process group pgid that were reparented to
// this process and exited. When the host is PID 1 of a container (or a child
// subreaper), killed kernel descendants become its children and would stay
// zombies forever; it reaps them by exact pid, so exit statuses of the host's
// other children are never consumed. It returns how many members of the group
// are still present.
func ReapGroup(pgid int) int { return reapGroupExcept(pgid, 0) }

// reapGroupExcept is ReapGroup leaving process keep (the group leader) alone.
func reapGroupExcept(pgid, keep int) int {
	if pgid <= 0 {
		return 0
	}
	self := os.Getpid()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	left := 0
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		raw, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		// pid (comm) state ppid pgrp ...; comm may contain spaces/parens.
		s := string(raw)
		i := strings.LastIndexByte(s, ')')
		if i < 0 {
			continue
		}
		f := strings.Fields(s[i+1:])
		if len(f) < 3 {
			continue
		}
		ppid, _ := strconv.Atoi(f[1])
		pgrp, _ := strconv.Atoi(f[2])
		if pgrp != pgid || pid == keep {
			continue
		}
		if ppid == self && f[0] == "Z" {
			var ws unix.WaitStatus
			if n, _ := unix.Wait4(pid, &ws, unix.WNOHANG, nil); n == pid {
				continue
			}
		}
		left++
	}
	return left
}

// emptyGroup kills and reaps every member of process group pgid except its
// leader. The leader must have exited but not be reaped: its zombie keeps the
// group id reserved, so the group signal cannot reach an unrelated process.
func emptyGroup(pgid int) {
	for {
		_ = unix.Kill(-pgid, unix.SIGKILL)
		if reapGroupExcept(pgid, pgid) == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
