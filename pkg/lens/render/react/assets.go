// Package react provides the legacy Lens custom-element compatibility adapter.
package react

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"sync"
)

const DefaultAssetBasePath = "/assets/lens"

// AssetsDirEnv points the legacy custom-element adapter at a generated Vite
// build directory. Direct React hosts consume @iota-uz/lens-web and do not need
// this directory or a Node installation when compiling the Go SDK.
const AssetsDirEnv = "LENS_ASSETS_DIR"

// dist contains only a tracked placeholder in a clean clone. `just lens build`
// writes the ignored compatibility bundle into it before a legacy host builds
// its Go binary.
//
//go:embed all:dist
var embeddedAssets embed.FS

type AssetBundle struct {
	Entry string
	// Revision is a deploy-scoped namespace, not a replacement for Vite's
	// content hashes. It makes every HTML document and lazy chunk resolve within
	// one atomic embedded manifest while old processes may still serve traffic.
	Revision    string
	Stylesheets []string
}

type manifestEntry struct {
	File           string   `json:"file"`
	CSS            []string `json:"css"`
	Imports        []string `json:"imports"`
	DynamicImports []string `json:"dynamicImports"`
}

const compatibilityAssetsHelp = "lens react compatibility runtime assets are unavailable: direct React hosts must consume @iota-uz/lens-web; legacy Go custom-element hosts must run `just lens build` before building or set LENS_ASSETS_DIR to a built Lens dist"

// assetSource resolves the optional compatibility bundle. It deliberately does
// no manifest I/O during package initialization: direct-package consumers can
// compile a clean SDK clone without Node. Embedded assets resolve once on first
// legacy use; a development directory resolves per call so rebuilds are visible
// on page reload.
type assetSource struct {
	fsys   fs.FS
	live   bool
	dir    string
	bundle AssetBundle
	err    error
	loaded bool
	mu     sync.Mutex
}

var source = newAssetSource(os.Getenv(AssetsDirEnv))

func newAssetSource(dir string) *assetSource {
	if dir = strings.TrimSpace(dir); dir != "" {
		return &assetSource{fsys: os.DirFS(dir), live: true, dir: dir}
	}
	dist, err := fs.Sub(embeddedAssets, "dist")
	if err != nil {
		panic(fmt.Sprintf("lens react: open embedded dist: %v", err))
	}
	return &assetSource{fsys: dist, dir: "embedded pkg/lens/render/react/dist"}
}

func (s *assetSource) assets() (AssetBundle, error) {
	if s.live {
		return s.load()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loaded {
		s.bundle, s.err = s.load()
		s.loaded = true
	}
	return s.bundle, s.err
}

func (s *assetSource) load() (AssetBundle, error) {
	data, err := fs.ReadFile(s.fsys, ".vite/manifest.json")
	if err != nil {
		return AssetBundle{}, fmt.Errorf("%s: read .vite/manifest.json: %w", s.dir, err)
	}
	bundle, err := parseAssetBundle(data)
	if err != nil {
		return AssetBundle{}, fmt.Errorf("%s: %w", s.dir, err)
	}
	return bundle, nil
}

func DistFS() fs.FS {
	return source.fsys
}

func Assets() AssetBundle {
	bundle, err := source.assets()
	if err != nil {
		panic(compatibilityAssetsError(err))
	}
	return AssetBundle{
		Entry:       bundle.Entry,
		Revision:    bundle.Revision,
		Stylesheets: append([]string(nil), bundle.Stylesheets...),
	}
}
func compatibilityAssetsError(err error) error {
	return fmt.Errorf("%s: %w", compatibilityAssetsHelp, err)
}

func loadAssetBundle(data []byte) AssetBundle {
	bundle, err := parseAssetBundle(data)
	if err != nil {
		panic(fmt.Sprintf("lens react: %v", err))
	}
	return bundle
}

func parseAssetBundle(data []byte) (AssetBundle, error) {
	manifest := map[string]manifestEntry{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return AssetBundle{}, fmt.Errorf("decode Vite manifest: %w", err)
	}

	entry, ok := manifest["index.html"]
	if !ok || entry.File == "" {
		return AssetBundle{}, errors.New("no index.html entry in the Vite manifest")
	}

	stylesheetSet := make(map[string]struct{})
	visited := make(map[string]struct{})
	var walk func(string)
	walk = func(key string) {
		if _, ok := visited[key]; ok {
			return
		}
		visited[key] = struct{}{}
		item, ok := manifest[key]
		if !ok {
			return
		}
		for _, stylesheet := range item.CSS {
			stylesheetSet[stylesheet] = struct{}{}
		}
		for _, imported := range item.Imports {
			walk(imported)
		}
		for _, imported := range item.DynamicImports {
			walk(imported)
		}
	}
	walk("index.html")

	stylesheets := make([]string, 0, len(stylesheetSet))
	for stylesheet := range stylesheetSet {
		stylesheets = append(stylesheets, stylesheet)
	}
	sort.Strings(stylesheets)

	digest := sha256.Sum256(data)
	return AssetBundle{
		Entry:       entry.File,
		Revision:    fmt.Sprintf("%x", digest[:6]),
		Stylesheets: stylesheets,
	}, nil
}
