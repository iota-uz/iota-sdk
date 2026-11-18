package ui

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
)

// ClientManifestPath is the client build's own manifest inside an artifact:
// the entry chunk, lazy chunks, stylesheets and shared modules of one
// package version (web/runtime/src/build/pipeline.ts, schema iota.client/1).
const ClientManifestPath = ClientDir + "manifest.json"

// maxClientManifest bounds what is read of a client manifest.
const maxClientManifest = 1 << 20

// versionStyles reads the stylesheets a version's client build recorded.
// Artifacts are immutable, so a successful answer (including "none") is kept
// for the life of the process; a failed read is not, and the version then
// renders without styles until the next index rebuild retries.
type versionStyles struct {
	mu   sync.Mutex
	byID map[string][]string
}

func (v *versionStyles) get(hash string) ([]string, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	css, ok := v.byID[hash]
	return css, ok
}

func (v *versionStyles) put(hash string, css []string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.byID == nil {
		v.byID = map[string][]string{}
	}
	v.byID[hash] = css
}

// stylesOf returns the stylesheet paths ("client/tailwind.css", ...) of an
// artifact, in link order. An artifact without a client manifest has none.
func (r *Runtime) stylesOf(ctx context.Context, hash string) []string {
	if css, ok := r.styles.get(hash); ok {
		return css
	}
	f, err := r.cfg.Artifacts.Open(ctx, hash, ClientManifestPath)
	if errors.Is(err, ErrArtifactNotFound) {
		r.styles.put(hash, nil)
		return nil
	}
	if err != nil {
		r.cfg.Logger.WithError(err).WithField("artifact", hash).Warn("extruntime ui: open client manifest")
		return nil
	}
	defer func() { _ = f.Close() }()
	raw, err := io.ReadAll(io.LimitReader(f, maxClientManifest))
	if err != nil {
		r.cfg.Logger.WithError(err).WithField("artifact", hash).Warn("extruntime ui: read client manifest")
		return nil
	}
	var doc struct {
		CSS []string `json:"css"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		r.cfg.Logger.WithError(err).WithField("artifact", hash).Warn("extruntime ui: decode client manifest")
		return nil
	}
	var css []string
	for _, p := range doc.CSS {
		// Only stylesheets of the client subtree are ever served.
		if clean, ok := cleanAssetPath(p); ok && strings.HasPrefix(clean, ClientDir) && strings.HasSuffix(clean, ".css") {
			css = append(css, clean)
		}
	}
	r.styles.put(hash, css)
	return css
}

// withStyles fills the stylesheets of the packages whose source did not name
// them. The snapshot is the source's: it is copied, never modified.
func (r *Runtime) withStyles(ctx context.Context, snap *Snapshot) *Snapshot {
	if snap == nil {
		return nil
	}
	out := *snap
	out.Packages = make([]ActivePackage, len(snap.Packages))
	for i, p := range snap.Packages {
		if p.Styles == nil {
			p.Styles = r.stylesOf(ctx, p.ArtifactHash)
		}
		out.Packages[i] = p
	}
	return &out
}
