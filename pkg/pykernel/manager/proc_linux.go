//go:build linux

package manager

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"syscall"
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
			_ = syscall.Kill(other, syscall.SIGKILL)
			rest = false
		}
	}
	tasks, err := os.ReadDir("/proc/" + pgid + "/task")
	if err != nil {
		return false
	}
	for _, t := range tasks {
		f, ok := procStat("/proc/" + pgid + "/task/" + t.Name() + "/stat")
		if ok && len(f) > 0 && f[0] == "R" {
			return false
		}
	}
	return rest
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
