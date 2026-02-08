package formatters

import (
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
)

// SchemaListFormatter formats schema list results as markdown tables.
type SchemaListFormatter struct{}

// NewSchemaListFormatter creates a new schema list formatter.
func NewSchemaListFormatter() *SchemaListFormatter {
	return &SchemaListFormatter{}
}

// Format renders a SchemaListPayload as markdown.
func (f *SchemaListFormatter) Format(payload any, opts context.FormatOptions) (string, error) {
	p, ok := payload.(SchemaListPayload)
	if !ok {
		return "", fmt.Errorf("SchemaListFormatter: expected SchemaListPayload, got %T", payload)
	}

	var b strings.Builder
	b.WriteString("## Available Tables\n\n")

	// Header
	if p.HasAccess {
		b.WriteString("| # | Table | ~Rows | Access | Description |\n")
		b.WriteString("| --- | --- | --- | --- | --- |\n")
	} else {
		b.WriteString("| # | Table | ~Rows | Description |\n")
		b.WriteString("| --- | --- | --- | --- |\n")
	}

	// Rows
	for i, table := range p.Tables {
		b.WriteString(fmt.Sprintf("| %d | %s | ", i+1, table.Name))

		if table.RowCount > 0 {
			b.WriteString(fmt.Sprintf("~%d | ", table.RowCount))
		} else {
			b.WriteString("- | ")
		}

		if p.HasAccess {
			if i < len(p.ViewInfos) {
				b.WriteString(fmt.Sprintf("%s | ", p.ViewInfos[i].Access))
			} else {
				b.WriteString("- | ")
			}
		}

		if table.Description != "" {
			b.WriteString(fmt.Sprintf("%s |\n", table.Description))
		} else {
			b.WriteString("- |\n")
		}
	}

	b.WriteString(fmt.Sprintf("\n%d table(s) found.", len(p.Tables)))

	return b.String(), nil
}

// SchemaDescribeFormatter formats schema describe results as markdown tables.
type SchemaDescribeFormatter struct{}

// NewSchemaDescribeFormatter creates a new schema describe formatter.
func NewSchemaDescribeFormatter() *SchemaDescribeFormatter {
	return &SchemaDescribeFormatter{}
}

// Format renders a SchemaDescribePayload as markdown.
func (f *SchemaDescribeFormatter) Format(payload any, opts context.FormatOptions) (string, error) {
	p, ok := payload.(SchemaDescribePayload)
	if !ok {
		return "", fmt.Errorf("SchemaDescribeFormatter: expected SchemaDescribePayload, got %T", payload)
	}

	// Check if any column has a description
	hasDescription := false
	for _, col := range p.Columns {
		if col.Description != "" {
			hasDescription = true
			break
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Table: %s (%s)\n\n", p.Name, p.Schema))

	// Header
	if hasDescription {
		b.WriteString("| # | Column | Type | Nullable | Default | Description |\n")
		b.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	} else {
		b.WriteString("| # | Column | Type | Nullable | Default |\n")
		b.WriteString("| --- | --- | --- | --- | --- |\n")
	}

	// Rows
	for i, col := range p.Columns {
		b.WriteString(fmt.Sprintf("| %d | %s | %s | ", i+1, col.Name, col.Type))

		if col.Nullable {
			b.WriteString("YES | ")
		} else {
			b.WriteString("NO | ")
		}

		if col.DefaultValue != nil {
			b.WriteString(fmt.Sprintf("%s ", *col.DefaultValue))
		} else {
			b.WriteString("- ")
		}

		if hasDescription {
			b.WriteString("| ")
			if col.Description != "" {
				b.WriteString(col.Description)
			} else {
				b.WriteString("-")
			}
		}

		b.WriteString("|\n")
	}

	b.WriteString(fmt.Sprintf("\n%d column(s)", len(p.Columns)))

	return b.String(), nil
}
