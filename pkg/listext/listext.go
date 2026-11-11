// Package listext is the extension point of native lists and exports for
// columns contributed by activated extension packages.
//
// A native list declares a stable public list ID (for example
// "crm.lists.clients") and the native entity it lists. It asks an Engine for
// the contributed columns: the Engine compiles them, through the typed data
// layer, into a join fragment plus select, filter and order expressions that
// the list splices into its own SQL. The same plan serves the table, its
// filter and sort controls and the export, so one declaration appears in all
// of them.
//
// The Engine never widens access. Overlay fields read as their declared
// default (or NULL) for a base row without an overlay row and for rows the
// subject's read rules hide, fields the subject may not read are left out of
// the plan, and all predicates are part of the list query itself, so they are
// applied before pagination, counting and export.
package listext

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Contribution is one column contributed to a list by an activated package.
type Contribution struct {
	// ID is the resource ID of the column contract. It is the stable public
	// key of the column in URLs and exports.
	ID      string
	Package string
	Version string
	Label   manifest.Label
	// Overlay and Field name the overlay field the column reads.
	Overlay    string
	Field      string
	Filterable bool
	Sortable   bool
	Exportable bool
}

// ColumnSource lists the columns activated packages contribute to a list, in
// the stable order they are displayed. Implementations read an in-memory snapshot of activated manifests; they must
// not call Node.
type ColumnSource interface {
	Columns(ctx context.Context, listID string) ([]Contribution, error)
}

// OverlayJoiner is implemented by *data.Store.
type OverlayJoiner interface {
	NativeOverlayJoin(ctx context.Context, q data.Querier, sub spi.Subject, nativeID, baseAlias string, refs []data.OverlayFieldRef, args *data.Args) (*data.OverlayJoin, error)
}

// Engine compiles contributed columns for native lists.
type Engine struct {
	source ColumnSource
	joiner OverlayJoiner
}

// NewEngine wires an engine over a column source and the data store.
func NewEngine(source ColumnSource, joiner OverlayJoiner) *Engine {
	return &Engine{source: source, joiner: joiner}
}

// Column is a contribution that is readable by the subject, with the declared
// type of its overlay field.
type Column struct {
	Contribution
	Type manifest.Type
	ref  data.OverlayFieldRef
	sel  string
}

// Title returns the label for lang, falling back to English, then any label,
// then the column ID.
func (c Column) Title(lang string) string {
	return LabelFor(c.Label, lang, c.ID)
}

// Purpose selects which columns a plan includes.
type Purpose int

const (
	// PurposeList includes every readable column.
	PurposeList Purpose = iota
	// PurposeExport includes only exportable columns.
	PurposeExport
)

// Op is a filter operator.
type Op string

const (
	OpEq       Op = "eq"
	OpContains Op = "contains"
	OpGte      Op = "gte"
	OpLte      Op = "lte"
)

// Filter is one user-supplied condition on a contributed column.
type Filter struct {
	Column string
	Op     Op
	Value  string
}

// Sort orders by a contributed column.
type Sort struct {
	Column string
	Desc   bool
}

// Request is the user-supplied part of a list query: filters and ordering by
// column ID. Entries naming unknown, unreadable or unfit columns are ignored
// and reported in Plan.Ignored.
type Request struct {
	Filters []Filter
	Sort    *Sort
}

// Empty reports whether the request changes nothing.
func (r Request) Empty() bool { return len(r.Filters) == 0 && r.Sort == nil }

// Input describes the native list a plan is compiled for.
type Input struct {
	// List is the public list ID the columns were contributed to.
	List string
	// Native is the native entity ID of the listed table ("crm.client").
	Native string
	// BaseAlias is the alias of the native table in the list query.
	BaseAlias string
	Purpose   Purpose
	Request   Request
	// UsedArgs is the number of placeholders the list query already uses.
	UsedArgs int
}

// Plan is the compiled SQL of the contributed columns.
type Plan struct {
	// Columns are the columns included in Select, in order.
	Columns []Column
	// Hidden lists contributions removed by field access.
	Hidden []Contribution
	// Ignored lists request entries that were not applied, with the reason.
	Ignored []string
	// Join is a LEFT JOIN fragment to append after the base table.
	Join string
	// Where holds conjunction terms (already parenthesized); possibly empty.
	Where []string
	// OrderBy holds ORDER BY terms to place before the list's own tiebreak.
	OrderBy []string
	// Select holds one SQL expression per column, each yielding text (or NULL).
	Select []string
	args   *data.Args
	used   int
}

// Args returns the full argument list: prior (the caller's own arguments, at
// most the number passed as Input.UsedArgs) followed by the plan's.
func (p *Plan) Args(prior []any) []any {
	vals := p.args.Values()
	out := make([]any, 0, len(vals))
	out = append(out, prior...)
	if len(out) < p.used {
		panic(fmt.Sprintf("listext: plan expects %d prior arguments, got %d", p.used, len(prior)))
	}
	return append(out[:p.used], vals[p.used:]...)
}

// SelectList renders ", expr AS ext_0, expr AS ext_1" (empty without
// columns), to append to a SELECT list.
func (p *Plan) SelectList() string {
	var b strings.Builder
	for i, e := range p.Select {
		fmt.Fprintf(&b, ", %s AS ext_%d", e, i)
	}
	return b.String()
}

// Compile builds the plan for the subject. Without contributions the plan is
// empty and costs no query.
func (e *Engine) Compile(ctx context.Context, q data.Querier, sub spi.Subject, in Input) (*Plan, error) {
	const op = serrors.Op("listext.Engine.Compile")
	plan := &Plan{args: data.NewArgs(in.UsedArgs), used: in.UsedArgs}
	if e == nil || e.source == nil {
		return plan, nil
	}
	contribs, err := e.source.Columns(ctx, in.List)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	if len(contribs) == 0 {
		return plan, nil
	}
	refs := make([]data.OverlayFieldRef, 0, len(contribs))
	usable := make([]Contribution, 0, len(contribs))
	for _, c := range contribs {
		usable = append(usable, c)
		refs = append(refs, data.OverlayFieldRef{Overlay: c.Overlay, Field: c.Field})
	}
	join, err := e.joiner.NativeOverlayJoin(ctx, q, sub, in.Native, in.BaseAlias, refs, plan.args)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	plan.Join = join.SQL
	byID := map[string]Column{}
	for i, c := range usable {
		ref := refs[i]
		if !join.Visible(ref) {
			plan.Hidden = append(plan.Hidden, c)
			continue
		}
		typ, _ := join.Type(ref)
		sel, err := join.Select(ref)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		col := Column{Contribution: c, Type: typ, ref: ref, sel: textExpr(typ, sel)}
		byID[c.ID] = col
		if in.Purpose == PurposeExport && !c.Exportable {
			continue
		}
		plan.Columns = append(plan.Columns, col)
		plan.Select = append(plan.Select, col.sel)
	}
	for _, f := range in.Request.Filters {
		col, ok := byID[f.Column]
		if !ok {
			plan.Ignored = append(plan.Ignored, fmt.Sprintf("filter %s: not an available column", f.Column))
			continue
		}
		if !col.Filterable {
			plan.Ignored = append(plan.Ignored, fmt.Sprintf("filter %s: column is not filterable", f.Column))
			continue
		}
		cond, err := condition(col.Type, f)
		if err != nil {
			plan.Ignored = append(plan.Ignored, fmt.Sprintf("filter %s: %v", f.Column, err))
			continue
		}
		sql, err := join.Filter(col.ref, cond, sub, plan.args)
		if err != nil {
			plan.Ignored = append(plan.Ignored, fmt.Sprintf("filter %s: %v", f.Column, err))
			continue
		}
		plan.Where = append(plan.Where, sql)
	}
	if s := in.Request.Sort; s != nil {
		col, ok := byID[s.Column]
		switch {
		case !ok:
			plan.Ignored = append(plan.Ignored, fmt.Sprintf("sort %s: not an available column", s.Column))
		case !col.Sortable:
			plan.Ignored = append(plan.Ignored, fmt.Sprintf("sort %s: column is not sortable", s.Column))
		default:
			term, err := join.OrderBy(col.ref, s.Desc)
			if err != nil {
				plan.Ignored = append(plan.Ignored, fmt.Sprintf("sort %s: %v", s.Column, err))
			} else {
				plan.OrderBy = append(plan.OrderBy, term+" NULLS LAST")
			}
		}
	}
	return plan, nil
}

// textExpr renders the field expression as text so rows scan uniformly.
// Datetimes are normalized to UTC RFC 3339 regardless of session time zone.
func textExpr(t manifest.Type, expr string) string {
	if t.Kind == manifest.KindDateTime {
		return fmt.Sprintf(`to_char((%s) AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')`, expr)
	}
	return "(" + expr + ")::text"
}

// Scan reads the plan's selected columns from the row's remaining fields.
// The row must have been selected with SelectList placed last.
func (p *Plan) Scan(rows pgx.Rows, dest ...any) ([]Value, error) {
	raw := make([]*string, len(p.Columns))
	all := append([]any(nil), dest...)
	for i := range raw {
		all = append(all, &raw[i])
	}
	if err := rows.Scan(all...); err != nil {
		return nil, err
	}
	out := make([]Value, len(raw))
	for i, r := range raw {
		out[i] = Value{Type: p.Columns[i].Type, Raw: r}
	}
	return out, nil
}
