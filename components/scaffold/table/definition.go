package table

import (
	"github.com/a-h/templ"
)

// TableDefinition represents an immutable table configuration
type TableDefinition struct {
	title      string
	dataURL    string
	columns    []TableColumn
	filters    []templ.Component
	actions    []templ.Component
	sideFilter templ.Component

	// Configuration options
	enableInfiniteScroll bool
	searchable           bool
	sortable             bool
}

// Getter methods for accessing table definition properties
func (td TableDefinition) Title() string {
	return td.title
}

func (td TableDefinition) DataURL() string {
	return td.dataURL
}

func (td TableDefinition) Columns() []TableColumn {
	// Return a copy to prevent external modification
	cols := make([]TableColumn, len(td.columns))
	copy(cols, td.columns)
	return cols
}

func (td TableDefinition) Filters() []templ.Component {
	// Return a copy
	filters := make([]templ.Component, len(td.filters))
	copy(filters, td.filters)
	return filters
}

func (td TableDefinition) Actions() []templ.Component {
	// Return a copy
	actions := make([]templ.Component, len(td.actions))
	copy(actions, td.actions)
	return actions
}

func (td TableDefinition) SideFilter() templ.Component {
	return td.sideFilter
}

func (td TableDefinition) EnableInfiniteScroll() bool {
	return td.enableInfiniteScroll
}

// TableDefinitionBuilder builds immutable TableDefinition instances
type TableDefinitionBuilder struct {
	definition TableDefinition
}

// NewTableDefinition creates a new builder for TableDefinition
func NewTableDefinition(title, dataURL string) *TableDefinitionBuilder {
	return &TableDefinitionBuilder{
		definition: TableDefinition{
			title:                title,
			dataURL:              dataURL,
			columns:              []TableColumn{},
			filters:              []templ.Component{},
			actions:              []templ.Component{},
			enableInfiniteScroll: true, // Default to enabled
			searchable:           true,
			sortable:             true,
		},
	}
}

// WithColumns sets the table columns
func (b *TableDefinitionBuilder) WithColumns(columns ...TableColumn) *TableDefinitionBuilder {
	b.definition.columns = make([]TableColumn, len(columns))
	copy(b.definition.columns, columns)
	return b
}

// WithFilters sets the table filters
func (b *TableDefinitionBuilder) WithFilters(filters ...templ.Component) *TableDefinitionBuilder {
	b.definition.filters = make([]templ.Component, len(filters))
	copy(b.definition.filters, filters)
	return b
}

// WithActions sets the table actions
func (b *TableDefinitionBuilder) WithActions(actions ...templ.Component) *TableDefinitionBuilder {
	b.definition.actions = make([]templ.Component, len(actions))
	copy(b.definition.actions, actions)
	return b
}

// WithSideFilter sets the side filter component
func (b *TableDefinitionBuilder) WithSideFilter(filter templ.Component) *TableDefinitionBuilder {
	b.definition.sideFilter = filter
	return b
}

// WithInfiniteScroll enables/disables infinite scroll
func (b *TableDefinitionBuilder) WithInfiniteScroll(enabled bool) *TableDefinitionBuilder {
	b.definition.enableInfiniteScroll = enabled
	return b
}

// WithSearchable enables/disables search functionality
func (b *TableDefinitionBuilder) WithSearchable(enabled bool) *TableDefinitionBuilder {
	b.definition.searchable = enabled
	return b
}

// WithSortable enables/disables sort functionality
func (b *TableDefinitionBuilder) WithSortable(enabled bool) *TableDefinitionBuilder {
	b.definition.sortable = enabled
	return b
}

// Build returns the immutable TableDefinition
func (b *TableDefinitionBuilder) Build() TableDefinition {
	// Return a copy to ensure immutability
	def := b.definition

	// Deep copy slices
	def.columns = make([]TableColumn, len(b.definition.columns))
	copy(def.columns, b.definition.columns)

	def.filters = make([]templ.Component, len(b.definition.filters))
	copy(def.filters, b.definition.filters)

	def.actions = make([]templ.Component, len(b.definition.actions))
	copy(def.actions, b.definition.actions)

	return def
}
