// Package historylink links lists and cards to the business history page.
package historylink

import (
	"context"
	"net/url"

	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/components/base/button"
	"github.com/iota-uz/iota-sdk/components/scaffold/actions"
	"github.com/iota-uz/iota-sdk/modules/logging/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

// BasePath is where the logging component mounts the history page by default.
const BasePath = "/admin/history"

// URL opens the history of every object of the resource, or of one object when id is set.
func URL(resource, id string) string {
	values := url.Values{"resource": {resource}}
	if id != "" {
		values.Set("id", id)
	}
	return BasePath + "?" + values.Encode()
}

// Visible reports whether the user may open the history page; the page still checks
// access to the resource itself.
func Visible(ctx context.Context) bool {
	u, err := composables.UseUser(ctx)
	return err == nil && u != nil && u.Can(permissions.ViewLogs)
}

func Label(ctx context.Context) string {
	return intl.TWithDefault(ctx, "BusinessHistory.Link", "History")
}

// Action is a toolbar button for a list.
func Action(ctx context.Context, resource string) actions.ActionProps {
	return actions.CustomAction(Label(ctx), icons.ClockCounterClockwise(icons.Props{Size: "18"}), actions.WithHref(URL(resource, "")))
}

// RowAction is an icon button for a row of a list.
func RowAction(ctx context.Context, resource, id string) actions.ActionProps {
	return actions.CustomAction("", icons.ClockCounterClockwise(icons.Props{Size: "20"}),
		actions.WithHref(URL(resource, id)),
		actions.WithSize(button.SizeSM),
		actions.WithAttrs(map[string]any{"class": "btn-fixed", "title": Label(ctx)}),
	)
}
