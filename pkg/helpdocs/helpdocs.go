// Package helpdocs is the contract between the Help Center and the components
// that contribute documentation to it at run time.
//
// A Source owns one mount in the Help Center tree ("extensions/reference") and
// answers per request: the reader (user, permissions, tenant) and the locale
// travel in ctx and the locale argument, so a source can list only what the
// reader may see and can change between requests (packages are activated
// without a restart). The Help Center serves a source's documents at
// <mount>/<path> next to its own markdown tree; it never caches them.
//
// Components contribute sources with composition.ContributeHelpSources.
package helpdocs

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned by Source.Get for a document that does not exist or
// that the reader may not read; the two are indistinguishable to the reader.
var ErrNotFound = errors.New("helpdocs: document not found")

// Node is one entry of a source's tree. A node with Children is a category;
// a node without children is a document whose Path Get accepts.
type Node struct {
	// Title is the localized sidebar label.
	Title string
	// Path is relative to the source mount, slash-separated, without a
	// leading slash or extension ("crm/commands"). Categories carry the path
	// prefix of their children.
	Path string
	// Order places the node among its siblings; lower first, then by Title.
	Order    int
	Children []Node
}

// Document is one rendered-on-demand markdown document.
type Document struct {
	Title string
	// Path is relative to the source mount, as in Node.Path.
	Path string
	// Markdown is the document body (CommonMark + GFM tables). Links to other
	// documents of the same source are relative to the mount
	// ("../reference/crm"); absolute links are left as they are.
	Markdown []byte
	// Updated is when the content last changed, zero when unknown.
	Updated time.Time
	// Audience is a short reader hint shown with the document
	// ("Developers", "Administrators"); empty for everyone.
	Audience string
}

// Source contributes a subtree of documents to the Help Center.
type Source interface {
	// Mount is the stable path prefix of the source ("extensions/reference"),
	// slash-separated, without leading or trailing slash. Mounts of different
	// sources must not overlap.
	Mount() string
	// Title is the localized label of the mount's category.
	Title(locale string) string
	// Order places the mount among its siblings in the Help Center tree.
	Order() int
	// Tree lists the documents the reader in ctx may read, in locale (falling
	// back to the source's default locale). An empty tree hides the mount.
	Tree(ctx context.Context, locale string) ([]Node, error)
	// Get returns one document, or ErrNotFound.
	Get(ctx context.Context, locale, path string) (Document, error)
}
