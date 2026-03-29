// Package spotlight provides this package.
package spotlight

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/iota-uz/go-i18n/v2/i18n"
)

const searchTextDelimiter = " | "

type QuickLink struct {
	trKey     string
	link      string
	access    AccessPolicy
	keywords  []string
	createdAt time.Time
}

// NewQuickLink creates a QuickLink with VisibilityPublic access (backward compatible).
// Use QuickLinkBuilder for more control over access policies and keywords.
func NewQuickLink(trKey, link string) *QuickLink {
	return &QuickLink{
		trKey:     trKey,
		link:      link,
		access:    AccessPolicy{Visibility: VisibilityPublic},
		createdAt: time.Now().UTC(),
	}
}

// QuickLinkBuilder provides a fluent API for creating QuickLinks with RBAC and keywords.
type QuickLinkBuilder struct {
	link *QuickLink
}

// NewQuickLinkBuilder creates a new builder for a QuickLink with the given translation key and URL.
// By default, the link has restricted visibility (requires explicit access configuration).
func NewQuickLinkBuilder(trKey, link string) *QuickLinkBuilder {
	return &QuickLinkBuilder{
		link: &QuickLink{
			trKey:     trKey,
			link:      link,
			access:    AccessPolicy{Visibility: VisibilityRestricted},
			createdAt: time.Now().UTC(),
		},
	}
}

// WithPermissions sets the required permissions for accessing this quick link.
// This sets VisibilityRestricted and adds the permissions to AllowedPermissions.
func (b *QuickLinkBuilder) WithPermissions(permissions ...string) *QuickLinkBuilder {
	b.link.access.Visibility = VisibilityRestricted
	b.link.access.AllowedPermissions = permissions
	return b
}

// WithRoles sets the required roles for accessing this quick link.
// This sets VisibilityRestricted and adds the roles to AllowedRoles.
func (b *QuickLinkBuilder) WithRoles(roles ...string) *QuickLinkBuilder {
	b.link.access.Visibility = VisibilityRestricted
	b.link.access.AllowedRoles = roles
	return b
}

// WithUsers sets the specific users who can access this quick link.
// This sets VisibilityRestricted and adds the user IDs to AllowedUsers.
func (b *QuickLinkBuilder) WithUsers(userIDs ...string) *QuickLinkBuilder {
	b.link.access.Visibility = VisibilityRestricted
	b.link.access.AllowedUsers = userIDs
	return b
}

// WithOwner restricts access to the owner with the given user ID.
// This sets VisibilityOwner.
func (b *QuickLinkBuilder) WithOwner(ownerID string) *QuickLinkBuilder {
	b.link.access.Visibility = VisibilityOwner
	b.link.access.OwnerID = ownerID
	return b
}

// WithAccess sets a custom access policy for this quick link.
func (b *QuickLinkBuilder) WithAccess(access AccessPolicy) *QuickLinkBuilder {
	b.link.access = access
	return b
}

// Public makes this quick link visible to everyone.
// This sets VisibilityPublic.
func (b *QuickLinkBuilder) Public() *QuickLinkBuilder {
	b.link.access.Visibility = VisibilityPublic
	return b
}

// WithKeywords adds search keywords/aliases for this quick link.
// Keywords are searchable but not displayed in the UI.
func (b *QuickLinkBuilder) WithKeywords(keywords ...string) *QuickLinkBuilder {
	b.link.keywords = append(b.link.keywords, keywords...)
	return b
}

// Build returns the configured QuickLink.
func (b *QuickLinkBuilder) Build() *QuickLink {
	return b.link
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
func (ql *QuickLinks) resolveAllTranslations(trKey string) (string, string) {
	if ql.bundle == nil || len(ql.languages) == 0 {
		return trKey, trKey
	}

	var title string
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
	return title, strings.Join(translations, searchTextDelimiter)
}

func (ql *QuickLinks) ListDocuments(_ context.Context, scope ProviderScope) ([]SearchDocument, error) {
	ql.mu.RLock()
	defer ql.mu.RUnlock()

	providerID := ql.ProviderID()
	out := make([]SearchDocument, 0, len(ql.items))
	for _, item := range ql.items {
		title, body := ql.resolveAllTranslations(item.trKey)

		// Include keywords in searchable body
		if len(item.keywords) > 0 {
			body = body + searchTextDelimiter + strings.Join(item.keywords, searchTextDelimiter)
		}

		out = append(out, SearchDocument{
			ID:          providerID + ":" + item.trKey + ":" + item.link,
			TenantID:    scope.TenantID,
			Provider:    providerID,
			EntityType:  "quick_link",
			Domain:      ResultDomainNavigate,
			Title:       title,
			Description: title,
			Body:        body,
			SearchText:  body,
			ExactTerms:  ExpandExactTerms(title, item.link, strings.Join(item.keywords, " ")),
			URL:         item.link,
			Language:    scope.Language,
			Metadata: map[string]string{
				"tr_key": item.trKey,
				"source": "quick_links",
			},
			Access:    item.access,
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
