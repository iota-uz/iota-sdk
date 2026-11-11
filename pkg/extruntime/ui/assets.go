package ui

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"
)

// Public URL layout of the runtime UI.
const (
	// AssetsPrefix is the immutable asset route: AssetsPrefix + <artifactHash> + "/" + <path>.
	AssetsPrefix = "/_ext/assets/"
	// RPCPath is the browser transport for useCommand / useQuery.
	RPCPath = "/_ext/rpc"
)

// ErrArtifactNotFound is returned by an ArtifactStore for an unknown hash or
// path.
var ErrArtifactNotFound = errors.New("extruntime ui: artifact not found")

// ArtifactFile is one file of an artifact.
type ArtifactFile interface {
	io.ReadSeeker
	io.Closer
}

// ArtifactStore serves the files of immutable, content-addressed artifacts.
// The same hash and path always yield the same bytes.
type ArtifactStore interface {
	Open(ctx context.Context, hash, filePath string) (ArtifactFile, error)
}

// ClientDir is the only subtree of an artifact the browser may load: the
// bundled client chunks and their assets. The server bundle, contracts,
// manifests and sources of the same artifact are never served.
const ClientDir = "client/"

var hashPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

// cleanAssetPath returns the normalized relative path or false when it could
// escape the artifact (".." segments, absolute or backslash paths, NUL).
func cleanAssetPath(p string) (string, bool) {
	if p == "" || strings.ContainsAny(p, "\\\x00") || strings.HasPrefix(p, "/") {
		return "", false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." || seg == "." || seg == "" {
			return "", false
		}
	}
	return path.Clean(p), true
}

func contentTypeOf(p string) string {
	switch strings.ToLower(path.Ext(p)) {
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json", ".map":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".woff2":
		return "font/woff2"
	}
	if t := mime.TypeByExtension(path.Ext(p)); t != "" {
		return t
	}
	return "application/octet-stream"
}

// assetHandler serves AssetsPrefix routes. The mux variable names are "hash"
// and "path".
func (c *Controller) assetHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, AssetsPrefix)
	hash, filePath, ok := strings.Cut(rest, "/")
	if !ok || !hashPattern.MatchString(hash) {
		http.NotFound(w, r)
		return
	}
	clean, ok := cleanAssetPath(filePath)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !c.authenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !strings.HasPrefix(clean, ClientDir) {
		http.NotFound(w, r)
		return
	}
	ix, err := c.Index(r.Context())
	if err != nil {
		c.cfg.Logger.WithError(err).Error("extruntime ui: index for asset")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !ix.ServesArtifact(hash) {
		http.NotFound(w, r) // not (or no longer) the artifact of an activated version
		return
	}
	etag := `"` + hash + `:` + clean + `"`
	if r.Header.Get("If-None-Match") == etag {
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusNotModified)
		return
	}
	f, err := c.cfg.Artifacts.Open(r.Context(), hash, clean)
	if errors.Is(err, ErrArtifactNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		c.cfg.Logger.WithError(err).WithField("artifact", hash).Error("extruntime ui: open artifact")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = f.Close() }()
	h := w.Header()
	h.Set("Content-Type", contentTypeOf(clean))
	h.Set("X-Content-Type-Options", "nosniff")
	// The path contains the content hash, so the bytes never change. The
	// session guards access, hence private.
	h.Set("Cache-Control", "private, max-age=31536000, immutable")
	h.Set("ETag", etag)
	http.ServeContent(w, r, clean, time.Time{}, f)
}
