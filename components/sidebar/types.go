// Package sidebar provides navigation components for application layout.
package sidebar

import (
	"context"
	"strings"

	"github.com/a-h/templ"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/utils/random"
)

// Link represents a navigation link in the sidebar.
type Link interface {
	IsLink() bool
	Position() int
	Href() string
	Text() string
	Icon() templ.Component
	IsActive(ctx context.Context) bool
	SetPosition(position int) Link
}

// Group represents a collection of navigation items that can be expanded/collapsed.
type Group interface {
	ID() string
	IsLink() bool
	Position() int
	Text() string
	Icon() templ.Component
	Children() []Item
	IsActive(ctx context.Context) bool
	SetPosition(position int) Group
}

// Item is the base interface for navigation elements in the sidebar.
type Item interface {
	IsLink() bool
	Position() int
	Icon() templ.Component
	IsActive(ctx context.Context) bool
}

func asLink(i Item) Link {
	return i.(Link)
}

func asGroup(i Item) Group {
	return i.(Group)
}

// NewGroup creates a new navigation group with the given text, icon, and child items.
func NewGroup(text string, icon templ.Component, children []Item) Group {
	return &group{
		id:       random.String(8, random.AlphaNumericSet),
		text:     text,
		position: 0,
		icon:     icon,
		children: children,
	}
}

type group struct {
	id       string
	text     string
	position int
	icon     templ.Component
	children []Item
}

func (g *group) SetPosition(position int) Group {
	return &group{
		id:       g.id,
		text:     g.text,
		position: position,
		icon:     g.icon,
		children: g.children,
	}
}

func (g *group) Icon() templ.Component {
	return g.icon
}

func (g *group) ID() string {
	return g.id
}

func (g *group) Text() string {
	return g.text
}

func (g *group) Position() int {
	return g.position
}

func (g *group) IsActive(ctx context.Context) bool {
	for _, child := range g.children {
		if child.IsActive(ctx) {
			return true
		}
	}
	return false
}

func (g *group) IsLink() bool {
	return false
}

func (g *group) Children() []Item {
	return g.children
}

// NewLink creates a new navigation link with the given URL, text, and icon.
func NewLink(href, text string, icon templ.Component) Link {
	return &link{
		href:     href,
		text:     text,
		position: 0,
		icon:     icon,
	}
}

type link struct {
	href     string
	text     string
	position int
	icon     templ.Component
}

func (l *link) SetPosition(position int) Link {
	return &link{
		href:     l.href,
		text:     l.text,
		position: position,
		icon:     l.icon,
	}
}

func (l *link) Icon() templ.Component {
	return l.icon
}

func (l *link) IsActive(ctx context.Context) bool {
	u := composables.UsePageCtx(ctx).URL
	return u.Path == l.href || strings.HasPrefix(u.Path, l.href+"/")
}

func (l *link) IsLink() bool {
	return true
}

func (l *link) Position() int {
	return l.position
}

func (l *link) Text() string {
	return l.text
}

func (l *link) Href() string {
	return l.href
}

// TabGroup represents a group of sidebar items organized under a tab
type TabGroup struct {
	Label string
	Value string
	Items []Item
}

// TabGroupCollection holds multiple tab groups for the sidebar
type TabGroupCollection struct {
	Groups       []TabGroup
	DefaultValue string
}
