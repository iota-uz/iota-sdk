package formatters

import (
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// SchemaListFormatter formats schema list results as markdown tables.
type SchemaListFormatter struct{}

// NewSchemaListFormatter creates a new schema list formatter.
func NewSchemaListFormatter() *SchemaListFormatter {
	return &SchemaListFormatter{}
}

// Format renders a SchemaListPayload as markdown.
func (f *SchemaListFormatter) Format(payload any, opts types.FormatOptions) (string, error) {
	p, ok := payload.(types.SchemaListPayload)
	if !ok {
		return "", fmt.Errorf("SchemaListFormatter: expected SchemaListPayload, got %T", payload)
	}

	var b strings.Builder

	// Header
	if p.HasAccess {
		b.WriteString("| Table | Est. Rows | Access | Description |\n")
		b.WriteString("| --- | --- | --- | --- |\n")
	} else {
		b.WriteString("| Table | Est. Rows | Description |\n")
		b.WriteString("| --- | --- | --- |\n")
	}

	// Rows
	maxCell := opts.MaxCellWidth
	for i, table := range p.Tables {
		b.WriteString(fmt.Sprintf("| %s | ", EscapeMarkdownCell(table.Name, maxCell)))
		b.WriteString(abbreviateCount(table.RowCount) + " | ")

		if p.HasAccess {
			if i < len(p.ViewInfos) {
				b.WriteString(EscapeMarkdownCell(p.ViewInfos[i].Access, maxCell) + " | ")
			} else {
				b.WriteString("- | ")
			}
		}

		if table.Description != "" {
			b.WriteString(EscapeMarkdownCell(table.Description, maxCell) + " |\n")
		} else {
			b.WriteString("- |\n")
		}
	}

	return b.String(), nil
}

// SchemaDescribeFormatter formats schema describe results as markdown tables.
type SchemaDescribeFormatter struct{}

// NewSchemaDescribeFormatter creates a new schema describe formatter.
func NewSchemaDescribeFormatter() *SchemaDescribeFormatter {
	return &SchemaDescribeFormatter{}
}

// Format renders a SchemaDescribePayload as markdown.
func (f *SchemaDescribeFormatter) Format(payload any, opts types.FormatOptions) (string, error) {
	p, ok := payload.(types.SchemaDescribePayload)
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

	// Header
	if hasDescription {
		b.WriteString("| Column | Type | Nullable | Default | Description |\n")
		b.WriteString("| --- | --- | --- | --- | --- |\n")
	} else {
		b.WriteString("| Column | Type | Nullable | Default |\n")
		b.WriteString("| --- | --- | --- | --- |\n")
	}

	// Rows
	for _, col := range p.Columns {
		b.WriteString(fmt.Sprintf("| %s | %s | ", EscapeMarkdownCell(col.Name, 0), EscapeMarkdownCell(col.Type, 0)))

		if col.Nullable {
			b.WriteString("YES | ")
		} else {
			b.WriteString("NO | ")
		}

		if col.DefaultValue != nil && *col.DefaultValue != "" {
			b.WriteString(EscapeMarkdownCell(*col.DefaultValue, 0) + " ")
		} else {
			b.WriteString("- ")
		}

		if hasDescription {
			b.WriteString("| ")
			if col.Description != "" {
				b.WriteString(EscapeMarkdownCell(col.Description, 0))
			} else {
				b.WriteString("-")
			}
		}

		b.WriteString("|\n")
	}

	return b.String(), nil
}

// abbreviateCount formats a row count as a human-friendly estimate.
//
//	0 → "-", 54 → "~50", 1234 → "~1.2K", 54000 → "~54K", 1243230 → "~1.2M"
func abbreviateCount(n int64) string {
	if n <= 0 {
		return "-"
	}
	switch {
	case n < 100:
		rounded := (n / 10) * 10
		if rounded == 0 {
			rounded = n
		}
		return fmt.Sprintf("~%d", rounded)
	case n < 1_000:
		return fmt.Sprintf("~%d", (n/100)*100)
	case n < 10_000:
		whole := n / 1_000
		frac := (n % 1_000) / 100
		if frac > 0 {
			return fmt.Sprintf("~%d.%dK", whole, frac)
		}
		return fmt.Sprintf("~%dK", whole)
	case n < 1_000_000:
		return fmt.Sprintf("~%dK", n/1_000)
	case n < 10_000_000:
		whole := n / 1_000_000
		frac := (n % 1_000_000) / 100_000
		if frac > 0 {
			return fmt.Sprintf("~%d.%dM", whole, frac)
		}
		return fmt.Sprintf("~%dM", whole)
	default:
		return fmt.Sprintf("~%dM", n/1_000_000)
	}
}
