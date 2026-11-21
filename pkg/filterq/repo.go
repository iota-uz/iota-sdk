package filterq

import (
	"time"

	"github.com/iota-uz/iota-sdk/pkg/repo"
)

// RepoField maps a filter field to a repository column for RepoFilters. Type
// and Blank mean the same as in Column; the SQL expression comes from the
// repository's own column mapping.
type RepoField[F any] struct {
	Column F
	Type   string
	Blank  bool
	// Expr optionally rewrites the repository column into the compared
	// expression, e.g. minor units to major ("col / 100.0") or a flag
	// ("col <> 0").
	Expr func(column string) string
}

// RepoFilters converts the conditions of fs on the listed fields into
// repo.FieldFilter values, for repositories built on pkg/repo. Each filter
// renders the same predicate as AppendSQL against the column the repository
// resolves, so a chip means the same in a hand-written query and in a
// repository-backed list.
func RepoFilters[F any](fs FilterSet, schema Schema, fields map[string]RepoField[F], now time.Time) []repo.FieldFilter[F] {
	out := make([]repo.FieldFilter[F], 0, len(fs))
	for _, c := range fs {
		f, ok := fields[c.Field]
		if !ok {
			continue
		}
		field, ok := schema.Field(c.Field)
		if !ok {
			continue
		}
		filter := &conditionFilter{typ: field.Type, col: Column{Type: f.Type, Blank: f.Blank}, expr: f.Expr, c: c, now: now}
		if _, _, ok := filter.render("x", 0); !ok {
			continue
		}
		out = append(out, repo.FieldFilter[F]{Column: f.Column, Filter: filter})
	}
	return out
}

// conditionFilter is a repo.Filter rendering one condition.
type conditionFilter struct {
	typ  FieldType
	col  Column
	expr func(string) string
	c    Condition
	now  time.Time
}

// render builds the predicate against column with placeholders numbered after
// offset existing arguments.
func (f *conditionFilter) render(column string, offset int) (string, []any, bool) {
	col := f.col
	col.Expr = column
	if f.expr != nil {
		col.Expr = "(" + f.expr(column) + ")"
	}
	p := &Params{offset: offset}
	pred, ok := predicate(f.typ, col, f.c, f.now, p)
	return pred, p.Args(), ok
}

func (f *conditionFilter) String(column string, argIdx int) string {
	pred, _, ok := f.render(column, argIdx-1)
	if !ok {
		return "TRUE"
	}
	return "(" + pred + ")"
}

func (f *conditionFilter) Value() []any {
	_, args, _ := f.render("x", 0)
	return args
}
