//go:build linux

package isolation

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	landlockCreateRulesetVersion = 1
	seccompGetActionAvail        = 2
	seccompGetNotifSizes         = 3
	seccompRetKillProcess        = 0x80000000
	seccompRetErrno              = 0x00050000
)

// hostReport probes what the host kernel supports without changing the host
// process: the Landlock ABI, seccomp filter actions, and whether the kernel
// would run as a uid for which RLIMIT_NPROC holds.
func hostReport(p Profile) Report {
	r := Report{Platform: runtime.GOOS + "/" + runtime.GOARCH}
	set := func(b Boundary, ok bool, detail string) {
		r.Boundaries = append(r.Boundaries, BoundaryStatus{Boundary: b, Enabled: ok, Detail: detail})
	}

	abi, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, landlockCreateRulesetVersion)
	llDetail := ""
	switch {
	case errno != 0:
		llDetail = "landlock unavailable: " + errno.Error()
		abi = 0
	case int(abi) < MinLandlockABI:
		llDetail = fmt.Sprintf("landlock ABI %d < required %d", abi, MinLandlockABI)
	default:
		llDetail = fmt.Sprintf("landlock ABI %d", abi)
	}
	r.LandlockABI = int(abi)
	llOK := errno == 0 && int(abi) >= MinLandlockABI

	_, _, nnpErr := unix.Syscall6(unix.SYS_PRCTL, unix.PR_GET_NO_NEW_PRIVS, 0, 0, 0, 0, 0)
	set(BoundaryNoNewPrivs, nnpErr == 0, errDetail(nnpErr, "prctl(PR_SET_NO_NEW_PRIVS) available"))
	set(BoundaryNoCapabilities, true, "capabilities are dropped by the child")
	set(BoundaryLandlockFS, llOK, llDetail)
	set(BoundaryLandlockNet, llOK, llDetail)
	set(BoundaryLandlockScope, llOK, llDetail)

	seccompOK, seccompDetail := true, "seccomp filters with KILL_PROCESS and ERRNO available"
	for _, action := range []uint32{seccompRetKillProcess, seccompRetErrno} {
		a := action
		if _, _, e := unix.Syscall(unix.SYS_SECCOMP, seccompGetActionAvail, 0, uintptr(unsafe.Pointer(&a))); e != 0 {
			seccompOK, seccompDetail = false, "seccomp unavailable: "+e.Error()
			break
		}
	}
	set(BoundarySeccomp, seccompOK, seccompDetail)
	set(BoundaryRlimits, true, "setrlimit/prlimit available")
	var sizes [3]uint16
	if _, _, e := unix.Syscall(unix.SYS_SECCOMP, seccompGetNotifSizes, 0, uintptr(unsafe.Pointer(&sizes))); e != 0 {
		set(BoundaryWorkdirQuota, false, "seccomp user notification unavailable: "+e.Error())
	} else {
		set(BoundaryWorkdirQuota, true, "seccomp user notification available")
	}

	uid := uint32(os.Getuid())
	if p.RunAs != nil {
		uid = p.RunAs.UID
	}
	if uid == 0 {
		set(BoundaryNPROC, false, "the kernel would run as uid 0, for which RLIMIT_NPROC is not enforced; set Profile.RunAs")
	} else {
		set(BoundaryNPROC, true, fmt.Sprintf("kernel runs as uid %d", uid))
	}
	return r
}

func errDetail(e syscall.Errno, ok string) string {
	if e != 0 {
		return e.Error()
	}
	return ok
}

// countTasks returns how many tasks (threads included) uid runs right now, the
// count RLIMIT_NPROC is checked against.
func countTasks(uid uint32) uint64 {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	var n uint64
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/proc", e.Name(), "status"))
		if err != nil {
			continue
		}
		var realUID int64 = -1
		var threads uint64
		for _, line := range strings.Split(string(raw), "\n") {
			switch {
			case strings.HasPrefix(line, "Uid:"):
				if f := strings.Fields(line); len(f) > 1 {
					realUID, _ = strconv.ParseInt(f[1], 10, 64)
				}
			case strings.HasPrefix(line, "Threads:"):
				if f := strings.Fields(line); len(f) > 1 {
					threads, _ = strconv.ParseUint(f[1], 10, 64)
				}
			}
		}
		if realUID == int64(uid) {
			n += threads
		}
	}
	return n
}
