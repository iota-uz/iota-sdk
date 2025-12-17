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

type sortFieldKey interface {
	~int | ~string
}

// SortBy defines sorting criteria for queries with generic field type support.
// Use with OrderBy function to generate ORDER BY clauses.
type SortByField[T sortFieldKey] struct {
	// Field is a generic field type that implements the Column interface.
	Field T
	// Ascending indicates the sort direction (true for ASC, false for DESC).
	Ascending bool
	// NullsFirst indicates whether NULL values should appear first (true) or last (false).
	NullsLast bool
}

type SortBy[T sortFieldKey] struct {
	Fields []SortByField[T]
}

func (s *SortBy[T]) ToSQL(mapping map[T]string) string {
	if len(s.Fields) == 0 {
		return ""
	}
	fields := make([]string, 0, len(s.Fields))
	for _, sort := range s.Fields {
		field := mapping[sort.Field]
		// Skip invalid fields (empty mappings)
		if field == "" {
			continue
		}
		if sort.Ascending {
			field += " ASC"
		} else {
			field += " DESC"
		}
		// Only add NULLS clause if explicitly set
		if sort.NullsLast {
			field += " NULLS LAST"
		}
		fields = append(fields, field)
	}
	// Return empty if no valid fields found
	if len(fields) == 0 {
		return ""
	}
	return fmt.Sprintf("ORDER BY %s", strings.Join(fields, ", "))
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

// existsFilter allows EXISTS subqueries (column is ignored, SQL is the full EXISTS clause)
type existsFilter struct {
	subquery string
	values   []any
}

func (f *existsFilter) String(column string, argIdx int) string {
	// Replace placeholders in subquery with actual argument indices
	// IMPORTANT: Iterate in reverse order to avoid cascading replacements
	subquery := f.subquery
	for i := len(f.values) - 1; i >= 0; i-- {
		placeholder := fmt.Sprintf("$%d", i+1)
		actualPlaceholder := fmt.Sprintf("$%d", argIdx+i)
		subquery = strings.ReplaceAll(subquery, placeholder, actualPlaceholder)
	}
	return subquery
}

func (f *existsFilter) Value() []any {
	return f.values
}

// subqueryFilter allows subquery-based lookups
type subqueryFilter struct {
	subquery string
	values   []any
}

func (f *subqueryFilter) String(column string, argIdx int) string {
	// Replace placeholders in subquery with actual argument indices
	// IMPORTANT: Iterate in reverse order to avoid cascading replacements
	subquery := f.subquery
	for i := len(f.values) - 1; i >= 0; i-- {
		placeholder := fmt.Sprintf("$%d", i+1)
		actualPlaceholder := fmt.Sprintf("$%d", argIdx+i)
		subquery = strings.ReplaceAll(subquery, placeholder, actualPlaceholder)
	}
	return fmt.Sprintf("%s IN (%s)", column, subquery)
}

func (f *subqueryFilter) Value() []any {
	return f.values
}

// rawFilter allows custom SQL expressions (escape hatch)
// Use sparingly - prefer typed filters when possible
type rawFilter struct {
	sql    string
	values []any
}

func (f *rawFilter) String(column string, argIdx int) string {
	// Replace placeholders in SQL with actual argument indices
	// IMPORTANT: Iterate in reverse order to avoid cascading replacements
	// (e.g., $1 -> $2, then $2 -> $3 would incorrectly change the first replacement)
	sql := f.sql
	for i := len(f.values) - 1; i >= 0; i-- {
		placeholder := fmt.Sprintf("$%d", i+1)
		actualPlaceholder := fmt.Sprintf("$%d", argIdx+i)
		sql = strings.ReplaceAll(sql, placeholder, actualPlaceholder)
	}
	return sql
}

func (f *rawFilter) Value() []any {
	return f.values
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

// Complex filters

// ExistsFilter creates an EXISTS subquery filter
// Example: ExistsFilter("EXISTS (SELECT 1 FROM table WHERE column = $1)", value)
func ExistsFilter(subquery string, values ...any) Filter {
	return &existsFilter{subquery, values}
}

// SubqueryFilter creates a subquery-based lookup filter
// Example: SubqueryFilter("SELECT id FROM table WHERE column = $1", value)
func SubqueryFilter(subquery string, values ...any) Filter {
	return &subqueryFilter{subquery, values}
}

// RawFilter creates a custom SQL expression filter (escape hatch)
// Use sparingly - prefer typed filters when possible
// Example: RawFilter("column = $1 OR column = $2", value1, value2)
func RawFilter(sql string, values ...any) Filter {
	return &rawFilter{sql, values}
}
