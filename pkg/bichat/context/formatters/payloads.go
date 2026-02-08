package formatters

// QueryResultFormatPayload is the payload for the "query-result" formatter.
// It extends the codec payload with presentation metadata.
type QueryResultFormatPayload struct {
	Query           string   `json:"query"`
	ExecutedSQL     string   `json:"executed_sql"`
	DurationMs      int64    `json:"duration_ms"`
	Columns         []string `json:"columns"`
	Rows            [][]any  `json:"rows"`
	RowCount        int      `json:"row_count"`
	Limit           int      `json:"limit"`
	Truncated       bool     `json:"truncated"`
	TruncatedReason string   `json:"truncated_reason,omitempty"`
}

// ExplainPlanPayload is the payload for the "explain-plan" formatter.
type ExplainPlanPayload struct {
	Query       string   `json:"query"`
	ExecutedSQL string   `json:"executed_sql"`
	DurationMs  int64    `json:"duration_ms"`
	PlanLines   []string `json:"plan_lines"`
	Truncated   bool     `json:"truncated"`
}

// SchemaListPayload is the payload for the "schema-list" formatter.
type SchemaListPayload struct {
	Tables    []SchemaListTable `json:"tables"`
	ViewInfos []ViewAccessInfo  `json:"view_infos,omitempty"`
	HasAccess bool              `json:"has_access"` // whether access control is enabled
}

// SchemaListTable represents a table in the schema list.
type SchemaListTable struct {
	Name        string `json:"name"`
	RowCount    int64  `json:"row_count"`
	Description string `json:"description,omitempty"`
}

// ViewAccessInfo represents access info for a view.
type ViewAccessInfo struct {
	Access string `json:"access"` // "ok" or "denied"
}

// SchemaDescribePayload is the payload for the "schema-describe" formatter.
type SchemaDescribePayload struct {
	Name    string                 `json:"name"`
	Schema  string                 `json:"schema"`
	Columns []SchemaDescribeColumn `json:"columns"`
}

// SchemaDescribeColumn represents a column in the schema describe result.
type SchemaDescribeColumn struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Nullable     bool    `json:"nullable"`
	DefaultValue *string `json:"default_value,omitempty"`
	Description  string  `json:"description,omitempty"`
}

// SearchResultsPayload is the generic payload for search results (KB, learnings, validated queries).
type SearchResultsPayload struct {
	Output any `json:"output"` // The original output struct to serialize as JSON
}

// ToolErrorPayload is the payload for the "tool-error" formatter.
type ToolErrorPayload struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Hints   []string `json:"hints,omitempty"`
}

// SQLDiagnosisPayload is the payload for the "sql-diagnosis" formatter.
type SQLDiagnosisPayload struct {
	Code       string   `json:"code"`
	Message    string   `json:"message"`
	Table      string   `json:"table,omitempty"`
	Column     string   `json:"column,omitempty"`
	Suggestion string   `json:"suggestion"`
	Hints      []string `json:"hints"`
}

// TimePayload is the payload for the "time" formatter.
type TimePayload struct {
	Output any `json:"output"` // The timeToolOutput struct
}

// ArtifactListPayload is the payload for the "artifact-list" formatter.
type ArtifactListPayload struct {
	Page       int             `json:"page"`
	TotalPages int             `json:"total_pages"`
	Artifacts  []ArtifactEntry `json:"artifacts"`
	HasNext    bool            `json:"has_next"`
	HitCap     bool            `json:"hit_cap"`
}

// ArtifactEntry represents an artifact in the list.
type ArtifactEntry struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
	CreatedAt string `json:"created_at"`
}

// ArtifactContentPayload is the payload for the "artifact-content" formatter.
type ArtifactContentPayload struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	MimeType   string `json:"mime_type"`
	Page       int    `json:"page"`
	TotalPages int    `json:"total_pages"`
	PageSize   int    `json:"page_size"`
	Content    string `json:"content"` // The page content
	HasNext    bool   `json:"has_next"`
	OutOfRange bool   `json:"out_of_range"`
}

// GenericJSONPayload is the payload for the "json" formatter (generic fallback).
type GenericJSONPayload struct {
	Output any `json:"output"`
}
