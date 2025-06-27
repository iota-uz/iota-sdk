package exportconfig

import (
	"fmt"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/excel"
)

// Query represents an SQL query with its arguments
type Query interface {
	SQL() string
	Args() []interface{}
}

// NewQuery creates a new Query with the given SQL and arguments
func NewQuery(sql string, args ...interface{}) Query {
	return &query{
		sql:  sql,
		args: args,
	}
}

type query struct {
	sql  string
	args []interface{}
}

func (q *query) SQL() string {
	return q.sql
}

func (q *query) Args() []interface{} {
	return q.args
}

type Option func(c *exportConfig)

// WithFilename sets the export filename
func WithFilename(filename string) Option {
	return func(c *exportConfig) {
		c.filename = filename
	}
}

// WithExportOptions sets Excel export options
func WithExportOptions(opts *excel.ExportOptions) Option {
	return func(c *exportConfig) {
		c.exportOpts = opts
	}
}

// WithStyleOptions sets Excel style options
func WithStyleOptions(opts *excel.StyleOptions) Option {
	return func(c *exportConfig) {
		c.styleOpts = opts
	}
}

// ExportConfig represents export configuration
type ExportConfig interface {
	Filename() string
	ExportOptions() *excel.ExportOptions
	StyleOptions() *excel.StyleOptions
}

// New creates a new export configuration
func New(opts ...Option) ExportConfig {
	config := &exportConfig{
		filename:   "",
		exportOpts: nil,
		styleOpts:  nil,
	}
	for _, opt := range opts {
		opt(config)
	}
	return config
}

type exportConfig struct {
	filename   string
	exportOpts *excel.ExportOptions
	styleOpts  *excel.StyleOptions
}

func (c *exportConfig) Filename() string {
	if c.filename == "" {
		return fmt.Sprintf("export_%s.xlsx", time.Now().Format("20060102_150405"))
	}
	// Ensure filename has .xlsx extension
	if len(c.filename) < 5 || c.filename[len(c.filename)-5:] != ".xlsx" {
		return c.filename + ".xlsx"
	}
	return c.filename
}

func (c *exportConfig) ExportOptions() *excel.ExportOptions {
	return c.exportOpts
}

func (c *exportConfig) StyleOptions() *excel.StyleOptions {
	return c.styleOpts
}
