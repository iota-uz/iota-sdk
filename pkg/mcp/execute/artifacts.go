package execute

import (
	"bytes"
	"context"
	"io"
	"mime"
	"os"
	"path/filepath"
)

type Artifact struct {
	FileRef   string `json:"file_ref"`
	Name      string `json:"name"`
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
}

type ArtifactFile struct {
	Name     string
	MimeType string
	Size     int64
	Content  io.Reader
}

// ArtifactStore persists run outputs and returns a reference the caller can
// download through the application's own access-checked file endpoint.
type ArtifactStore interface {
	Save(ctx context.Context, f ArtifactFile) (fileRef string, err error)
}

// artifact reads the validated descriptor once, within the size limit, and
// stores exactly those bytes: code still running in the kernel may change the
// file, so the declared size is the length of what was read.
func (r *Runner) artifact(ctx context.Context, name string, f *os.File) (Artifact, bool) {
	body, err := io.ReadAll(io.LimitReader(f, r.cfg.MaxArtifactBytes+1))
	if err != nil || int64(len(body)) > r.cfg.MaxArtifactBytes {
		return Artifact{}, false
	}
	size := int64(len(body))
	mimeType := mime.TypeByExtension(filepath.Ext(name))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	ref, err := r.cfg.Artifacts.Save(ctx, ArtifactFile{
		Name: name, MimeType: mimeType, Size: size, Content: bytes.NewReader(body),
	})
	if err != nil {
		return Artifact{}, false
	}
	return Artifact{FileRef: ref, Name: name, MimeType: mimeType, SizeBytes: size}, true
}
