// Package bundle serves a client bundle the automations module embeds: a Solid screen with the
// standard client host, built by its web/scripts/build-bundle.mjs into a dist directory with a
// bundle.json, so an application that lists the module serves the page without merging the feature into
// its own client bundle.
package bundle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/clienthost"
)

// ReleasePlaceholder is the SDK release a bundle is built with; the handler writes the serving release
// over it.
const ReleasePlaceholder = "0.0.0-iota-automations-release"

type bundleFile struct {
	SDKCommit          string   `json:"sdkCommit"`
	ReleasePlaceholder string   `json:"releasePlaceholder"`
	ProtocolVersion    string   `json:"protocolVersion"`
	Entry              string   `json:"entry"`
	Styles             []string `json:"styles"`
}

// Bundle is a client bundle: the host manifest that renders its screen and the handler that serves its
// files under its assets path.
type Bundle struct {
	Manifest clienthost.Manifest
	Handler  http.Handler
}

// Load reads the bundle in files (a dist directory) served as SDK release under assetsPath:
// <assetsPath>/<release>/assets/<file>. The bundle is built with a release placeholder (the client host
// checks the release of the page against the one it carries), and the release is written over it when a
// file is served. A release candidate that only stamps a new version therefore serves a matching bundle
// without a rebuild; the release is part of the URL, so a browser never mixes the files of two releases
// from its cache.
func Load(files fs.FS, assetsPath, pkg, release string) (Bundle, error) {
	raw, err := fs.ReadFile(files, "bundle.json")
	if err != nil {
		return Bundle{}, fmt.Errorf("%s bundle: %w (build it with web/scripts/build-bundle.mjs)", pkg, err)
	}
	var b bundleFile
	if err := json.Unmarshal(raw, &b); err != nil {
		return Bundle{}, fmt.Errorf("%s bundle: bundle.json: %w", pkg, err)
	}
	if b.ReleasePlaceholder == "" || b.Entry == "" {
		return Bundle{}, fmt.Errorf("%s bundle: bundle.json has no entry or release placeholder: rebuild it", pkg)
	}
	base := path.Join(assetsPath, release)
	m := clienthost.Manifest{
		Package: pkg, PackageVersion: release,
		SDKCommit: b.SDKCommit, SDKReleaseVersion: release, ProtocolVersion: b.ProtocolVersion,
		Entry: path.Join(base, b.Entry),
	}
	for _, s := range b.Styles {
		m.Styles = append(m.Styles, path.Join(base, s))
	}
	return Bundle{Manifest: m, Handler: &assetHandler{files: files, prefix: base + "/", release: release, placeholder: []byte(b.ReleasePlaceholder)}}, nil
}

// assetHandler serves <assetsPath>/<release>/assets/<file> with the release written in; names carry
// content hashes and the URL the release, so they are cached for good.
type assetHandler struct {
	files       fs.FS
	prefix      string
	release     string
	placeholder []byte
	mu          sync.Mutex
	cache       map[string][]byte
}

func (h *assetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rest, ok := strings.CutPrefix(r.URL.Path, h.prefix)
	if !ok || !strings.HasPrefix(rest, "assets/") || strings.Contains(rest, "..") {
		http.NotFound(w, r)
		return
	}
	body, err := h.file(rest)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if ct := mime.TypeByExtension(path.Ext(rest)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(body)
}

func (h *assetHandler) file(name string) ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if b, ok := h.cache[name]; ok {
		return b, nil
	}
	b, err := fs.ReadFile(h.files, name)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(name, ".js") {
		b = bytes.ReplaceAll(b, h.placeholder, []byte(h.release))
	}
	if h.cache == nil {
		h.cache = map[string][]byte{}
	}
	h.cache[name] = b
	return b, nil
}
