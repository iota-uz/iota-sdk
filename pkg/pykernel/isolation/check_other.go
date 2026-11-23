//go:build !linux

package isolation

import (
	"runtime"
)

// hostReport reports every boundary missing: the sandbox needs Linux
// (Landlock and seccomp).
func hostReport(Profile) Report {
	r := Report{Platform: runtime.GOOS + "/" + runtime.GOARCH}
	for _, b := range RequiredBoundaries {
		r.Boundaries = append(r.Boundaries, BoundaryStatus{Boundary: b, Detail: "the sandbox requires Linux"})
	}
	return r
}

func countTasks(uint32) uint64 { return 0 }
