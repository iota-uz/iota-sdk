package spotlight

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/iota-uz/go-i18n/v2/i18n"
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
	mu        sync.RWMutex
	items     []*QuickLink
	bundle    *i18n.Bundle
	languages []string
}

func NewQuickLinks(bundle *i18n.Bundle, languages []string) *QuickLinks {
	return &QuickLinks{
		items:     make([]*QuickLink, 0, 16),
		bundle:    bundle,
		languages: languages,
	}
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

// resolveAllTranslations resolves the translation key into a title (English)
// and a body containing all unique translations joined by " | " for multi-language search.
func (ql *QuickLinks) resolveAllTranslations(trKey string) (title, body string) {
	if ql.bundle == nil || len(ql.languages) == 0 {
		return trKey, trKey
	}

	seen := make(map[string]struct{}, len(ql.languages))
	translations := make([]string, 0, len(ql.languages))

	for _, lang := range ql.languages {
		localizer := i18n.NewLocalizer(ql.bundle, lang)
		translated, err := localizer.Localize(&i18n.LocalizeConfig{
			MessageID: trKey,
			DefaultMessage: &i18n.Message{
				ID:    trKey,
				Other: trKey,
			},
		})
		if err != nil || translated == trKey {
			continue
		}
		if title == "" {
			title = translated
		}
		if _, exists := seen[translated]; !exists {
			seen[translated] = struct{}{}
			translations = append(translations, translated)
		}
	}

	if title == "" {
		title = trKey
	}
	if len(translations) == 0 {
		return title, title
	}
	return title, strings.Join(translations, " | ")
}

func (ql *QuickLinks) ListDocuments(_ context.Context, scope ProviderScope) ([]SearchDocument, error) {
	ql.mu.RLock()
	defer ql.mu.RUnlock()

	providerID := ql.ProviderID()
	out := make([]SearchDocument, 0, len(ql.items))
	for _, item := range ql.items {
		title, body := ql.resolveAllTranslations(item.trKey)
		out = append(out, SearchDocument{
			ID:         providerID + ":" + item.trKey + ":" + item.link,
			TenantID:   scope.TenantID,
			Provider:   providerID,
			EntityType: "quick_link",
			Title:      title,
			Body:       body,
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
