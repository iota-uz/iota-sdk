package notifications

import (
	"fmt"
	"net/url"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
)

type Props struct {
	Notifications []notification.Notification
	UnreadCount   int64
	Cursor        string
	NextCursor    string
	UnreadOnly    bool
	HasMore       bool
}

func (p *Props) URL(cursor string) string {
	return "/notifications?cursor=" + url.QueryEscape(cursor) + "&unread=" + fmt.Sprint(p.UnreadOnly)
}
func safeActionURL(raw string) string {
	safe, ok := action.SafeRelativeURL(raw)
	if !ok {
		return ""
	}
	return safe
}
