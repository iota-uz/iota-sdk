// Package spotlight is a package that provides a way to show a list of items in a spotlight.
package spotlight

import (
	"context"
	"io"
	"sort"

	"github.com/a-h/templ"
	"github.com/lithammer/fuzzysearch/fuzzy"

	spotlightui "github.com/iota-uz/iota-sdk/components/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

type Spotlight interface {
	Find(ctx context.Context, q string) []Item
	Register(...Item)
	RegisterDataSource(DataSource)
}

type DataSource interface {
	Find(ctx context.Context, q string) []Item
}

type Item interface {
	Label(ctx context.Context) string
	templ.Component
}

func NewLocalizedItem(icon templ.Component, trKey, link string) Item {
	return &localizedItem{
		trKey: trKey,
		icon:  icon,
		link:  link,
	}
}

type localizedItem struct {
	trKey string
	icon  templ.Component
	link  string
}

func (i *localizedItem) Label(ctx context.Context) string {
	return intl.MustT(ctx, i.trKey)
}

func (i *localizedItem) Render(ctx context.Context, w io.Writer) error {
	label := intl.MustT(ctx, i.trKey)
	return spotlightui.SpotlightItem(label, i.link, i.icon).Render(ctx, w)
}

func NewItem(icon templ.Component, label, link string) Item {
	return &item{
		label: label,
		icon:  icon,
		link:  link,
	}
}

type item struct {
	label string
	icon  templ.Component
	link  string
}

func (i *item) Label(ctx context.Context) string {
	return i.label
}

func (i *item) Render(ctx context.Context, w io.Writer) error {
	return spotlightui.SpotlightItem(i.label, i.link, i.icon).Render(ctx, w)
}

func New() Spotlight {
	return &spotlight{
		items:       make([]Item, 0),
		dataSources: make([]DataSource, 0),
	}
}

type spotlight struct {
	items       []Item
	dataSources []DataSource
}

func (s *spotlight) Register(i ...Item) {
	s.items = append(s.items, i...)
}

func (s *spotlight) RegisterDataSource(ds DataSource) {
	s.dataSources = append(s.dataSources, ds)
}

func (s *spotlight) Find(ctx context.Context, q string) []Item {
	words := make([]string, len(s.items))
	for i, it := range s.items {
		words[i] = it.Label(ctx)
	}
	ranks := fuzzy.RankFindNormalizedFold(q, words)
	sort.Sort(ranks)
	filteredItems := make([]Item, 0, len(ranks))
	for _, rank := range ranks {
		filteredItems = append(filteredItems, s.items[rank.OriginalIndex])
	}
	for _, ds := range s.dataSources {
		items := ds.Find(ctx, q)
		if len(items) > 0 {
			filteredItems = append(filteredItems, items...)
		}
	}
	return filteredItems
}
