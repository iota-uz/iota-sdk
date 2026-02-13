package assets

import (
	"embed"
	"io/fs"
)

// embeddedFS contains both optional built assets (dist/) and an always-available
// fallback asset bundle (fallback/). Dist assets are selected when they include
// a Vite manifest; otherwise fallback assets are used.
//
//go:embed all:dist all:fallback
var embeddedFS embed.FS

// AppletFS returns the filesystem that should be used for serving BiChat assets.
// Preference order:
// 1. Built assets from dist/ when a manifest exists
// 2. Fallback assets from fallback/
func AppletFS() fs.FS {
	if dist, err := fs.Sub(embeddedFS, "dist"); err == nil {
		if _, statErr := fs.Stat(dist, ".vite/manifest.json"); statErr == nil {
			return dist
		}
	}

	fallback, err := fs.Sub(embeddedFS, "fallback")
	if err == nil {
		return fallback
	}

	// This should be unreachable because fallback is embedded in-source.
	return emptyFS{}
}

type emptyFS struct{}

func (emptyFS) Open(string) (fs.File, error) {
	return nil, fs.ErrNotExist
}
