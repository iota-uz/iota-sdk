//go:build unix

package artifact

import (
	"os"
	"syscall"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// securePrivateDir makes dir a private directory of the current user: it
// must be a real directory (not a symlink) owned by this user, and group and
// world get no access. The materialized artifacts are code the workers
// import, so another local user must not be able to place or edit files in
// the directory they are read from.
func securePrivateDir(dir string) error {
	st, err := os.Lstat(dir)
	if err != nil {
		return serrors.Wrap(opDir, err)
	}
	if !st.IsDir() {
		return serrors.New(serrors.FailedPrecondition, "artifact cache "+dir+" is not a directory (or is a symlink)").WithOp(opDir)
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok || int(sys.Uid) != os.Geteuid() {
		return serrors.New(serrors.FailedPrecondition, "artifact cache "+dir+" is not owned by the user running the service; set EXTRUNTIME_CACHEDIR to a private directory").WithOp(opDir)
	}
	if st.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return serrors.Wrap(opDir, err)
		}
	}
	return nil
}
