package notifications

import (
	"github.com/iota-uz/iota-sdk/components/scaffold/filterbuilder"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/pkg/filterq"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
)

// unreadField is the chip key of the read state (see the controller registry).
const unreadField = "unread"

type Props struct {
	Notifications []notification.Notification
	UnreadCount   int64
	Cursor        string
	NextCursor    string
	HasMore       bool
	// Filters is the filter chip bar; a nil Registry hides it.
	Filters filterbuilder.Props
}

// UnreadOnly reports whether the chips show only unread notifications.
func (p *Props) UnreadOnly() bool {
	for _, c := range p.Filters.Filters.Field(unreadField) {
		if c.FlagValue() == "true" {
			return true
		}
	}
	return false
}

// AllActive reports whether the "All" tab is current: no chip on read state.
func (p *Props) AllActive() bool { return !p.Filters.Filters.Has(unreadField) }

// URL links a page of the list, keeping the chips.
func (p *Props) URL(cursor string) string {
	return p.link(p.Filters.Filters, cursor)
}

// TabURL links the "All" or "Unread" tab: the current chips with the read
// state chip replaced, from the first page.
func (p *Props) TabURL(unread bool) string {
	fs := p.Filters.Filters.Without(unreadField)
	if unread {
		fs = append(fs, filterq.Condition{Field: unreadField, Op: filterq.OpIs, Values: []string{"true"}})
	}
	return p.link(fs, "")
}

func (p *Props) link(fs filterq.FilterSet, cursor string) string {
	q := filterq.Encode(fs)
	q.Set(filterq.PresenceParam, "1")
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	return "/notifications?" + q.Encode()
}

func safeActionURL(raw string) string {
	safe, ok := action.SafeRelativeURL(raw)
	if !ok {
		return ""
	}
	return safe
}
