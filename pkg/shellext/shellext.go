// Package shellext is the request-scoped extension point of the authenticated
// shell. A host component installs contributors on the request context; the
// authenticated layout renders their markup and the navigation middleware
// lets them project into the navigation model. It has no knowledge of who
// contributes, so the layout and the sidebar stay independent of the
// extension runtime.
package shellext

import (
	"context"
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

type markupKey struct{}
type navKey struct{}

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
