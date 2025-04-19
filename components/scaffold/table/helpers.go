package scaffold

import (
	"github.com/a-h/templ"
)

// --- Interfaces ---

type RowOpt func(r *tableRowImpl)
type ColumnOpt func(c *tableColumnImpl)

type TableColumn interface {
	Key() string
	Label() string
	Class() string
	Width() string
}

type TableRow interface {
	Cells() []templ.Component
	Attrs() templ.Attributes
	ApplyOpts(opts ...RowOpt) TableRow
}

// --- Private Implementations ---

type tableColumnImpl struct {
	key   string
	label string
	class string
	width string
}

func (c *tableColumnImpl) Key() string   { return c.key }
func (c *tableColumnImpl) Label() string { return c.label }
func (c *tableColumnImpl) Class() string { return c.class }
func (c *tableColumnImpl) Width() string { return c.width }

type tableRowImpl struct {
	cells []templ.Component
	attrs templ.Attributes
}

func (r *tableRowImpl) Cells() []templ.Component {
	return r.cells
}

func (r *tableRowImpl) Attrs() templ.Attributes {
	return r.attrs
}

func (r *tableRowImpl) ApplyOpts(opts ...RowOpt) TableRow {
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// --- Row Options ---

func WithDrawer(fetchURL string) RowOpt {
	return func(r *tableRowImpl) {
		r.attrs["class"] = r.attrs["class"].(string) + " cursor-pointer"
		r.attrs["hx-get"] = fetchURL
		r.attrs["hx-target"] = "#view-drawer"
		r.attrs["hx-swap"] = "innerHTML"
	}
}

// --- Column Options ---

func WithClass(classes string) ColumnOpt {
	return func(c *tableColumnImpl) {
		c.class = classes
	}
}

// --- Table Configuration ---

type TableConfigOpt func(c *TableConfig)

type TableConfig struct {
	Title      string
	DataURL    string
	Filters    []templ.Component
	Columns    []TableColumn
	Rows       []TableRow
	SideFilter templ.Component
}

func NewTableConfig(title, dataURL string, opts ...TableConfigOpt) *TableConfig {
	t := &TableConfig{
		Title:   title,
		DataURL: dataURL,
		Columns: []TableColumn{},
		Filters: []templ.Component{},
		Rows:    []TableRow{},
	}
	for _, o := range opts {
		o(t)
	}
	return t
}

func Column(key, label string, opts ...ColumnOpt) TableColumn {
	col := &tableColumnImpl{
		key:   key,
		label: label,
	}
	for _, opt := range opts {
		opt(col)
	}
	return col
}

func Row(cells ...templ.Component) TableRow {
	return &tableRowImpl{
		cells: cells,
		attrs: templ.Attributes{
			"class": "hide-on-load",
		},
	}
}

func (c *TableConfig) AddCols(cols ...TableColumn) *TableConfig {
	c.Columns = append(c.Columns, cols...)
	return c
}

func (c *TableConfig) AddFilters(filters ...templ.Component) *TableConfig {
	c.Filters = append(c.Filters, filters...)
	return c
}

func (c *TableConfig) AddRows(rows ...TableRow) *TableConfig {
	c.Rows = append(c.Rows, rows...)
	return c
}

func (c *TableConfig) SetSideFilter(filter templ.Component) *TableConfig {
	c.SideFilter = filter
	return c
}
