//go:build linux

package manager

import (
	"bytes"
	"os"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
)

// atRest reports whether the kernel with group leader pid is at rest: every
// other live process of its process group is killed (an exec's lifetime ends
// with the processes it started), and no thread of the leader is runnable.
func atRest(pid int) bool {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	pgid := strconv.Itoa(pid)
	rest := true
	for _, e := range entries {
		if e.Name() == pgid {
			continue
		}
		f, ok := procStat("/proc/" + e.Name() + "/stat")
		if !ok || len(f) < 3 || f[2] != pgid || f[0] == "Z" || f[0] == "X" {
			continue
		}
		if other, err := strconv.Atoi(e.Name()); err == nil {
			isolation.KillMember(other, pid)
			rest = false
		}
	}
	tasks, err := os.ReadDir("/proc/" + pgid + "/task")
	if err != nil {
		return false
	}
	for _, t := range tasks {
		f, ok := procStat("/proc/" + pgid + "/task/" + t.Name() + "/stat")
		if !ok || len(f) == 0 || f[0] == "R" {
			return false // running, or not observable: not at rest
		}
	}
	return rest
}

// pristine reports whether the kernel runs only its own two threads (the
// bridge reader and the exec worker) and no other process.
func pristine(pid int) bool {
	tasks, err := os.ReadDir("/proc/" + strconv.Itoa(pid) + "/task")
	if err != nil || len(tasks) > 2 {
		return false
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	want := strconv.Itoa(pid)
	for _, e := range entries {
		if e.Name() == want {
			continue
		}
		if f, ok := procStat("/proc/" + e.Name() + "/stat"); ok && len(f) > 2 && f[2] == want && f[0] != "Z" && f[0] != "X" {
			return false
		}
	}
	return true
}

// groupStopped reports whether no thread of any live member of process group
// pgid can run user code: each is stopped, or blocked in the kernel with the
// stop pending.
func groupStopped(pgid int) bool {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	want := strconv.Itoa(pgid)
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		f, ok := procStat("/proc/" + e.Name() + "/stat")
		if !ok || len(f) < 3 || f[2] != want {
			continue
		}
		if st := f[0]; st == "Z" || st == "X" {
			continue
		}
		tasks, err := os.ReadDir("/proc/" + e.Name() + "/task")
		if err != nil {
			return false
		}
		for _, t := range tasks {
			tf, ok := procStat("/proc/" + e.Name() + "/task/" + t.Name() + "/stat")
			if !ok || len(tf) == 0 {
				return false // a thread we cannot see is not known to be stopped
			}
			// D: blocked in the kernel (e.g. on a write the host performs);
			// it handles the pending stop before it returns to user code.
			if st := tf[0]; st != "T" && st != "t" && st != "D" && st != "Z" && st != "X" {
				return false
			}
		}
	}
	return true
}

// procStat returns the fields of a /proc stat file after the command name.
func procStat(path string) ([]string, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	i := bytes.LastIndexByte(raw, ')')
	if i < 0 {
		return nil, false
	}
	return strings.Fields(string(raw[i+1:])), true
}
