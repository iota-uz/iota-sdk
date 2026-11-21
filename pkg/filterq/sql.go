package filterq

import (
	"strconv"
	"strings"
	"time"
)

// Column tells AppendSQL how one filter field reads in SQL.
type Column struct {
	// Expr is the SQL expression the condition compares, e.g.
	// "t.origin_account_id". It is inserted verbatim: never build it from
	// request data.
	Expr string
	// Type is the PostgreSQL type values are cast to ("uuid", "text",
	// "bigint"...). Empty means "text" for reference fields; date, number and
	// bool fields always compare as date, numeric and boolean.
	Type string
	// Blank also treats an empty string as "no value" for OpEmpty and OpIsNot,
	// for text columns that store "" instead of NULL.
	Blank bool
	// Custom renders the condition itself, for computed fields ("overdue",
	// "has balance") that are not a plain comparison. It returns the SQL
	// predicate and false when the condition cannot be expressed (then it is
	// skipped). Values go through p.Add, never into the string.
	Custom func(c Condition, p *Params) (string, bool)
}

// Params collects positional query arguments while predicates are rendered.
type Params struct {
	args []any
	// offset shifts placeholders when the arguments are appended to a query
	// built elsewhere (see RepoFilters).
	offset int
}

// Add appends a value and returns its placeholder ($N).
func (p *Params) Add(v any) string {
	p.args = append(p.args, v)
	return "$" + strconv.Itoa(p.offset+len(p.args))
}

// Args returns every argument collected so far, in placeholder order.
func (p *Params) Args() []any { return p.args }

// AppendSQL renders the conditions of fs whose fields are listed in columns
// as SQL predicates joined with AND, each prefixed with " AND ", so the result
// appends to an existing WHERE clause. Placeholders continue after args, which
// the returned slice extends. Conditions on fields missing from columns or
// from the schema are skipped: the caller either handles them itself or they
// do not apply to this query.
//
// Semantics match the builder: values of one condition are OR-ed, conditions
// are AND-ed; "is not" keeps records without a value; date presets resolve
// against now.
func AppendSQL(fs FilterSet, schema Schema, columns map[string]Column, now time.Time, args []any) (string, []any) {
	p := &Params{args: args}
	var b strings.Builder
	for _, c := range fs {
		col, ok := columns[c.Field]
		if !ok {
			continue
		}
		field, ok := schema.Field(c.Field)
		if !ok {
			continue
		}
		pred, ok := predicate(field.Type, col, c, now, p)
		if !ok {
			continue
		}
		b.WriteString(" AND (")
		b.WriteString(pred)
		b.WriteString(")")
	}
	return b.String(), p.args
}

func predicate(t FieldType, col Column, c Condition, now time.Time, p *Params) (string, bool) {
	if col.Custom != nil {
		return col.Custom(c, p)
	}
	if c.Op == OpEmpty {
		if col.Blank {
			return col.Expr + " IS NULL OR " + col.Expr + " = ''", true
		}
		return col.Expr + " IS NULL", true
	}
	switch t {
	case FieldTypeReference:
		return referencePredicate(col, c, p)
	case FieldTypeBool:
		if len(c.Values) != 1 {
			return "", false
		}
		if c.Values[0] == "true" {
			return col.Expr + " IS TRUE", true
		}
		return col.Expr + " IS NOT TRUE", true
	case FieldTypeDate:
		return datePredicate("("+col.Expr+")::date", c, now, p)
	case FieldTypeNumber:
		return numberPredicate("("+col.Expr+")::numeric", c, p)
	}
	return "", false
}

func referencePredicate(col Column, c Condition, p *Params) (string, bool) {
	if len(c.Values) == 0 {
		return "", false
	}
	typ := col.Type
	if typ == "" {
		typ = "text"
	}
	list := p.Add(c.Values) + "::" + typ + "[]"
	switch c.Op {
	case OpIs:
		return col.Expr + " = ANY(" + list + ")", true
	case OpIsNot:
		// A record without a value is "not X" too.
		pred := "NOT COALESCE(" + col.Expr + " = ANY(" + list + "), false)"
		return pred, true
	case OpBefore, OpAfter, OpOn, OpBetween, OpEq, OpGt, OpLt, OpEmpty:
		// Not reference operators; empty is rendered by the caller.
	}
	return "", false
}

func datePredicate(expr string, c Condition, now time.Time, p *Params) (string, bool) {
	if c.Op == OpBetween {
		from, to, ok := c.DateRange(now)
		if !ok {
			return "", false
		}
		return expr + " BETWEEN " + p.Add(from.Format(DateLayout)) + "::date AND " +
			p.Add(to.Format(DateLayout)) + "::date", true
	}
	if len(c.Values) != 1 {
		return "", false
	}
	v := p.Add(c.Values[0]) + "::date"
	switch c.Op {
	case OpOn:
		return expr + " = " + v, true
	case OpBefore:
		return expr + " < " + v, true
	case OpAfter:
		return expr + " > " + v, true
	case OpIs, OpIsNot, OpBetween, OpEq, OpGt, OpLt, OpEmpty:
		// Between is rendered above; the rest are not date operators.
	}
	return "", false
}

func numberPredicate(expr string, c Condition, p *Params) (string, bool) {
	if c.Op == OpBetween {
		if len(c.Values) != 2 {
			return "", false
		}
		// SYMMETRIC: "from 500 to 100" means the same range as "from 100 to 500".
		return expr + " BETWEEN SYMMETRIC " + p.Add(c.Values[0]) + "::numeric AND " +
			p.Add(c.Values[1]) + "::numeric", true
	}
	if len(c.Values) != 1 {
		return "", false
	}
	v := p.Add(c.Values[0]) + "::numeric"
	switch c.Op {
	case OpEq:
		return expr + " = " + v, true
	case OpGt:
		return expr + " > " + v, true
	case OpLt:
		return expr + " < " + v, true
	case OpIs, OpIsNot, OpBefore, OpAfter, OpOn, OpBetween, OpEmpty:
		// Between is rendered above; the rest are not number operators.
	}
	return "", false
}
