// Package registry loads Lens documents from embedded or filesystem-backed presets.
package registry

import (
	"fmt"
	"io/fs"

	lensspec "github.com/iota-uz/iota-sdk/pkg/lens/spec"
)

func LoadFS(fsys fs.FS, name string) (lensspec.Document, error) {
	return lensspec.LoadFS(fsys, name)
}

func MustLoadFS(fsys fs.FS, name string) lensspec.Document {
	doc, err := LoadFS(fsys, name)
	if err != nil {
		panic(err)
	}
	return doc
}

func CatalogFS(fsys fs.FS, names ...string) (*StaticCatalog, error) {
	entries := make([]StaticEntry, 0, len(names))
	for _, name := range names {
		doc, err := LoadFS(fsys, name)
		if err != nil {
			return nil, err
		}
		entries = append(entries, StaticEntry{
			Entry: Entry{
				Key:      name,
				Source:   EntrySourcePreset,
				ReadOnly: true,
			},
			Document: doc,
		})
	}
	return NewStaticCatalog(entries...)
}

func MustCatalogFS(fsys fs.FS, names ...string) *StaticCatalog {
	catalog, err := CatalogFS(fsys, names...)
	if err != nil {
		panic(fmt.Errorf("lens preset catalog: %w", err))
	}
	return catalog
}
