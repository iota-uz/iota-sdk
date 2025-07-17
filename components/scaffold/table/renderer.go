package table

import (
	"github.com/a-h/templ"
)

// TableRenderer handles rendering table components
type TableRenderer struct {
	definition TableDefinition
	data       *TableData
}

// NewTableRenderer creates a new renderer with definition and data
func NewTableRenderer(definition TableDefinition, data *TableData) *TableRenderer {
	return &TableRenderer{
		definition: definition,
		data:       data,
	}
}

// RenderFull renders the complete table including configuration and data
func (r *TableRenderer) RenderFull() templ.Component {
	// Create a TableConfig for backward compatibility
	cfg := &TableConfig{
		Title:      r.definition.Title(),
		DataURL:    r.definition.DataURL(),
		Columns:    r.definition.Columns(),
		Filters:    r.definition.Filters(),
		Actions:    r.definition.Actions(),
		SideFilter: r.definition.SideFilter(),
		Rows:       r.data.Rows(),
	}

	// Set infinite scroll if enabled and has data
	if r.definition.EnableInfiniteScroll() && r.data != nil {
		cfg.Infinite = &InfiniteScrollConfig{
			HasMore: r.data.HasMore(),
			Page:    r.data.Pagination().Page,
			PerPage: r.data.Pagination().PerPage,
		}
	}

	return Page(cfg)
}

// RenderTable renders just the table component (without page wrapper)
func (r *TableRenderer) RenderTable() templ.Component {
	cfg := &TableConfig{
		Title:      r.definition.Title(),
		DataURL:    r.definition.DataURL(),
		Columns:    r.definition.Columns(),
		Filters:    r.definition.Filters(),
		Actions:    r.definition.Actions(),
		SideFilter: r.definition.SideFilter(),
		Rows:       r.data.Rows(),
	}

	if r.definition.EnableInfiniteScroll() && r.data != nil {
		cfg.Infinite = &InfiniteScrollConfig{
			HasMore: r.data.HasMore(),
			Page:    r.data.Pagination().Page,
			PerPage: r.data.Pagination().PerPage,
		}
	}

	return Content(cfg)
}

// RenderRows renders only the data rows (for HTMX requests)
func (r *TableRenderer) RenderRows() templ.Component {
	cfg := &TableConfig{
		DataURL: r.definition.DataURL(),
		Columns: r.definition.Columns(),
		Rows:    r.data.Rows(),
	}

	if r.definition.EnableInfiniteScroll() && r.data != nil {
		cfg.Infinite = &InfiniteScrollConfig{
			HasMore: r.data.HasMore(),
			Page:    r.data.Pagination().Page,
			PerPage: r.data.Pagination().PerPage,
		}
	}

	return Rows(cfg)
}

// Static rendering methods that don't require instance

// RenderWithDefinition renders a table with definition and data separately
func RenderWithDefinition(def TableDefinition, data *TableData) templ.Component {
	renderer := NewTableRenderer(def, data)
	return renderer.RenderFull()
}

// RenderRowsWithDefinition renders only rows with definition and data
func RenderRowsWithDefinition(def TableDefinition, data *TableData) templ.Component {
	renderer := NewTableRenderer(def, data)
	return renderer.RenderRows()
}
