package spotlight

import (
	"context"
	"io"
	"sort"

	"github.com/a-h/templ"
	spotlightui "github.com/iota-uz/iota-sdk/components/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/lithammer/fuzzysearch/fuzzy"
)

// Item represents a renderable spotlight entry.
type Item interface {
	templ.Component
}

// NewItem creates a simple Item with a static label and link.
func NewItem(icon templ.Component, label, link string) Item {
	return &item{label: label, icon: icon, link: link}
}

type item struct {
	label string
	icon  templ.Component
	link  string
}

func (i *item) Render(ctx context.Context, w io.Writer) error {
	return spotlightui.LinkItem(i.label, i.link, i.icon).Render(ctx, w)
}

func NewQuickLink(icon templ.Component, trKey, link string) *QuickLink {
	return &QuickLink{trKey: trKey, icon: icon, link: link}
}

type QuickLink struct {
	trKey     string
	icon      templ.Component
	link      string
	canAccess func(ctx context.Context) bool
}

// WithAccessCheck sets an optional access check function.
// When set, the QuickLink is only shown if canAccess returns true.
func (ql *QuickLink) WithAccessCheck(fn func(ctx context.Context) bool) *QuickLink {
	ql.canAccess = fn
	return ql
}

func (i *QuickLink) Render(ctx context.Context, w io.Writer) error {
	label := intl.MustT(ctx, i.trKey)
	return spotlightui.LinkItem(label, i.link, i.icon).Render(ctx, w)
}

type QuickLinks struct {
	items []*QuickLink
}

func (ql *QuickLinks) Find(ctx context.Context, q string) []Item {
	// Filter items by access check first
	accessible := make([]*QuickLink, 0, len(ql.items))
	for _, it := range ql.items {
		if it.canAccess != nil && !it.canAccess(ctx) {
			continue
		}
		accessible = append(accessible, it)
	}

	words := make([]string, len(accessible))
	for i, it := range accessible {
		words[i] = intl.MustT(ctx, it.trKey)
	}
	ranks := fuzzy.RankFindNormalizedFold(q, words)
	sort.Sort(ranks)

	result := make([]Item, 0, len(ranks))
	for _, rank := range ranks {
		result = append(result, accessible[rank.OriginalIndex])
	}
	return result
}

func (ql *QuickLinks) Add(links ...*QuickLink) {
	ql.items = append(ql.items, links...)
}
