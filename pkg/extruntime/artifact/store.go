package artifact

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Kind classifies a stored archive.
type Kind string

const (
	KindSource Kind = "source"
	KindBuild  Kind = "build"
)

const (
	opPut  = serrors.Op("extruntime.artifact.Store.Put")
	opGet  = serrors.Op("extruntime.artifact.Store.Get")
	opDir  = serrors.Op("extruntime.artifact.Store.Dir")
	opOpen = serrors.Op("extruntime.artifact.Store.Open")
)

// ArtifactStore serves files of an immutable build artifact by content hash.
// The UI asset route reads client chunks through it.
type ArtifactStore interface {
	Open(hash, path string) (io.ReadCloser, error)
}

// Execer is the write side shared by pgx.Tx and pools.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

var hashPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)

// ValidHash reports whether h is safe to use as a directory name.
func ValidHash(h string) bool { return hashPattern.MatchString(h) && !strings.Contains(h, "..") }

// Store keeps archives in extruntime.artifacts and a materialized copy of
// each build tree at <cacheDir>/<hash>/..., which workers import from.
type Store struct {
	db       *pgxpool.Pool
	cacheDir string

	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

var _ ArtifactStore = (*Store)(nil)

// NewStore creates the cache directory.
func NewStore(db *pgxpool.Pool, cacheDir string) (*Store, error) {
	if db == nil || cacheDir == "" {
		return nil, serrors.New(serrors.Invalid, "artifact store: database and cache directory are required")
	}
	abs, err := filepath.Abs(cacheDir)
	if err != nil {
		return nil, serrors.Wrap(opDir, err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, serrors.Wrap(opDir, err)
	}
	return &Store{db: db, cacheDir: abs, locks: map[string]*sync.Mutex{}}, nil
}

// CacheDir is the absolute materialization root.
func (s *Store) CacheDir() string { return s.cacheDir }

// Put stores an archive under its hash. Storing the same hash again is a
// no-op; rows are immutable. q is a pool or the caller's transaction.
func (s *Store) Put(ctx context.Context, q Execer, hash string, kind Kind, archive []byte) error {
	if !ValidHash(hash) {
		return serrors.New(serrors.Invalid, "artifact hash "+hash+" is not valid").WithOp(opPut)
	}
	if q == nil {
		q = poolExec{s.db}
	}
	if _, err := q.Exec(ctx, `INSERT INTO extruntime.artifacts (hash, kind, content, size) VALUES ($1, $2, $3, $4)
		ON CONFLICT (hash) DO NOTHING`, hash, string(kind), archive, int64(len(archive))); err != nil {
		return serrors.FromDB(opPut, err)
	}
	return nil
}

// Archive returns the stored tar.
func (s *Store) Archive(ctx context.Context, hash string) ([]byte, error) {
	var content []byte
	err := s.db.QueryRow(ctx, `SELECT content FROM extruntime.artifacts WHERE hash = $1`, hash).Scan(&content)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, serrors.New(serrors.NotFound, "artifact "+hash+" not found").WithOp(opGet)
	}
	if err != nil {
		return nil, serrors.FromDB(opGet, err)
	}
	return content, nil
}

// Tree returns the files of a stored archive.
func (s *Store) Tree(ctx context.Context, hash string) (build.Tree, error) {
	b, err := s.Archive(ctx, hash)
	if err != nil {
		return nil, err
	}
	return Unpack(b)
}

func (s *Store) lock(hash string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.locks[hash]
	if !ok {
		l = &sync.Mutex{}
		s.locks[hash] = l
	}
	return l
}

// Dir materializes a build artifact and returns its directory. The directory
// appears atomically (rename), so a directory that exists is complete.
func (s *Store) Dir(ctx context.Context, hash string) (string, error) {
	if !ValidHash(hash) {
		return "", serrors.New(serrors.Invalid, "artifact hash "+hash+" is not valid").WithOp(opDir)
	}
	dir := filepath.Join(s.cacheDir, hash)
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		return dir, nil
	}
	l := s.lock(hash)
	l.Lock()
	defer l.Unlock()
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		return dir, nil
	}
	tree, err := s.Tree(ctx, hash)
	if err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(s.cacheDir, ".tmp-"+hash[:8]+"-")
	if err != nil {
		return "", serrors.Wrap(opDir, err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tmp)
		}
	}()
	for p, content := range tree {
		clean, err := CleanPath(p)
		if err != nil {
			return "", serrors.Wrap(opDir, err)
		}
		target := filepath.Join(tmp, filepath.FromSlash(clean))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", serrors.Wrap(opDir, err)
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			return "", serrors.Wrap(opDir, err)
		}
	}
	if err := os.Rename(tmp, dir); err != nil {
		if st, serr := os.Stat(dir); serr == nil && st.IsDir() {
			return dir, nil
		}
		return "", serrors.Wrap(opDir, err)
	}
	cleanup = false
	return dir, nil
}

// OpenContext opens one file of a build artifact.
func (s *Store) OpenContext(ctx context.Context, hash, p string) (io.ReadCloser, error) {
	f, err := s.OpenFile(ctx, hash, p)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// OpenFile opens one file of a build artifact as a seekable file.
func (s *Store) OpenFile(ctx context.Context, hash, p string) (*os.File, error) {
	clean, err := CleanPath(p)
	if err != nil {
		return nil, serrors.Wrap(opOpen, err)
	}
	dir, err := s.Dir(ctx, hash)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(dir, filepath.FromSlash(clean)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, serrors.New(serrors.NotFound, fmt.Sprintf("artifact %s has no file %s", hash, clean)).WithOp(opOpen)
	}
	if err != nil {
		return nil, serrors.Wrap(opOpen, err)
	}
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		_ = f.Close()
		return nil, serrors.New(serrors.NotFound, fmt.Sprintf("artifact %s has no file %s", hash, clean)).WithOp(opOpen)
	}
	return f, nil
}

// Open implements ArtifactStore.
func (s *Store) Open(hash, p string) (io.ReadCloser, error) {
	return s.OpenContext(context.Background(), hash, p)
}

// ReadFile returns one file of a build artifact.
func (s *Store) ReadFile(ctx context.Context, hash, p string) ([]byte, error) {
	r, err := s.OpenContext(ctx, hash, p)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, serrors.Wrap(opOpen, err)
	}
	return buf.Bytes(), nil
}

type poolExec struct{ db *pgxpool.Pool }

func (p poolExec) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return p.db.Exec(ctx, sql, args...)
}
