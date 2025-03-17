package scaffold

import (
	"fmt"
	"log"
	"time"
)

type TableColumn struct {
	Key    string
	Label  string
	Class  string
	Width  string
	Format func(any) string
}

type TableConfig struct {
	Columns []TableColumn
	Title   string
}

type TableData struct {
	Items []map[string]any
}

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
