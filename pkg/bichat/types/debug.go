package types

// DebugLimits describes the configured token limits for a model/session.
type DebugLimits struct {
	PolicyMaxTokens         int `json:"policyMaxTokens"`
	ModelMaxTokens          int `json:"modelMaxTokens"`
	EffectiveMaxTokens      int `json:"effectiveMaxTokens"`
	CompletionReserveTokens int `json:"completionReserveTokens"`
}

// DebugUsage represents token consumption metrics for a single assistant response.
type DebugUsage struct {
	PromptTokens     int     `json:"promptTokens"`
	CompletionTokens int     `json:"completionTokens"`
	TotalTokens      int     `json:"totalTokens"`
	CachedTokens     int     `json:"cachedTokens"`
	Cost             float64 `json:"cost"`
}

// DebugToolCall represents tool call details captured in debug traces.
type DebugToolCall struct {
	CallID     string `json:"callId,omitempty"`
	Name       string `json:"name,omitempty"`
	Arguments  string `json:"arguments,omitempty"`
	Result     string `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
}

// DebugGeneration represents one LLM generation attempt within a trace.
type DebugGeneration struct {
	ID                string          `json:"id,omitempty"`
	RequestID         string          `json:"requestId,omitempty"`
	Model             string          `json:"model,omitempty"`
	Provider          string          `json:"provider,omitempty"`
	FinishReason      string          `json:"finishReason,omitempty"`
	PromptTokens      int             `json:"promptTokens,omitempty"`
	CompletionTokens  int             `json:"completionTokens,omitempty"`
	TotalTokens       int             `json:"totalTokens,omitempty"`
	CachedTokens      int             `json:"cachedTokens,omitempty"`
	Cost              float64         `json:"cost,omitempty"`
	LatencyMs         int64           `json:"latencyMs,omitempty"`
	Input             string          `json:"input,omitempty"`
	Output            string          `json:"output,omitempty"`
	Thinking          string          `json:"thinking,omitempty"`
	ObservationReason string          `json:"observationReason,omitempty"`
	StartedAt         string          `json:"startedAt,omitempty"`
	CompletedAt       string          `json:"completedAt,omitempty"`
	ToolCalls         []DebugToolCall `json:"toolCalls,omitempty"`
}

// DebugSpan represents a deterministic operation span inside a trace.
type DebugSpan struct {
	ID           string                 `json:"id,omitempty"`
	ParentID     string                 `json:"parentId,omitempty"`
	GenerationID string                 `json:"generationId,omitempty"`
	Name         string                 `json:"name,omitempty"`
	Type         string                 `json:"type,omitempty"`
	Status       string                 `json:"status,omitempty"`
	Level        string                 `json:"level,omitempty"`
	CallID       string                 `json:"callId,omitempty"`
	ToolName     string                 `json:"toolName,omitempty"`
	Input        string                 `json:"input,omitempty"`
	Output       string                 `json:"output,omitempty"`
	Error        string                 `json:"error,omitempty"`
	DurationMs   int64                  `json:"durationMs,omitempty"`
	StartedAt    string                 `json:"startedAt,omitempty"`
	CompletedAt  string                 `json:"completedAt,omitempty"`
	Attributes   map[string]interface{} `json:"attributes,omitempty"`
}

// DebugEvent represents point-in-time observability events for a trace.
type DebugEvent struct {
	ID           string                 `json:"id,omitempty"`
	Name         string                 `json:"name,omitempty"`
	Type         string                 `json:"type,omitempty"`
	Level        string                 `json:"level,omitempty"`
	Message      string                 `json:"message,omitempty"`
	Reason       string                 `json:"reason,omitempty"`
	SpanID       string                 `json:"spanId,omitempty"`
	GenerationID string                 `json:"generationId,omitempty"`
	Timestamp    string                 `json:"timestamp,omitempty"`
	Attributes   map[string]interface{} `json:"attributes,omitempty"`
}

// DebugTrace stores deterministic debug metadata for one assistant response.
type DebugTrace struct {
	SchemaVersion     string            `json:"schemaVersion,omitempty"`
	StartedAt         string            `json:"startedAt,omitempty"`
	CompletedAt       string            `json:"completedAt,omitempty"`
	Usage             *DebugUsage       `json:"usage,omitempty"`
	GenerationMs      int64             `json:"generationMs,omitempty"`
	Tools             []DebugToolCall   `json:"tools,omitempty"`
	Attempts          []DebugGeneration `json:"attempts,omitempty"`
	Spans             []DebugSpan       `json:"spans,omitempty"`
	Events            []DebugEvent      `json:"events,omitempty"`
	TraceID           string            `json:"traceId,omitempty"`
	TraceURL          string            `json:"traceUrl,omitempty"`
	SessionID         string            `json:"sessionId,omitempty"`
	Thinking          string            `json:"thinking,omitempty"`
	ObservationReason string            `json:"observationReason,omitempty"`
}
