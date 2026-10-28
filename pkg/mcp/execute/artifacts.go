package execute

import (
	"context"
	"io"
	"io/fs"
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

// collectArtifacts stores regular files from ./out. The directory is written
// by untrusted code, so links are skipped and every file is re-checked after
// opening to defeat a swap between the listing and the read.
func (r *Runner) collectArtifacts(ctx context.Context, dir string) []Artifact {
	out := []Artifact{}
	if r.cfg.Artifacts == nil {
		return out
	}
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || len(out) >= r.cfg.MaxArtifacts {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > r.cfg.MaxArtifactBytes {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer func() { _ = f.Close() }()
		opened, err := f.Stat()
		if err != nil || !os.SameFile(info, opened) {
			return nil
		}
		name, _ := filepath.Rel(dir, path)
		mimeType := mime.TypeByExtension(filepath.Ext(name))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		ref, err := r.cfg.Artifacts.Save(ctx, ArtifactFile{
			Name: name, MimeType: mimeType, Size: opened.Size(),
			Content: io.LimitReader(f, r.cfg.MaxArtifactBytes),
		})
		if err != nil {
			return nil
		}
		out = append(out, Artifact{FileRef: ref, Name: name, MimeType: mimeType, SizeBytes: opened.Size()})
		return nil
	})
	return out
}
