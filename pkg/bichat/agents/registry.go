package agents

import (
	"fmt"
	"strings"
	"sync"
)

// Agent represents a minimal agent interface for registry operations.
// This interface is embedded by ExtendedAgent (defined in agent.go) which adds
// execution-related methods like Metadata(), Tools(), SystemPrompt(), etc.
//
// The registry only needs Name() and Description() for discovery and
// basic agent management. Extended functionality is provided by ExtendedAgent.
type Agent interface {
	// Name returns the unique agent identifier.
	Name() string

	// Description returns a human-readable description of the agent's purpose.
	Description() string
}

// AgentRegistry manages agent registration and discovery.
// It provides thread-safe access to registered agents and can generate
// a markdown description of all agents for use in parent agent system prompts.
//
// The registry stores ExtendedAgent instances (not just Agent) because
// delegation and execution require access to Metadata(), Tools(), etc.
type AgentRegistry struct {
	mu     sync.RWMutex
	agents map[string]ExtendedAgent
}

// NewAgentRegistry creates a new empty agent registry.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		agents: make(map[string]ExtendedAgent),
	}
}

// Register adds an agent to the registry.
// Returns an error if the agent name is empty or already registered.
// The agent must implement ExtendedAgent for delegation support.
func (r *AgentRegistry) Register(agent ExtendedAgent) error {
	if agent == nil {
		return fmt.Errorf("agent cannot be nil")
	}

	name := agent.Name()
	if name == "" {
		return fmt.Errorf("agent name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[name]; exists {
		return fmt.Errorf("agent %q already registered", name)
	}

	r.agents[name] = agent
	return nil
}

// Get retrieves an agent by name.
// Returns the agent and true if found, nil and false otherwise.
func (r *AgentRegistry) Get(name string) (ExtendedAgent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[name]
	return agent, exists
}

// All returns a slice of all registered agents.
// The order is not guaranteed.
func (r *AgentRegistry) All() []ExtendedAgent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ExtendedAgent, 0, len(r.agents))
	for _, agent := range r.agents {
		result = append(result, agent)
	}
	return result
}

// Describe generates a markdown description of all registered agents.
// This is intended for inclusion in parent agent system prompts to enable
// agent discovery and delegation.
//
// Example output:
//
//	# Available Agents
//
//	## sql-analyst
//	Analyzes SQL databases and generates queries
//
//	## chart-generator
//	Creates visualizations from data
func (r *AgentRegistry) Describe() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.agents) == 0 {
		return "# Available Agents\n\nNo agents registered."
	}

	var sb strings.Builder
	sb.WriteString("# Available Agents\n\n")

	for _, agent := range r.agents {
		sb.WriteString("## ")
		sb.WriteString(agent.Name())
		sb.WriteString("\n")
		sb.WriteString(agent.Description())
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// ToolRegistry manages tool registration and discovery.
// It provides thread-safe access to registered tools and supports batch registration.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewToolRegistry creates a new empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
// Returns an error if the tool name is empty or already registered.
func (r *ToolRegistry) Register(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}

	name := tool.Name()
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q already registered", name)
	}

	r.tools[name] = tool
	return nil
}

// RegisterMany adds multiple tools to the registry in a single operation.
// Returns an error if any tool fails validation or is already registered.
// If an error occurs, no tools are registered (atomic operation).
func (r *ToolRegistry) RegisterMany(tools ...Tool) error {
	if len(tools) == 0 {
		return nil
	}

	// Validate all tools first (before acquiring lock)
	names := make(map[string]bool, len(tools))
	for _, tool := range tools {
		if tool == nil {
			return fmt.Errorf("tool cannot be nil")
		}

		name := tool.Name()
		if name == "" {
			return fmt.Errorf("tool name cannot be empty")
		}

		if names[name] {
			return fmt.Errorf("duplicate tool name in batch: %q", name)
		}
		names[name] = true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for conflicts with existing tools
	for name := range names {
		if _, exists := r.tools[name]; exists {
			return fmt.Errorf("tool %q already registered", name)
		}
	}

	// Register all tools
	for _, tool := range tools {
		r.tools[tool.Name()] = tool
	}

	return nil
}

// Get retrieves a tool by name.
// Returns the tool and true if found, nil and false otherwise.
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	return tool, exists
}

// All returns a slice of all registered tools.
// The order is not guaranteed.
func (r *ToolRegistry) All() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

// ModelRegistry manages model registration and discovery.
// It provides thread-safe access to registered models, default model management,
// and capability-based model selection.
type ModelRegistry struct {
	mu           sync.RWMutex
	models       map[string]Model
	defaultModel string
}

// NewModelRegistry creates a new empty model registry.
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		models: make(map[string]Model),
	}
}

// Register adds a model to the registry with the given name.
// The name is used for lookup and should be unique across all models.
// Returns an error if the name is empty, model is nil, or name is already registered.
func (r *ModelRegistry) Register(name string, model Model) error {
	if name == "" {
		return fmt.Errorf("model name cannot be empty")
	}

	if model == nil {
		return fmt.Errorf("model cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.models[name]; exists {
		return fmt.Errorf("model %q already registered", name)
	}

	r.models[name] = model
	return nil
}

// Get retrieves a model by name.
// Returns the model and true if found, nil and false otherwise.
func (r *ModelRegistry) Get(name string) (Model, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, exists := r.models[name]
	return model, exists
}

// Default returns the default model.
// Returns an error if no default model is set or the default model no longer exists.
func (r *ModelRegistry) Default() (Model, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.defaultModel == "" {
		return nil, fmt.Errorf("no default model set")
	}

	model, exists := r.models[r.defaultModel]
	if !exists {
		return nil, fmt.Errorf("default model %q not found", r.defaultModel)
	}

	return model, nil
}

// SetDefault sets the default model by name.
// Returns an error if the model is not registered.
func (r *ModelRegistry) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.models[name]; !exists {
		return fmt.Errorf("model %q not registered", name)
	}

	r.defaultModel = name
	return nil
}

// HasCapability checks if a model supports a specific capability.
// Returns false if the model is not found.
func (r *ModelRegistry) HasCapability(name string, cap Capability) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, exists := r.models[name]
	if !exists {
		return false
	}

	return model.HasCapability(cap)
}
