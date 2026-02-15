package spotlight

import (
	"context"
	"html"
	"io"

	"github.com/a-h/templ"
	spotlightui "github.com/iota-uz/iota-sdk/components/spotlight"
)

func HitToComponent(hit SearchHit) templ.Component {
	return spotlightui.LinkItem(hit.Document.Title, hit.Document.URL, nil)
}

func GroupToComponent(title string, items []templ.Component, startIdx int) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		child := spotlightui.SpotlightItems(items, startIdx)
		return spotlightui.SpotlightGroup(title).Render(templ.WithChildren(ctx, child), w)
	})
}

func ActionToComponent(action AgentAction) templ.Component {
	if !action.NeedsConfirmation {
		return spotlightui.LinkItem(action.Label, action.TargetURL, nil)
	}
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		escapedURL := html.EscapeString(action.TargetURL)
		escapedLabel := html.EscapeString(action.Label)
		_, err := io.WriteString(w, `<button type="button" class="text-left text-sm text-primary-700 hover:underline" onclick="spotlightConfirmAndGo('`+escapedURL+`')">`+escapedLabel+`</button>`)
		return err
	})
}
