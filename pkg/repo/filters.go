package repo

import (
	"fmt"
	"reflect"
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
	Value() []any
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

func (f *eqFilter) Value() []any {
	return []any{f.value}
}

type notEqFilter struct {
	value any
}

func (f *notEqFilter) String(column string, argIdx int) string {
	return fmt.Sprintf("%s != $%d", column, argIdx)
}

func (f *notEqFilter) Value() []any {
	return []any{f.value}
}

type gtFilter struct {
	value any
}

func (f *gtFilter) String(column string, argIdx int) string {
	return fmt.Sprintf("%s > $%d", column, argIdx)
}

func (f *gtFilter) Value() []any {
	return []any{f.value}
}

type gteFilter struct {
	value any
}

func (f *gteFilter) String(column string, argIdx int) string {
	return fmt.Sprintf("%s >= $%d", column, argIdx)
}

func (f *gteFilter) Value() []any {
	return []any{f.value}
}

type ltFilter struct {
	value any
}

func (f *ltFilter) String(column string, argIdx int) string {
	return fmt.Sprintf("%s < $%d", column, argIdx)
}

func (f *ltFilter) Value() []any {
	return []any{f.value}
}

type lteFilter struct {
	value any
}

func (f *lteFilter) String(column string, argIdx int) string {
	return fmt.Sprintf("%s <= $%d", column, argIdx)
}

func (f *lteFilter) Value() []any {
	return []any{f.value}
}

type inFilter struct {
	value reflect.Value
}

func (f *inFilter) String(column string, argIdx int) string {
	placeholders := make([]string, f.value.Len())
	for i := range f.value.Len() {
		placeholders[i] = fmt.Sprintf("$%d", argIdx+i)
	}
	return fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ", "))
}

func (f *inFilter) Value() []any {
	var res []any
	for i := 0; i < f.value.Len(); i++ {
		res = append(res, f.value.Index(i).Interface())
	}
	return res
}

type notInFilter struct {
	value reflect.Value
}

func (f *notInFilter) String(column string, argIdx int) string {
	placeholders := make([]string, f.value.Len())
	for i := range f.value.Len() {
		placeholders[i] = fmt.Sprintf("$%d", argIdx+i)
	}
	return fmt.Sprintf("%s NOT IN (%s)", column, strings.Join(placeholders, ", "))
}

func (f *notInFilter) Value() []any {
	var res []any
	for i := 0; i < f.value.Len(); i++ {
		res = append(res, f.value.Index(i).Interface())
	}
	return res
}

type likeFilter struct {
	value any
}

func (f *likeFilter) String(column string, argIdx int) string {
	return fmt.Sprintf("%s LIKE $%d", column, argIdx)
}

func (f *likeFilter) Value() []any {
	return []any{f.value}
}

type notLikeFilter struct {
	value any
}

func (f *notLikeFilter) String(column string, argIdx int) string {
	return fmt.Sprintf("%s NOT LIKE $%d", column, argIdx)
}

func (f *notLikeFilter) Value() []any {
	return []any{f.value}
}

// ==============================
// === Filter Constructors    ===
// ==============================

func Eq(value any) Filter    { return &eqFilter{value} }
func NotEq(value any) Filter { return &notEqFilter{value} }
func Gt(value any) Filter    { return &gtFilter{value} }
func Gte(value any) Filter   { return &gteFilter{value} }
func Lt(value any) Filter    { return &ltFilter{value} }
func Lte(value any) Filter   { return &lteFilter{value} }
func In(value any) Filter {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Slice {
		panic("value must be a slice")
	}
	return &inFilter{v}
}
func NotIn(value any) Filter {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Slice {
		panic("value must be a slice")
	}
	return &notInFilter{v}
}
func Like(value any) Filter    { return &likeFilter{value} }
func NotLike(value any) Filter { return &notLikeFilter{value} }
