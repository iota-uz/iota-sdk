// Package shellext is the request-scoped extension point of the authenticated
// shell. A host component installs contributors on the request context; the
// authenticated layout renders their markup and the navigation middleware
// lets them project into the navigation model. It has no knowledge of who
// contributes, so the layout and the sidebar stay independent of the
// extension runtime.
package shellext

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/a-h/templ"

	"github.com/iota-uz/iota-sdk/pkg/types"
)

// Markup renders extra markup at the end of the authenticated layout (for
// example the boot data and entry module of a shell renderer). It returns nil
// when it has nothing to render for the request.
type Markup func(ctx context.Context) templ.Component

// Navigation projects contributed navigation into the model the Templ sidebar
// and pkg/shellcontext are both built from. Implementations answer from
// memory, per request, and filter by the permissions of the request user.
type Navigation interface {
	// Items receives the localized navigation items of the request before
	// permission filtering and returns the projected set.
	Items(ctx context.Context, items []types.NavigationItem) []types.NavigationItem
	// Workspaces receives the declared workspaces and returns the projected set.
	Workspaces(ctx context.Context, workspaces []types.NavWorkspace) []types.NavWorkspace
}

// Module locates one client export of an activated package: the immutable asset
// URL of its entry chunk and the export that holds the component. It is the
// JSON the browser host reads (slot contributions, cell renderers).
type Module struct {
	ID      string `json:"id"`
	Package string `json:"package"`
	Version string `json:"version"`
	Module  string `json:"module"`
	Export  string `json:"export,omitempty"`
	// Styles are the immutable asset URLs of the stylesheets of the package
	// version the module belongs to (Tailwind utilities, scoped CSS modules).
	// The browser host links each once per document before the module's first
	// render.
	Styles []string `json:"styles,omitempty"`
}

// Cell is a custom renderer for one cell of a native list.
type Cell struct {
	Renderer Module
	// Value is the cell's value as wire JSON (nil for NULL).
	Value any
	// Row holds the row's contributed values by column ID and "ref".
	Row map[string]any
}

// Islands lets Go-rendered pages host components of extension packages: the
// pages stay server-rendered and meaningful without JavaScript, and the
// browser runtime mounts the contributions into the elements these methods
// emit. A host component installs it per request.
type Islands interface {
	// Slot returns the mount element for the contributions to slot id, or
	// nil when nothing applies to the viewer. props is the JSON object the
	// slot's declaration types.
	Slot(ctx context.Context, id string, props json.RawMessage) templ.Component
	// Cell wraps fallback (the standard server-rendered cell) in the mount
	// element of a package renderer, or returns fallback when the viewer
	// cannot be served one.
	Cell(ctx context.Context, cell Cell, fallback templ.Component) templ.Component
}

type markupKey struct{}
type navKey struct{}
type islandsKey struct{}

// WithIslands returns ctx carrying the islands host.
func WithIslands(ctx context.Context, i Islands) context.Context {
	return context.WithValue(ctx, islandsKey{}, i)
}

// IslandsFrom returns the islands host of ctx, if any.
func IslandsFrom(ctx context.Context) (Islands, bool) {
	i, ok := ctx.Value(islandsKey{}).(Islands)
	return i, ok
}

// Slot renders the contributions of extension packages to the slot id, which
// the calling page declares (stable ID, typed props, optional permission).
// props must marshal to a JSON object. Without the extension runtime, or with
// no applicable contribution, it renders nothing.
func Slot(ctx context.Context, id string, props any) templ.Component {
	raw, err := json.Marshal(props)
	if err != nil {
		return failed(fmt.Errorf("shellext: slot %s props: %w", id, err))
	}
	if i, ok := IslandsFrom(ctx); ok {
		if c := i.Slot(ctx, id, raw); c != nil {
			return c
		}
	}
	return templ.NopComponent
}

// CellIsland wraps fallback in the mount element of a package cell renderer
// when the extension runtime can serve one, and renders fallback otherwise.
func CellIsland(ctx context.Context, cell Cell, fallback templ.Component) templ.Component {
	if i, ok := IslandsFrom(ctx); ok {
		if c := i.Cell(ctx, cell, fallback); c != nil {
			return c
		}
	}
	return fallback
}

func failed(err error) templ.Component {
	return templ.ComponentFunc(func(context.Context, io.Writer) error { return err })
}

// WithMarkup returns ctx with m added to the layout contributors.
func WithMarkup(ctx context.Context, m Markup) context.Context {
	prev, _ := ctx.Value(markupKey{}).([]Markup)
	next := append(append([]Markup(nil), prev...), m)
	return context.WithValue(ctx, markupKey{}, next)
}

// WithNavigation returns ctx carrying the navigation contributor.
func WithNavigation(ctx context.Context, n Navigation) context.Context {
	return context.WithValue(ctx, navKey{}, n)
}

// NavigationFrom returns the navigation contributor of ctx, if any.
func NavigationFrom(ctx context.Context) (Navigation, bool) {
	n, ok := ctx.Value(navKey{}).(Navigation)
	return n, ok
}

// Component renders the markup of every contributor, in installation order.
func Component() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		ms, _ := ctx.Value(markupKey{}).([]Markup)
		for _, m := range ms {
			c := m(ctx)
			if c == nil {
				continue
			}
			if err := c.Render(ctx, w); err != nil {
				return err
			}
		}
		return nil
	})
}
