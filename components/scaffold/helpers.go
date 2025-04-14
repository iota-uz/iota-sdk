package scaffold

import (
	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/components/scaffold/filters"
)

// --- Interfaces ---

type TableColumn interface {
	Key() string
	Label() string
	Class() string
	Width() string
}

type TableRow interface {
	Cells() []templ.Component
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
}

func (r *tableRowImpl) Cells() []templ.Component {
	return r.cells
}

// --- Column Option Type ---

type ColumnOpt func(c *tableColumnImpl)

func WithClass(classes string) ColumnOpt {
	return func(c *tableColumnImpl) {
		c.class = classes
	}
}

// --- Table Configuration ---

type TableConfig struct {
	Title      string
	DataURL    string
	Filters    []*filters.TableFilter
	Columns    []TableColumn
	Rows       []TableRow
	SideFilter templ.Component
}

func NewTableConfig(title, dataURL string) *TableConfig {
	return &TableConfig{
		Title:   title,
		DataURL: dataURL,
		Columns: []TableColumn{},
		Filters: []*filters.TableFilter{},
		Rows:    []TableRow{},
	}
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
	return &tableRowImpl{cells: cells}
}

func (c *TableConfig) AddCols(cols ...TableColumn) *TableConfig {
	c.Columns = append(c.Columns, cols...)
	return c
}

func (c *TableConfig) AddFilters(filters ...*filters.TableFilter) *TableConfig {
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
