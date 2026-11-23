package execute

import (
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

func (r *Runner) artifact(ctx context.Context, name string, f *os.File, size int64) (Artifact, bool) {
	mimeType := mime.TypeByExtension(filepath.Ext(name))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	ref, err := r.cfg.Artifacts.Save(ctx, ArtifactFile{
		Name: name, MimeType: mimeType, Size: size,
		Content: io.LimitReader(f, r.cfg.MaxArtifactBytes),
	})
	if err != nil {
		return Artifact{}, false
	}
	return Artifact{FileRef: ref, Name: name, MimeType: mimeType, SizeBytes: size}, true
}
