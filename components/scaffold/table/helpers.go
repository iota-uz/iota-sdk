package table

import (
	"net/http"
	"net/url"
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
	Sortable() bool
	SortDir() SortDirection
	SortURL() string
}

type TableRow interface {
	Cells() []templ.Component
	Attrs() templ.Attributes
	ApplyOpts(opts ...RowOpt) TableRow
}

// --- Private Implementations ---

type tableColumnImpl struct {
	key      string
	label    string
	class    string
	width    string
	sortable bool
	sortDir  SortDirection
	sortURL  string
}

func (c *tableColumnImpl) Key() string            { return c.key }
func (c *tableColumnImpl) Label() string          { return c.label }
func (c *tableColumnImpl) Class() string          { return c.class }
func (c *tableColumnImpl) Width() string          { return c.width }
func (c *tableColumnImpl) Sortable() bool         { return c.sortable }
func (c *tableColumnImpl) SortDir() SortDirection { return c.sortDir }
func (c *tableColumnImpl) SortURL() string        { return c.sortURL }

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

func WithSortable(sortable bool) ColumnOpt {
	return func(c *tableColumnImpl) {
		c.sortable = sortable
	}
}

func WithSortDir(sortDir SortDirection) ColumnOpt {
	return func(c *tableColumnImpl) {
		c.sortDir = sortDir
	}
}

func WithSortURL(sortURL string) ColumnOpt {
	return func(c *tableColumnImpl) {
		c.sortURL = sortURL
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

	// Optional: reference to definition for advanced usage
	definition *TableDefinition
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

// GenerateSortURL generates a sort URL for a column based on current sort state
func GenerateSortURL(baseURL, fieldKey, currentSortField, currentSortOrder string) string {
	return GenerateSortURLWithParams(baseURL, fieldKey, currentSortField, currentSortOrder, nil)
}

// GenerateSortURLWithParams generates a sort URL for a column with additional query parameters
func GenerateSortURLWithParams(baseURL, fieldKey, currentSortField, currentSortOrder string, existingParams url.Values) string {
	params := url.Values{}

	// Copy existing parameters if provided
	for k, v := range existingParams {
		params[k] = v
	}

	// If clicking on the same field, cycle through: none -> asc -> desc -> none
	if fieldKey == currentSortField {
		switch currentSortOrder {
		case "asc":
			params.Set("sort", fieldKey)
			params.Set("order", "desc")
		case "desc":
			// Reset to no sorting (remove sort/order params)
			params.Del("sort")
			params.Del("order")
		default:
			params.Set("sort", fieldKey)
			params.Set("order", "asc")
		}
	} else {
		// Different field, start with ascending
		params.Set("sort", fieldKey)
		params.Set("order", "asc")
	}

	if len(params) == 0 {
		return baseURL
	}

	return baseURL + "?" + params.Encode()
}

// GetSortDirection returns the sort direction for a field
func GetSortDirection(fieldKey, currentSortField, currentSortOrder string) SortDirection {
	if fieldKey == currentSortField {
		return ParseSortDirection(currentSortOrder)
	}
	return SortDirectionNone
}

// --- New methods for separation of concerns ---

// ToDefinition extracts the table definition from config
func (c *TableConfig) ToDefinition() TableDefinition {
	if c.definition != nil {
		return *c.definition
	}

	// Build definition from current config
	builder := NewTableDefinition(c.Title, c.DataURL).
		WithColumns(c.Columns...).
		WithFilters(c.Filters...).
		WithActions(c.Actions...).
		WithSideFilter(c.SideFilter)

	if c.Infinite != nil {
		builder.WithInfiniteScroll(true)
	}

	return builder.Build()
}

// ToData extracts the table data from config
func (c *TableConfig) ToData() *TableData {
	data := NewTableData().WithRows(c.Rows...)

	if c.Infinite != nil {
		// Calculate total from hasMore flag
		// This is approximate but works for infinite scroll
		total := int64(c.Infinite.Page * c.Infinite.PerPage)
		if c.Infinite.HasMore {
			total++ // Indicate there's at least one more item
		}
		data.WithPagination(c.Infinite.Page, c.Infinite.PerPage, total)
	}

	return data
}

// FromDefinitionAndData creates a TableConfig from definition and data
func FromDefinitionAndData(def TableDefinition, data *TableData) *TableConfig {
	cfg := &TableConfig{
		Title:      def.Title(),
		DataURL:    def.DataURL(),
		Columns:    def.Columns(),
		Filters:    def.Filters(),
		Actions:    def.Actions(),
		SideFilter: def.SideFilter(),
		Rows:       data.Rows(),
		definition: &def,
	}

	if def.EnableInfiniteScroll() {
		pagination := data.Pagination()
		cfg.Infinite = &InfiniteScrollConfig{
			HasMore: pagination.HasMore,
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		}
	}

	return cfg
}
