package observability

// AttributeKey represents a type-safe attribute key for observations.
type AttributeKey string

// Common attribute keys for observations.
const (
	// Generation attributes
	AttrModelName           AttributeKey = "model.name"
	AttrModelProvider       AttributeKey = "model.provider"
	AttrModelTemperature    AttributeKey = "model.temperature"
	AttrModelMaxTokens      AttributeKey = "model.max_tokens"
	AttrGenerationID        AttributeKey = "generation.id"
	AttrGenerationLatencyMs AttributeKey = "generation.latency_ms"
	AttrGenerationTokens    AttributeKey = "generation.tokens"
	AttrFinishReason        AttributeKey = "generation.finish_reason"

	// Tool attributes
	AttrToolName       AttributeKey = "tool.name"
	AttrToolCallID     AttributeKey = "tool.call_id"
	AttrToolInput      AttributeKey = "tool.input"
	AttrToolOutput     AttributeKey = "tool.output"
	AttrToolError      AttributeKey = "tool.error"
	AttrToolDurationMs AttributeKey = "tool.duration_ms"

	// Context attributes
	AttrContextBlocks         AttributeKey = "context.blocks"
	AttrContextTokens         AttributeKey = "context.tokens"
	AttrContextWindow         AttributeKey = "context.window"
	AttrContextOverflow       AttributeKey = "context.overflow"
	AttrContextCompactionUsed AttributeKey = "context.compaction_used"

	// Session attributes
	AttrSessionID    AttributeKey = "session.id"
	AttrSessionTitle AttributeKey = "session.title"
	AttrUserID       AttributeKey = "user.id"
	AttrTenantID     AttributeKey = "tenant.id"

	// Cost attributes
	AttrCostUSD           AttributeKey = "cost.usd"
	AttrCostPromptUSD     AttributeKey = "cost.prompt_usd"
	AttrCostCompletionUSD AttributeKey = "cost.completion_usd"

	// Trace attributes
	AttrTraceID       AttributeKey = "trace.id"
	AttrTraceParentID AttributeKey = "trace.parent_id"
	AttrTraceName     AttributeKey = "trace.name"
	AttrTraceStatus   AttributeKey = "trace.status"
)

// Attributes is a type-safe map for observation metadata.
type Attributes map[string]interface{}

// NewAttributes creates a new empty Attributes map.
func NewAttributes() Attributes {
	return make(Attributes)
}

// Set adds a key-value pair to the attributes.
func (a Attributes) Set(key AttributeKey, value interface{}) Attributes {
	a[string(key)] = value
	return a
}

// SetString adds a string attribute.
func (a Attributes) SetString(key AttributeKey, value string) Attributes {
	a[string(key)] = value
	return a
}

// SetInt adds an int attribute.
func (a Attributes) SetInt(key AttributeKey, value int) Attributes {
	a[string(key)] = value
	return a
}

// SetInt64 adds an int64 attribute.
func (a Attributes) SetInt64(key AttributeKey, value int64) Attributes {
	a[string(key)] = value
	return a
}

// SetFloat64 adds a float64 attribute.
func (a Attributes) SetFloat64(key AttributeKey, value float64) Attributes {
	a[string(key)] = value
	return a
}

// SetBool adds a bool attribute.
func (a Attributes) SetBool(key AttributeKey, value bool) Attributes {
	a[string(key)] = value
	return a
}

// Get retrieves a value from attributes.
func (a Attributes) Get(key AttributeKey) (interface{}, bool) {
	val, ok := a[string(key)]
	return val, ok
}

// GetString retrieves a string value from attributes.
func (a Attributes) GetString(key AttributeKey) (string, bool) {
	val, ok := a[string(key)]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetInt retrieves an int value from attributes.
func (a Attributes) GetInt(key AttributeKey) (int, bool) {
	val, ok := a[string(key)]
	if !ok {
		return 0, false
	}
	i, ok := val.(int)
	return i, ok
}

// GetInt64 retrieves an int64 value from attributes.
func (a Attributes) GetInt64(key AttributeKey) (int64, bool) {
	val, ok := a[string(key)]
	if !ok {
		return 0, false
	}
	i, ok := val.(int64)
	return i, ok
}

// GetFloat64 retrieves a float64 value from attributes.
func (a Attributes) GetFloat64(key AttributeKey) (float64, bool) {
	val, ok := a[string(key)]
	if !ok {
		return 0, false
	}
	f, ok := val.(float64)
	return f, ok
}

// GetBool retrieves a bool value from attributes.
func (a Attributes) GetBool(key AttributeKey) (bool, bool) {
	val, ok := a[string(key)]
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

// Merge merges another Attributes map into this one.
// Values from the other map override values in this map.
func (a Attributes) Merge(other Attributes) Attributes {
	for k, v := range other {
		a[k] = v
	}
	return a
}

// Copy creates a shallow copy of the attributes.
func (a Attributes) Copy() Attributes {
	copy := make(Attributes, len(a))
	for k, v := range a {
		copy[k] = v
	}
	return copy
}
