package types

// ToolCall represents a request to execute a tool with specific arguments.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCallResult represents the result of executing a tool call.
type ToolCallResult struct {
	// ToolCall embeds the original tool call information
	ToolCall

	// Output contains the result of the tool execution
	Output string

	// Error contains any error that occurred during tool execution
	Error error

	// DurationMs is the execution duration in milliseconds
	DurationMs int64
}
