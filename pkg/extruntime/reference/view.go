package reference

import (
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// view is the model one reader sees. A reader with Extensions.Read gets every
// namespace whole; others get the visible resources and endpoints, without
// the operational parts (schedules, subscriptions and the packages' guard
// lists), and namespaces that end up empty are dropped.
func (f *full) view(sub spi.Subject, locale string) *Model {
	all := CanReadAll(sub)
	lookup := func(id string) (manifest.Resource, bool) {
		r, ok := f.resources[id]
		return r, ok
	}
	m := &Model{Namespaces: []Namespace{}, Full: all}
	for _, ns := range f.namespaces {
		out := Namespace{Name: ns.Name, Package: ns.Package, Native: ns.Native, Items: []Item{}}
		for _, it := range ns.Items {
			if !Visible(sub, it.res, lookup) {
				continue
			}
			it = localize(it, sub, locale)
			it.ExtendedBy = visibleIDs(it.ExtendedBy, func(id string) bool {
				r, ok := lookup(id)
				return ok && Visible(sub, r, lookup)
			})
			out.Items = append(out.Items, it)
		}
		for _, e := range ns.Endpoints {
			if EndpointVisible(sub, e) {
				out.Endpoints = append(out.Endpoints, e)
			}
		}
		if all {
			out.Schedules, out.Subscriptions, out.Guards = ns.Schedules, ns.Subscriptions, ns.Guards
		}
		if !all {
			for i := range out.Items {
				out.Items[i].Subscribers = nil
			}
		}
		if len(out.Items) == 0 && len(out.Endpoints) == 0 && len(out.Schedules) == 0 && len(out.Subscriptions) == 0 {
			continue
		}
		m.Namespaces = append(m.Namespaces, out)
	}
	return m
}

// viewAll is the model of a reader that sees everything, in locale.
func (f *full) viewAll(locale string) *Model {
	return f.view(spi.Subject{Permissions: []string{PermRead}}, locale)
}

// localize resolves the labels of an item in locale and drops the fields the
// reader cannot read.
func localize(it Item, sub spi.Subject, locale string) Item {
	it.Label = pickLabel(it.labels, locale)
	r := it.res
	if len(it.Fields) > 0 {
		fields := make([]FieldDoc, 0, len(it.Fields))
		for i, fd := range it.Fields {
			if i >= len(r.Fields) || !FieldVisible(sub, r, fd.ID) {
				continue
			}
			fd.Label = pickLabel(r.Fields[i].Label, locale)
			fields = append(fields, fd)
		}
		it.Fields = fields
	}
	if it.Nav != nil && r.Nav != nil {
		nav := *it.Nav
		nav.Label = pickLabel(r.Nav.Label, locale)
		it.Nav = &nav
	}
	if it.Navigation != nil {
		nav := *it.Navigation
		nav.Groups = relabel(nav.Groups, len(r.Groups), func(i int) manifest.Label { return r.Groups[i].Label }, locale)
		nav.Workspaces = relabel(nav.Workspaces, len(r.Workspaces), func(i int) manifest.Label { return r.Workspaces[i].Label }, locale)
		nav.Palette = relabel(nav.Palette, len(r.Palette), func(i int) manifest.Label { return r.Palette[i].Label }, locale)
		it.Navigation = &nav
	}
	return it
}

func relabel(in []IDLabel, n int, label func(int) manifest.Label, locale string) []IDLabel {
	if len(in) == 0 || len(in) != n {
		return in
	}
	out := make([]IDLabel, len(in))
	for i, x := range in {
		x.Label = pickLabel(label(i), locale)
		out[i] = x
	}
	return out
}

func visibleIDs(ids []string, visible func(string) bool) []string {
	var out []string
	for _, id := range ids {
		if visible(id) {
			out = append(out, id)
		}
	}
	return out
}
