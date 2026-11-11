// Package shellcontext publishes the server-resolved shell model to the
// browser so Templ pages and Solid routes share one navigation boundary.
package shellcontext

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/a-h/templ"

	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

const (
	// ElementID is the id of the JSON script element read by the client host.
	ElementID = "iota-shell-context"
	// Version is the major.minor of the wire contract.
	Version = "1.0.0"
)

// NavigationItem is the flat, renderer-neutral form of a navigation entry.
type NavigationItem struct {
	Key       string `json:"key"`
	Title     string `json:"title"`
	Href      string `json:"href,omitempty"`
	Parent    string `json:"parent,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	Pinned    bool   `json:"pinned,omitempty"`
	Beta      bool   `json:"beta,omitempty"`
	Order     int    `json:"order"`
}

// Context is the payload serialized into the page.
type Context struct {
	Version    string           `json:"version"`
	Navigation []NavigationItem `json:"navigation"`
}

// Flatten converts a navigation tree into stable, unique keys. Items without a
// key fall back to their href, then to a path of positions, so a patch from an
// extension can always address what the sidebar shows.
func Flatten(items []types.NavigationItem) []NavigationItem {
	out := make([]NavigationItem, 0, len(items))
	used := make(map[string]struct{})
	var walk func(items []types.NavigationItem, parent, workspace, path string)
	walk = func(items []types.NavigationItem, parent, workspace, path string) {
		for index, item := range items {
			position := path + strconv.Itoa(index)
			key := item.Key
			if key == "" {
				key = item.Href
			}
			if key == "" {
				key = "item:" + position
			}
			if _, taken := used[key]; taken {
				base := key
				key = base + "#" + position
				for n := 2; ; n++ {
					if _, clash := used[key]; !clash {
						break
					}
					key = base + "#" + position + "-" + strconv.Itoa(n)
				}
			}
			used[key] = struct{}{}
			itemWorkspace := item.Workspace
			if itemWorkspace == "" {
				itemWorkspace = workspace
			}
			out = append(out, NavigationItem{
				Key: key, Title: item.Name, Href: item.Href, Parent: parent,
				Workspace: itemWorkspace, Pinned: item.Pinned, Beta: item.IsBeta, Order: index,
			})
			walk(item.Children, key, itemWorkspace, position+".")
		}
	}
	walk(items, "", "", "")
	return out
}

// FromContext builds the shell context from the request-scoped navigation the
// sidebar middleware stores. A request without navigation yields an empty list.
func FromContext(ctx context.Context) Context {
	items, _ := ctx.Value(constants.NavItemsKey).([]types.NavigationItem)
	return Context{Version: Version, Navigation: Flatten(items)}
}

// Script renders the context as an application/json script element. JSON is
// escaped by encoding/json, which neutralizes "</script>" sequences.
func Script(value Context) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		encoded, err := json.Marshal(value)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, `<script id="%s" type="application/json">%s</script>`, ElementID, encoded)
		return err
	})
}

// Component renders the shell context for the current request.
func Component() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		return Script(FromContext(ctx)).Render(ctx, w)
	})
}
