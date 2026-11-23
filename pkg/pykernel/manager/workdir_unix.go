//go:build unix

package manager

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"

	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
)

// prepareState readies a session workdir that a kernel (possibly still alive)
// can write: it creates the state directory and hands host-created entries to
// the kernel uid. Every
// operation is relative to an O_NOFOLLOW directory fd and touches a single
// path component, so a symlink or rename race planted by the kernel can never
// redirect a host (possibly root) write, chown or unlink outside the workdir.
func prepareState(workdir string, cred *isolation.Credential) error {
	if err := os.MkdirAll(workdir, 0o700); err != nil {
		return err
	}
	wfd, err := unix.Open(workdir, unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open workdir: %w", err)
	}
	defer func() { _ = unix.Close(wfd) }()
	if cred != nil {
		if err := handOver(wfd, *cred); err != nil {
			return err
		}
	}
	ctrl, err := realDirAt(wfd, stateDir, cred)
	if err != nil {
		return err
	}
	defer func() { _ = unix.Close(ctrl) }()
	tmp, err := realDirAt(ctrl, "tmp", cred)
	if err != nil {
		return err
	}
	_ = unix.Close(tmp)

	return nil
}

// maxHandOver bounds how many top-level entries one handOver inspects.
const maxHandOver = 1 << 17

// handOver gives the workdir and its direct entries owned by the host to the
// kernel uid. Entries the kernel created are already its own; deeper host
// entries are not expected (the host creates top-level directories only).
func handOver(wfd int, cred isolation.Credential) error {
	if err := unix.Fchown(wfd, int(cred.UID), int(cred.GID)); err != nil {
		return fmt.Errorf("hand workdir to the kernel uid: %w", err)
	}
	dup, err := unix.Dup(wfd)
	if err != nil {
		return err
	}
	dir := os.NewFile(uintptr(dup), "workdir")
	defer func() { _ = dir.Close() }()
	var names []string
	for len(names) < maxHandOver {
		chunk, err := dir.Readdirnames(1024)
		names = append(names, chunk...)
		if err != nil {
			break // io.EOF: done
		}
	}
	self := uint32(os.Geteuid())
	for _, name := range names {
		var st unix.Stat_t
		if unix.Fstatat(wfd, name, &st, unix.AT_SYMLINK_NOFOLLOW) != nil || st.Uid != self {
			continue
		}
		if err := unix.Fchownat(wfd, name, int(cred.UID), int(cred.GID), unix.AT_SYMLINK_NOFOLLOW); err != nil && !errors.Is(err, unix.ENOENT) {
			return fmt.Errorf("hand %s to the kernel uid: %w", name, err)
		}
	}
	return nil
}

// realDirAt opens name below dirfd as a real directory, replacing a symlink or
// file the kernel may have put there, and returns its fd.
func realDirAt(dirfd int, name string, cred *isolation.Credential) (int, error) {
	for attempt := 0; attempt < 3; attempt++ {
		fd, err := unix.Openat(dirfd, name, unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_RDONLY|unix.O_CLOEXEC, 0)
		if err == nil {
			if cred != nil {
				_ = unix.Fchown(fd, int(cred.UID), int(cred.GID))
			}
			return fd, nil
		}
		switch {
		case errors.Is(err, unix.ENOENT):
		case errors.Is(err, unix.ELOOP), errors.Is(err, unix.ENOTDIR):
			if err := unix.Unlinkat(dirfd, name, 0); err != nil && !errors.Is(err, unix.ENOENT) {
				return -1, fmt.Errorf("replace %s: %w", name, err)
			}
		default:
			return -1, fmt.Errorf("open %s: %w", name, err)
		}
		if err := unix.Mkdirat(dirfd, name, 0o700); err != nil && !errors.Is(err, unix.EEXIST) {
			return -1, fmt.Errorf("create %s: %w", name, err)
		}
	}
	return -1, fmt.Errorf("create %s: the kernel keeps replacing it", name)
}

// dirIdentity returns the device and inode of the directory at path.
func dirIdentity(path string) (uint64, uint64, error) {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return 0, 0, err
	}
	return uint64(st.Dev), st.Ino, nil //nolint:unconvert // Dev is int32 on some platforms
}
