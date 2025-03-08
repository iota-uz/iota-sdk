// Package spotlight is a package that provides a way to show a list of items in a spotlight.
package spotlight

import (
	"github.com/a-h/templ"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"sort"
)

type Spotlight interface {
	Find(localizer *i18n.Localizer, q string) []Item
	Register(...Item)
}

type Item interface {
	Icon() templ.Component
	Localized(localizer *i18n.Localizer) string
	Link() string
}

func NewItem(icon templ.Component, trKey, link string) Item {
	return &item{
		trKey: trKey,
		icon:  icon,
		link:  link,
	}
}

type item struct {
	trKey string
	icon  templ.Component
	link  string
}

func (i *item) Icon() templ.Component {
	return i.icon
}

func (i *item) Localized(localizer *i18n.Localizer) string {
	return localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: i.trKey,
	})
}

func (i *item) Link() string {
	return i.link
}

func New() Spotlight {
	return &spotlight{}
}

type spotlight struct {
	items []Item
}

func (s *spotlight) Register(i ...Item) {
	s.items = append(s.items, i...)
}

func (s *spotlight) Find(localizer *i18n.Localizer, q string) []Item {
	words := make([]string, len(s.items))
	for i, it := range s.items {
		words[i] = it.Localized(localizer)
	}
	ranks := fuzzy.RankFindNormalizedFold(q, words)
	sort.Sort(ranks)
	filteredItems := make([]Item, 0, len(ranks))
	for _, rank := range ranks {
		filteredItems = append(filteredItems, s.items[rank.OriginalIndex])
	}
	return filteredItems
}
