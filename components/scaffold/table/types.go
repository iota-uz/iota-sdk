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
