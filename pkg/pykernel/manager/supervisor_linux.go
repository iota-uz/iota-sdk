//go:build linux

package manager

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
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
	maxFileChunk     = 1 << 20  // bytes copied per step of an emulated file write
	maxStreamPayload = 16 << 20 // bytes one emulated stream write moves at most
	quotaBlock       = 4096     // allocation unit: sizes round up, every entry costs one
	// entryCost is reserved per approved creation until it is measured: the
	// entry's own block and its name in the parent directory's index (an
	// ext4 dirent is at most 8+255 bytes, rounded).
	entryCost        = quotaBlock + 264
	rwfHipri         = 0x1
	rwfDsync         = 0x2
	rwfSync          = 0x4
	rwfNowait        = 0x8
	rwfNoappend      = 0x20
	pipeBuf          = 4096
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
	creates, spawns                                       map[int32]bool
}

var callsByArch = map[string]quotaCalls{
	"amd64": {
		write: 1, pwrite64: 18, writev: 20, pwritev: 296, pwritev2: 328, ftruncate: 77,
		creates: map[int32]bool{2: true, 257: true, 85: true, 437: true, 83: true, 258: true, 133: true, 259: true, 86: true, 265: true, 88: true, 266: true},
		spawns:  map[int32]bool{56: true, 57: true, 58: true},
	},
	"arm64": {
		write: 64, pwrite64: 68, writev: 66, pwritev: 70, pwritev2: 287, ftruncate: 46,
		creates: map[int32]bool{56: true, 437: true, 34: true, 33: true, 37: true, 36: true},
		spawns:  map[int32]bool{220: true},
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
	fdMu       sync.RWMutex // guards fd against stream goroutines after run ends
	fdClosed   bool
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
	defer func() {
		s.fdMu.Lock()
		s.fdClosed = true
		_ = unix.Close(s.fd)
		s.fdMu.Unlock()
	}()
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

// used is the workdir's charged allocation: every entry at its block-rounded
// size and at least one block (files in the ledger or measured), plus one
// block for each creation approved since the last measurement.
func (s *supervisor) used() int64 {
	t := s.baseline + (s.entryPrev+s.entryCur)*entryCost
	for _, size := range s.ledger {
		t += size
	}
	return t
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
	var walked, other int64
	_ = filepath.WalkDir(s.workdir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // a vanished entry is not an error for a measurement
		}
		walked++
		if s.entries > 0 && walked > s.entries {
			return filepath.SkipAll
		}
		info, err := d.Info()
		if err != nil {
			return nil //nolint:nilerr // vanished meanwhile
		}
		// The larger of the apparent size and the allocated blocks: some
		// filesystems (overlayfs) report a directory's size below its blocks.
		size := info.Size()
		st, ok := info.Sys().(*syscall.Stat_t)
		if ok {
			size = max(size, st.Blocks*512)
		}
		if ok && d.Type().IsRegular() {
			visible[inode{uint64(st.Dev), st.Ino}] = charge(size) //nolint:unconvert // Dev width differs by arch
		} else {
			// Directories (their index grows with entries), symlinks
			// (targets up to a block), FIFOs: at least one block each.
			other += charge(size)
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
		} else if !held[ino.ino] {
			delete(s.ledger, ino)
		}
	}
	var baseline int64
	for ino, size := range visible {
		if _, ok := s.ledger[ino]; !ok {
			baseline += size
		}
	}
	s.baseline = baseline + other
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
// Inodes are matched by number only: /proc/<pid>/maps reports the device of
// the underlying layer on overlayfs and btrfs, so a mismatch errs on keeping
// the charge.
func (s *supervisor) heldInodes() (map[uint64]bool, error) {
	pids, _ := s.groupStats()
	held := map[uint64]bool{}
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
				held[st.Ino] = true
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
			if ino, err := strconv.ParseUint(fields[4], 10, 64); err == nil {
				held[ino] = true
			}
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
	switch {
	case s.calls.creates[n.nr]:
		s.respond(n.id, 0, s.approveCreate(), true)
	case s.calls.spawns[n.nr]:
		// New threads and processes only while an exec runs (or before the
		// kernel is ready): nothing can start work while the kernel is idle.
		if s.k.spawnAllowed() {
			s.respond(n.id, 0, 0, true)
		} else {
			s.respond(n.id, 0, unix.EAGAIN, false)
		}
	default:
		s.emulate(n)
	}
}

// approveCreate counts one entry creation; the call itself then runs (its
// arguments do not matter to the count).
func (s *supervisor) approveCreate() syscall.Errno {
	full := func() bool {
		return (s.entries > 0 && s.usedEntries()+1 > s.entries) || s.used()+entryCost > s.quota
	}
	if full() && time.Since(s.lastRecon) >= reconcileMinGap {
		_ = s.reconcile()
	}
	if full() {
		s.k.noteQuota()
		return unix.EDQUOT
	}
	s.entryCur++
	return 0
}

// respond answers notification id: errno != 0 fails the call; otherwise cont
// lets it run, or val is its result (emulated).
func (s *supervisor) respond(id uint64, val int64, errno syscall.Errno, cont bool) {
	s.fdMu.RLock()
	defer s.fdMu.RUnlock()
	if s.fdClosed {
		return
	}
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

// dupTarget duplicates the caller's fd into the host through a pidfd of the
// calling thread, and confirms the notification is still pending afterwards,
// so the duplicate belongs to the task that made the call.
func (s *supervisor) dupTarget(n notification, fd int) (int, error) {
	pidfd, err := unix.PidfdOpen(n.tid, pidfdThread)
	if err != nil {
		// Before PIDFD_THREAD (Linux 6.9) only a group leader has a pidfd;
		// the sandbox refuses threads with their own fd table, so the
		// leader's table is the caller's.
		tgid, terr := threadGroup(n.tid)
		if terr != nil {
			return -1, err
		}
		if pidfd, err = unix.PidfdOpen(tgid, 0); err != nil {
			return -1, err
		}
	}
	defer unix.Close(pidfd)
	if !s.idValid(n.id) {
		return -1, unix.ESRCH
	}
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

// span is one piece of the caller's data: memory it wants written.
type span struct {
	addr uintptr
	len  uint64
}

// spans returns the caller's data as address ranges, without copying it.
func (s *supervisor) spans(n notification) ([]span, syscall.Errno) {
	c := s.calls
	switch n.nr {
	case c.write, c.pwrite64:
		return []span{{uintptr(n.args[1]), n.args[2]}}, 0
	case c.writev, c.pwritev, c.pwritev2:
		cnt := int64(n.args[2])
		if cnt < 0 || cnt > maxIovecs {
			return nil, unix.EINVAL
		}
		if cnt == 0 {
			return nil, 0
		}
		iov := make([]byte, 16*cnt)
		if err := s.readMem(n.tid, uintptr(n.args[1]), iov); err != nil {
			return nil, unix.EFAULT
		}
		out := make([]span, 0, cnt)
		for i := int64(0); i < cnt; i++ {
			out = append(out, span{uintptr(binary.NativeEndian.Uint64(iov[16*i:])), binary.NativeEndian.Uint64(iov[16*i+8:])})
		}
		return out, 0
	}
	return nil, unix.ENOSYS
}

func total(spans []span) uint64 {
	var t uint64
	for _, sp := range spans {
		t += sp.len
	}
	return t
}

// emulate performs a write-family call for the kernel.
func (s *supervisor) emulate(n notification) {
	dup, err := s.dupTarget(n, int(int32(n.args[0])))
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
	if n.nr == s.calls.ftruncate {
		s.truncate(n, dup, st)
		return
	}
	spans, errno := s.spans(n)
	if errno != 0 {
		_ = unix.Close(dup)
		s.respond(n.id, 0, errno, false)
		return
	}
	if st.Mode&unix.S_IFMT == unix.S_IFREG {
		defer unix.Close(dup)
		s.writeFile(n, dup, st, spans)
		return
	}
	s.writeStream(n, dup, spans)
}

// fileWrite is where and how a write-family call writes a regular file.
type fileWrite struct {
	at         int64 // the offset the data lands at
	positional bool  // pwrite-style: the file position does not move
	flags      int   // RWF_* to repeat on the duplicate (pwritev2)
}

// placement resolves the offset of a write exactly once, from state only the
// supervisor changes (all extending calls go through it), so no other thread
// can move the write after it was checked.
func (s *supervisor) placement(n notification, dup int, size int64) (fileWrite, syscall.Errno) {
	c := s.calls
	flags, err := unix.FcntlInt(uintptr(dup), unix.F_GETFL, 0)
	if err != nil {
		return fileWrite{}, unix.EBADF
	}
	appendMode := flags&unix.O_APPEND != 0
	w := fileWrite{}
	switch n.nr {
	case c.pwrite64, c.pwritev:
		w.at, w.positional = int64(n.args[3]), true
	case c.pwritev2:
		rwf := int(n.args[5])
		if rwf&^(rwfHipri|rwfDsync|rwfSync|rwfNowait|rwfAppend|rwfNoappend) != 0 {
			return fileWrite{}, unix.EOPNOTSUPP
		}
		w.flags = rwf &^ (rwfAppend | rwfNoappend)
		if off := int64(n.args[3]); off != -1 {
			w.at, w.positional = off, true
		}
		if rwf&rwfAppend != 0 {
			appendMode = true
		}
		if rwf&rwfNoappend != 0 && w.positional {
			appendMode = false
		}
	}
	if w.positional && w.at < 0 {
		return fileWrite{}, unix.EINVAL
	}
	switch {
	case appendMode:
		w.at = size
	case !w.positional:
		pos, err := unix.Seek(dup, 0, 1)
		if err != nil {
			return fileWrite{}, errnoOf(err, unix.ESPIPE)
		}
		w.at = pos
	}
	return w, 0
}

// writeFile writes the caller's data to the regular file behind dup at the
// checked offset, within the quota and the file size limit, in chunks copied
// out of the caller, and charges the result.
func (s *supervisor) writeFile(n notification, dup int, st unix.Stat_t, spans []span) {
	w, errno := s.placement(n, dup, st.Size)
	if errno != 0 {
		s.respond(n.id, 0, errno, false)
		return
	}
	want := total(spans)
	if want == 0 {
		s.respond(n.id, 0, 0, false)
		return
	}
	if want > math.MaxInt64-uint64(w.at) {
		s.respond(n.id, 0, unix.EFBIG, false)
		return
	}
	end := w.at + int64(want)
	if s.fileSize > 0 && end > s.fileSize {
		if w.at >= s.fileSize {
			s.signal(n.tid, unix.SIGXFSZ)
			s.respond(n.id, 0, unix.EFBIG, false)
			return
		}
		end = s.fileSize
	}
	if limit := s.maxEnd(st); end > limit {
		s.k.noteQuota() // refused or cut short by the quota
		if w.at >= limit {
			s.respond(n.id, 0, unix.EDQUOT, false)
			return
		}
		end = limit
	}
	// Reserve the growth before writing; the charge settles from fstat.
	s.reserve(st, end)
	var written int64
	buf := make([]byte, 0, maxFileChunk)
	for _, sp := range spans {
		for off := uint64(0); off < sp.len && w.at+written < end; {
			chunk := min(sp.len-off, uint64(maxFileChunk), uint64(end-w.at-written))
			buf = buf[:chunk]
			if err := s.readMem(n.tid, sp.addr+uintptr(off), buf); err != nil {
				s.settle(dup, st)
				s.finishFile(n, dup, w, written, unix.EFAULT)
				return
			}
			m, err := pwritev2(dup, buf, w.at+written, w.flags)
			if m > 0 {
				written += int64(m)
				off += uint64(m)
			}
			if err != nil {
				s.settle(dup, st)
				s.finishFile(n, dup, w, written, errnoOf(err, unix.EIO))
				return
			}
			if m == 0 {
				break
			}
		}
	}
	s.settle(dup, st)
	s.finishFile(n, dup, w, written, 0)
}

// finishFile moves the file position of an unpositioned write and answers:
// the bytes written, or the error when nothing was.
func (s *supervisor) finishFile(n notification, dup int, w fileWrite, written int64, errno syscall.Errno) {
	if !w.positional && written > 0 {
		_, _ = unix.Seek(dup, w.at+written, 0)
	}
	if written > 0 || errno == 0 {
		s.respond(n.id, written, 0, false)
		return
	}
	s.respond(n.id, 0, errno, false)
}

// pwritev2 writes buf at off with the caller's remaining RWF flags.
func pwritev2(fd int, buf []byte, off int64, flags int) (int, error) {
	if flags == 0 {
		return unix.Pwrite(fd, buf, off)
	}
	return unix.Pwritev2(fd, [][]byte{buf}, off, flags)
}

// charge is what an entry of the given size costs against the quota.
func charge(size int64) int64 { return max(blocks(size), quotaBlock) }

func blocks(size int64) int64 {
	if size <= 0 {
		return 0
	}
	return (size + quotaBlock - 1) / quotaBlock * quotaBlock
}

// maxEnd is the largest end the quota allows for the file st describes: the
// free room plus what the file is charged now, reconciling once when that
// looks too small.
func (s *supervisor) maxEnd(st unix.Stat_t) int64 {
	key := inode{uint64(st.Dev), st.Ino} //nolint:unconvert // Dev width differs by arch
	current := func() int64 {
		if v, ok := s.ledger[key]; ok {
			return v
		}
		return charge(st.Size)
	}
	room := func() int64 { return s.quota - s.used() + current() }
	r := room()
	if r <= charge(st.Size) && time.Since(s.lastRecon) >= reconcileMinGap {
		_ = s.reconcile()
		r = room()
	}
	if r <= 0 {
		return 0
	}
	return r / quotaBlock * quotaBlock
}

func (s *supervisor) reserve(st unix.Stat_t, end int64) {
	key := inode{uint64(st.Dev), st.Ino} //nolint:unconvert // Dev width differs by arch
	if cur, ok := s.ledger[key]; !ok || charge(end) > cur {
		s.ledger[key] = max(charge(end), charge(st.Size))
	}
}

// settle replaces the reservation with the file's real size, which never
// drops below what it was before this call (other holders may shrink it;
// the next reconcile credits that).
func (s *supervisor) settle(dup int, before unix.Stat_t) {
	var st unix.Stat_t
	if unix.Fstat(dup, &st) == nil {
		key := inode{uint64(st.Dev), st.Ino} //nolint:unconvert // Dev width differs by arch
		s.ledger[key] = max(charge(st.Size), charge(before.Size))
	}
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
		s.signal(n.tid, unix.SIGXFSZ)
		s.respond(n.id, 0, unix.EFBIG, false)
		return
	}
	if length > st.Size {
		if length > s.maxEnd(st) {
			s.k.noteQuota()
			s.respond(n.id, 0, unix.EDQUOT, false)
			return
		}
		s.reserve(st, length)
	}
	err := unix.Ftruncate(dup, length)
	s.settle(dup, st)
	if err != nil {
		s.respond(n.id, 0, errnoOf(err, unix.EIO), false)
		return
	}
	s.respond(n.id, 0, 0, false)
}

// writeStream writes to a pipe, socket or character device. The data is
// copied out on the supervisor thread (it holds the kernel's credentials);
// the write itself is non-blocking (RWF_NOWAIT) and retried on POLLOUT in a
// bounded number of goroutines that stop when the kernel is disposed, so a
// stalled reader never holds a host thread. With WAIT_KILLABLE_RECV the
// caller cannot be interrupted meanwhile, so the write happens exactly once.
func (s *supervisor) writeStream(n notification, dup int, spans []span) {
	want := total(spans)
	data := make([]byte, 0, min(want, maxStreamPayload))
	for _, sp := range spans {
		if uint64(len(data)) >= maxStreamPayload {
			break
		}
		l := min(sp.len, maxStreamPayload-uint64(len(data)))
		chunk := make([]byte, l)
		if err := s.readMem(n.tid, sp.addr, chunk); err != nil {
			if len(data) == 0 {
				_ = unix.Close(dup)
				s.respond(n.id, 0, unix.EFAULT, false)
				return
			}
			break
		}
		data = append(data, chunk...)
	}
	if !s.idValid(n.id) {
		_ = unix.Close(dup)
		return
	}
	if !s.k.addStream() {
		_ = unix.Close(dup)
		s.respond(n.id, 0, unix.EAGAIN, false)
		return
	}
	go func() {
		defer s.k.doneStream()
		defer unix.Close(dup)
		written, errno := s.pump(dup, data, n)
		if errno == unix.EPIPE {
			s.signal(n.tid, unix.SIGPIPE) // as the kernel does before EPIPE
		}
		if written > 0 || errno == 0 {
			s.respond(n.id, written, 0, false)
			return
		}
		if errno != unix.ESRCH {
			s.respond(n.id, 0, errno, false)
		}
	}()
}

// pump writes data without blocking, waiting for POLLOUT in between, until it
// is all written, the caller is gone or the kernel is disposed.
func (s *supervisor) pump(dup int, data []byte, n notification) (int64, syscall.Errno) {
	flags, _ := unix.FcntlInt(uintptr(dup), unix.F_GETFL, 0)
	nonblocking := flags&unix.O_NONBLOCK != 0
	var written int64
	nowait := true
	for len(data) > 0 {
		var m int
		var err error
		if nowait {
			m, err = unix.Pwritev2(dup, [][]byte{data}, -1, rwfNowait)
			if errors.Is(err, unix.EOPNOTSUPP) {
				nowait = false // FIFOs opened by path, some devices
			}
		}
		if !nowait {
			// Without RWF_NOWAIT: once POLLOUT says there is room, a write of
			// at most PIPE_BUF cannot block.
			pfd := []unix.PollFd{{Fd: int32(dup), Events: unix.POLLOUT}}
			if c, _ := unix.Poll(pfd, 0); c > 0 && pfd[0].Revents&unix.POLLOUT != 0 {
				m, err = unix.Write(dup, data[:min(len(data), pipeBuf)])
			} else {
				m, err = 0, unix.EAGAIN
			}
		}
		if m > 0 {
			written += int64(m)
			data = data[m:]
			continue
		}
		if err == nil {
			break
		}
		if !errors.Is(err, unix.EAGAIN) {
			return written, errnoOf(err, unix.EIO)
		}
		if nonblocking || written > 0 && len(data) == 0 {
			return written, unix.EAGAIN
		}
		for {
			select {
			case <-s.k.closed:
				return written, unix.ESRCH
			default:
			}
			if !s.idValid(n.id) {
				return written, unix.ESRCH
			}
			pfd := []unix.PollFd{{Fd: int32(dup), Events: unix.POLLOUT}}
			if c, _ := unix.Poll(pfd, supervisorTick); c > 0 {
				break
			}
		}
	}
	return written, 0
}

// signal sends sig to the calling thread, as the kernel would for the call.
func (s *supervisor) signal(tid int, sig unix.Signal) {
	if tgid, err := threadGroup(tid); err == nil {
		_ = unix.Tgkill(tgid, tid, sig)
	}
}

// idValid reports whether notification id is still pending: its task is
// alive and waiting, so what was read on its behalf belongs to it.
func (s *supervisor) idValid(id uint64) bool {
	s.fdMu.RLock()
	defer s.fdMu.RUnlock()
	if s.fdClosed {
		return false
	}
	v := id
	_, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(s.fd), unix.SECCOMP_IOCTL_NOTIF_ID_VALID, uintptr(unsafe.Pointer(&v)))
	return e == 0
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
