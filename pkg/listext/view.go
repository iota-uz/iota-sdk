package listext

import (
	"net/url"
	"strconv"
)

// View is what a list template renders: the columns, the per-row cells and
// the state of the filter and sort controls. A nil *View renders nothing.
type View struct {
	Lang    string
	Columns []Column
	Request Request
	// Native is the native entity ID of the listed rows ("crm.client").
	Native string
	rows   map[string][]Value
}

// NewView builds the view of a plan. Rows are added with SetRow.
func NewView(p *Plan, req Request, lang string) *View {
	if p == nil || len(p.Columns) == 0 {
		return nil
	}
	return &View{Lang: lang, Columns: p.Columns, Request: req, Native: p.Native, rows: map[string][]Value{}}
}

// SetRow stores the values of one row, aligned with Columns.
func (v *View) SetRow(key string, values []Value) {
	if v != nil {
		v.rows[key] = values
	}
}

// Cells returns the cells of a row; a row without values yields empty cells
// so the table keeps its shape.
func (v *View) Cells(key string) []Cell {
	if v == nil {
		return nil
	}
	values := v.rows[key]
	out := make([]Cell, len(v.Columns))
	row := map[string]any{"ref": map[string]string{"entity": v.Native, "key": key}}
	for i, c := range v.Columns {
		out[i] = Cell{Column: c}
		if i < len(values) {
			out[i].Value = values[i]
		} else {
			out[i].Value = Value{Type: c.Type}
		}
		row[c.ID] = out[i].Value.WireJSON()
	}
	for i := range out {
		out[i].Row = row
	}
	return out
}

// Title is the localized column label.
func (v *View) Title(c Column) string { return c.Title(v.Lang) }

// FilterValue returns the current equality / contains filter text of a column.
func (v *View) FilterValue(c Column, op Op) string {
	for _, f := range v.Request.Filters {
		if f.Column == c.ID && (f.Op == op || (op == OpEq && f.Op == OpContains)) {
			return f.Value
		}
	}
	return ""
}

// SortDir returns "asc", "desc" or "" for the column.
func (v *View) SortDir(c Column) string {
	s := v.Request.Sort
	if s == nil || s.Column != c.ID {
		return ""
	}
	if s.Desc {
		return "desc"
	}
	return "asc"
}

// SortURL is the URL that sorts by the column (toggling the direction),
// keeping the other query parameters.
func (v *View) SortURL(c Column, base string, query url.Values) string {
	q := url.Values{}
	for k, vs := range query {
		if k == ParamSort || k == ParamDir || k == "page" {
			continue
		}
		q[k] = append([]string(nil), vs...)
	}
	dir := "asc"
	if v.SortDir(c) == "asc" {
		dir = "desc"
	}
	q.Set(ParamSort, c.ID)
	q.Set(ParamDir, dir)
	q.Set("page", strconv.Itoa(1))
	return base + "?" + q.Encode()
}

// HasFilterable reports whether any column offers a filter control.
func (v *View) HasFilterable() bool {
	if v == nil {
		return false
	}
	for _, c := range v.Columns {
		if c.Filterable {
			return true
		}
	}
	return false
}
