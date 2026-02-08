package applet

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
)

// ViteManifest represents a Vite build manifest.json structure
type ViteManifest map[string]ViteManifestEntry

// ViteManifestEntry represents a single entry in the Vite manifest
type ViteManifestEntry struct {
	File    string   `json:"file"`
	Src     string   `json:"src,omitempty"`
	IsEntry bool     `json:"isEntry,omitempty"`
	CSS     []string `json:"css,omitempty"`
	Imports []string `json:"imports,omitempty"`
}

// ResolvedAssets contains resolved asset paths from a manifest
type ResolvedAssets struct {
	CSSFiles []string
	JSFiles  []string
}

// loadManifest loads and parses a Vite manifest.json file from the filesystem
func loadManifest(manifestFS fs.FS, manifestPath string) (ViteManifest, error) {
	manifestBytes, err := fs.ReadFile(manifestFS, manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest at %q: %w", manifestPath, err)
	}

	var manifest ViteManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return manifest, nil
}

// resolveAssetsFromManifest resolves CSS and JS files from a Vite manifest
// based on the entrypoint file name.
func resolveAssetsFromManifest(
	manifest ViteManifest,
	entrypoint string,
	basePath string,
) (*ResolvedAssets, error) {
	assets := &ResolvedAssets{
		CSSFiles: make([]string, 0),
		JSFiles:  make([]string, 0),
	}

	// Find the entry in the manifest that matches the entrypoint
	var entry *ViteManifestEntry
	for key, e := range manifest {
		// Match by src (source file) or by key (which might be the entrypoint path)
		if e.Src == entrypoint || key == entrypoint {
			entry = &e
			break
		}
	}

	if entry == nil {
		return nil, fmt.Errorf("entrypoint %q not found in manifest", entrypoint)
	}

	// Collect CSS files
	for _, cssFile := range entry.CSS {
		assets.CSSFiles = append(assets.CSSFiles, path.Join(basePath, cssFile))
	}

	// Collect JS file
	if entry.File != "" {
		assets.JSFiles = append(assets.JSFiles, path.Join(basePath, entry.File))
	}

	// Also process imports (chunks) recursively
	processed := make(map[string]bool)
	var processEntry func(string)
	processEntry = func(key string) {
		if processed[key] {
			return
		}
		processed[key] = true

		e, exists := manifest[key]
		if !exists {
			return
		}

		// Add CSS files from imported chunks
		for _, cssFile := range e.CSS {
			cssPath := path.Join(basePath, cssFile)
			// Avoid duplicates
			found := false
			for _, existing := range assets.CSSFiles {
				if existing == cssPath {
					found = true
					break
				}
			}
			if !found {
				assets.CSSFiles = append(assets.CSSFiles, cssPath)
			}
		}

		// Process nested imports
		for _, imp := range e.Imports {
			processEntry(imp)
		}
	}

	// Process imports of the main entry
	for _, imp := range entry.Imports {
		processEntry(imp)
	}

	return assets, nil
}
