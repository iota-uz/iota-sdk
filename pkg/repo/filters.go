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

type Column interface {
	ToSQL() string
}

// SortBy defines sorting criteria for queries with generic field type support.
// Use with OrderBy function to generate ORDER BY clauses.
type SortBy[T any] struct {
	// Fields represents the list of fields to sort by.
	Fields []T
	// Ascending indicates the sort direction (true for ASC, false for DESC).
	Ascending bool
}

//func (s *SortBy[T]) ToSQL() string {
//	fields := make([]string, len(s.Fields))
//	for i, field := range s.Fields {
//		fields[i] = field.ToSQL()
//	}
//	return OrderBy(fields, s.Ascending)
//}

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

type iLikeFilter struct {
	value any
}

func (f *iLikeFilter) String(column string, argIdx int) string {
	return fmt.Sprintf("%s ILIKE $%d", column, argIdx)
}

func (f *iLikeFilter) Value() []any {
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

type betweenFilter struct {
	start any
	end   any
}

func (f *betweenFilter) String(column string, argIdx int) string {
	return fmt.Sprintf("%s BETWEEN $%d AND $%d", column, argIdx, argIdx+1)
}

func (f *betweenFilter) Value() []any {
	return []any{f.start, f.end}
}

type orFilter struct {
	filters []Filter
}

func (f *orFilter) String(column string, argIdx int) string {
	parts := make([]string, len(f.filters))
	for i, filter := range f.filters {
		parts[i] = filter.String(column, argIdx)
	}
	return fmt.Sprintf("(%s)", strings.Join(parts, " OR "))
}

func (f *orFilter) Value() []any {
	var values []any
	for _, filter := range f.filters {
		values = append(values, filter.Value()...)
	}
	return values
}

type andFilter struct {
	filters []Filter
}

func (f *andFilter) String(column string, argIdx int) string {
	parts := make([]string, len(f.filters))
	for i, filter := range f.filters {
		parts[i] = filter.String(column, argIdx)
	}
	return fmt.Sprintf("(%s)", strings.Join(parts, " AND "))
}

func (f *andFilter) Value() []any {
	var values []any
	for _, filter := range f.filters {
		values = append(values, filter.Value()...)
	}
	return values
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
func Like(value any) Filter         { return &likeFilter{value} }
func ILike(value any) Filter        { return &iLikeFilter{value} }
func NotLike(value any) Filter      { return &notLikeFilter{value} }
func Between(start, end any) Filter { return &betweenFilter{start, end} }

// Logic operators

func Or(filters ...Filter) Filter {
	return &orFilter{filters}
}

func And(filters ...Filter) Filter {
	return &andFilter{filters}
}
