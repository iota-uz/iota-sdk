package types

// ToolCall represents a request to execute a tool with specific arguments.
type ToolCall struct {
	// ID is a unique identifier for this tool call
	ID string

	// Name is the name of the tool to execute
	Name string

	// Arguments contains the JSON-encoded arguments for the tool
	Arguments string
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
