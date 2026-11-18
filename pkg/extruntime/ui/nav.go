package ui

import (
	"context"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/listext"
	"github.com/iota-uz/iota-sdk/pkg/shellext"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// Navigation returns the request-scoped navigation contributor. It projects
// the activated pages with a nav entry, groups and patches into the same
// navigation items the Templ sidebar and pkg/shellcontext are built from, so
// the legacy sidebar and a Solid shell show one navigation. It is
// permission-filtered per request and answers from the cached index.
func (r *Runtime) Navigation() shellext.Navigation { return navProjection{r} }

type navProjection struct{ r *Runtime }

func (n navProjection) Items(ctx context.Context, items []types.NavigationItem) []types.NavigationItem {
	ix, err := n.r.Index(ctx)
	if err != nil {
		n.r.cfg.Logger.WithError(err).Error("extruntime ui: navigation snapshot")
		return items
	}
	sub, err := listext.SubjectFromContext(ctx)
	if err != nil {
		return items
	}
	return ProjectNavigation(ix, sub, listext.LangFromContext(ctx), items)
}

func (n navProjection) Workspaces(ctx context.Context, ws []types.NavWorkspace) []types.NavWorkspace {
	ix, err := n.r.Index(ctx)
	if err != nil || len(ws) == 0 || len(ix.Navigation.Workspaces) == 0 {
		// An application without workspaces shows one flat sidebar; adding
		// package workspaces would pull every standard item into one of them.
		return ws
	}
	lang := listext.LangFromContext(ctx)
	out := append([]types.NavWorkspace(nil), ws...)
	taken := map[string]bool{}
	for _, w := range out {
		taken[w.Key] = true
	}
	for _, w := range ix.Navigation.Workspaces {
		if taken[w.ID] {
			continue
		}
		taken[w.ID] = true
		out = append(out, types.NavWorkspace{Key: w.ID, Label: listext.LabelFor(w.Label, lang, w.ID), Order: w.Order})
	}
	return out
}

// ProjectNavigation applies the activated navigation to localized items.
// Pages whose permission the subject lacks are not added. Patches (hide,
// rename, move, reorder) address any item by its public key, standard or
// contributed. Hiding removes the entry from navigation only: the route
// remains served and its permission is checked by the page host.
func ProjectNavigation(ix *Index, sub spi.Subject, lang string, in []types.NavigationItem) []types.NavigationItem {
	if len(ix.Pages) == 0 && len(ix.Navigation.Patches) == 0 && len(ix.Navigation.Groups) == 0 {
		return in
	}
	tree := &navTree{items: cloneNav(in)}
	groups := map[string]NavGroup{}
	for _, g := range ix.Navigation.Groups {
		if _, dup := groups[g.ID]; !dup {
			groups[g.ID] = g
		}
	}
	workspaces := map[string]bool{}
	for _, w := range ix.Navigation.Workspaces {
		workspaces[w.ID] = true
	}

	pages := make([]*Page, 0, len(ix.Pages))
	for _, p := range ix.Pages {
		if p.Nav == nil || strings.Contains(p.Pattern, ":") || !sub.Has(p.Permission) {
			continue
		}
		pages = append(pages, p)
	}
	sort.SliceStable(pages, func(i, j int) bool {
		if pages[i].Nav.Order != pages[j].Nav.Order {
			return pages[i].Nav.Order < pages[j].Nav.Order
		}
		return pages[i].ID < pages[j].ID
	})
	for _, p := range pages {
		if tree.find(p.ID) != nil {
			continue
		}
		item := types.NavigationItem{Key: p.ID, Name: listext.LabelFor(p.Nav.Label, lang, p.ID), Href: p.Pattern}
		parent := p.Nav.Parent
		if parent == "" {
			parent = p.Nav.Group
		}
		switch {
		case parent == "":
			tree.insert("", item, p.Nav.Order)
		case tree.find(parent) != nil:
			tree.insert(parent, item, p.Nav.Order)
		case groups[parent].ID != "":
			g := groups[parent]
			if tree.find(g.ID) == nil {
				container := types.NavigationItem{Key: g.ID, Name: listext.LabelFor(g.Label, lang, g.ID)}
				if workspaces[g.ID] {
					container.Workspace = g.ID
				}
				tree.insert("", container, g.Order)
			}
			tree.insert(g.ID, item, p.Nav.Order)
		case workspaces[parent]:
			item.Workspace = parent
			tree.insert("", item, p.Nav.Order)
		default:
			tree.insert("", item, p.Nav.Order)
		}
	}
	for _, patch := range ix.Navigation.Patches {
		tree.patch(patch, lang)
	}
	return tree.items
}

type navTree struct{ items []types.NavigationItem }

func cloneNav(in []types.NavigationItem) []types.NavigationItem {
	if in == nil {
		return nil
	}
	out := make([]types.NavigationItem, len(in))
	for i, it := range in {
		it.Children = cloneNav(it.Children)
		out[i] = it
	}
	return out
}

// find returns a pointer to the item with key.
func (t *navTree) find(key string) *types.NavigationItem {
	var walk func(items []types.NavigationItem) *types.NavigationItem
	walk = func(items []types.NavigationItem) *types.NavigationItem {
		for i := range items {
			if items[i].Key == key {
				return &items[i]
			}
			if c := walk(items[i].Children); c != nil {
				return c
			}
		}
		return nil
	}
	return walk(t.items)
}

// siblings returns the slice that holds the children of parent ("" = root).
func (t *navTree) siblings(parent string) *[]types.NavigationItem {
	if parent == "" {
		return &t.items
	}
	if p := t.find(parent); p != nil {
		return &p.Children
	}
	return nil
}

// insert places item among the children of parent. order 0 appends; a
// positive order is the 1-based position, clamped to the end.
func (t *navTree) insert(parent string, item types.NavigationItem, order int) {
	list := t.siblings(parent)
	if list == nil {
		list = &t.items
	}
	at := len(*list)
	if order > 0 && order-1 < at {
		at = order - 1
	}
	*list = append((*list)[:at], append([]types.NavigationItem{item}, (*list)[at:]...)...)
}

// remove detaches the item with key and returns it.
func (t *navTree) remove(key string) (types.NavigationItem, bool) {
	var walk func(items *[]types.NavigationItem) (types.NavigationItem, bool)
	walk = func(items *[]types.NavigationItem) (types.NavigationItem, bool) {
		for i := range *items {
			if (*items)[i].Key == key {
				it := (*items)[i]
				*items = append((*items)[:i], (*items)[i+1:]...)
				return it, true
			}
			if it, ok := walk(&(*items)[i].Children); ok {
				return it, true
			}
		}
		return types.NavigationItem{}, false
	}
	return walk(&t.items)
}

func (t *navTree) patch(p manifest.NavPatch, lang string) {
	if p.Hide {
		t.remove(p.Ref)
		return
	}
	target := t.find(p.Ref)
	if target == nil {
		return
	}
	if len(p.Rename) > 0 {
		target.Name = listext.LabelFor(p.Rename, lang, target.Name)
	}
	if p.Parent == "" && p.Order == nil {
		return
	}
	// Moving into the item itself or one of its descendants would detach it.
	if p.Parent != "" && (p.Parent == p.Ref || isDescendant(target, p.Parent)) {
		return
	}
	if p.Parent != "" && t.find(p.Parent) == nil {
		return
	}
	parent := p.Parent
	if parent == "" {
		origin, ok := t.parentKey(p.Ref)
		if !ok {
			return
		}
		parent = origin
	}
	item, ok := t.remove(p.Ref)
	if !ok {
		return
	}
	order := 0
	if p.Order != nil {
		order = *p.Order
	}
	t.insert(parent, item, order)
}

// parentKey returns the key of the item's parent ("" for a root item).
func (t *navTree) parentKey(key string) (string, bool) {
	var walk func(items []types.NavigationItem, parent string) (string, bool)
	walk = func(items []types.NavigationItem, parent string) (string, bool) {
		for i := range items {
			if items[i].Key == key {
				return parent, true
			}
			if p, ok := walk(items[i].Children, items[i].Key); ok {
				return p, true
			}
		}
		return "", false
	}
	return walk(t.items, "")
}

func isDescendant(item *types.NavigationItem, key string) bool {
	for i := range item.Children {
		if item.Children[i].Key == key || isDescendant(&item.Children[i], key) {
			return true
		}
	}
	return false
}
