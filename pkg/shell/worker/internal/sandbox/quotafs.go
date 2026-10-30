package sandbox

import (
	"context"
	stdfs "io/fs"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"syscall"

	gbfs "github.com/ewhauser/gbash/fs"
)

// quotaFS caps the bytes stored by the guest. Growth is charged on every
// write, so appends, copies, archive extraction and truncate extension (which
// gbash implements as zero writes) all count; truncation, removal and
// overwriting renames release the bytes they drop. Hard links are refused so a
// file cannot be released twice.
type quotaFS struct {
	gbfs.FileSystem
	limit    int64
	mu       sync.Mutex
	used     int64
	exceeded atomic.Bool
}

func newQuotaFS(base gbfs.FileSystem, limit int64) *quotaFS {
	return &quotaFS{FileSystem: base, limit: limit}
}

func (q *quotaFS) OpenFile(ctx context.Context, name string, flag int, perm stdfs.FileMode) (gbfs.File, error) {
	writable := flag&(os.O_WRONLY|os.O_RDWR) != 0
	var freed int64
	if writable && flag&os.O_TRUNC != 0 {
		q.mu.Lock()
		defer q.mu.Unlock()
		if info, err := q.Stat(ctx, name); err == nil && info.Mode().IsRegular() {
			freed = info.Size()
		}
	}
	f, err := q.FileSystem.OpenFile(ctx, name, flag, perm)
	if err != nil {
		return nil, err
	}
	if freed > 0 {
		q.releaseLocked(freed)
	}
	if !writable {
		return f, nil
	}
	return &quotaFile{File: f, fs: q, name: name, append: flag&os.O_APPEND != 0}, nil
}

func (q *quotaFS) Remove(ctx context.Context, name string, recursive bool) error {
	freed := q.treeSize(ctx, name)
	if err := q.FileSystem.Remove(ctx, name, recursive); err != nil {
		return err
	}
	q.release(freed)
	return nil
}

func (q *quotaFS) Rename(ctx context.Context, oldName, newName string) error {
	var freed int64
	if gbfs.Resolve(q.Getwd(), oldName) != gbfs.Resolve(q.Getwd(), newName) {
		if info, err := q.Lstat(ctx, newName); err == nil && info.Mode().IsRegular() {
			freed = info.Size()
		}
	}
	if err := q.FileSystem.Rename(ctx, oldName, newName); err != nil {
		return err
	}
	q.release(freed)
	return nil
}

func (q *quotaFS) Link(_ context.Context, oldName, newName string) error {
	return &os.LinkError{Op: "link", Old: oldName, New: newName, Err: syscall.EPERM}
}

func (q *quotaFS) treeSize(ctx context.Context, name string) int64 {
	info, err := q.Lstat(ctx, name)
	if err != nil {
		return 0
	}
	if info.Mode().IsRegular() {
		return info.Size()
	}
	if !info.IsDir() {
		return 0
	}
	entries, err := q.ReadDir(ctx, name)
	if err != nil {
		return 0
	}
	var total int64
	for _, entry := range entries {
		total += q.treeSize(ctx, path.Join(gbfs.Resolve(q.Getwd(), name), entry.Name()))
	}
	return total
}

func (q *quotaFS) release(n int64) {
	if n == 0 {
		return
	}
	q.mu.Lock()
	q.releaseLocked(n)
	q.mu.Unlock()
}

func (q *quotaFS) releaseLocked(n int64) {
	q.used = max(q.used-n, 0)
}

type quotaFile struct {
	gbfs.File
	fs     *quotaFS
	name   string
	append bool
	offset int64
}

func (f *quotaFile) Read(p []byte) (int, error) {
	n, err := f.File.Read(p)
	f.fs.mu.Lock()
	f.offset += int64(n)
	f.fs.mu.Unlock()
	return n, err
}

func (f *quotaFile) Write(p []byte) (int, error) {
	q := f.fs
	q.mu.Lock()
	defer q.mu.Unlock()
	before := f.size()
	if f.append {
		f.offset = before
	}
	if q.used+max(f.offset+int64(len(p))-before, 0) > q.limit {
		q.exceeded.Store(true)
		return 0, &os.PathError{Op: "write", Path: f.name, Err: syscall.ENOSPC}
	}
	n, err := f.File.Write(p)
	f.offset += int64(n)
	q.used += max(f.size()-before, 0)
	return n, err
}

func (f *quotaFile) size() int64 {
	info, err := f.Stat()
	if err != nil {
		return 0
	}
	return info.Size()
}
