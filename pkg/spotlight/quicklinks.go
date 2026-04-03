// Package spotlight provides quick-link indexing and keyword helpers for the
// Spotlight search experience.
package spotlight

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/agnivade/levenshtein"
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
	index     map[string]int
	bundle    *i18n.Bundle
	languages []string
}

func NewQuickLinks(bundle *i18n.Bundle, languages []string) *QuickLinks {
	return &QuickLinks{
		items:     make([]*QuickLink, 0, 16),
		index:     make(map[string]int, 16),
		bundle:    bundle,
		languages: languages,
	}
}

func (ql *QuickLinks) Add(links ...*QuickLink) {
	ql.mu.Lock()
	defer ql.mu.Unlock()
	for _, link := range links {
		if link == nil {
			continue
		}
		key := quickLinkKey(link)
		if idx, exists := ql.index[key]; exists {
			ql.items[idx] = mergeQuickLinks(ql.items[idx], link)
			continue
		}
		ql.index[key] = len(ql.items)
		ql.items = append(ql.items, link)
	}
}

func (ql *QuickLinks) ProviderID() string {
	return "core.quick_links"
}

func (ql *QuickLinks) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{EntityTypes: []string{"quick_link", "route"}}
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

func (ql *QuickLinks) StreamDocuments(_ context.Context, scope ProviderScope, emit DocumentBatchEmitter) error {
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
			ExactTerms:  ExpandExactTerms(append([]string{title, item.link}, item.keywords...)...),
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
	if len(out) == 0 {
		return nil
	}
	return emit(out)
}

const (
	fuzzySearchMaxResults    = 8
	fuzzySearchMaxDistance   = 3
	fuzzyScoreExactPrefix    = 1.0
	fuzzyScoreContains       = 0.8
	fuzzyScoreLevenshteinMax = 0.6
)

// FuzzySearch performs in-memory fuzzy matching against all quick links,
// applying RBAC filtering based on the request's principal. Results are
// scored (exact prefix > substring > Levenshtein) and capped at 8.
func (ql *QuickLinks) FuzzySearch(query string, req SearchRequest) []SearchHit {
	if query == "" {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	queryWords := strings.Fields(query)

	principal, hasPrincipal := principalFromRequest(req)

	ql.mu.RLock()
	defer ql.mu.RUnlock()

	providerID := ql.ProviderID()
	type scored struct {
		hit   SearchHit
		score float64
	}
	candidates := make([]scored, 0, len(ql.items))

	for _, item := range ql.items {
		// RBAC check
		if item.access.Visibility != VisibilityPublic {
			if !hasPrincipal || !canReadPolicy(item.access, principal) {
				continue
			}
		}

		title, body := ql.resolveAllTranslations(item.trKey)

		// Collect all searchable text fragments (translations + keywords)
		searchableFragments := ql.collectSearchableFragments(title, body, item.keywords)

		best := ql.bestFuzzyScore(queryWords, searchableFragments)
		if best <= 0 {
			continue
		}

		// Build SearchDocument matching StreamDocuments shape
		fullBody := body
		if len(item.keywords) > 0 {
			fullBody = body + searchTextDelimiter + strings.Join(item.keywords, searchTextDelimiter)
		}

		doc := SearchDocument{
			ID:          providerID + ":" + item.trKey + ":" + item.link,
			TenantID:    req.TenantID,
			Provider:    providerID,
			EntityType:  "quick_link",
			Domain:      ResultDomainNavigate,
			Title:       title,
			Description: item.link,
			Body:        fullBody,
			SearchText:  fullBody,
			ExactTerms:  ExpandExactTerms(append([]string{title, item.link}, item.keywords...)...),
			URL:         item.link,
			Language:    req.Language,
			Metadata: map[string]string{
				"tr_key": item.trKey,
				"source": "quick_links",
			},
			Access:    item.access,
			UpdatedAt: item.createdAt,
		}

		candidates = append(candidates, scored{
			hit: SearchHit{
				Document:   doc,
				FinalScore: best,
				WhyMatched: "",
			},
			score: best,
		})
	}

	// Sort by score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	limit := fuzzySearchMaxResults
	if len(candidates) < limit {
		limit = len(candidates)
	}
	results := make([]SearchHit, limit)
	for i := 0; i < limit; i++ {
		results[i] = candidates[i].hit
	}
	return results
}

// collectSearchableFragments extracts individual lowercase text fragments
// from the title, body translations, and keywords.
func (ql *QuickLinks) collectSearchableFragments(title, body string, keywords []string) []string {
	fragments := make([]string, 0, 8)
	fragments = append(fragments, strings.ToLower(title))
	for _, part := range strings.Split(body, searchTextDelimiter) {
		part = strings.TrimSpace(part)
		if part != "" {
			fragments = append(fragments, strings.ToLower(part))
		}
	}
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw != "" {
			fragments = append(fragments, strings.ToLower(kw))
		}
	}
	return fragments
}

// bestFuzzyScore computes the best match score for the query words against
// the given text fragments.  Returns 0 if no match is good enough.
func (ql *QuickLinks) bestFuzzyScore(queryWords, fragments []string) float64 {
	var best float64
	for _, fragment := range fragments {
		for _, qw := range queryWords {
			s := scoreSingle(qw, fragment)
			if s > best {
				best = s
			}
		}
	}
	return best
}

// scoreSingle scores a single query word against a single text fragment.
func scoreSingle(queryWord, text string) float64 {
	// Exact prefix match (highest)
	if strings.HasPrefix(text, queryWord) {
		return fuzzyScoreExactPrefix
	}
	// Substring match
	if strings.Contains(text, queryWord) {
		return fuzzyScoreContains
	}
	// Also check individual words in the text
	for _, word := range strings.Fields(text) {
		if strings.HasPrefix(word, queryWord) {
			return fuzzyScoreExactPrefix * 0.95 // slightly lower than full-text prefix
		}
	}
	// Levenshtein distance on individual words
	for _, word := range strings.Fields(text) {
		dist := levenshtein.ComputeDistance(queryWord, word)
		if dist <= fuzzySearchMaxDistance {
			// Normalize: distance 0 = fuzzyScoreLevenshteinMax, distance 3 = ~0.1
			score := fuzzyScoreLevenshteinMax * (1.0 - float64(dist)/float64(fuzzySearchMaxDistance+1))
			if score > 0 {
				return score
			}
		}
	}
	return 0
}

func quickLinkKey(link *QuickLink) string {
	return link.trKey + "::" + link.link
}

func mergeQuickLinks(base, incoming *QuickLink) *QuickLink {
	if base == nil {
		return incoming
	}
	if incoming == nil {
		return base
	}
	merged := *base
	if incoming.trKey != "" {
		merged.trKey = incoming.trKey
	}
	if incoming.link != "" {
		merged.link = incoming.link
	}
	merged.access = mergeAccessPolicy(base.access, incoming.access)
	merged.keywords = mergeUniqueStrings(base.keywords, incoming.keywords)
	if incoming.createdAt.After(base.createdAt) {
		merged.createdAt = incoming.createdAt
	}
	return &merged
}

func mergeAccessPolicy(base, incoming AccessPolicy) AccessPolicy {
	if incoming.Visibility == "" {
		return base
	}
	base.Visibility = moreRestrictiveVisibility(base.Visibility, incoming.Visibility)
	if incoming.OwnerID != "" {
		base.OwnerID = incoming.OwnerID
	}
	base.AllowedUsers = mergeUniqueStrings(base.AllowedUsers, incoming.AllowedUsers)
	base.AllowedRoles = mergeUniqueStrings(base.AllowedRoles, incoming.AllowedRoles)
	base.AllowedPermissions = mergeUniqueStrings(base.AllowedPermissions, incoming.AllowedPermissions)
	return base
}

func mergeUniqueStrings(left, right []string) []string {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(left)+len(right))
	merged := make([]string, 0, len(left)+len(right))
	for _, values := range [][]string{left, right} {
		for _, value := range values {
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			if _, exists := seen[trimmed]; exists {
				continue
			}
			seen[trimmed] = struct{}{}
			merged = append(merged, trimmed)
		}
	}
	return merged
}

func moreRestrictiveVisibility(left, right Visibility) Visibility {
	if visibilityRank(right) > visibilityRank(left) {
		return right
	}
	if left == "" {
		return right
	}
	return left
}

func visibilityRank(value Visibility) int {
	switch value {
	case VisibilityOwner:
		return 3
	case VisibilityRestricted:
		return 2
	case VisibilityPublic:
		return 1
	default:
		return 0
	}
}
