// Package artifact stores immutable source and build trees as content
// addressed tar archives in PostgreSQL and materializes them into a local
// cache directory for the worker pool and the UI asset route.
package artifact

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Limits bound what an untrusted archive or tree may contain.
const (
	MaxFileBytes = 8 << 20
	MaxTreeBytes = 128 << 20
	MaxFiles     = 5000
	MaxPathBytes = 255
)

const (
	opPack   = serrors.Op("extruntime.artifact.Pack")
	opUnpack = serrors.Op("extruntime.artifact.Unpack")
)

// CleanPath validates and normalizes a relative slash path of a tree. It
// rejects absolute paths, parent segments, empty segments, backslashes,
// control characters and the reserved .git and node_modules roots.
func CleanPath(p string) (string, error) {
	p = strings.TrimPrefix(p, "./")
	if p == "" || len(p) > MaxPathBytes {
		return "", serrors.New(serrors.Invalid, fmt.Sprintf("invalid file path %q", p))
	}
	if strings.HasPrefix(p, "/") || strings.Contains(p, "\\") || strings.HasSuffix(p, "/") {
		return "", serrors.New(serrors.Invalid, fmt.Sprintf("invalid file path %q", p))
	}
	for _, r := range p {
		if r < 0x20 || r == 0x7f {
			return "", serrors.New(serrors.Invalid, fmt.Sprintf("invalid file path %q", p))
		}
	}
	segments := strings.Split(p, "/")
	for _, s := range segments {
		if s == "" || s == "." || s == ".." {
			return "", serrors.New(serrors.Invalid, fmt.Sprintf("invalid file path %q", p))
		}
	}
	if segments[0] == ".git" || segments[0] == "node_modules" {
		return "", serrors.New(serrors.Invalid, fmt.Sprintf("path %q is in a reserved directory", p))
	}
	if path.Clean(p) != p {
		return "", serrors.New(serrors.Invalid, fmt.Sprintf("invalid file path %q", p))
	}
	return p, nil
}

// Validate checks every path and the size limits of a tree.
func Validate(t build.Tree) error {
	var total int64
	if len(t) > MaxFiles {
		return serrors.New(serrors.Invalid, fmt.Sprintf("tree has %d files, the limit is %d", len(t), MaxFiles))
	}
	for p, c := range t {
		if _, err := CleanPath(p); err != nil {
			return err
		}
		if len(c) > MaxFileBytes {
			return serrors.New(serrors.Invalid, fmt.Sprintf("file %q is larger than %d bytes", p, MaxFileBytes))
		}
		total += int64(len(c))
	}
	if total > MaxTreeBytes {
		return serrors.New(serrors.Invalid, fmt.Sprintf("tree is larger than %d bytes", MaxTreeBytes))
	}
	return nil
}

// HashBytes is the hex sha256 of content.
func HashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Paths returns the sorted paths of a tree.
func Paths(t build.Tree) []string {
	out := make([]string, 0, len(t))
	for p := range t {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// TreeHash is a canonical identity of a tree: the sha256 over its sorted
// "path NUL file-hash" lines. Import followed by export yields the same hash.
func TreeHash(t build.Tree) string {
	h := sha256.New()
	for _, p := range Paths(t) {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(HashBytes(t[p])))
		_, _ = h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Pack renders a tree as a deterministic tar archive (sorted entries, fixed
// mode, zero times and owners), so equal trees give byte-equal archives.
func Pack(t build.Tree) ([]byte, error) {
	if err := Validate(t); err != nil {
		return nil, serrors.Wrap(opPack, err)
	}
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	for _, p := range Paths(t) {
		hdr := &tar.Header{Typeflag: tar.TypeReg, Name: p, Mode: 0o644, Size: int64(len(t[p])), Format: tar.FormatPAX}
		if err := w.WriteHeader(hdr); err != nil {
			return nil, serrors.Wrap(opPack, err)
		}
		if _, err := w.Write(t[p]); err != nil {
			return nil, serrors.Wrap(opPack, err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, serrors.Wrap(opPack, err)
	}
	return buf.Bytes(), nil
}

// Unpack reads a tar archive into a tree. Only regular files and directories
// are accepted: links, devices and fifos are refused.
func Unpack(data []byte) (build.Tree, error) {
	return UnpackReader(bytes.NewReader(data))
}

// UnpackReader is Unpack over a stream.
func UnpackReader(r io.Reader) (build.Tree, error) {
	tr := tar.NewReader(r)
	out := build.Tree{}
	var total int64
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, serrors.New(serrors.Invalid, "invalid tar archive: "+err.Error()).WithOp(opUnpack)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
		default:
			return nil, serrors.New(serrors.Invalid, fmt.Sprintf("archive entry %q is not a regular file", hdr.Name)).WithOp(opUnpack)
		}
		p, err := CleanPath(hdr.Name)
		if err != nil {
			return nil, serrors.Wrap(opUnpack, err)
		}
		if hdr.Size > MaxFileBytes {
			return nil, serrors.New(serrors.Invalid, fmt.Sprintf("file %q is larger than %d bytes", p, MaxFileBytes)).WithOp(opUnpack)
		}
		content, err := io.ReadAll(io.LimitReader(tr, MaxFileBytes+1))
		if err != nil || len(content) > MaxFileBytes {
			return nil, serrors.New(serrors.Invalid, fmt.Sprintf("cannot read file %q", p)).WithOp(opUnpack)
		}
		if _, dup := out[p]; dup {
			return nil, serrors.New(serrors.Invalid, fmt.Sprintf("archive contains %q twice", p)).WithOp(opUnpack)
		}
		total += int64(len(content))
		if len(out) >= MaxFiles || total > MaxTreeBytes {
			return nil, serrors.New(serrors.Invalid, "archive exceeds the tree size limits").WithOp(opUnpack)
		}
		out[p] = content
	}
	return out, nil
}

// PackZip renders a tree as a deterministic zip archive.
func PackZip(t build.Tree) ([]byte, error) {
	if err := Validate(t); err != nil {
		return nil, serrors.Wrap(opPack, err)
	}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, p := range Paths(t) {
		f, err := w.CreateHeader(&zip.FileHeader{Name: p, Method: zip.Deflate})
		if err != nil {
			return nil, serrors.Wrap(opPack, err)
		}
		if _, err := f.Write(t[p]); err != nil {
			return nil, serrors.Wrap(opPack, err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, serrors.Wrap(opPack, err)
	}
	return buf.Bytes(), nil
}

// UnpackZip reads a zip archive into a tree with the same rules as Unpack.
func UnpackZip(data []byte) (build.Tree, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, serrors.New(serrors.Invalid, "invalid zip archive: "+err.Error()).WithOp(opUnpack)
	}
	out := build.Tree{}
	var total int64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !f.FileInfo().Mode().IsRegular() {
			return nil, serrors.New(serrors.Invalid, fmt.Sprintf("archive entry %q is not a regular file", f.Name)).WithOp(opUnpack)
		}
		p, err := CleanPath(f.Name)
		if err != nil {
			return nil, serrors.Wrap(opUnpack, err)
		}
		if f.UncompressedSize64 > MaxFileBytes {
			return nil, serrors.New(serrors.Invalid, fmt.Sprintf("file %q is larger than %d bytes", p, MaxFileBytes)).WithOp(opUnpack)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, serrors.Wrap(opUnpack, err)
		}
		content, err := io.ReadAll(io.LimitReader(rc, MaxFileBytes+1))
		_ = rc.Close()
		if err != nil || len(content) > MaxFileBytes {
			return nil, serrors.New(serrors.Invalid, fmt.Sprintf("cannot read file %q", p)).WithOp(opUnpack)
		}
		if _, dup := out[p]; dup {
			return nil, serrors.New(serrors.Invalid, fmt.Sprintf("archive contains %q twice", p)).WithOp(opUnpack)
		}
		total += int64(len(content))
		if len(out) >= MaxFiles || total > MaxTreeBytes {
			return nil, serrors.New(serrors.Invalid, "archive exceeds the tree size limits").WithOp(opUnpack)
		}
		out[p] = content
	}
	return out, nil
}
