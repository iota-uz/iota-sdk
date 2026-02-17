package spotlight

import (
	"context"
	"html"
	"io"

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
	case "client":
		return icons.Users(icons.Props{Size: size})
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

func HitToComponent(hit SearchHit) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		title := resolveDisplayTitle(ctx, hit.Document.Title, hit.Document.Metadata["tr_key"])
		// Skip items with empty titles
		if title == "" {
			return nil
		}
		icon := iconForEntityType(hit.Document.EntityType)
		return spotlightui.LinkItem(title, hit.Document.URL, hit.Document.URL, icon).Render(ctx, w)
	})
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

func ActionToComponent(action AgentAction) templ.Component {
	if !action.NeedsConfirmation {
		return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			label := resolveDisplayTitle(ctx, action.Label, action.LabelTrKey)
			icon := icons.ArrowSquareOut(icons.Props{Size: "16"})
			return spotlightui.LinkItem(label, "", action.TargetURL, icon).Render(ctx, w)
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
