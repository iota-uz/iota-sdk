// Package spotlight provides this package.
package spotlight

import (
	"context"
	"fmt"
	"html"
	"io"
	"regexp"
	"strings"
	"text/template"

	"github.com/a-h/templ"
	"github.com/iota-uz/go-i18n/v2/i18n"
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/sirupsen/logrus"
	spotlightui "github.com/iota-uz/iota-sdk/components/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

func iconForEntityType(doc SearchDocument) templ.Component {
	size := "16"
	if icon := iconByName(strings.TrimSpace(doc.Metadata["icon_name"]), size); icon != nil {
		return icon
	}
	switch doc.EntityType {
	case "quick_link", "route", "page", "navigation":
		return icons.Compass(icons.Props{Size: size})
	case "user":
		return icons.User(icons.Props{Size: size})
	case "group", "role", "staff":
		return icons.IdentificationCard(icons.Props{Size: size})
	case "client":
		return icons.Users(icons.Props{Size: size})
	case "chat", "conversation", "thread":
		return icons.ChatCircleText(icons.Props{Size: size})
	case "project":
		return icons.Folder(icons.Props{Size: size})
	case "order":
		return icons.Cube(icons.Props{Size: size})
	case "knowledge", "kb", "doc", "docs":
		return icons.Book(icons.Props{Size: size})
	default:
		return icons.File(icons.Props{Size: size})
	}
}

func HitToComponent(hit SearchHit, query string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		title := resolveDisplayTitle(ctx, hit.Document.Title, hit.Document.Metadata["tr_key"])
		if title == "" {
			return nil
		}
		return spotlightui.LinkItem(spotlightui.LinkItemProps{
			Key:      resultKey(hit),
			Title:    title,
			Subtitle: resultSubtitle(hit.Document, title),
			Meta:     resultMeta(ctx, hit, query),
			Link:     hit.Document.URL,
			Badges:   resultBadges(ctx, hit),
			Icon:     iconForEntityType(hit.Document),
		}).Render(ctx, w)
	})
}

func iconByName(name, size string) templ.Component {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "":
		return nil
	case "compass", "navigate":
		return icons.Compass(icons.Props{Size: size})
	case "user":
		return icons.User(icons.Props{Size: size})
	case "id-card", "identification-card":
		return icons.IdentificationCard(icons.Props{Size: size})
	case "users", "people":
		return icons.Users(icons.Props{Size: size})
	case "file-text":
		return icons.FileText(icons.Props{Size: size})
	case "car", "car-profile":
		return icons.CarProfile(icons.Props{Size: size})
	case "warning", "seal-warning":
		return icons.SealWarning(icons.Props{Size: size})
	case "chat", "chat-circle-text":
		return icons.ChatCircleText(icons.Props{Size: size})
	case "buildings", "organization":
		return icons.Buildings(icons.Props{Size: size})
	case "folder", "project":
		return icons.Folder(icons.Props{Size: size})
	case "cube", "order":
		return icons.Cube(icons.Props{Size: size})
	case "book", "knowledge":
		return icons.Book(icons.Props{Size: size})
	default:
		return nil
	}
}

func displayWhyMatched(ctx context.Context, reason string) string {
	switch reason {
	case "", "meilisearch":
		return ""
	case "exact_terms":
		return spotlightText(ctx, "Spotlight.Match.Exact", "Exact match")
	default:
		return reason
	}
}

func GroupToComponent(title string, items []templ.Component, startIdx int, hits []SearchHit) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		child := buildGroupChild(ctx, items, startIdx, hits)
		return spotlightui.SpotlightGroup(title).Render(templ.WithChildren(ctx, child), w)
	})
}

// maxVisibleWithoutExact is the number of items shown before collapsing
// in groups that have no exact matches.
const maxVisibleWithoutExact = 3

// buildGroupChild decides whether to collapse items in the group and returns
// the appropriate templ component.
func buildGroupChild(ctx context.Context, items []templ.Component, startIdx int, hits []SearchHit) templ.Component {
	if len(hits) != len(items) {
		logrus.WithFields(logrus.Fields{
			"hits": len(hits), "items": len(items),
		}).Warn("spotlight: GroupToComponent hits/items length mismatch, skipping collapse")
		return spotlightui.SpotlightItems(items, startIdx)
	}

	// Check if this group has any exact match
	hasExact := false
	for _, hit := range hits {
		if hit.WhyMatched == "exact_terms" {
			hasExact = true
			break
		}
	}

	if hasExact {
		return buildExactCollapse(ctx, items, startIdx, hits)
	}
	if len(items) > maxVisibleWithoutExact {
		return buildScoreCollapse(ctx, items, startIdx)
	}
	return spotlightui.SpotlightItems(items, startIdx)
}

// buildExactCollapse shows exact-match items and collapses non-exact.
func buildExactCollapse(ctx context.Context, items []templ.Component, startIdx int, hits []SearchHit) templ.Component {
	var exactItems, moreItems []templ.Component
	var exactIndices, moreIndices []int
	idx := startIdx
	for i, hit := range hits {
		if hit.WhyMatched == "exact_terms" {
			exactItems = append(exactItems, items[i])
			exactIndices = append(exactIndices, idx)
			idx++
		}
	}
	for i, hit := range hits {
		if hit.WhyMatched != "exact_terms" {
			moreItems = append(moreItems, items[i])
			moreIndices = append(moreIndices, idx)
			idx++
		}
	}
	if len(moreItems) == 0 {
		return spotlightui.SpotlightItems(items, startIdx)
	}
	label := moreResultsLabel(ctx, len(moreItems))
	return spotlightui.SpotlightItemsCollapsible(exactItems, exactIndices, moreItems, moreIndices, label)
}

// buildScoreCollapse shows the top N items by score and collapses the rest.
func buildScoreCollapse(ctx context.Context, items []templ.Component, startIdx int) templ.Component {
	visible := items[:maxVisibleWithoutExact]
	more := items[maxVisibleWithoutExact:]

	visibleIndices := make([]int, len(visible))
	for i := range visible {
		visibleIndices[i] = startIdx + i
	}
	moreIndices := make([]int, len(more))
	for i := range more {
		moreIndices[i] = startIdx + maxVisibleWithoutExact + i
	}
	label := moreResultsLabel(ctx, len(more))
	return spotlightui.SpotlightItemsCollapsible(visible, visibleIndices, more, moreIndices, label)
}

func moreResultsLabel(ctx context.Context, count int) string {
	if localizer, ok := intl.UseLocalizer(ctx); ok {
		if translated, err := localizer.Localize(&i18n.LocalizeConfig{
			MessageID: "Spotlight.MoreResults",
			DefaultMessage: &i18n.Message{
				ID:    "Spotlight.MoreResults",
				One:   "{{.Count}} more result",
				Other: "{{.Count}} more results",
			},
			TemplateData: map[string]interface{}{"Count": count},
			PluralCount:  count,
		}); err == nil && translated != "" {
			return translated
		}
	}
	return executeTemplateFallback("{{.Count}} more results", map[string]interface{}{"Count": count})
}

// resolveDisplayTitle localizes a title at render time using the request's localizer.
// If trKey is empty or no localizer is available, it returns the original title.
func resolveDisplayTitle(ctx context.Context, title, trKey string) string {
	if trKey == "" {
		return title
	}
	if localizer, ok := intl.UseLocalizer(ctx); ok {
		if translated, err := localizer.Localize(&i18n.LocalizeConfig{
			MessageID: trKey,
			DefaultMessage: &i18n.Message{
				ID:    trKey,
				Other: trKey,
			},
		}); err == nil && translated != trKey {
			return translated
		}
	}
	return title
}

func executeTemplateFallback(tmpl string, data map[string]interface{}) string {
	t, err := template.New("").Parse(tmpl)
	if err != nil {
		return tmpl
	}
	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return tmpl
	}
	return buf.String()
}

func spotlightText(ctx context.Context, key, fallback string) string {
	if localizer, ok := intl.UseLocalizer(ctx); ok {
		if translated, err := localizer.Localize(&i18n.LocalizeConfig{
			MessageID: key,
			DefaultMessage: &i18n.Message{
				ID:    key,
				Other: fallback,
			},
		}); err == nil && translated != "" {
			return translated
		}
	}
	return fallback
}

func resultKey(hit SearchHit) string {
	if hit.Document.ID != "" {
		return hit.Document.ID
	}
	if hit.Document.URL != "" {
		return hit.Document.URL
	}
	return hit.Document.Title
}

func resultSubtitle(doc SearchDocument, title string) string {
	candidates := []string{doc.Description, doc.Body}
	title = strings.TrimSpace(title)
	for _, candidate := range candidates {
		lines := splitSummaryLines(candidate)
		for _, line := range lines {
			if line == "" || line == title {
				continue
			}
			return line
		}
	}
	return ""
}

func splitSummaryLines(value string) []string {
	raw := strings.FieldsFunc(value, func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func resultMeta(ctx context.Context, hit SearchHit, query string) string {
	reason := displayWhyMatched(ctx, hit.WhyMatched)
	query = strings.TrimSpace(query)
	switch {
	case reason != "" && query != "":
		return fmt.Sprintf("%s: %s", reason, query)
	case reason != "":
		return reason
	case query != "" && strings.EqualFold(strings.TrimSpace(hit.Document.Title), query):
		return fmt.Sprintf("%s: %s", spotlightText(ctx, "Spotlight.Match.Best", "Best match"), query)
	default:
		return ""
	}
}

func resultBadges(ctx context.Context, hit SearchHit) []spotlightui.ResultBadge {
	badges := make([]spotlightui.ResultBadge, 0, 2)
	if entityBadge := entityBadgeForHit(ctx, hit.Document); entityBadge.Label != "" {
		badges = append(badges, entityBadge)
	}
	if hit.WhyMatched == "exact_terms" {
		badges = append(badges, spotlightui.ResultBadge{
			Label: spotlightText(ctx, "Spotlight.Badge.Exact", "Exact"),
			Tone:  "exact",
		})
	}
	return badges
}

func entityBadgeForHit(ctx context.Context, doc SearchDocument) spotlightui.ResultBadge {
	if key := strings.TrimSpace(doc.Metadata["badge_label_key"]); key != "" {
		return spotlightui.ResultBadge{
			Label: spotlightText(ctx, key, humanizeEntityType(doc.EntityType)),
			Tone:  badgeToneForEntity(doc),
		}
	}
	if label := strings.TrimSpace(doc.Metadata["badge_label"]); label != "" {
		return spotlightui.ResultBadge{
			Label: label,
			Tone:  badgeToneForEntity(doc),
		}
	}

	switch strings.ToLower(strings.TrimSpace(doc.EntityType)) {
	case "user", "group", "role":
		return spotlightui.ResultBadge{Label: spotlightText(ctx, "Spotlight.Badge.Staff", "Staff"), Tone: "directory"}
	case "client", "person":
		return spotlightui.ResultBadge{Label: spotlightText(ctx, "Spotlight.Badge.Person", "Person"), Tone: "directory"}
	case "quick_link", "route", "page", "navigation":
		return spotlightui.ResultBadge{Label: spotlightText(ctx, "Spotlight.Badge.Navigate", "Navigate"), Tone: "navigation"}
	case "chat", "conversation", "thread":
		return spotlightui.ResultBadge{Label: humanizeEntityType(doc.EntityType), Tone: "conversation"}
	default:
		if label := humanizeEntityType(doc.EntityType); label != "" {
			return spotlightui.ResultBadge{Label: label, Tone: badgeToneForEntity(doc)}
		}
		return spotlightui.ResultBadge{}
	}
}

func badgeToneForEntity(doc SearchDocument) string {
	if tone := strings.TrimSpace(doc.Metadata["badge_tone"]); tone != "" {
		return tone
	}
	switch strings.ToLower(strings.TrimSpace(doc.EntityType)) {
	case "quick_link", "route", "page", "navigation":
		return "navigation"
	case "user", "group", "role", "staff", "person", "client":
		return "directory"
	case "chat", "conversation", "thread":
		return "conversation"
	default:
		return "entity"
	}
}

var entityWordBoundary = regexp.MustCompile(`[_\-\s]+`)

func humanizeEntityType(entityType string) string {
	entityType = strings.TrimSpace(entityType)
	if entityType == "" {
		return ""
	}
	parts := entityWordBoundary.Split(entityType, -1)
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}

func ActionToComponent(action AgentAction) templ.Component {
	if !action.NeedsConfirmation {
		return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			label := resolveDisplayTitle(ctx, action.Label, action.LabelTrKey)
			icon := icons.ArrowSquareOut(icons.Props{Size: "16"})
			return spotlightui.LinkItem(spotlightui.LinkItemProps{
				Key:    action.TargetURL,
				Title:  label,
				Link:   action.TargetURL,
				Icon:   icon,
				Badges: []spotlightui.ResultBadge{{Label: spotlightText(ctx, "Spotlight.Badge.Action", "Action"), Tone: "match"}},
			}).Render(ctx, w)
		})
	}
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		label := resolveDisplayTitle(ctx, action.Label, action.LabelTrKey)
		escapedURL := html.EscapeString(action.TargetURL)
		escapedLabel := html.EscapeString(label)
		_, err := io.WriteString(w, `<button type="button" class="js-spotlight-confirm text-left text-sm text-primary-700 hover:underline" data-spotlight-url="`+escapedURL+`">`+escapedLabel+`</button>`)
		return err
	})
}
