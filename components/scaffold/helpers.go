// Package scaffold provides utilities for generating dynamic UI components.
//
// It simplifies the creation of consistent data tables and other UI elements
// based on configuration and data, reducing boilerplate code.
package scaffold

import (
	"fmt"
	"log"
	"time"
)

// TableColumn defines a column in a dynamic table.
type TableColumn struct {
	Key    string            // Field key in the data source
	Label  string            // Display label for the column header
	Class  string            // CSS classes for the column
	Width  string            // Width specification (e.g., "100px" or "20%")
	Format func(any) string  // Optional formatter for cell values
}

// TableConfig holds the configuration for a dynamic table.
type TableConfig struct {
	Columns []TableColumn  // Column definitions
	Title   string         // Table title displayed at the top
}

// TableData contains the data to be displayed in the table.
type TableData struct {
	Items []map[string]any  // Row items with key-value pairs
}

// NewTableConfig creates a new empty table configuration.
func NewTableConfig() *TableConfig {
	return &TableConfig{
		Columns: []TableColumn{},
	}
}

// AddColumn adds a column to the table configuration
func (c *TableConfig) AddColumn(key, label, class string) *TableConfig {
	c.Columns = append(c.Columns, TableColumn{
		Key:   key,
		Label: label,
		Class: class,
	})
	return c
}

// AddDateColumn adds a date column with automatic formatting
func (c *TableConfig) AddDateColumn(key, label string) *TableConfig {
	c.Columns = append(c.Columns, TableColumn{
		Key:   key,
		Label: label,
		Format: func(value any) string {
			if ts, ok := value.(time.Time); ok {
				return fmt.Sprintf(`<div x-data="relativeformat"><span x-text="format('%s')">%s</span></div>`,
					ts.Format(time.RFC3339),
					ts.Format("2006-01-02 15:04:05"))
			}
			log.Printf("expected time.Time, got %T", value)
			return fmt.Sprintf("%v", value)
		},
	})
	return c
}

// AddActionsColumn adds an actions column with edit button
func (c *TableConfig) AddActionsColumn() *TableConfig {
	c.Columns = append(c.Columns, TableColumn{
		Key:   "actions",
		Label: "Actions",
		Class: "w-16",
	})
	return c
}

// NewData creates a new empty TableData
func NewData() TableData {
	return TableData{
		Items: []map[string]any{},
	}
}

// AddItem adds an item to the table data
func (d *TableData) AddItem(item map[string]any) *TableData {
	d.Items = append(d.Items, item)
	return d
}