//go:build linux

package manager

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
)

const (
	supervisorTick  = 100 // ms between polls of the listener
	refreshInterval = 500 * time.Millisecond
	idleWindow      = 2 * time.Second
	// idleCPUShare is the CPU a kernel may use while no exec runs. More means
	// code kept running after its exec reported done (or never stopped): the
	// kernel is killed.
	idleCPUShare = 0.10
	clockTicks   = 100 // USER_HZ
	maxIovecs    = 1024
)

// The calls the sandbox's notification filter hands to the supervisor, by
// architecture (see isolation/sandbox.py _QUOTA_CALLS).
type quotaCalls struct {
	write, pwrite64, writev, pwritev, pwritev2 int32
	ftruncate, copyFileRange, sendfile, splice int32
	creates                                    map[int32]bool
}

var callsByArch = map[string]quotaCalls{
	"amd64": {
		write: 1, pwrite64: 18, writev: 20, pwritev: 296, pwritev2: 328,
		ftruncate: 77, copyFileRange: 326, sendfile: 40, splice: 275,
		creates: map[int32]bool{2: true, 257: true, 85: true, 437: true, 83: true, 258: true, 133: true, 259: true, 86: true, 265: true, 88: true, 266: true},
	},
	"arm64": {
		write: 64, pwrite64: 68, writev: 66, pwritev: 70, pwritev2: 287,
		ftruncate: 46, copyFileRange: 285, sendfile: 71, splice: 76,
		creates: map[int32]bool{56: true, 437: true, 34: true, 33: true, 37: true, 36: true},
	},
}

// recvListener reads the kernel's first byte from the bridge: "N" carries the
// seccomp notification listener (SCM_RIGHTS), "X" means there is none.
func recvListener(conn net.Conn, timeout time.Duration) (int, error) {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return -1, errors.New("control socket is not a unix socket")
	}
	_ = uc.SetReadDeadline(time.Now().Add(timeout))
	defer func() { _ = uc.SetReadDeadline(time.Time{}) }()
	buf := make([]byte, 1)
	oob := make([]byte, unix.CmsgSpace(4*4))
	n, oobn, _, _, err := uc.ReadMsgUnix(buf, oob)
	if err != nil {
		return -1, fmt.Errorf("read the supervisor handshake: %w", err)
	}
	var fds []int
	if oobn > 0 {
		msgs, perr := unix.ParseSocketControlMessage(oob[:oobn])
		if perr == nil {
			for _, m := range msgs {
				if got, err := unix.ParseUnixRights(&m); err == nil {
					fds = append(fds, got...)
				}
			}
		}
	}
	if n != 1 || buf[0] != 'N' || len(fds) != 1 {
		for _, fd := range fds {
			_ = unix.Close(fd)
		}
		return -1, nil
	}
	unix.CloseOnExec(fds[0])
	return fds[0], nil
}

// supervisor approves, for one kernel, every file write, extension and entry
// creation against the workdir quota before it happens, and kills a kernel
// that burns CPU while no exec runs. It reads /proc of the kernel's tasks,
// so it runs on a locked OS thread whose filesystem uid is the kernel's.
type supervisor struct {
	k        *kernel
	fd       int
	calls    quotaCalls
	pgid     int
	workdir  string
	quota    int64
	entries  int64
	cred     *isolation.Credential
	measured usage
	cur      usage // approved since the last refresh started
	prev     usage // approved during the refresh before
	lastRef  time.Time

	cpuStart   uint64
	cpuAt      time.Time
	execsAt    int
	notifSize  int
	respSize   int
	lastIdleCk time.Time
}

func startSupervisor(k *kernel, fd int, cred *isolation.Credential) error {
	calls, ok := callsByArch[runtime.GOARCH]
	if !ok {
		return fmt.Errorf("no quota supervisor for %s", runtime.GOARCH)
	}
	var sizes [3]uint16 // seccomp_notif, seccomp_notif_resp, seccomp_data
	if _, _, e := unix.Syscall(unix.SYS_SECCOMP, 3, 0, uintptr(unsafe.Pointer(&sizes))); e != 0 {
		return fmt.Errorf("seccomp notif sizes: %w", e)
	}
	s := &supervisor{
		k: k, fd: fd, calls: calls, pgid: k.pid, workdir: k.opts.workdir,
		quota: k.opts.workdirQuota, entries: k.opts.workdirEntries, cred: cred,
		notifSize: int(sizes[0]), respSize: int(sizes[1]),
	}
	if s.notifSize < 80 || s.respSize < 24 {
		return fmt.Errorf("unexpected seccomp notification layout %v", sizes)
	}
	ready := make(chan error, 1)
	go s.run(ready)
	return <-ready
}

func (s *supervisor) run(ready chan<- error) {
	// Never unlocked: the thread with a changed fsuid exits with the goroutine.
	runtime.LockOSThread()
	defer unix.Close(s.fd)
	if s.cred != nil {
		if err := unix.Setfsgid(int(s.cred.GID)); err != nil {
			ready <- err
			return
		}
		if err := unix.Setfsuid(int(s.cred.UID)); err != nil {
			ready <- err
			return
		}
	}
	if err := s.refresh(); err != nil {
		ready <- fmt.Errorf("inspect the kernel: %w", err)
		return
	}
	ready <- nil
	notif := make([]byte, s.notifSize)
	resp := make([]byte, s.respSize)
	for {
		select {
		case <-s.k.closed:
			return
		default:
		}
		pfd := []unix.PollFd{{Fd: int32(s.fd), Events: unix.POLLIN}}
		n, err := unix.Poll(pfd, supervisorTick)
		if err != nil && !errors.Is(err, unix.EINTR) {
			return
		}
		if n > 0 {
			if pfd[0].Revents&unix.POLLIN != 0 {
				s.handle(notif, resp)
			} else if pfd[0].Revents&(unix.POLLHUP|unix.POLLERR) != 0 {
				return // every task of the filter is gone
			}
		}
		if time.Since(s.lastRef) >= refreshInterval {
			_ = s.refresh()
		}
		if time.Since(s.lastIdleCk) >= time.Second {
			s.checkIdle()
		}
	}
}

// refresh measures the workdir (visible files and unlinked files the kernel
// still holds open) and retires approvals older than the previous refresh.
func (s *supervisor) refresh() error {
	s.prev, s.cur = s.cur, usage{}
	visible := dirUsage(s.workdir, s.entries)
	hidden, err := s.heldUnlinked()
	if err != nil {
		return err
	}
	s.measured = usage{bytes: visible.bytes + hidden, entries: visible.entries}
	s.lastRef = time.Now()
	return nil
}

func (s *supervisor) total() usage {
	return usage{
		bytes:   s.measured.bytes + s.prev.bytes + s.cur.bytes,
		entries: s.measured.entries + s.prev.entries + s.cur.entries,
	}
}

// groupPIDs lists the processes of the kernel's process group.
func (s *supervisor) groupPIDs() ([]int, uint64) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, 0
	}
	var pids []int
	var cpu uint64
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		raw, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		i := bytes.LastIndexByte(raw, ')')
		if i < 0 {
			continue
		}
		f := strings.Fields(string(raw[i+1:]))
		if len(f) < 13 {
			continue
		}
		if pgrp, _ := strconv.Atoi(f[2]); pgrp != s.pgid {
			continue
		}
		pids = append(pids, pid)
		utime, _ := strconv.ParseUint(f[11], 10, 64)
		stime, _ := strconv.ParseUint(f[12], 10, 64)
		cpu += utime + stime
	}
	return pids, cpu
}

// heldUnlinked sums regular files that the kernel's processes hold open after
// they were unlinked: they still occupy the disk but no walk can see them.
func (s *supervisor) heldUnlinked() (int64, error) {
	pids, _ := s.groupPIDs()
	type key struct{ dev, ino uint64 }
	seen := map[key]bool{}
	var total int64
	for _, pid := range pids {
		dir := "/proc/" + strconv.Itoa(pid) + "/fd"
		ents, err := os.ReadDir(dir)
		if err != nil {
			if errors.Is(err, os.ErrPermission) {
				return 0, err
			}
			continue // exited meanwhile
		}
		for _, e := range ents {
			p := filepath.Join(dir, e.Name())
			target, err := os.Readlink(p)
			if err != nil || !strings.HasSuffix(target, " (deleted)") {
				continue
			}
			var st unix.Stat_t
			if unix.Stat(p, &st) != nil || st.Mode&unix.S_IFMT != unix.S_IFREG {
				continue
			}
			k := key{uint64(st.Dev), st.Ino} //nolint:unconvert // Dev width differs by arch
			if !seen[k] {
				seen[k] = true
				total += st.Size
			}
		}
	}
	return total, nil
}

// checkIdle kills the kernel when it used more than idleCPUShare of a window
// in which no exec ran: work that continues after a worker-declared done.
func (s *supervisor) checkIdle() {
	s.lastIdleCk = time.Now()
	idle, execs, lastActive := s.k.activity()
	_, cpu := s.groupPIDs()
	now := time.Now()
	if !idle || execs != s.execsAt || s.cpuAt.IsZero() || lastActive.After(s.cpuAt.Add(-time.Second)) {
		s.cpuStart, s.cpuAt, s.execsAt = cpu, now, execs
		return
	}
	elapsed := now.Sub(s.cpuAt)
	if elapsed < idleWindow {
		return
	}
	used := time.Duration(cpu-s.cpuStart) * time.Second / clockTicks
	if cpu >= s.cpuStart && float64(used) > idleCPUShare*float64(elapsed) {
		s.k.killIdle()
	}
	s.cpuStart, s.cpuAt = cpu, now
}

// handle receives one notification and answers it.
func (s *supervisor) handle(notif, resp []byte) {
	for i := range notif {
		notif[i] = 0
	}
	if err := ioctl(s.fd, unix.SECCOMP_IOCTL_NOTIF_RECV, notif); err != nil {
		return // the task died before we read it
	}
	id := binary.NativeEndian.Uint64(notif[0:])
	tid := int(binary.NativeEndian.Uint32(notif[8:]))
	nr := int32(binary.NativeEndian.Uint32(notif[16:]))
	var args [6]uint64
	for i := range args {
		args[i] = binary.NativeEndian.Uint64(notif[32+8*i:])
	}
	errno := s.decide(tid, nr, args)
	// The task must still be waiting on this exact notification, so what we
	// read from /proc belonged to it.
	if errno == 0 && !s.idValid(id) {
		return
	}
	for i := range resp {
		resp[i] = 0
	}
	binary.NativeEndian.PutUint64(resp[0:], id)
	if errno == 0 {
		binary.NativeEndian.PutUint32(resp[20:], unix.SECCOMP_USER_NOTIF_FLAG_CONTINUE)
	} else {
		binary.NativeEndian.PutUint32(resp[16:], uint32(-int32(errno)))
	}
	_ = ioctl(s.fd, unix.SECCOMP_IOCTL_NOTIF_SEND, resp)
}

func (s *supervisor) idValid(id uint64) bool {
	v := id
	_, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(s.fd), unix.SECCOMP_IOCTL_NOTIF_ID_VALID, uintptr(unsafe.Pointer(&v)))
	return e == 0
}

// decide returns 0 to let the call run, or the errno to fail it with.
func (s *supervisor) decide(tid int, nr int32, args [6]uint64) syscall.Errno {
	c := s.calls
	if c.creates[nr] {
		t := s.total()
		if s.entries > 0 && t.entries+1 > s.entries {
			s.k.noteQuota()
			return unix.EDQUOT
		}
		s.cur.entries++
		return 0
	}
	var fd int
	switch nr {
	case c.copyFileRange, c.splice:
		fd = int(int32(args[2]))
	default:
		fd = int(int32(args[0]))
	}
	f, err := s.target(tid, fd)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0 // not an open fd (EBADF follows) or the task is gone
		}
		return unix.EPERM // cannot inspect: fail closed
	}
	if !f.regular {
		return 0 // pipes, sockets, /dev/null
	}
	end, err := s.end(tid, nr, args, f)
	if err != nil {
		return unix.EPERM
	}
	growth := end - f.size
	if growth <= 0 {
		return 0
	}
	if s.total().bytes+growth > s.quota {
		s.k.noteQuota()
		return unix.EDQUOT
	}
	s.cur.bytes += growth
	return 0
}

type fdTarget struct {
	regular bool
	size    int64
	pos     int64
	append  bool
}

func (s *supervisor) target(tid, fd int) (fdTarget, error) {
	base := "/proc/" + strconv.Itoa(tid)
	var st unix.Stat_t
	if err := unix.Stat(base+"/fd/"+strconv.Itoa(fd), &st); err != nil {
		if errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ESRCH) {
			return fdTarget{}, os.ErrNotExist
		}
		return fdTarget{}, err
	}
	t := fdTarget{regular: st.Mode&unix.S_IFMT == unix.S_IFREG, size: st.Size}
	if !t.regular {
		return t, nil
	}
	raw, err := os.ReadFile(base + "/fdinfo/" + strconv.Itoa(fd))
	if err != nil {
		return fdTarget{}, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch k {
		case "pos":
			t.pos, _ = strconv.ParseInt(v, 10, 64)
		case "flags":
			flags, _ := strconv.ParseUint(v, 8, 64)
			t.append = flags&unix.O_APPEND != 0
		}
	}
	return t, nil
}

// end computes the file size the call can produce at most.
func (s *supervisor) end(tid int, nr int32, args [6]uint64, f fdTarget) (int64, error) {
	c := s.calls
	at := f.pos
	if f.append {
		at = f.size
	}
	switch nr {
	case c.write:
		return at + int64(args[2]), nil
	case c.pwrite64:
		return int64(args[3]) + int64(args[2]), nil
	case c.writev, c.pwritev, c.pwritev2:
		n, err := s.iovLen(tid, uintptr(args[1]), int(args[2]))
		if err != nil {
			return 0, err
		}
		switch {
		case nr == c.writev:
		case nr == c.pwritev2 && (int64(args[3]) == -1 || args[5]&0x10 != 0): // current pos / RWF_APPEND
			if args[5]&0x10 != 0 {
				at = f.size
			}
		default:
			at = int64(args[3])
		}
		return at + n, nil
	case c.ftruncate:
		return int64(args[1]), nil
	case c.copyFileRange, c.splice:
		if args[3] != 0 {
			off, err := s.readU64(tid, uintptr(args[3]))
			if err != nil {
				return 0, err
			}
			at = int64(off)
		}
		return at + int64(args[4]), nil
	case c.sendfile:
		return at + int64(args[3]), nil
	}
	return 0, fmt.Errorf("unexpected syscall %d", nr)
}

func (s *supervisor) iovLen(tid int, addr uintptr, cnt int) (int64, error) {
	if cnt <= 0 {
		return 0, nil
	}
	if cnt > maxIovecs {
		cnt = maxIovecs
	}
	buf := make([]byte, 16*cnt)
	if err := s.readMem(tid, addr, buf); err != nil {
		return 0, err
	}
	var total int64
	for i := 0; i < cnt; i++ {
		total += int64(binary.NativeEndian.Uint64(buf[16*i+8:]))
	}
	return total, nil
}

func (s *supervisor) readU64(tid int, addr uintptr) (uint64, error) {
	buf := make([]byte, 8)
	if err := s.readMem(tid, addr, buf); err != nil {
		return 0, err
	}
	return binary.NativeEndian.Uint64(buf), nil
}

func (s *supervisor) readMem(tid int, addr uintptr, buf []byte) error {
	f, err := os.Open("/proc/" + strconv.Itoa(tid) + "/mem")
	if err != nil {
		return err
	}
	defer f.Close()
	n, err := f.ReadAt(buf, int64(addr))
	if n != len(buf) {
		if err == nil {
			err = errors.New("short read")
		}
		return err
	}
	return nil
}

func ioctl(fd int, req uint, buf []byte) error {
	_, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&buf[0])))
	if e != 0 {
		return e
	}
	return nil
}
