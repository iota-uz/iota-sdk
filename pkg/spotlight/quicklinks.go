package spotlight

import (
	"context"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

type QuickLink struct {
	trKey string
	icon  templ.Component
	link  string
}

func NewQuickLink(icon templ.Component, trKey, link string) *QuickLink {
	return &QuickLink{trKey: trKey, icon: icon, link: link}
}

type QuickLinks struct {
	mu    sync.RWMutex
	items []*QuickLink
}

func NewQuickLinks() *QuickLinks {
	return &QuickLinks{items: make([]*QuickLink, 0, 16)}
}

func (ql *QuickLinks) Add(links ...*QuickLink) {
	ql.mu.Lock()
	defer ql.mu.Unlock()
	ql.items = append(ql.items, links...)
}

func (ql *QuickLinks) ProviderID() string {
	return "core.quick_links"
}

func (ql *QuickLinks) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{SupportsWatch: false, EntityTypes: []string{"quick_link", "route"}}
}

func (ql *QuickLinks) ListDocuments(ctx context.Context, scope ProviderScope) ([]SearchDocument, error) {
	ql.mu.RLock()
	defer ql.mu.RUnlock()

	out := make([]SearchDocument, 0, len(ql.items))
	for idx, item := range ql.items {
		label := intl.MustT(ctx, item.trKey)
		out = append(out, SearchDocument{
			ID:         item.trKey + ":" + item.link + ":" + string(rune(idx)),
			EntityType: "quick_link",
			Title:      label,
			Body:       label,
			URL:        item.link,
			Language:   scope.Language,
			Metadata: map[string]string{
				"tr_key": item.trKey,
				"source": "quick_links",
			},
			Access:    AccessPolicy{Visibility: VisibilityPublic},
			UpdatedAt: time.Now().UTC(),
		})
	}
	return out, nil
}

func (ql *QuickLinks) Watch(_ context.Context, _ ProviderScope) (<-chan DocumentEvent, error) {
	return nil, nil
}
