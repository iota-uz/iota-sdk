//go:build linux

package manager

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
)

// groupMembers lists the processes of process group pgid. Every observation
// must be positive: an entry that vanished (ENOENT) is gone, but one that
// cannot be read might be a member, so ok is false then.
func groupMembers(pgid int) ([]int, bool) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, false
	}
	var members []int
	want := strconv.Itoa(pgid)
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		f, err := procStat("/proc/" + e.Name() + "/stat")
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil || len(f) < 3 {
			return nil, false
		}
		if f[2] == want {
			members = append(members, pid)
		}
	}
	return members, true
}

// taskStates returns the scheduler state of every thread of process pid
// (none when the process is gone); ok is false when one cannot be read.
func taskStates(pid int) ([]string, bool) {
	base := "/proc/" + strconv.Itoa(pid) + "/task"
	tasks, err := os.ReadDir(base)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, true
	}
	if err != nil {
		return nil, false
	}
	states := make([]string, 0, len(tasks))
	for _, t := range tasks {
		f, err := procStat(base + "/" + t.Name() + "/stat")
		if errors.Is(err, fs.ErrNotExist) {
			continue // the thread exited
		}
		if err != nil || len(f) == 0 {
			return nil, false
		}
		states = append(states, f[0])
	}
	return states, true
}

// atRest reports whether the kernel with group leader pid is at rest: every
// other live process of its process group is killed (an exec's lifetime ends
// with the processes it started), and no thread of the leader is runnable.
// Anything it cannot observe makes it not at rest.
func atRest(pid int) bool {
	members, ok := groupMembers(pid)
	if !ok {
		return false
	}
	rest, sawLeader := true, false
	for _, m := range members {
		if m == pid {
			sawLeader = true
			continue
		}
		states, ok := taskStates(m)
		if !ok {
			return false
		}
		if live(states) {
			isolation.KillMember(m, pid)
			rest = false
		}
	}
	if !sawLeader {
		return false
	}
	states, ok := taskStates(pid)
	if !ok || len(states) == 0 {
		return false
	}
	for _, st := range states {
		if st == "R" {
			return false
		}
	}
	return rest
}

func live(states []string) bool {
	for _, st := range states {
		if st != "Z" && st != "X" {
			return true
		}
	}
	return false
}

// groupStopped reports, from positive observation only, that no thread of
// any member of process group pgid can run user code: the leader is seen,
// and every thread of every member is stopped, exited, or blocked in the
// kernel (D) with the stop pending — it handles the stop before it returns
// to user code.
func groupStopped(pgid int) bool {
	members, ok := groupMembers(pgid)
	if !ok {
		return false
	}
	sawLeader := false
	for _, m := range members {
		states, ok := taskStates(m)
		if !ok {
			return false
		}
		if m == pgid {
			if len(states) == 0 {
				return false // the leader vanished while being observed
			}
			sawLeader = true
		}
		for _, st := range states {
			switch st {
			case "T", "t", "D", "Z", "X":
			default:
				return false
			}
		}
	}
	return sawLeader
}

// procStat returns the fields of a /proc stat file after the command name.
func procStat(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	i := bytes.LastIndexByte(raw, ')')
	if i < 0 {
		return nil, errors.New("malformed stat")
	}
	return strings.Fields(string(raw[i+1:])), nil
}
