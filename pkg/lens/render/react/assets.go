// Package react embeds and serves the Lens React custom element runtime.
package react

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
)

const DefaultAssetBasePath = "/assets/lens"

//go:embed all:dist
var embeddedAssets embed.FS

type AssetBundle struct {
	Entry       string
	Stylesheets []string
}

type manifestEntry struct {
	File           string   `json:"file"`
	CSS            []string `json:"css"`
	Imports        []string `json:"imports"`
	DynamicImports []string `json:"dynamicImports"`
}

var productionAssets = mustLoadAssetBundle()

func DistFS() fs.FS {
	dist, err := fs.Sub(embeddedAssets, "dist")
	if err != nil {
		panic(fmt.Sprintf("lens react: open embedded dist: %v", err))
	}
	return dist
}

func Assets() AssetBundle {
	return AssetBundle{
		Entry:       productionAssets.Entry,
		Stylesheets: append([]string(nil), productionAssets.Stylesheets...),
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
	manifest := map[string]manifestEntry{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		panic(fmt.Sprintf("lens react: decode Vite manifest: %v", err))
	}

	entry, ok := manifest["index.html"]
	if !ok || entry.File == "" {
		panic("lens react: Vite manifest has no index.html entry")
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
	}
	walk("index.html")

	stylesheets := make([]string, 0, len(stylesheetSet))
	for stylesheet := range stylesheetSet {
		stylesheets = append(stylesheets, stylesheet)
	}
	sort.Strings(stylesheets)

	return AssetBundle{Entry: entry.File, Stylesheets: stylesheets}
}
