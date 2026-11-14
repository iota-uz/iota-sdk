package table

import "github.com/iota-uz/iota-sdk/components/base"

// Query parameter constants
const (
	QueryParamSearch = "Search"
	QueryParamPage   = "page"
	QueryParamLimit  = "limit"
	QueryParamSort   = "sort"
	QueryParamOrder  = "order"
)

// SortDirection re-exports the base sort direction type.
type SortDirection = base.SortDirection

const (
	SortDirectionNone = base.SortDirectionNone
	SortDirectionAsc  = base.SortDirectionAsc
	SortDirectionDesc = base.SortDirectionDesc
)

type StickyPosition = base.StickyPosition
type Addon = base.Addon

const (
	StickyPositionLeft  = base.StickyPositionLeft
	StickyPositionRight = base.StickyPositionRight
	StickyPositionNone  = base.StickyPositionNone
)

type ScrollbarPosition = base.ScrollbarPosition

const (
	ScrollbarPositionTop    = base.ScrollbarPositionTop
	ScrollbarPositionBottom = base.ScrollbarPositionBottom
)

// ParseSortDirection parses sort direction from a string.
func ParseSortDirection(value string) SortDirection {
	return base.ParseSortDirection(value)
}

// MobileRole is the part a column plays in the card layout that tables use on
// narrow screens (below 640px). The zero value lets the table pick a role.
type MobileRole string

const (
	// MobileRoleAuto picks a role: the first column becomes the primary one
	// unless another column declares it, a right-pinned column or one keyed
	// "actions" (row actions) is hidden when the row itself is clickable, columns that hide on
	// tablets stay hidden, and everything else is secondary.
	MobileRoleAuto MobileRole = ""
	// MobileRolePrimary is the card title.
	MobileRolePrimary MobileRole = "primary"
	// MobileRoleAmount is shown at the card's right edge in a larger font.
	MobileRoleAmount MobileRole = "amount"
	// MobileRoleSecondary is a small supporting line under the title.
	MobileRoleSecondary MobileRole = "secondary"
	// MobileRoleHidden is not drawn in the card layout.
	MobileRoleHidden MobileRole = "hidden-mobile"
)

// Valid reports whether r is one of the declared roles.
func (r MobileRole) Valid() bool {
	switch r {
	case MobileRoleAuto, MobileRolePrimary, MobileRoleAmount, MobileRoleSecondary, MobileRoleHidden:
		return true
	}
	return false
}

// MobileLayout selects how a table is drawn below 640px.
type MobileLayout string

const (
	// MobileLayoutAuto draws rows as cards, except in editable tables.
	MobileLayoutAuto MobileLayout = ""
	// MobileLayoutCards draws every row as a card.
	MobileLayoutCards MobileLayout = "cards"
	// MobileLayoutTable keeps the regular table, scrolled sideways.
	MobileLayoutTable MobileLayout = "table"
)
