//go:build unix

package execute

import (
	"context"
	"os"
	"path"
	"sort"

	"golang.org/x/sys/unix"
)

const (
	maxArtifactDepth   = 16
	maxArtifactEntries = 10_000
)

// collectArtifacts stores regular files from <workdir>/out. The tree is
// written by untrusted code that may still be running, so it is traversed only
// through directory descriptors pinned from the workdir: every component is
// opened relative to its parent with O_NOFOLLOW, nothing is resolved by path,
// and the file that is read is the descriptor that was validated (a regular
// file within the size limit). A symlink or a swapped directory therefore
// never makes the host read outside the workdir.
func (r *Runner) collectArtifacts(ctx context.Context, workdir string) []Artifact {
	out := []Artifact{}
	if r.cfg.Artifacts == nil {
		return out
	}
	wfd, err := unix.Open(workdir, unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return out
	}
	defer func() { _ = unix.Close(wfd) }()
	ofd, err := unix.Openat(wfd, outputDir, unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return out
	}
	seen := 0
	r.walkArtifacts(ctx, ofd, "", 0, &seen, &out)
	return out
}

// walkArtifacts consumes dirfd.
func (r *Runner) walkArtifacts(ctx context.Context, dirfd int, rel string, depth int, seen *int, out *[]Artifact) {
	dir := os.NewFile(uintptr(dirfd), "artifacts")
	defer func() { _ = dir.Close() }()
	names, _ := dir.Readdirnames(maxArtifactEntries)
	sort.Strings(names)
	for _, name := range names {
		if len(*out) >= r.cfg.MaxArtifacts || *seen >= maxArtifactEntries || ctx.Err() != nil {
			return
		}
		*seen++
		// O_NONBLOCK: a FIFO planted in ./out must not block the host.
		fd, err := unix.Openat(dirfd, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err != nil {
			continue
		}
		var st unix.Stat_t
		if unix.Fstat(fd, &st) != nil {
			_ = unix.Close(fd)
			continue
		}
		switch st.Mode & unix.S_IFMT {
		case unix.S_IFDIR:
			if depth+1 < maxArtifactDepth {
				r.walkArtifacts(ctx, fd, path.Join(rel, name), depth+1, seen, out)
			} else {
				_ = unix.Close(fd)
			}
		case unix.S_IFREG:
			f := os.NewFile(uintptr(fd), name)
			if st.Size <= r.cfg.MaxArtifactBytes {
				if a, ok := r.artifact(ctx, path.Join(rel, name), f); ok {
					*out = append(*out, a)
				}
			}
			_ = f.Close()
		default:
			_ = unix.Close(fd)
		}
	}
}
