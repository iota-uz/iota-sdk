package spotlight

import (
	"context"
	"sync"
	"time"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

type QuickLink struct {
	trKey     string
	link      string
	createdAt time.Time
}

func NewQuickLink(trKey, link string) *QuickLink {
	return &QuickLink{trKey: trKey, link: link, createdAt: time.Now().UTC()}
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

	providerID := ql.ProviderID()
	out := make([]SearchDocument, 0, len(ql.items))
	for _, item := range ql.items {
		label := item.trKey
		if localizer, ok := intl.UseLocalizer(ctx); ok {
			if translated, err := localizer.Localize(&i18n.LocalizeConfig{
				MessageID: item.trKey,
				DefaultMessage: &i18n.Message{
					ID:    item.trKey,
					Other: item.trKey,
				},
			}); err == nil {
				label = translated
			}
		}
		out = append(out, SearchDocument{
			ID:         providerID + ":" + item.trKey + ":" + item.link,
			TenantID:   scope.TenantID,
			Provider:   providerID,
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
			UpdatedAt: item.createdAt,
		})
	}
	return out, nil
}

func (ql *QuickLinks) Watch(_ context.Context, _ ProviderScope) (<-chan DocumentEvent, error) {
	changes := make(chan DocumentEvent)
	close(changes)
	return changes, nil
}
