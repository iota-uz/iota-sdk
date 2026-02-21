package agents

import (
	"context"
	"sync"
)

// AgentMetadata contains descriptive information about an agent.
// This metadata helps with agent selection, routing, and documentation.
type AgentMetadata struct {
	// Name is the unique identifier for this agent.
	// This is used in delegation and registry lookups.
	Name string

	// Description is a human-readable description of what this agent does.
	// This helps users and parent agents understand the agent's capabilities.
	Description string

	// WhenToUse provides guidance on when to delegate to this agent.
	// This is used by parent agents to decide which child to invoke.
	// Example: "Use this agent when you need to query SQL databases"
	WhenToUse string

	// Model is the LLM model identifier this agent uses.
	// Example: "gpt-5.2", "claude-opus-4-6", "gpt-5-mini"
	Model string

	// TerminationTools is a list of tool names that cause the agent to stop.
	// When any of these tools are called, the ReAct loop terminates.
	// Common values: ["final_answer"]
	TerminationTools []string
}

// ExtendedAgent extends the base Agent interface with execution capabilities.
// This interface is used by the Executor to run ReAct loops.
//
// Implementation pattern:
//   - Define a struct that implements this interface
//   - Use BaseAgent helper for common functionality
//   - Override OnToolCall for custom tool routing logic
//   - Register with AgentRegistry for delegation
//
// Example:
//
//	type MyAgent struct {
//	    *BaseAgent
//	    customTool Tool
//	}
//
//	func (a *MyAgent) OnToolCall(ctx context.Context, name, input string) (string, error) {
//	    if name == a.customTool.Name() {
//	        return a.customTool.Call(ctx, input)
//	    }
//	    return "", ErrToolNotFound
//	}
type ExtendedAgent interface {
	Agent // Embeds Name() and Description() from registry.go

	// Metadata returns descriptive information about this agent.
	Metadata() AgentMetadata

	// Tools returns the list of tools available to this agent.
	// These tools will be included in the system prompt and tool definitions
	// sent to the LLM during execution.
	Tools() []Tool

	// SystemPrompt returns the system prompt for this agent.
	// The prompt can be dynamic based on context (tenant, user, etc.).
	// The returned string will be sent as the system message to the LLM.
	SystemPrompt(ctx context.Context) string

	// OnToolCall handles tool execution requests from the LLM.
	// This is called by the executor when the LLM requests a tool call.
	// The implementation should route to the appropriate tool and return the result.
	//
	// Parameters:
	//   - ctx: Request context with tenant ID, user info, etc.
	//   - toolName: The name of the tool to execute
	//   - input: JSON string containing tool parameters
	//
	// Returns:
	//   - string: Tool execution result (will be sent back to LLM)
	//   - error: Error if tool not found or execution failed
	OnToolCall(ctx context.Context, toolName, input string) (string, error)
}

// BaseAgent provides a default implementation of the ExtendedAgent interface
// using the functional options pattern.
//
// This helper simplifies agent creation by handling common boilerplate:
//   - Metadata management (implements both Agent and ExtendedAgent)
//   - Tool registration and lookup
//   - System prompt rendering
//   - Thread-safe tool routing
//
// Usage:
//
//	agent := NewBaseAgent(
//	    WithName("sql_agent"),
//	    WithDescription("Executes SQL queries"),
//	    WithModel("gpt-5.2"),
//	    WithTools(sqlTool, schemaTool),
//	    WithSystemPrompt("You are a SQL expert..."),
//	    WithTerminationTools("final_answer"),
//	)
type BaseAgent struct {
	metadata     AgentMetadata
	tools        []Tool
	systemPrompt string
	toolMap      map[string]Tool // For O(1) tool lookup
	mu           sync.RWMutex    // Protects toolMap during concurrent access
}

// NewBaseAgent creates a BaseAgent with the given options.
// This is the recommended way to create agents for most use cases.
//
// Example:
//
//	agent := NewBaseAgent(
//	    WithName("research_agent"),
//	    WithDescription("Conducts research using web search"),
//	    WithModel("gpt-5.2"),
//	    WithTools(searchTool, summaryTool),
//	    WithSystemPrompt("You are a research assistant..."),
//	    WithTerminationTools("final_answer"),
//	)
func NewBaseAgent(opts ...AgentOption) *BaseAgent {
	agent := &BaseAgent{
		metadata: AgentMetadata{
			Model:            "gpt-5.2", // Default model (SOTA)
			TerminationTools: []string{ToolFinalAnswer},
		},
		tools:   []Tool{},
		toolMap: make(map[string]Tool),
	}

	// Apply all options
	for _, opt := range opts {
		opt(agent)
	}

	// Build tool map for fast lookup
	agent.rebuildToolMap()

	return agent
}

// rebuildToolMap creates the tool lookup map from the tools slice.
// This is called after tool changes to keep the map in sync.
func (a *BaseAgent) rebuildToolMap() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.toolMap = make(map[string]Tool, len(a.tools))
	for _, tool := range a.tools {
		a.toolMap[tool.Name()] = tool
	}
}

// Name returns the agent's name (implements Agent interface from registry.go).
func (a *BaseAgent) Name() string {
	return a.metadata.Name
}

// Description returns the agent's description (implements Agent interface from registry.go).
func (a *BaseAgent) Description() string {
	return a.metadata.Description
}

// Metadata returns the agent's metadata (implements ExtendedAgent interface).
func (a *BaseAgent) Metadata() AgentMetadata {
	return a.metadata
}

// Tools returns the agent's tools (implements ExtendedAgent interface).
func (a *BaseAgent) Tools() []Tool {
	return a.tools
}

// SystemPrompt returns the agent's system prompt.
// The base implementation returns a static prompt.
// Override this method for dynamic prompts based on context.
func (a *BaseAgent) SystemPrompt(ctx context.Context) string {
	return a.systemPrompt
}

// OnToolCall routes tool calls to the appropriate tool.
// This uses the tool map for O(1) lookup.
func (a *BaseAgent) OnToolCall(ctx context.Context, toolName, input string) (string, error) {
	a.mu.RLock()
	tool, exists := a.toolMap[toolName]
	a.mu.RUnlock()

	if !exists {
		return "", ErrToolNotFound
	}

	return tool.Call(ctx, input)
}

// AgentOption is a functional option for configuring BaseAgent.
type AgentOption func(*BaseAgent)

// WithName sets the agent's name.
// The name must be unique within the agent registry.
//
// Example:
//
//	WithName("sql_expert")
func WithName(name string) AgentOption {
	return func(a *BaseAgent) {
		a.metadata.Name = name
	}
}

// WithDescription sets the agent's description.
// This should be a clear, concise explanation of what the agent does.
//
// Example:
//
//	WithDescription("Executes SQL queries and analyzes database schemas")
func WithDescription(description string) AgentOption {
	return func(a *BaseAgent) {
		a.metadata.Description = description
	}
}

// WithWhenToUse sets guidance for when to delegate to this agent.
// This helps parent agents make delegation decisions.
//
// Example:
//
//	WithWhenToUse("Use this agent when you need to query databases or analyze SQL schemas")
func WithWhenToUse(whenToUse string) AgentOption {
	return func(a *BaseAgent) {
		a.metadata.WhenToUse = whenToUse
	}
}

// WithTools sets the agent's tools.
// These tools will be available during ReAct execution.
//
// Example:
//
//	WithTools(sqlTool, schemaTool, chartTool)
func WithTools(tools ...Tool) AgentOption {
	return func(a *BaseAgent) {
		a.tools = tools
		a.rebuildToolMap()
	}
}

// WithSystemPrompt sets the agent's system prompt.
// This is the instruction sent to the LLM at the start of execution.
//
// Example:
//
//	WithSystemPrompt("You are an expert SQL database analyst. Help users query and understand their data.")
func WithSystemPrompt(prompt string) AgentOption {
	return func(a *BaseAgent) {
		a.systemPrompt = prompt
	}
}

// WithModel sets the LLM model this agent uses.
// The model must be registered in the model registry.
//
// Example:
//
//	WithModel("gpt-5.2")
//	WithModel("claude-opus-4-6")
func WithModel(model string) AgentOption {
	return func(a *BaseAgent) {
		a.metadata.Model = model
	}
}

// WithTerminationTools sets the tools that terminate execution.
// When any of these tools are called, the ReAct loop stops.
//
// Example:
//
//	WithTerminationTools("final_answer", "submit_result")
func WithTerminationTools(tools ...string) AgentOption {
	return func(a *BaseAgent) {
		a.metadata.TerminationTools = tools
	}
}
