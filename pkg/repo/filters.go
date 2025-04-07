package repo

import (
	"fmt"
	"strings"
)

type FieldFilter[T any] struct {
	Column T
	Filter Filter
}

// SortBy defines sorting criteria for queries with generic field type support.
// Use with OrderBy function to generate ORDER BY clauses.
type SortBy[T any] struct {
	// Fields represents the list of fields to sort by.
	Fields []T
	// Ascending indicates the sort direction (true for ASC, false for DESC).
	Ascending bool
}

// Filter defines a query filter with a SQL clause generator and bound value.
type Filter interface {
	Value() any
	String(column string, argIdx int) string
}

// =====================
// === Filter Types  ===
// =====================

type eqFilter struct {
	value any
}

func (f *eqFilter) String(column string, argIdx int) string {
	return fmt.Sprintf("%s = $%d", column, argIdx)
}

func (f *eqFilter) Value() any {
	return f.value
}

type notEqFilter struct {
	value any
}

func (f *notEqFilter) String(column string, argIdx int) string {
	return fmt.Sprintf("%s != $%d", column, argIdx)
}

func (f *notEqFilter) Value() any {
	return f.value
}

type gtFilter struct {
	value any
}

func (f *gtFilter) String(column string, argIdx int) string {
	return fmt.Sprintf("%s > $%d", column, argIdx)
}

func (f *gtFilter) Value() any {
	return f.value
}

type gteFilter struct {
	value any
}

func (f *gteFilter) String(column string, argIdx int) string {
	return fmt.Sprintf("%s >= $%d", column, argIdx)
}

func (f *gteFilter) Value() any {
	return f.value
}

type ltFilter struct {
	value any
}

func (f *ltFilter) String(column string, argIdx int) string {
	return fmt.Sprintf("%s < $%d", column, argIdx)
}

func (f *ltFilter) Value() any {
	return f.value
}

type lteFilter struct {
	value any
}

func (f *lteFilter) String(column string, argIdx int) string {
	return fmt.Sprintf("%s <= $%d", column, argIdx)
}

func (f *lteFilter) Value() any {
	return f.value
}

type inFilter struct {
	value any
}

func (f *inFilter) String(column string, argIdx int) string {
	switch v := f.value.(type) {
	case []interface{}:
		// Format as column IN (value1, value2, ...) with individual parameters
		placeholders := make([]string, len(v))
		for i := range v {
			placeholders[i] = fmt.Sprintf("$%d", argIdx+i)
		}
		return fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ", "))
	default:
		// Fall back to original behavior
		return fmt.Sprintf("%s IN ($%d)", column, argIdx)
	}
}

func (f *inFilter) Value() any {
	return f.value
}

type notInFilter struct {
	value any
}

func (f *notInFilter) String(column string, argIdx int) string {
	switch v := f.value.(type) {
	case []interface{}:
		// Format as column NOT IN (value1, value2, ...) with individual parameters
		placeholders := make([]string, len(v))
		for i := range v {
			placeholders[i] = fmt.Sprintf("$%d", argIdx+i)
		}
		return fmt.Sprintf("%s NOT IN (%s)", column, strings.Join(placeholders, ", "))
	default:
		// Fall back to original behavior
		return fmt.Sprintf("%s NOT IN ($%d)", column, argIdx)
	}
}

func (f *notInFilter) Value() any {
	return f.value
}

type likeFilter struct {
	value any
}

func (f *likeFilter) String(column string, argIdx int) string {
	return fmt.Sprintf("%s LIKE $%d", column, argIdx)
}

func (f *likeFilter) Value() any {
	return f.value
}

type notLikeFilter struct {
	value any
}

func (f *notLikeFilter) String(column string, argIdx int) string {
	return fmt.Sprintf("%s NOT LIKE $%d", column, argIdx)
}

func (f *notLikeFilter) Value() any {
	return f.value
}

// ==============================
// === Filter Constructors    ===
// ==============================

func Eq(value any) Filter      { return &eqFilter{value} }
func NotEq(value any) Filter   { return &notEqFilter{value} }
func Gt(value any) Filter      { return &gtFilter{value} }
func Gte(value any) Filter     { return &gteFilter{value} }
func Lt(value any) Filter      { return &ltFilter{value} }
func Lte(value any) Filter     { return &lteFilter{value} }
func In(value any) Filter      { return &inFilter{value} }
func NotIn(value any) Filter   { return &notInFilter{value} }
func Like(value any) Filter    { return &likeFilter{value} }
func NotLike(value any) Filter { return &notLikeFilter{value} }
