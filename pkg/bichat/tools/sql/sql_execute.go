// Package sql provides this package.
package sql

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	tools "github.com/iota-uz/iota-sdk/pkg/bichat/tools"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/formatters"
	"github.com/iota-uz/iota-sdk/pkg/bichat/permissions"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// SQLExecuteToolOption configures a SQLExecuteTool.
type SQLExecuteToolOption func(*SQLExecuteTool)

// SQLExecuteTool executes SQL queries against a database via bichatsql.QueryExecutor.
// It validates queries to ensure they are read-only and enforces row limits.
// Optionally checks view permissions before executing queries.
type SQLExecuteTool struct {
	executor   bichatsql.QueryExecutor
	viewAccess permissions.ViewAccessControl
}

// NewSQLExecuteTool creates a new SQL execute tool.
// The executor parameter provides database access and should be provided by the consumer.
// Optional WithViewAccessControl option enables permission checking.
func NewSQLExecuteTool(executor bichatsql.QueryExecutor, opts ...SQLExecuteToolOption) *SQLExecuteTool {
	tool := &SQLExecuteTool{
		executor: executor,
	}

	for _, opt := range opts {
		opt(tool)
	}

	return tool
}

// WithViewAccessControl adds view permission checking to the SQL execute tool.
// When configured, the tool will validate that the user has permission to access
// all views referenced in the SQL query before execution.
func WithViewAccessControl(vac permissions.ViewAccessControl) SQLExecuteToolOption {
	return func(t *SQLExecuteTool) {
		t.viewAccess = vac
	}
}

// Name returns the tool name.
func (t *SQLExecuteTool) Name() string {
	return "sql_execute"
}

// Description returns the tool description for the LLM.
func (t *SQLExecuteTool) Description() string {
	return "Execute a read-only SQL query (SELECT or WITH...SELECT only) against the configured database. " +
		"Always use schema-qualified table names (e.g., schema.table). " +
		"Use small limits for previews (default 25, max 1000). Query timeout is 30 seconds; results limited to 1000 rows. " +
		"Supports positional parameters $1..$n via params array. Set explain_plan=true to return an EXPLAIN plan instead of rows. " +
		"Always validate table/column names using schema_list and schema_describe first. " +
		"Searching: for structured IDs (UUIDs, order IDs) use exact equality (=); for names, policy numbers, license plates use ILIKE with wildcards (e.g. WHERE name ILIKE '%ali%'). " +
		"When a query returns 0 rows but the user expects results, try a broader/fuzzy search; if you find close matches, use ask_user_question to let the user pick. " +
		"Resolve-then-query: once you identify an entity by name, get its concrete ID and use that for follow-up queries. " +
		"On error: the response includes diagnosis (code, table, column, suggestions). For COLUMN_NOT_FOUND/TABLE_NOT_FOUND/TYPE_MISMATCH/SYNTAX_ERROR/AMBIGUOUS_COLUMN, use schema tools to fix and retry (max 2 retries), then explain to the user if it persists. " +
		"Returns plain Markdown including a preview table and the executed SQL."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *SQLExecuteTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The SQL SELECT query to execute. Must be read-only.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of rows to return (default: 25, max: 1000). Use small limits for previews; use export_query_to_excel for large exports.",
				"default":     25,
				"minimum":     1,
				"maximum":     1000,
			},
			"params": map[string]any{
				"type":        "array",
				"description": "Positional parameters for placeholders $1..$n, e.g. [123, \"Alice\"].",
				"items":       map[string]any{},
			},
			"explain_plan": map[string]any{
				"type":        "boolean",
				"description": "If true, return the query execution plan instead of results",
				"default":     false,
			},
		},
		"required": []string{"query"},
	}
}

// sqlExecuteInput represents the parsed input parameters.
type sqlExecuteInput struct {
	Query       string `json:"query"`
	Params      []any  `json:"params,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	ExplainPlan bool   `json:"explain_plan,omitempty"`
}

// placeholderPattern matches PostgreSQL placeholder syntax ($1, $2, etc.)
var placeholderPattern = regexp.MustCompile(`\$\d+`)

const (
	defaultSQLExecuteLimit = 25
	maxSQLExecuteLimit     = 1000

	// previewMaxRows caps the markdown preview for token efficiency.
	previewMaxRows = 25

	// explainMaxLines caps explain output lines.
	explainMaxLines = 200
)

// CallStructured executes the SQL query and returns a structured result.
func (t *SQLExecuteTool) CallStructured(ctx context.Context, input string) (*types.ToolResult, error) {
	const op serrors.Op = "SQLExecuteTool.CallStructured"

	params, err := agents.ParseToolInput[sqlExecuteInput](input)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: fmt.Sprintf("failed to parse input: %v", err),
				Hints:   []string{tools.HintCheckRequiredFields, tools.HintCheckFieldTypes},
			},
		}, agents.ErrStructuredToolOutput
	}

	if params.Query == "" {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: "query parameter is required",
				Hints:   []string{tools.HintCheckRequiredFields},
			},
		}, nil
	}

	if params.Limit == 0 {
		params.Limit = defaultSQLExecuteLimit
	}
	if params.Limit < 1 {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: "limit must be a positive integer",
				Hints:   []string{tools.HintCheckFieldTypes, "Use limit between 1 and 1000"},
			},
		}, nil
	}
	if params.Limit > maxSQLExecuteLimit {
		params.Limit = maxSQLExecuteLimit
	}

	normalizedQuery := NormalizeSQL(params.Query)

	if err := ValidateReadOnlyQuery(normalizedQuery); err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodePolicyViolation),
				Message: err.Error(),
				Hints:   []string{tools.HintOnlySelectAllowed, tools.HintNoWriteOperations, tools.HintUseSchemaList},
			},
		}, agents.ErrStructuredToolOutput
	}

	if t.viewAccess != nil {
		deniedViews, err := t.viewAccess.CheckQueryPermissions(ctx, normalizedQuery)
		if err != nil {
			return &types.ToolResult{
				CodecID: types.CodecToolError,
				Payload: types.ToolErrorPayload{
					Code:    string(tools.ErrCodeQueryError),
					Message: fmt.Sprintf("failed to check query permissions: %v", err),
					Hints:   []string{"Contact administrator if this error persists"},
				},
			}, serrors.E(op, err)
		}

		if len(deniedViews) > 0 {
			user, userErr := composables.UseUser(ctx)
			userName := "User"
			if userErr == nil {
				userName = fmt.Sprintf("%s %s", user.FirstName(), user.LastName())
			}

			errMsg := permissions.FormatPermissionError(userName, deniedViews)

			return &types.ToolResult{
				CodecID: types.CodecToolError,
				Payload: types.ToolErrorPayload{
					Code:    string(tools.ErrCodePermissionDenied),
					Message: errMsg,
					Hints:   []string{tools.HintRequestAccess, tools.HintCheckAccessibleViews},
				},
			}, nil
		}
	}

	if err := validateQueryParameters(normalizedQuery, params.Params); err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: err.Error(),
				Hints:   []string{"Use parameter binding for SQL injection protection", "Provide params as a JSON array matching $1..$n placeholders", tools.HintCheckSQLSyntax},
			},
		}, agents.ErrStructuredToolOutput
	}

	// Explain plan mode
	if params.ExplainPlan {
		explainSQL := fmt.Sprintf("EXPLAIN (FORMAT TEXT, VERBOSE) %s", normalizedQuery)
		start := time.Now()
		explainResult, err := t.executor.ExecuteQuery(ctx, explainSQL, params.Params, 30*time.Second)
		duration := time.Since(start)
		if err != nil {
			return &types.ToolResult{
				CodecID: types.CodecToolError,
				Payload: types.ToolErrorPayload{
					Code:    string(tools.ErrCodeQueryError),
					Message: fmt.Sprintf("EXPLAIN failed: %v", err),
					Hints:   []string{tools.HintCheckSQLSyntax, tools.HintVerifyTableNames, tools.HintCheckJoinConditions},
				},
			}, nil
		}

		planLines := extractExplainLines(explainResult, explainMaxLines)
		return &types.ToolResult{
			CodecID: types.CodecExplainPlan,
			Payload: types.ExplainPlanPayload{
				Query:       normalizedQuery,
				ExecutedSQL: explainSQL,
				DurationMs:  duration.Milliseconds(),
				PlanLines:   planLines,
				Truncated:   len(explainResult.Rows) > len(planLines),
			},
		}, nil
	}

	// Execute with tool-level limit enforced at SQL layer
	effectiveLimit := params.Limit
	fetchLimit := effectiveLimit + 1

	executedSQL := WrapQueryWithLimit(normalizedQuery, fetchLimit)

	start := time.Now()
	result, err := t.executor.ExecuteQuery(ctx, executedSQL, params.Params, 30*time.Second)
	duration := time.Since(start)
	if err != nil {
		diagnosis := ClassifySQLError(err)
		return &types.ToolResult{
			CodecID: types.CodecSQLDiagnosis,
			Payload: types.SQLDiagnosisPayload{
				Code:       string(diagnosis.Code),
				Message:    diagnosis.Message,
				Table:      diagnosis.Table,
				Column:     diagnosis.Column,
				Suggestion: diagnosis.Suggestion,
				Hints:      diagnosis.Hints,
			},
		}, nil
	}

	truncated := false
	truncatedReason := ""

	rows := result.Rows
	if len(rows) > effectiveLimit {
		truncated = true
		truncatedReason = "limit"
		rows = rows[:effectiveLimit]
	}
	// bichatsql.SafeQueryExecutor sets Truncated=true when its own
	// maxResultRows cap fires (typically higher than the tool-level limit).
	// Custom QueryExecutor implementations may do the same.
	if result.Truncated {
		truncated = true
		if truncatedReason == "" {
			truncatedReason = "system_cap"
		}
	}

	previewRows := MinInt(len(rows), previewMaxRows)

	var hints []string
	if len(rows) == 0 {
		hints = emptyResultHints(normalizedQuery)
	}

	return &types.ToolResult{
		CodecID: types.CodecQueryResult,
		Payload: types.QueryResultFormatPayload{
			Query:           normalizedQuery,
			ExecutedSQL:     executedSQL,
			DurationMs:      duration.Milliseconds(),
			Columns:         result.Columns,
			Rows:            rows[:previewRows],
			RowCount:        len(rows),
			Limit:           effectiveLimit,
			Truncated:       truncated,
			TruncatedReason: truncatedReason,
			Hints:           hints,
		},
	}, nil
}

// Call executes the SQL query and returns results as plain markdown/text.
func (t *SQLExecuteTool) Call(ctx context.Context, input string) (string, error) {
	result, err := t.CallStructured(ctx, input)
	if err != nil {
		if result != nil {
			registry := formatters.DefaultFormatterRegistry()
			if f := registry.Get(result.CodecID); f != nil {
				formatted, fmtErr := f.Format(result.Payload, types.DefaultFormatOptions())
				if fmtErr == nil {
					if errors.Is(err, agents.ErrStructuredToolOutput) {
						return formatted, nil
					}
					return formatted, err
				}
			}
			formatted, _ := agents.FormatToolOutput(result.Payload)
			return formatted, err
		}
		return "", err
	}
	return tools.FormatStructuredResult(result, nil)
}

// ValidateReadOnlyQuery validates that the query is a SELECT statement.
func ValidateReadOnlyQuery(query string) error {
	tokens := tokenizeSQLForValidation(query)
	if len(tokens) == 0 {
		return fmt.Errorf("only SELECT queries are allowed")
	}

	// Must start with SELECT or WITH (for CTEs)
	if tokens[0] != "SELECT" && tokens[0] != "WITH" {
		return fmt.Errorf("only SELECT queries are allowed")
	}

	// Blacklist dangerous keywords
	dangerousKeywords := map[string]struct{}{
		"INSERT":   {},
		"UPDATE":   {},
		"DELETE":   {},
		"DROP":     {},
		"CREATE":   {},
		"ALTER":    {},
		"TRUNCATE": {},
		"GRANT":    {},
		"REVOKE":   {},
		"EXEC":     {},
		"EXECUTE":  {},
	}

	for _, token := range tokens {
		if _, blocked := dangerousKeywords[token]; blocked {
			return fmt.Errorf("query contains disallowed keyword: %s", token)
		}
	}

	return nil
}

func tokenizeSQLForValidation(query string) []string {
	src := strings.TrimSpace(query)
	if src == "" {
		return nil
	}

	tokens := make([]string, 0, 16)
	n := len(src)
	i := 0

	for i < n {
		ch := src[i]

		// Skip whitespace.
		if isWhitespace(ch) {
			i++
			continue
		}

		// Skip line comments: -- comment
		if ch == '-' && i+1 < n && src[i+1] == '-' {
			i += 2
			for i < n && src[i] != '\n' {
				i++
			}
			continue
		}

		// Skip block comments: /* comment */
		if ch == '/' && i+1 < n && src[i+1] == '*' {
			i += 2
			for i+1 < n && (src[i] != '*' || src[i+1] != '/') {
				i++
			}
			if i+1 < n {
				i += 2
			}
			continue
		}

		// Skip single-quoted strings, handling escaped quotes ('').
		if ch == '\'' {
			i++
			for i < n {
				if src[i] == '\'' {
					if i+1 < n && src[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			continue
		}

		// Skip double-quoted identifiers, handling escaped quotes ("").
		if ch == '"' {
			i++
			for i < n {
				if src[i] == '"' {
					if i+1 < n && src[i+1] == '"' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			continue
		}

		// Skip dollar-quoted literals ($$...$$ or $tag$...$tag$) and placeholders ($1).
		if ch == '$' {
			// Placeholder ($1, $2, ...)
			if i+1 < n && isDigit(src[i+1]) {
				i += 2
				for i < n && isDigit(src[i]) {
					i++
				}
				continue
			}

			// Dollar-quoted string.
			j := i + 1
			for j < n && isIdentifierPart(src[j]) {
				j++
			}
			if j < n && src[j] == '$' {
				tag := src[i : j+1]
				closeIdx := strings.Index(src[j+1:], tag)
				if closeIdx >= 0 {
					i = j + 1 + closeIdx + len(tag)
					continue
				}
			}
		}

		// Capture identifier-like token.
		if isIdentifierStart(ch) {
			start := i
			i++
			for i < n && isIdentifierPart(src[i]) {
				i++
			}
			tokens = append(tokens, strings.ToUpper(src[start:i]))
			continue
		}

		i++
	}

	return tokens
}

func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '\f'
}

func isIdentifierStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentifierPart(ch byte) bool {
	return isIdentifierStart(ch) || isDigit(ch)
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

// validateQueryParameters checks for placeholder/parameter mismatches.
// Returns an error if the query contains placeholders but no params are provided.
func validateQueryParameters(query string, params []any) error {
	placeholders := placeholderPattern.FindAllString(query, -1)

	// If placeholders found but no params provided
	if len(placeholders) > 0 && len(params) == 0 {
		// Extract unique placeholders for clearer error message
		uniquePlaceholders := make(map[string]bool)
		for _, ph := range placeholders {
			uniquePlaceholders[ph] = true
		}

		placeholderList := make([]string, 0, len(uniquePlaceholders))
		for ph := range uniquePlaceholders {
			placeholderList = append(placeholderList, ph)
		}

		return fmt.Errorf("query contains placeholders (%s) but no params provided. Use parameter binding for SQL injection protection", strings.Join(placeholderList, ", "))
	}

	// If params provided but no placeholders found
	if len(params) > 0 && len(placeholders) == 0 {
		return fmt.Errorf("params provided but query contains no placeholders")
	}

	// If both are present, ensure max placeholder index is within params length.
	if len(placeholders) > 0 && len(params) > 0 {
		maxIdx := 0
		for _, ph := range placeholders {
			// ph is like "$12"
			n, err := strconv.Atoi(strings.TrimPrefix(ph, "$"))
			if err != nil {
				continue
			}
			if n > maxIdx {
				maxIdx = n
			}
		}
		if maxIdx > len(params) {
			return fmt.Errorf("query references placeholder $%d but params has length %d", maxIdx, len(params))
		}
	}

	return nil
}

func NormalizeSQL(q string) string {
	q = strings.TrimSpace(q)
	q = strings.TrimSuffix(q, ";")
	return strings.TrimSpace(q)
}

func WrapQueryWithLimit(query string, limit int) string {
	// Wrap to enforce tool-level LIMIT without rewriting the user's SQL.
	// NOTE: query must already be normalized (no trailing semicolon).
	return fmt.Sprintf("SELECT * FROM (%s) AS _bichat_q LIMIT %d", query, limit)
}

func extractExplainLines(result *bichatsql.QueryResult, maxLines int) []string {
	if result == nil || len(result.Rows) == 0 {
		return nil
	}
	lines := make([]string, 0, MinInt(len(result.Rows), maxLines))
	for _, row := range result.Rows {
		if len(lines) >= maxLines {
			break
		}
		if len(row) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprint(row[0]))
	}
	return lines
}

// emptyResultHints returns contextual hints when a query returns zero rows,
// nudging the LLM to retry with broader matching before telling the user "not found".
func emptyResultHints(query string) []string {
	upper := strings.ToUpper(query)
	if !strings.Contains(upper, "WHERE") {
		// No WHERE clause — nothing to broaden.
		return nil
	}

	hints := []string{
		"Query returned 0 rows. If you filtered by a user-provided name, identifier, or keyword, " +
			"it may contain typos or formatting differences. Try a broader search: " +
			"use ILIKE '%partial%' instead of exact =, or split the name into parts. " +
			"If you find close but not exact matches, use ask_user_question to confirm with the user.",
	}

	// If the query uses exact equality on string-like columns, suggest ILIKE specifically.
	if strings.Contains(upper, "= '") && !strings.Contains(upper, "ILIKE") && !strings.Contains(upper, "LIKE") {
		hints = append(hints,
			"Your query uses exact string matching (= '...'). Consider replacing with ILIKE '%...%' for fuzzy matching.")
	}

	return hints
}

func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Note: SafeQueryExecutor in pkg/bichat/sql replaces the previous
// DefaultQueryExecutor. Construct it via bichatsql.NewSafeQueryExecutor;
// it implements the same QueryExecutor interface.
//
// Value formatting (FormatValue, FormatNumeric, NumericToString) lives in
// pkg/bichat/sql/format.go.
