package presentation

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
)

//go:embed assets/dist/.vite/manifest.json
var manifestFS embed.FS

type ViteManifest map[string]ViteManifestEntry

type ViteManifestEntry struct {
	File    string   `json:"file"`
	Src     string   `json:"src,omitempty"`
	IsEntry bool     `json:"isEntry,omitempty"`
	CSS     []string `json:"css,omitempty"`
	Imports []string `json:"imports,omitempty"`
}

type ViteAssets struct {
	CSSFiles []string
	JSFiles  []string
}

func LoadViteAssets(basePath string) (*ViteAssets, error) {
	manifestBytes, err := manifestFS.ReadFile("assets/dist/.vite/manifest.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest ViteManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	assets := &ViteAssets{
		CSSFiles: make([]string, 0),
		JSFiles:  make([]string, 0),
	}

	for _, entry := range manifest {
		if !entry.IsEntry {
			continue
		}

		for _, cssFile := range entry.CSS {
			assets.CSSFiles = append(assets.CSSFiles, filepath.Join(basePath, cssFile))
		}

		if entry.File != "" {
			assets.JSFiles = append(assets.JSFiles, filepath.Join(basePath, entry.File))
		}
	}

	return assets, nil
}
