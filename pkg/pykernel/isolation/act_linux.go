//go:build linux

package isolation

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// verifyActFor proves the host can act for the confined process pid the way
// the quota supervisor does: from a thread with the kernel's credentials,
// duplicate one of its fds (pidfd_getfd) and read its memory.
func verifyActFor(pid int, cred *Credential) error {
	errc := make(chan error, 1)
	go func() {
		runtime.LockOSThread() // never unlocked: a switched thread dies with the goroutine
		errc <- actFor(pid, cred)
	}()
	return <-errc
}

func actFor(pid int, cred *Credential) error {
	if cred != nil {
		if _, _, e := unix.RawSyscall(unix.SYS_SETRESGID, uintptr(cred.GID), uintptr(cred.GID), ^uintptr(0)); e != 0 {
			return fmt.Errorf("setresgid: %w", e)
		}
		if _, _, e := unix.RawSyscall(unix.SYS_SETRESUID, uintptr(cred.UID), uintptr(cred.UID), ^uintptr(0)); e != 0 {
			return fmt.Errorf("setresuid: %w", e)
		}
	}
	pidfd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		return fmt.Errorf("pidfd_open: %w", err)
	}
	defer unix.Close(pidfd)
	fd, err := unix.PidfdGetfd(pidfd, 1, 0)
	if err != nil {
		return fmt.Errorf("pidfd_getfd: %w", err)
	}
	_ = unix.Close(fd)
	addr, err := readableAddr(pid)
	if err != nil {
		return err
	}
	buf := make([]byte, 8)
	local := []unix.Iovec{{Base: &buf[0]}}
	local[0].SetLen(len(buf))
	if _, err := unix.ProcessVMReadv(pid, local, []unix.RemoteIovec{{Base: addr, Len: len(buf)}}, 0); err == nil {
		return nil
	}
	f, err := os.Open("/proc/" + strconv.Itoa(pid) + "/mem")
	if err != nil {
		return fmt.Errorf("read memory: %w", err)
	}
	defer f.Close()
	if _, err := f.ReadAt(buf, int64(addr)); err != nil {
		return fmt.Errorf("read memory: %w", err)
	}
	return nil
}

func readableAddr(pid int) (uintptr, error) {
	f, err := os.Open("/proc/" + strconv.Itoa(pid) + "/maps")
	if err != nil {
		return 0, fmt.Errorf("maps: %w", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 || !strings.HasPrefix(fields[1], "r") {
			continue
		}
		start, _, _ := strings.Cut(fields[0], "-")
		a, err := strconv.ParseUint(start, 16, 64)
		if err == nil {
			return uintptr(a), nil
		}
	}
	return 0, fmt.Errorf("no readable mapping")
}
