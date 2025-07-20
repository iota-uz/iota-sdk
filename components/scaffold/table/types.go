package table

import "github.com/iota-uz/iota-sdk/components/base"

// Re-export SortDirection from base package for backwards compatibility
type SortDirection = base.SortDirection

const (
	SortDirectionNone = base.SortDirectionNone
	SortDirectionAsc  = base.SortDirectionAsc
	SortDirectionDesc = base.SortDirectionDesc
)

// Re-export function for backwards compatibility
func ParseSortDirection(value string) SortDirection {
	return base.ParseSortDirection(value)
}
