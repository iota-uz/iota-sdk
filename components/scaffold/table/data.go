package table

import (
	"fmt"
	"net/url"
)

// TableData represents the runtime data for a table
type TableData struct {
	rows        []TableRow
	pagination  PaginationInfo
	queryParams url.Values
}

// PaginationInfo contains pagination state
type PaginationInfo struct {
	Page    int
	PerPage int
	Total   int64
	HasMore bool
}

// NewTableData creates a new TableData instance
func NewTableData() *TableData {
	return &TableData{
		rows:        []TableRow{},
		queryParams: url.Values{},
		pagination: PaginationInfo{
			Page:    1,
			PerPage: 20,
		},
	}
}

// WithRows sets the table rows
func (td *TableData) WithRows(rows ...TableRow) *TableData {
	td.rows = rows
	return td
}

// AddRow adds a single row to the table
func (td *TableData) AddRow(row TableRow) *TableData {
	td.rows = append(td.rows, row)
	return td
}

// WithPagination sets the pagination info
func (td *TableData) WithPagination(page, perPage int, total int64) *TableData {
	td.pagination = PaginationInfo{
		Page:    page,
		PerPage: perPage,
		Total:   total,
		HasMore: int64((page-1)*perPage+perPage) < total,
	}
	return td
}

// WithQueryParams sets the current query parameters
func (td *TableData) WithQueryParams(params url.Values) *TableData {
	td.queryParams = params
	return td
}

// Rows returns the table rows
func (td *TableData) Rows() []TableRow {
	return td.rows
}

// Pagination returns the pagination info
func (td *TableData) Pagination() PaginationInfo {
	return td.pagination
}

// QueryParams returns the current query parameters
func (td *TableData) QueryParams() url.Values {
	return td.queryParams
}

// IsEmpty returns true if there are no rows
func (td *TableData) IsEmpty() bool {
	return len(td.rows) == 0
}

// HasMore returns true if there are more pages available
func (td *TableData) HasMore() bool {
	return td.pagination.HasMore
}

// NextPageURL generates the URL for the next page
func (td *TableData) NextPageURL(baseURL string) string {
	params := url.Values{}
	// Copy existing query params
	for key, values := range td.queryParams {
		for _, value := range values {
			params.Add(key, value)
		}
	}
	// Update page and limit
	params.Set("page", fmt.Sprintf("%d", td.pagination.Page+1))
	params.Set("limit", fmt.Sprintf("%d", td.pagination.PerPage))

	return baseURL + "?" + params.Encode()
}
