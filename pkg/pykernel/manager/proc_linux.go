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

// groupStopped reports whether every live member of process group pgid is
// stopped by a signal.
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
		if st := f[0]; st != "T" && st != "Z" && st != "X" {
			return false
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
