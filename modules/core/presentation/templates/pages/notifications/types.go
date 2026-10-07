package notifications

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
)

type Props struct {
	Notifications []notification.Notification
	UnreadCount   int64
	Page          int
	UnreadOnly    bool
	HasMore       bool
}

func (p *Props) URL(page int) string {
	return fmt.Sprintf("/notifications?page=%d&unread=%t", page, p.UnreadOnly)
}
func safeActionURL(raw string) string {
	safe, ok := action.SafeRelativeURL(raw)
	if !ok {
		return ""
	}
	return safe
}
