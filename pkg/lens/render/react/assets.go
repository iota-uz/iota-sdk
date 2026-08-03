// Package react embeds and serves the Lens React custom element runtime.
package react

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sort"
	"strings"
)

const DefaultAssetBasePath = "/assets/lens"

// AssetsDirEnv points the runtime at a Vite build directory on disk instead of
// the embedded bundle. The bundle is embedded into the binary, so without this
// a rebuilt runtime is invisible until the Go binary itself is rebuilt and
// restarted. Set it to <sdk>/web/lens/dist while iterating on the runtime; the
// manifest is then re-read per request, so a page reload is enough to see a
// fresh `vite build`. Leave it unset in production.
const AssetsDirEnv = "LENS_ASSETS_DIR"

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

// assetSource resolves the runtime bundle. The embedded source resolves once at
// init and fails loudly there, which is what catches a binary built without a
// bundle. The on-disk source resolves per call so a rebuild is picked up by a
// page reload.
type assetSource struct {
	fsys   fs.FS
	live   bool
	bundle AssetBundle
}

var source = newAssetSource(os.Getenv(AssetsDirEnv))

func newAssetSource(dir string) *assetSource {
	if dir = strings.TrimSpace(dir); dir != "" {
		return &assetSource{fsys: os.DirFS(dir), live: true}
	}
	dist, err := fs.Sub(embeddedAssets, "dist")
	if err != nil {
		panic(fmt.Sprintf("lens react: open embedded dist: %v", err))
	}
	return &assetSource{fsys: dist, bundle: mustLoadAssetBundle()}
}

func (s *assetSource) assets() AssetBundle {
	if !s.live {
		return s.bundle
	}
	data, err := fs.ReadFile(s.fsys, ".vite/manifest.json")
	if err == nil {
		var bundle AssetBundle
		if bundle, err = parseAssetBundle(data); err == nil {
			return bundle
		}
	}
	// Dev only: a missing or half-written manifest means "run just lens build",
	// not "take the process down".
	slog.Error("lens react: reading the on-disk Vite manifest", "dir", os.Getenv(AssetsDirEnv), "error", err)
	return AssetBundle{}
}

func DistFS() fs.FS {
	return source.fsys
}

func Assets() AssetBundle {
	bundle := source.assets()
	return AssetBundle{
		Entry:       bundle.Entry,
		Revision:    bundle.Revision,
		Stylesheets: append([]string(nil), bundle.Stylesheets...),
	}
}

func mustLoadAssetBundle() AssetBundle {
	data, err := embeddedAssets.ReadFile("dist/.vite/manifest.json")
	if err != nil {
		panic(fmt.Sprintf("lens react: read Vite manifest: %v", err))
	}
	return loadAssetBundle(data)
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
		return AssetBundle{}, errors.New("Vite manifest has no index.html entry")
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
