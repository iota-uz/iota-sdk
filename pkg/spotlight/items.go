package spotlight

import (
	"context"
	"io"

	"github.com/a-h/templ"
	spotlightui "github.com/iota-uz/iota-sdk/components/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

// Item represents a renderable spotlight entry.
type Item interface {
	Label(ctx context.Context) string
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

func (i *item) Label(ctx context.Context) string { return i.label }

func (i *item) Render(ctx context.Context, w io.Writer) error {
	return spotlightui.LinkItem(i.label, i.link, i.icon).Render(ctx, w)
}

// NewLocalizedItem creates an Item whose label is localized at render time.
func NewLocalizedItem(icon templ.Component, trKey, link string) Item {
	return &localizedItem{trKey: trKey, icon: icon, link: link}
}

type localizedItem struct {
	trKey string
	icon  templ.Component
	link  string
}

func (i *localizedItem) Label(ctx context.Context) string { return intl.MustT(ctx, i.trKey) }

func (i *localizedItem) Render(ctx context.Context, w io.Writer) error {
	label := intl.MustT(ctx, i.trKey)
	return spotlightui.LinkItem(label, i.link, i.icon).Render(ctx, w)
}
