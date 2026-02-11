package formatters

import (
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// QueryResultFormatter formats SQL query results as markdown tables.
type QueryResultFormatter struct{}

// NewQueryResultFormatter creates a new query result formatter.
func NewQueryResultFormatter() *QueryResultFormatter {
	return &QueryResultFormatter{}
}

// Format renders a QueryResultFormatPayload as markdown.
func (f *QueryResultFormatter) Format(payload any, opts types.FormatOptions) (string, error) {
	p, ok := payload.(types.QueryResultFormatPayload)
	if !ok {
		return "", fmt.Errorf("QueryResultFormatter: expected QueryResultFormatPayload, got %T", payload)
	}

	maxRows := opts.MaxRows
	if maxRows <= 0 {
		maxRows = 25
	}
	maxCellWidth := opts.MaxCellWidth
	if maxCellWidth <= 0 {
		maxCellWidth = 80
	}

	var b strings.Builder
	b.WriteString("Query executed successfully.\n\n")
	b.WriteString(fmt.Sprintf("- Query: `%s`\n", p.Query))
	b.WriteString(fmt.Sprintf("- Duration: %dms\n", p.DurationMs))
	b.WriteString(fmt.Sprintf("- Returned: %d row(s)\n", p.RowCount))
	b.WriteString(fmt.Sprintf("- Limit: %d\n", p.Limit))
	if p.Truncated {
		reason := p.TruncatedReason
		if reason == "" {
			reason = "limit"
		}
		b.WriteString(fmt.Sprintf("- Truncated: yes (`%s`)\n", reason))
	} else {
		b.WriteString("- Truncated: no\n")
	}
	b.WriteString("\n\n")

	if len(p.Columns) == 0 {
		b.WriteString("No columns returned.\n")
		b.WriteString("\nExecuted SQL:\n\n```sql\n")
		b.WriteString(p.ExecutedSQL)
		b.WriteString("\n```\n")
		return b.String(), nil
	}
	if len(p.Rows) == 0 {
		b.WriteString("No rows returned.\n")
		if len(p.Hints) > 0 {
			b.WriteString("\n**Hints:**\n")
			for _, hint := range p.Hints {
				b.WriteString(fmt.Sprintf("- %s\n", hint))
			}
		}
		b.WriteString("\nExecuted SQL:\n\n```sql\n")
		b.WriteString(p.ExecutedSQL)
		b.WriteString("\n```\n")
		return b.String(), nil
	}

	// Limit rows for preview
	rows := p.Rows
	if len(rows) > maxRows {
		rows = rows[:maxRows]
	}

	// Header
	b.WriteString("| ")
	for i, c := range p.Columns {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(EscapeMarkdownCell(c, maxCellWidth))
	}
	b.WriteString(" |\n")

	// Separator
	b.WriteString("|")
	for range p.Columns {
		b.WriteString(" --- |")
	}
	b.WriteString("\n")

	// Rows
	for _, row := range rows {
		b.WriteString("| ")
		for i := range p.Columns {
			if i > 0 {
				b.WriteString(" | ")
			}
			var v any
			if i < len(row) {
				v = row[i]
			}
			b.WriteString(EscapeMarkdownCell(formatPreviewValue(v), maxCellWidth))
		}
		b.WriteString(" |\n")
	}

	if p.Truncated {
		b.WriteString("\n")
		b.WriteString("Use a follow-up query with tighter WHERE filters or a smaller projection. For more rows, increase limit (max 1000) or use export_query_to_excel for large exports.\n")
	}

	b.WriteString("\nExecuted SQL:\n\n```sql\n")
	b.WriteString(p.ExecutedSQL)
	b.WriteString("\n```\n")

	return b.String(), nil
}

// ExplainPlanFormatter formats EXPLAIN plan output.
type ExplainPlanFormatter struct{}

// NewExplainPlanFormatter creates a new explain plan formatter.
func NewExplainPlanFormatter() *ExplainPlanFormatter {
	return &ExplainPlanFormatter{}
}

// Format renders an ExplainPlanPayload as markdown.
func (f *ExplainPlanFormatter) Format(payload any, opts types.FormatOptions) (string, error) {
	p, ok := payload.(types.ExplainPlanPayload)
	if !ok {
		return "", fmt.Errorf("ExplainPlanFormatter: expected ExplainPlanPayload, got %T", payload)
	}

	var b strings.Builder
	b.WriteString("Explain plan generated successfully.\n\n")
	b.WriteString(fmt.Sprintf("- Query: `%s`\n", p.Query))
	b.WriteString(fmt.Sprintf("- Duration: %dms\n\n", p.DurationMs))

	// Plan
	if len(p.PlanLines) == 0 {
		b.WriteString("No plan output.\n")
	} else {
		b.WriteString("```text\n")
		for _, l := range p.PlanLines {
			b.WriteString(l)
			b.WriteString("\n")
		}
		if p.Truncated {
			b.WriteString("...\n")
		}
		b.WriteString("```\n")
	}

	b.WriteString("\nExecuted SQL:\n\n```sql\n")
	b.WriteString(p.ExecutedSQL)
	b.WriteString("\n```\n")

	return b.String(), nil
}

func formatPreviewValue(v any) string {
	if v == nil {
		return "NULL"
	}
	return fmt.Sprint(v)
}
