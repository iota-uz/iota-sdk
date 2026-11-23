//go:build linux

package manager

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
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
	supervisorTick   = 100 // ms between polls of the listener
	reconcileEvery   = 2 * time.Second
	reconcileMinGap  = 250 * time.Millisecond
	idleCheckEvery   = time.Second
	idleCPUBudget    = time.Second // net CPU a kernel may accumulate while no exec runs
	idleCPUAllowance = 0.02        // share of an idle interval that is forgiven
	clockTicks       = 100         // USER_HZ
	maxIovecs        = 1024
	maxFileChunk     = 1 << 20 // bytes the host writes per emulated call to a file
	maxStreamChunk   = 64 << 10
	pidfdThread      = 0x80 // PIDFD_THREAD (O_EXCL)
	rwfAppend        = 0x10
	notifRespError   = 16
	notifRespFlags   = 20
	notifRespVal     = 8
	notifArgsOffset  = 32
	notifTIDOffset   = 8
	notifNROffset    = 16
	notifMinSize     = 80
	notifRespMinSize = 24
)

// The calls the sandbox's notification filter hands to the supervisor (see
// isolation/sandbox.py _QUOTA_CALLS).
type quotaCalls struct {
	write, pwrite64, writev, pwritev, pwritev2, ftruncate int32
	creates                                               map[int32]bool
}

var callsByArch = map[string]quotaCalls{
	"amd64": {
		write: 1, pwrite64: 18, writev: 20, pwritev: 296, pwritev2: 328, ftruncate: 77,
		creates: map[int32]bool{2: true, 257: true, 85: true, 437: true, 83: true, 258: true, 133: true, 259: true, 86: true, 265: true, 88: true, 266: true},
	},
	"arm64": {
		write: 64, pwrite64: 68, writev: 66, pwritev: 70, pwritev2: 287, ftruncate: 46,
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
		if msgs, perr := unix.ParseSocketControlMessage(oob[:oobn]); perr == nil {
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

type inode struct{ dev, ino uint64 }

// supervisor enforces the workdir quota for one kernel at allocation time and
// guards its idle CPU.
//
// Every write-family call of the kernel is performed by the supervisor
// itself, on a duplicate of the caller's open file description (pidfd_getfd),
// with data it copied out of the caller first. What is checked is therefore
// exactly what is written: no other thread can change the buffer, the length
// or the file between the check and the write, and fd numbers mean nothing.
// File sizes it produced are kept in a per-inode ledger that only shrinks
// when a reconcile sees the inode gone from the workdir and from every fd and
// mapping of the kernel, so data in unlinked-but-open or mapped files stays
// charged. Entry creation is counted before it is let through. A reconcile
// that finds usage over the quota anyway kills the kernel (backstop).
//
// It reads /proc and memory of the kernel's tasks, so it runs on a locked OS
// thread whose credentials are the kernel's (a root host switches only that
// thread). If it stops for any reason other than disposal, the kernel is
// killed: no kernel ever runs without its supervisor.
type supervisor struct {
	k        *kernel
	fd       int
	calls    quotaCalls
	pgid     int
	workdir  string
	quota    int64
	entries  int64
	fileSize int64
	cred     *isolation.Credential

	ledger     map[inode]int64 // sizes of files the kernel grew
	baseline   int64           // visible files not in the ledger
	walked     int64           // entries seen by the last reconcile
	entryCur   int64           // creations approved since the last reconcile started
	entryPrev  int64           // creations approved during the one before
	lastRecon  time.Time
	notifSize  int
	respSize   int
	idleCPU    time.Duration
	lastCPU    uint64
	cpuSeen    bool
	idleSeen   bool
	execsSeen  int
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
		quota: k.opts.workdirQuota, entries: k.opts.workdirEntries, fileSize: k.opts.fileSize, cred: cred,
		ledger: map[inode]int64{}, notifSize: int(sizes[0]), respSize: int(sizes[1]),
	}
	if s.notifSize < notifMinSize || s.respSize < notifRespMinSize {
		return fmt.Errorf("unexpected seccomp notification layout %v", sizes)
	}
	ready := make(chan error, 1)
	go s.run(ready)
	return <-ready
}

// becomeKernel gives the locked thread the kernel's credentials, so /proc,
// memory reads and pidfd_getfd pass the ptrace checks without capabilities.
// Raw syscalls change only this thread; it exits with the goroutine.
func (s *supervisor) becomeKernel() error {
	if s.cred == nil {
		return nil
	}
	if _, _, e := unix.RawSyscall(unix.SYS_SETRESGID, uintptr(s.cred.GID), uintptr(s.cred.GID), ^uintptr(0)); e != 0 {
		return fmt.Errorf("setresgid: %w", e)
	}
	if _, _, e := unix.RawSyscall(unix.SYS_SETRESUID, uintptr(s.cred.UID), uintptr(s.cred.UID), ^uintptr(0)); e != 0 {
		return fmt.Errorf("setresuid: %w", e)
	}
	return nil
}

func (s *supervisor) run(ready chan<- error) {
	runtime.LockOSThread() // never unlocked: the switched thread dies with the goroutine
	defer unix.Close(s.fd)
	disposed := false
	defer func() {
		if !disposed {
			_ = s.k.proc.Kill()
		}
	}()
	if err := s.becomeKernel(); err != nil {
		ready <- err
		return
	}
	if err := s.reconcile(); err != nil {
		ready <- fmt.Errorf("inspect the kernel: %w", err)
		return
	}
	ready <- nil
	notif := make([]byte, s.notifSize)
	for {
		select {
		case <-s.k.closed:
			disposed = true
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
				s.handle(notif)
			} else if pfd[0].Revents&(unix.POLLHUP|unix.POLLERR) != 0 {
				disposed = true // every task of the filter is gone
				return
			}
		}
		if time.Since(s.lastRecon) >= reconcileEvery {
			if err := s.reconcile(); err == nil && s.over() {
				s.k.noteQuota()
				return // backstop: the deferred kill stops the kernel
			}
		}
		if time.Since(s.lastIdleCk) >= idleCheckEvery {
			s.checkIdle()
		}
	}
}

func (s *supervisor) used() int64 {
	total := s.baseline
	for _, size := range s.ledger {
		total += size
	}
	return total
}

func (s *supervisor) usedEntries() int64 { return s.walked + s.entryPrev + s.entryCur }

func (s *supervisor) over() bool {
	return s.used() > s.quota || (s.entries > 0 && s.walked > s.entries)
}

// reconcile measures the workdir and drops ledger entries for inodes that no
// longer exist anywhere the kernel can reach. On any error nothing is credited.
func (s *supervisor) reconcile() error {
	s.lastRecon = time.Now()
	startCur := s.entryCur
	visible := map[inode]int64{}
	var walked int64
	_ = filepath.WalkDir(s.workdir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // a vanished entry is not an error for a measurement
		}
		walked++
		if s.entries > 0 && walked > s.entries {
			return filepath.SkipAll
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				if st, ok := info.Sys().(*syscall.Stat_t); ok {
					visible[inode{uint64(st.Dev), st.Ino}] = info.Size() //nolint:unconvert // Dev width differs by arch
				}
			}
		}
		return nil
	})
	held, err := s.heldInodes()
	if err != nil {
		return err
	}
	for ino := range s.ledger {
		if size, ok := visible[ino]; ok {
			s.ledger[ino] = size
		} else if !held[ino] {
			delete(s.ledger, ino)
		}
	}
	var baseline int64
	for ino, size := range visible {
		if _, ok := s.ledger[ino]; !ok {
			baseline += size
		}
	}
	s.baseline = baseline
	s.walked = walked
	// Creations approved before this walk started are in it now.
	s.entryPrev, s.entryCur = s.entryCur-startCur, 0
	if s.entryPrev < 0 {
		s.entryPrev = 0
	}
	return nil
}

// groupStats lists the processes of the kernel's process group and their
// total CPU, children included.
func (s *supervisor) groupStats() ([]int, uint64) {
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
		if len(f) < 15 {
			continue
		}
		if pgrp, _ := strconv.Atoi(f[2]); pgrp != s.pgid {
			continue
		}
		pids = append(pids, pid)
		for _, idx := range []int{11, 12, 13, 14} { // utime stime cutime cstime
			v, _ := strconv.ParseUint(f[idx], 10, 64)
			cpu += v
		}
	}
	return pids, cpu
}

// heldInodes returns every inode an fd or a mapping of the kernel references.
func (s *supervisor) heldInodes() (map[inode]bool, error) {
	pids, _ := s.groupStats()
	held := map[inode]bool{}
	for _, pid := range pids {
		base := "/proc/" + strconv.Itoa(pid)
		ents, err := os.ReadDir(base + "/fd")
		if err != nil {
			if errors.Is(err, os.ErrPermission) {
				return nil, err
			}
			continue // exited meanwhile
		}
		for _, e := range ents {
			var st unix.Stat_t
			if unix.Stat(base+"/fd/"+e.Name(), &st) == nil && st.Mode&unix.S_IFMT == unix.S_IFREG {
				held[inode{uint64(st.Dev), st.Ino}] = true //nolint:unconvert // Dev width differs by arch
			}
		}
		f, err := os.Open(base + "/maps")
		if err != nil {
			if errors.Is(err, os.ErrPermission) {
				return nil, err
			}
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			// addr perms offset dev(major:minor hex) inode path
			fields := strings.Fields(sc.Text())
			if len(fields) < 5 || fields[4] == "0" {
				continue
			}
			maj, min, ok := strings.Cut(fields[3], ":")
			if !ok {
				continue
			}
			ma, _ := strconv.ParseUint(maj, 16, 32)
			mi, _ := strconv.ParseUint(min, 16, 32)
			ino, _ := strconv.ParseUint(fields[4], 10, 64)
			held[inode{unix.Mkdev(uint32(ma), uint32(mi)), ino}] = true
		}
		_ = f.Close()
	}
	return held, nil
}

// checkIdle charges the CPU the kernel uses during intervals in which no
// exec ran at all — work that continues after a worker-declared done, in
// threads or children (children's CPU is included once they are reaped, and
// the kernel is their subreaper). The charge is kept for the kernel's life,
// minus a small allowance per idle interval, so frequent execs do not reset
// it; past idleCPUBudget the kernel is killed.
func (s *supervisor) checkIdle() {
	now := time.Now()
	elapsed := now.Sub(s.lastIdleCk)
	s.lastIdleCk = now
	idle, execs, _ := s.k.activity()
	_, cpu := s.groupStats()
	delta := uint64(0)
	if s.cpuSeen && cpu > s.lastCPU {
		delta = cpu - s.lastCPU
	}
	wasIdle := s.cpuSeen && s.idleSeen && idle && execs == s.execsSeen
	s.lastCPU, s.cpuSeen, s.idleSeen, s.execsSeen = cpu, true, idle, execs
	if !wasIdle {
		return
	}
	s.idleCPU += time.Duration(delta)*time.Second/clockTicks - time.Duration(idleCPUAllowance*float64(elapsed))
	if s.idleCPU < 0 {
		s.idleCPU = 0
	}
	if s.idleCPU > idleCPUBudget {
		s.k.killIdle()
	}
}

type notification struct {
	id   uint64
	tid  int
	nr   int32
	args [6]uint64
}

func (s *supervisor) handle(buf []byte) {
	for i := range buf {
		buf[i] = 0
	}
	if err := ioctl(s.fd, unix.SECCOMP_IOCTL_NOTIF_RECV, buf); err != nil {
		return // the task died before we read it
	}
	n := notification{
		id:  binary.NativeEndian.Uint64(buf[0:]),
		tid: int(binary.NativeEndian.Uint32(buf[notifTIDOffset:])),
		nr:  int32(binary.NativeEndian.Uint32(buf[notifNROffset:])),
	}
	for i := range n.args {
		n.args[i] = binary.NativeEndian.Uint64(buf[notifArgsOffset+8*i:])
	}
	if s.calls.creates[n.nr] {
		s.respond(n.id, 0, s.approveCreate(), true)
		return
	}
	s.emulate(n)
}

// approveCreate counts one entry creation; the call itself then runs (its
// arguments do not matter to the count).
func (s *supervisor) approveCreate() syscall.Errno {
	if s.entries <= 0 {
		return 0
	}
	if s.usedEntries()+1 > s.entries && time.Since(s.lastRecon) >= reconcileMinGap {
		_ = s.reconcile()
	}
	if s.usedEntries()+1 > s.entries {
		s.k.noteQuota()
		return unix.EDQUOT
	}
	s.entryCur++
	return 0
}

// respond answers notification id: errno != 0 fails the call; otherwise cont
// lets it run, or val is its result (emulated).
func (s *supervisor) respond(id uint64, val int64, errno syscall.Errno, cont bool) {
	resp := make([]byte, s.respSize)
	binary.NativeEndian.PutUint64(resp[0:], id)
	switch {
	case errno != 0:
		binary.NativeEndian.PutUint32(resp[notifRespError:], uint32(-int32(errno)))
	case cont:
		binary.NativeEndian.PutUint32(resp[notifRespFlags:], unix.SECCOMP_USER_NOTIF_FLAG_CONTINUE)
	default:
		binary.NativeEndian.PutUint64(resp[notifRespVal:], uint64(val))
	}
	_ = ioctl(s.fd, unix.SECCOMP_IOCTL_NOTIF_SEND, resp)
}

// dupTarget duplicates the caller's fd into the host.
func dupTarget(tid, fd int) (int, error) {
	pidfd, err := unix.PidfdOpen(tid, pidfdThread)
	if err != nil {
		// Before PIDFD_THREAD (Linux 6.9) only a group leader has a pidfd;
		// threads share its fd table.
		tgid, terr := threadGroup(tid)
		if terr != nil {
			return -1, err
		}
		if pidfd, err = unix.PidfdOpen(tgid, 0); err != nil {
			return -1, err
		}
	}
	defer unix.Close(pidfd)
	return unix.PidfdGetfd(pidfd, fd, 0)
}

func threadGroup(tid int) (int, error) {
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(tid) + "/status")
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if v, ok := strings.CutPrefix(line, "Tgid:"); ok {
			return strconv.Atoi(strings.TrimSpace(v))
		}
	}
	return 0, errors.New("no Tgid")
}

// emulate performs a write-family call for the kernel.
func (s *supervisor) emulate(n notification) {
	c := s.calls
	dup, err := dupTarget(n.tid, int(int32(n.args[0])))
	if err != nil {
		switch {
		case errors.Is(err, unix.EBADF):
			s.respond(n.id, 0, unix.EBADF, false)
		case errors.Is(err, unix.ESRCH):
			// the task is gone; nothing to answer
		default:
			s.respond(n.id, 0, unix.EPERM, false) // cannot act for it: fail closed
		}
		return
	}
	var st unix.Stat_t
	if err := unix.Fstat(dup, &st); err != nil {
		_ = unix.Close(dup)
		s.respond(n.id, 0, unix.EBADF, false)
		return
	}
	if n.nr == c.ftruncate {
		s.truncate(n, dup, st)
		return
	}
	regular := st.Mode&unix.S_IFMT == unix.S_IFREG
	limit := maxStreamChunk
	if regular {
		limit = maxFileChunk
	}
	data, err := s.payload(n, limit)
	if err != nil {
		_ = unix.Close(dup)
		s.respond(n.id, 0, errnoOf(err, unix.EFAULT), false)
		return
	}
	if !regular {
		// Pipes, sockets, /dev/null: no quota, but the write may block, so
		// it runs off the supervisor thread (on the duplicate, which is the
		// object that was checked).
		go func() {
			defer unix.Close(dup)
			s.finishStream(n, dup, data)
		}()
		return
	}
	defer unix.Close(dup)
	s.writeFile(n, dup, st, data)
}

// payload copies the bytes the call wants to write out of the caller, at
// most limit; a shorter write is a legal result the caller handles.
func (s *supervisor) payload(n notification, limit int) ([]byte, error) {
	c := s.calls
	switch n.nr {
	case c.write, c.pwrite64:
		size := int(min(n.args[2], uint64(limit)))
		buf := make([]byte, size)
		if size == 0 {
			return buf, nil
		}
		return buf, s.readMem(n.tid, uintptr(n.args[1]), buf)
	case c.writev, c.pwritev, c.pwritev2:
		cnt := int(min(n.args[2], maxIovecs))
		if cnt <= 0 {
			return nil, nil
		}
		iov := make([]byte, 16*cnt)
		if err := s.readMem(n.tid, uintptr(n.args[1]), iov); err != nil {
			return nil, err
		}
		var out []byte
		for i := 0; i < cnt && len(out) < limit; i++ {
			base := binary.NativeEndian.Uint64(iov[16*i:])
			l := binary.NativeEndian.Uint64(iov[16*i+8:])
			l = min(l, uint64(limit-len(out)))
			if l == 0 {
				continue
			}
			chunk := make([]byte, l)
			if err := s.readMem(n.tid, uintptr(base), chunk); err != nil {
				return nil, err
			}
			out = append(out, chunk...)
		}
		return out, nil
	}
	return nil, unix.ENOSYS
}

func (s *supervisor) finishStream(n notification, dup int, data []byte) {
	c := s.calls
	var written int
	var err error
	switch n.nr {
	case c.pwrite64, c.pwritev, c.pwritev2:
		if off := int64(n.args[3]); n.nr != c.pwrite64 && off == -1 {
			written, err = unix.Write(dup, data)
		} else {
			written, err = unix.Pwrite(dup, data, off) // ESPIPE for pipes, as the kernel would say
		}
	default:
		written, err = unix.Write(dup, data)
	}
	if err != nil {
		s.respond(n.id, 0, errnoOf(err, unix.EIO), false)
		return
	}
	s.respond(n.id, int64(written), 0, false)
}

// writeFile writes data to the regular file behind dup, within the quota and
// the file size limit, and charges the growth.
func (s *supervisor) writeFile(n notification, dup int, st unix.Stat_t, data []byte) {
	c := s.calls
	flags, err := unix.FcntlInt(uintptr(dup), unix.F_GETFL, 0)
	if err != nil {
		s.respond(n.id, 0, unix.EBADF, false)
		return
	}
	appendMode := flags&unix.O_APPEND != 0
	positional := false
	var at int64
	switch n.nr {
	case c.pwrite64, c.pwritev:
		at, positional = int64(n.args[3]), true
	case c.pwritev2:
		if int64(n.args[3]) != -1 {
			at, positional = int64(n.args[3]), true
		}
		appendMode = appendMode || n.args[5]&rwfAppend != 0
	}
	if positional && at < 0 {
		s.respond(n.id, 0, unix.EINVAL, false)
		return
	}
	if !positional {
		if at, err = unix.Seek(dup, 0, 1); err != nil {
			s.respond(n.id, 0, errnoOf(err, unix.ESPIPE), false)
			return
		}
	}
	if appendMode {
		at = st.Size // pwrite on an O_APPEND file appends as well
	}
	want := int64(len(data))
	if s.fileSize > 0 && at+want > s.fileSize {
		want = s.fileSize - at
		if want <= 0 {
			s.respond(n.id, 0, unix.EFBIG, false)
			return
		}
	}
	if allowed := s.allow(at, want, st.Size); allowed < want {
		if allowed <= 0 {
			s.k.noteQuota()
			s.respond(n.id, 0, unix.EDQUOT, false)
			return
		}
		want = allowed
	}
	var written int
	switch {
	case appendMode, !positional:
		// The shared description carries the position and O_APPEND.
		written, err = unix.Write(dup, data[:want])
	default:
		written, err = unix.Pwrite(dup, data[:want], at)
	}
	s.charge(dup)
	if err != nil {
		s.respond(n.id, 0, errnoOf(err, unix.EIO), false)
		return
	}
	s.respond(n.id, int64(written), 0, false)
}

// allow returns how many of want bytes written at offset at of a file of the
// given size fit in the quota (reconciling once when they seem not to).
func (s *supervisor) allow(at, want, size int64) int64 {
	fits := func() int64 {
		growth := at + want - size
		if growth <= 0 {
			return want
		}
		room := s.quota - s.used()
		if growth <= room {
			return want
		}
		return want - (growth - room)
	}
	got := fits()
	if got < want && time.Since(s.lastRecon) >= reconcileMinGap {
		_ = s.reconcile()
		got = fits()
	}
	return got
}

func (s *supervisor) truncate(n notification, dup int, st unix.Stat_t) {
	defer unix.Close(dup)
	length := int64(n.args[1])
	switch {
	case length < 0:
		s.respond(n.id, 0, unix.EINVAL, false)
		return
	case st.Mode&unix.S_IFMT != unix.S_IFREG:
		s.respond(n.id, 0, unix.EINVAL, false)
		return
	case s.fileSize > 0 && length > s.fileSize:
		s.respond(n.id, 0, unix.EFBIG, false)
		return
	}
	if length > st.Size && s.allow(st.Size, length-st.Size, st.Size) < length-st.Size {
		s.k.noteQuota()
		s.respond(n.id, 0, unix.EDQUOT, false)
		return
	}
	err := unix.Ftruncate(dup, length)
	s.charge(dup)
	if err != nil {
		s.respond(n.id, 0, errnoOf(err, unix.EIO), false)
		return
	}
	s.respond(n.id, 0, 0, false)
}

// charge records the current size of the file behind dup in the ledger.
func (s *supervisor) charge(dup int) {
	var st unix.Stat_t
	if unix.Fstat(dup, &st) == nil {
		s.ledger[inode{uint64(st.Dev), st.Ino}] = st.Size //nolint:unconvert // Dev width differs by arch
	}
}

func (s *supervisor) readMem(tid int, addr uintptr, buf []byte) error {
	if len(buf) == 0 {
		return nil
	}
	local := []unix.Iovec{{Base: &buf[0]}}
	local[0].SetLen(len(buf))
	remote := []unix.RemoteIovec{{Base: addr, Len: len(buf)}}
	n, err := unix.ProcessVMReadv(tid, local, remote, 0)
	if errors.Is(err, unix.EPERM) || errors.Is(err, unix.ENOSYS) {
		// Container seccomp profiles often refuse process_vm_readv; the same
		// bytes are readable through /proc with the kernel's credentials.
		return readProcMem(tid, addr, buf)
	}
	if err != nil {
		return err
	}
	if n != len(buf) {
		return unix.EFAULT
	}
	return nil
}

func readProcMem(tid int, addr uintptr, buf []byte) error {
	f, err := os.Open("/proc/" + strconv.Itoa(tid) + "/mem")
	if err != nil {
		return err
	}
	defer f.Close()
	n, err := f.ReadAt(buf, int64(addr))
	if n != len(buf) {
		if err == nil {
			err = unix.EFAULT
		}
		return err
	}
	return nil
}

func errnoOf(err error, fallback syscall.Errno) syscall.Errno {
	var e syscall.Errno
	if errors.As(err, &e) {
		return e
	}
	return fallback
}

func ioctl(fd int, req uint, buf []byte) error {
	_, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&buf[0])))
	if e != 0 {
		return e
	}
	return nil
}
