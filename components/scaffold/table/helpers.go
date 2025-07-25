package table

import (
	"net/http"
	"strconv"

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

func WithInfiniteScroll(hasMore bool, page, perPage int) TableConfigOpt {
	return func(c *TableConfig) {
		c.Infinite.HasMore = hasMore
		c.Infinite.Page = page
		c.Infinite.PerPage = perPage
	}
}

type InfiniteScrollConfig struct {
	HasMore bool
	Page    int
	PerPage int
}

type TableConfig struct {
	Title      string
	DataURL    string
	Filters    []templ.Component
	Actions    []templ.Component // Actions like Create button
	Columns    []TableColumn
	Rows       []TableRow
	Infinite   *InfiniteScrollConfig
	SideFilter templ.Component
}

func NewTableConfig(title, dataURL string, opts ...TableConfigOpt) *TableConfig {
	t := &TableConfig{
		Title:    title,
		DataURL:  dataURL,
		Infinite: &InfiniteScrollConfig{},
		Columns:  []TableColumn{},
		Filters:  []templ.Component{},
		Actions:  []templ.Component{},
		Rows:     []TableRow{},
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

func (c *TableConfig) AddActions(actions ...templ.Component) *TableConfig {
	c.Actions = append(c.Actions, actions...)
	return c
}

// --- Query Parameter Utilities ---

// UseSearchQuery gets the "Search" query parameter from the request
func UseSearchQuery(r *http.Request) string {
	return r.URL.Query().Get("Search")
}

// UsePageQuery gets the "page" query parameter from the request and converts it to int
func UsePageQuery(r *http.Request) int {
	pageStr := r.URL.Query().Get("page")
	if pageStr == "" {
		return 1 // default to page 1
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		return 1
	}
	return page
}

// UseLimitQuery gets the "limit" query parameter from the request and converts it to int
func UseLimitQuery(r *http.Request) int {
	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		return 20 // default to 20 items per page
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		return 20
	}
	if limit > 100 {
		return 100 // cap at maximum
	}
	return limit
}

// UseSortQuery gets the "sort" query parameter from the request
func UseSortQuery(r *http.Request) string {
	return r.URL.Query().Get("sort")
}

// UseOrderQuery gets the "order" query parameter from the request (asc/desc)
func UseOrderQuery(r *http.Request) string {
	order := r.URL.Query().Get("order")
	if order != "desc" {
		return "asc" // default to ascending
	}
	return order
}
