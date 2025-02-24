package ast

import (
	"github.com/iota-uz/iota-sdk/pkg/schema/types"
)

// ParserOptions configures the SQL parser behavior
type ParserOptions struct {
	StrictMode     bool
	SkipComments   bool
	MaxErrors      int
	SkipValidation bool
}

// Parser represents an SQL parser
type Parser struct {
	dialect string
	options ParserOptions
}

// ParseSQL parses SQL content into a SchemaTree using the default postgres dialect
func ParseSQL(content string) (*types.SchemaTree, error) {
	p := NewParser("postgres", ParserOptions{})
	return p.Parse(content)
}

// NewParser creates a new SQL parser instance
func NewParser(dialect string, opts ParserOptions) *Parser {
	return &Parser{
		dialect: dialect,
		options: opts,
	}
}

// GetDialect returns the dialect name used by the parser
func (p *Parser) GetDialect() string {
	return p.dialect
}
