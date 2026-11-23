//go:build !linux

package isolation

import (
	"io/fs"
	"os"
	"path/filepath"
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

// ChownTree hands dir and everything below it to cred without following
// symlinks.
func ChownTree(dir string, cred Credential) error {
	return filepath.WalkDir(dir, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Lchown(path, int(cred.UID), int(cred.GID))
	})
}
