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

// Re-export SortDirection from base package for backwards compatibility
type SortDirection = base.SortDirection

const (
	SortDirectionNone = base.SortDirectionNone
	SortDirectionAsc  = base.SortDirectionAsc
	SortDirectionDesc = base.SortDirectionDesc
)

type StickyPosition = base.StickyPosition

const (
	StickyPositionLeft  = base.StickyPositionLeft
	StickyPositionRight = base.StickyPositionRight
	StickyPositionNone  = base.StickyPositionNone
)

// Re-export function for backwards compatibility
func ParseSortDirection(value string) SortDirection {
	return base.ParseSortDirection(value)
}
