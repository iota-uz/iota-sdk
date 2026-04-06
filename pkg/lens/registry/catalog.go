package registry

import (
	"context"
	"fmt"

	lensspec "github.com/iota-uz/iota-sdk/pkg/lens/spec"
)

type EntrySource string

const (
	EntrySourcePreset EntrySource = "preset"
)

type Entry struct {
	Key         string
	Source      EntrySource
	DocumentID  string
	Title       string
	Description string
	ReadOnly    bool
}

type Catalog interface {
	Entries(context.Context) ([]Entry, error)
	Load(context.Context, string) (lensspec.Document, error)
}

type StaticEntry struct {
	Entry    Entry
	Document lensspec.Document
}

type StaticCatalog struct {
	entries map[string]StaticEntry
	order   []string
}

func NewStaticCatalog(entries ...StaticEntry) (*StaticCatalog, error) {
	catalog := &StaticCatalog{
		entries: make(map[string]StaticEntry, len(entries)),
		order:   make([]string, 0, len(entries)),
	}
	for _, entry := range entries {
		key := entry.Entry.Key
		if key == "" {
			return nil, fmt.Errorf("catalog entry key is required")
		}
		if _, exists := catalog.entries[key]; exists {
			return nil, fmt.Errorf("duplicate catalog entry key %q", key)
		}
		if entry.Entry.DocumentID == "" {
			entry.Entry.DocumentID = entry.Document.ID
		}
		if entry.Entry.Title == "" {
			entry.Entry.Title = entry.Document.Title.Resolve("")
		}
		if entry.Entry.Description == "" {
			entry.Entry.Description = entry.Document.Description.Resolve("")
		}
		catalog.entries[key] = entry
		catalog.order = append(catalog.order, key)
	}
	return catalog, nil
}

func (c *StaticCatalog) Entries(context.Context) ([]Entry, error) {
	if c == nil {
		return nil, nil
	}
	entries := make([]Entry, 0, len(c.order))
	for _, key := range c.order {
		entries = append(entries, c.entries[key].Entry)
	}
	return entries, nil
}

func (c *StaticCatalog) Load(_ context.Context, key string) (lensspec.Document, error) {
	if c == nil {
		return lensspec.Document{}, fmt.Errorf("catalog is nil")
	}
	entry, ok := c.entries[key]
	if !ok {
		return lensspec.Document{}, fmt.Errorf("catalog entry %q not found", key)
	}
	return entry.Document, nil
}
