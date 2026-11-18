package controllers

import (
	"context"
	"html/template"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/iota-uz/iota-sdk/modules/helpcenter/presentation/templates/pages/help"
	"github.com/iota-uz/iota-sdk/modules/helpcenter/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/helpcenter/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
)

// searchResultLimit is how many hits the search dropdown shows.
const searchResultLimit = 8

// searchExcerptRunes bounds the fallback excerpt taken from document content.
const searchExcerptRunes = 180

// sourceHitLimit caps the contributed-source title matches in the dropdown so
// they never crowd out the full-text hits of the file content.
const sourceHitLimit = searchResultLimit / 2

// searchFetchLimit over-fetches from the index. The KB index is shared: it
// holds every content locale and documents the Help Center hides, so cutting
// the index answer to the display limit first lets other-language and hidden
// hits crowd out the reader's own articles.
func (c *HelpCenterController) searchFetchLimit() int {
	return searchResultLimit * (len(c.contentService.Locales()) + 2)
}

// searchHits keeps the hits a reader can open, in index order, up to limit:
// the reader's content locale only, never a hidden path.
func (c *HelpCenterController) searchHits(ctx context.Context, tree []viewmodels.CategoryNode, results []kb.SearchResult, limit int) []viewmodels.SearchHit {
	locale := c.contentService.Locale(ctx)
	hits := make([]viewmodels.SearchHit, 0, limit)
	seen := make(map[string]struct{}, limit)
	for _, result := range results {
		rel, ok := localeRelPath(result.Document.Path, locale)
		if !ok || c.contentService.IsHidden(rel) {
			continue
		}
		if _, dup := seen[rel]; dup {
			continue
		}
		seen[rel] = struct{}{}
		hits = append(hits, viewmodels.SearchHit{
			Title:   result.Document.Title,
			Path:    rel,
			Section: searchSection(tree, rel),
			Excerpt: c.sanitizeSearchExcerpt(result),
			Score:   result.Score,
		})
		if len(hits) == limit {
			break
		}
	}
	return hits
}

// Title-match scores of sourceHits, best first.
const (
	sourceScorePhrase   = 1.0 // the whole query is part of the document title
	sourceScoreTitle    = 0.8 // every query word is part of the document title
	sourceScoreCategory = 0.5 // every query word is part of the title or of a category above it
)

// sourceHits matches the query against the titles of contributed-source
// documents and of the categories above them, in the tree the reader was
// served. Source documents are dynamic and differ per reader, so they are
// never put into the shared full-text index: matching the reader's own tree
// per request cannot reveal a document the reader may not see, and only titles
// are matched, not document bodies.
func (c *HelpCenterController) sourceHits(tree []viewmodels.CategoryNode, query string) []viewmodels.SearchHit {
	query = strings.ToLower(strings.Join(strings.Fields(query), " "))
	words := strings.Fields(query)
	if len(words) == 0 || !c.contentService.HasSources() {
		return nil
	}
	var hits []viewmodels.SearchHit
	var walk func(nodes []viewmodels.CategoryNode, categories []string)
	walk = func(nodes []viewmodels.CategoryNode, categories []string) {
		for _, node := range nodes {
			if node.Path == "" {
				walk(node.Children, append(slices.Clip(categories), node.Title))
				continue
			}
			if !c.contentService.IsSourcePath(node.Path) {
				continue
			}
			score := sourceMatchScore(query, words, node.Title, categories)
			if score == 0 {
				continue
			}
			section := ""
			if len(categories) > 0 {
				section = help.CategoryDisplayTitle(categories[len(categories)-1])
			}
			hits = append(hits, viewmodels.SearchHit{Title: node.Title, Path: node.Path, Section: section, Score: score})
		}
	}
	walk(tree, nil)
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > sourceHitLimit {
		hits = hits[:sourceHitLimit]
	}
	return hits
}

func sourceMatchScore(query string, words []string, title string, categories []string) float64 {
	lowerTitle := strings.ToLower(title)
	if strings.Contains(lowerTitle, query) {
		return sourceScorePhrase
	}
	if containsAll(lowerTitle, words) {
		return sourceScoreTitle
	}
	if containsAll(strings.ToLower(strings.Join(append(slices.Clone(categories), title), " ")), words) {
		return sourceScoreCategory
	}
	return 0
}

func containsAll(text string, words []string) bool {
	for _, word := range words {
		if !strings.Contains(text, word) {
			return false
		}
	}
	return true
}

// localeRelPath returns the document path inside <locale>/, or false when the
// indexed document belongs to another language.
func localeRelPath(docPath, locale string) (string, bool) {
	docPath = filepath.ToSlash(docPath)
	var rel string
	switch {
	case strings.HasPrefix(docPath, locale+"/"):
		rel = strings.TrimPrefix(docPath, locale+"/")
	case strings.Contains(docPath, "/"+locale+"/"):
		rel = docPath[strings.LastIndex(docPath, "/"+locale+"/")+len(locale)+2:]
	default:
		return "", false
	}
	rel = path.Clean(rel)
	return rel, rel != "." && rel != "" && !strings.HasPrefix(rel, "../")
}

// searchSection names the sidebar category that lists the document.
func searchSection(tree []viewmodels.CategoryNode, docPath string) string {
	nav, found := services.Locate(tree, docPath)
	if !found || len(nav.Categories) == 0 {
		return ""
	}
	return help.CategoryDisplayTitle(nav.Categories[len(nav.Categories)-1])
}

func (c *HelpCenterController) sanitizeSearchExcerpt(result kb.SearchResult) template.HTML {
	excerpt := strings.TrimSpace(result.Excerpt)
	if excerpt == "" && len(result.Fragments) > 0 {
		excerpt = strings.TrimSpace(result.Fragments[0])
	}
	if excerpt == "" {
		excerpt = truncateRunes(plainExcerpt(template.HTMLEscapeString(result.Document.Content)), searchExcerptRunes)
	} else {
		excerpt = plainExcerpt(excerpt)
	}
	return template.HTML(c.snippetPolicy.Sanitize(excerpt))
}

func truncateRunes(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	runes := []rune(text)
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

var (
	excerptLinkTarget = regexp.MustCompile(`\]\([^)\s]*\)?|[\w/.-]*\.md\)`)
	excerptLinkOpen   = regexp.MustCompile(`!?\[`)
	excerptMarkup     = regexp.MustCompile("(?m)^\\s*(?:#{1,6}|>|[-*+]|\\d+\\.)\\s+|\\s[-*+]\\s+|#{1,6}\\s+|\\[![A-Z]+\\]|\\*\\*|__|`+|\\|")
	excerptSpace      = regexp.MustCompile(`\s+`)
	// A table header rule survives the pipe removal as «--- ---».
	excerptTableRule = regexp.MustCompile(`(?:^|\s):?-{3,}:?(?:\s:?-{3,}:?)*(?:\s|$)`)
	// Article HTML reaches the excerpt escaped by the index highlighter. Only
	// that escaped markup goes, including a tag cut off at the fragment
	// boundary; the highlighter's real <mark> tags stay and arbitrary article
	// HTML is never unescaped.
	excerptArticleHTML = regexp.MustCompile(`(?i)&lt;/?(?:details|summary|p|strong|a|em|code|pre|ul|ol|li|table|thead|tbody|tr|th|td|h[1-6]|blockquote|br|hr|img|span|div|small|b)\b[^<]*?(?:&gt;|$|<)`)
	excerptEntities    = strings.NewReplacer("&gt;", ">", "&#34;", `"`, "&quot;", `"`, "&#39;", "'")
	// Mermaid diagrams are indexed as source: «E --> Исправление F"Переоформление"».
	excerptDiagram = regexp.MustCompile(`\bclick\s+\w+\s+href(?:\s+"[^"]*")*|\b\w+\s*[-=.]{2,}>|\|[^|]*\||\b[A-Za-z]\w*[\[({]+"?|"[\])}]+|\]|"`)
)

// plainExcerpt turns an index fragment back into reading text. The index holds
// raw markdown, so a hit read «## Goal Create an OSAGO…» or ended in
// «](/help/doc/…)». Only the markup goes; the highlight <mark> tags stay.
func plainExcerpt(excerpt string) string {
	excerpt = excerptArticleHTML.ReplaceAllStringFunc(excerpt, func(tag string) string {
		if strings.HasSuffix(tag, "<") {
			return " <"
		}
		return " "
	})
	excerpt = excerptEntities.Replace(excerpt)
	excerpt = excerptMarkup.ReplaceAllString(excerpt, " ")
	excerpt = excerptTableRule.ReplaceAllString(excerpt, " ")
	excerpt = excerptLinkTarget.ReplaceAllString(excerpt, "")
	excerpt = excerptDiagram.ReplaceAllString(excerpt, " ")
	excerpt = excerptLinkOpen.ReplaceAllString(excerpt, "")
	return strings.TrimSpace(excerptSpace.ReplaceAllString(excerpt, " "))
}
