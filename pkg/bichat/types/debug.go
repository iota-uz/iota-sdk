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

// DebugTrace stores deterministic debug metadata for one assistant response.
type DebugTrace struct {
	Usage             *DebugUsage     `json:"usage,omitempty"`
	GenerationMs      int64           `json:"generationMs,omitempty"`
	Tools             []DebugToolCall `json:"tools,omitempty"`
	TraceID           string          `json:"traceId,omitempty"`
	TraceURL          string          `json:"traceUrl,omitempty"`
	SessionID         string          `json:"sessionId,omitempty"`
	Thinking          string          `json:"thinking,omitempty"`
	ObservationReason string          `json:"observationReason,omitempty"`
}
