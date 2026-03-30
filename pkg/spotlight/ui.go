// Package spotlight provides this package.
package spotlight

import (
	"context"
	"fmt"
	"html"
	"io"
	"strings"

	"github.com/a-h/templ"
	"github.com/iota-uz/go-i18n/v2/i18n"
	icons "github.com/iota-uz/icons/phosphor"
	spotlightui "github.com/iota-uz/iota-sdk/components/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

func iconForEntityType(entityType string) templ.Component {
	size := "16"
	switch entityType {
	case "quick_link", "route", "page", "navigation":
		return icons.Compass(icons.Props{Size: size})
	case "user":
		return icons.User(icons.Props{Size: size})
	case "group", "role", "staff":
		return icons.IdentificationCard(icons.Props{Size: size})
	case "client":
		return icons.Users(icons.Props{Size: size})
	case "policy":
		return icons.FileText(icons.Props{Size: size})
	case "vehicle":
		return icons.CarProfile(icons.Props{Size: size})
	case "claim":
		return icons.SealWarning(icons.Props{Size: size})
	case "chat", "conversation", "thread":
		return icons.ChatCircleText(icons.Props{Size: size})
	case "organization":
		return icons.Buildings(icons.Props{Size: size})
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
			Icon:     iconForEntityType(hit.Document.EntityType),
		}).Render(ctx, w)
	})
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

func GroupToComponent(title string, items []templ.Component, startIdx int) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		child := spotlightui.SpotlightItems(items, startIdx)
		return spotlightui.SpotlightGroup(title).Render(templ.WithChildren(ctx, child), w)
	})
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
	if entityBadge := entityBadgeForHit(ctx, hit.Document.EntityType); entityBadge.Label != "" {
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

func entityBadgeForHit(ctx context.Context, entityType string) spotlightui.ResultBadge {
	switch strings.ToLower(strings.TrimSpace(entityType)) {
	case "policy":
		return spotlightui.ResultBadge{Label: spotlightText(ctx, "Spotlight.Badge.Policy", "Policy"), Tone: "policy"}
	case "vehicle":
		return spotlightui.ResultBadge{Label: spotlightText(ctx, "Spotlight.Badge.Vehicle", "Vehicle"), Tone: "vehicle"}
	case "claim":
		return spotlightui.ResultBadge{Label: spotlightText(ctx, "Spotlight.Badge.Claim", "Claim"), Tone: "claim"}
	case "chat", "conversation", "thread":
		return spotlightui.ResultBadge{Label: spotlightText(ctx, "Spotlight.Badge.Chat", "Chat"), Tone: "chat"}
	case "organization":
		return spotlightui.ResultBadge{Label: spotlightText(ctx, "Spotlight.Badge.Organization", "Organization"), Tone: "organization"}
	case "user", "group", "role":
		return spotlightui.ResultBadge{Label: spotlightText(ctx, "Spotlight.Badge.Staff", "Staff"), Tone: "staff"}
	case "client", "person":
		return spotlightui.ResultBadge{Label: spotlightText(ctx, "Spotlight.Badge.Person", "Person"), Tone: "people"}
	case "quick_link", "route", "page", "navigation":
		return spotlightui.ResultBadge{Label: spotlightText(ctx, "Spotlight.Badge.Navigate", "Navigate"), Tone: "navigation"}
	default:
		return spotlightui.ResultBadge{}
	}
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
