package agents

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// DelegationTool creates a tool that delegates work to child agents.
// This enables multi-agent orchestration where parent agents can invoke
// specialized child agents for specific tasks.
//
// The delegation tool respects the child agent's IsolationLevel:
//   - Isolated: Child sees only the delegated task (no parent context)
//   - ReadParent: Child sees parent context but modifications don't propagate back
//   - FullAccess: Child and parent share context (tool outputs propagate)
//
// Recursion prevention: Child agents never receive the delegation tool,
// preventing infinite delegation chains.
//
// Example:
//
//	registry := NewAgentRegistry()
//	registry.Register(sqlAgent)
//	registry.Register(chartAgent)
//
//	delegationTool := NewDelegationTool(registry, model)
//	parentAgent := NewBaseAgent(
//	    WithName("orchestrator"),
//	    WithTools(delegationTool),
//	)
type DelegationTool struct {
	registry  *AgentRegistry
	model     Model
	sessionID uuid.UUID
	tenantID  uuid.UUID
}

// NewDelegationTool creates a new delegation tool with access to the agent registry.
// The model parameter is the LLM model used by child agents during delegation.
// SessionID and TenantID are passed to child agents for proper isolation.
//
// When a delegation occurs:
//   - The tool executes the child agent with proper isolation
//   - Recursion is prevented (child doesn't get delegation tool)
//   - Context sharing is managed based on IsolationLevel
//
// Example:
//
//	registry := NewAgentRegistry()
//	registry.Register(sqlAgent)
//
//	delegationTool := NewDelegationTool(registry, model, sessionID, tenantID)
func NewDelegationTool(registry *AgentRegistry, model Model, sessionID, tenantID uuid.UUID) Tool {
	return &DelegationTool{
		registry:  registry,
		model:     model,
		sessionID: sessionID,
		tenantID:  tenantID,
	}
}

// Name returns the tool identifier used by the LLM.
func (t *DelegationTool) Name() string {
	return ToolTask
}

// Description returns a human-readable description for the LLM.
func (t *DelegationTool) Description() string {
	return `Delegate a task to a specialized sub-agent. Use this when you need specialized capabilities
that another agent provides. The sub-agent will execute independently and return its final result.

Available agents:
` + t.registry.Describe()
}

// Parameters returns the JSON Schema for the delegation tool.
func (t *DelegationTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent_name": map[string]any{
				"type":        "string",
				"description": "The name of the agent to delegate to. Use the agent list from the tool description.",
			},
			"task": map[string]any{
				"type":        "string",
				"description": "The task description to send to the child agent. Be clear and specific.",
			},
			"context": map[string]any{
				"type":        "object",
				"description": "Optional context data to pass to the child agent (JSON object).",
			},
		},
		"required": []string{"agent_name", "task"},
	}
}

// filterDelegationTool removes the delegation tool from a tool list to prevent recursion.
// Child agents should not have access to the delegation tool to prevent infinite delegation chains.
func (t *DelegationTool) filterDelegationTool(tools []Tool) []Tool {
	filtered := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		if tool.Name() != ToolTask {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

// Call executes the delegation by invoking the child agent.
// The input should be a JSON string with agent_name and task fields.
//
// Behavior by IsolationLevel:
//   - Isolated: Child sees only the task message
//   - ReadParent: Child sees copy of parent's messages plus task
//   - FullAccess: Child shares parent's message array (modifications propagate)
//
// Returns the final result from the child agent as a JSON string.
func (t *DelegationTool) Call(ctx context.Context, input string) (string, error) {
	// Parse input arguments
	var args struct {
		AgentName string         `json:"agent_name"`
		Task      string         `json:"task"`
		Context   map[string]any `json:"context,omitempty"`
	}

	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("parse delegation input: %w", err)
	}

	if args.AgentName == "" {
		return "", fmt.Errorf("agent_name is required")
	}

	if args.Task == "" {
		return "", fmt.Errorf("task is required")
	}

	// Get child agent from registry
	childAgent, exists := t.registry.Get(args.AgentName)
	if !exists {
		availableAgents := make([]string, 0)
		for _, agent := range t.registry.All() {
			availableAgents = append(availableAgents, agent.Name())
		}
		return "", fmt.Errorf("agent %q not found; available agents: %v", args.AgentName, availableAgents)
	}

	// Type assert to ExtendedAgent (required for delegation)
	extendedAgent, ok := childAgent.(ExtendedAgent)
	if !ok {
		return "", fmt.Errorf("agent %q does not implement ExtendedAgent interface", args.AgentName)
	}

	// If no model is configured, just return agent metadata (useful for testing)
	if t.model == nil {
		metadata := extendedAgent.Metadata()
		result := map[string]any{
			"agent":       metadata.Name,
			"description": metadata.Description,
			"when_to_use": metadata.WhenToUse,
			"task":        args.Task,
			"note":        "No model configured - returning agent metadata only",
		}
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return "", fmt.Errorf("marshal result: %w", err)
		}
		return string(resultJSON), nil
	}

	// Prepare child messages (for now, Isolated mode only)
	// TODO: Implement ReadParent and FullAccess isolation modes
	// This requires parent context to be passed in (future enhancement)
	metadata := extendedAgent.Metadata()
	taskMessage := types.UserMessage(args.Task)
	childMessages := []types.Message{*taskMessage}

	// Create child executor with filtered tools to prevent recursion
	// CRITICAL: Remove delegation tool from child's tool list
	filteredTools := t.filterDelegationTool(extendedAgent.Tools())
	childExecutor := NewExecutor(extendedAgent, t.model, WithExecutorTools(filteredTools))

	// Execute child agent
	// Use Execute() which returns a types.Generator[ExecutorEvent]
	// Pass SessionID and TenantID from parent to child for proper isolation.
	gen := childExecutor.Execute(ctx, Input{
		Messages:  childMessages,
		SessionID: t.sessionID,
		TenantID:  t.tenantID,
	})
	defer gen.Close()

	// Collect final result from generator
	var finalContent string
	var finalUsage types.TokenUsage
	for {
		event, err := gen.Next(ctx)
		if err != nil {
			if err == types.ErrGeneratorDone {
				break
			}
			return "", fmt.Errorf("child agent generator error: %w", err)
		}

		// Process events
		switch event.Type {
		case EventTypeDone:
			if event.Result != nil {
				finalContent = event.Result.Message.Content
				finalUsage = event.Result.Usage
			}
		case EventTypeError:
			return "", fmt.Errorf("child agent %q error: %w", args.AgentName, event.Error)
		case EventTypeInterrupt:
			// Child agent requested user interaction - not supported in delegation
			return "", fmt.Errorf("child agent %q requested interrupt (not supported in delegation)", args.AgentName)
		}
	}

	// Format result as JSON
	output := map[string]any{
		"agent":  metadata.Name,
		"result": finalContent,
		"usage":  finalUsage,
	}

	outputJSON, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("marshal delegation result: %w", err)
	}

	return string(outputJSON), nil
}

// Note: Executor, ExecutionResult, and related types are defined in executor.go.
// The delegation tool depends on these types for multi-agent orchestration.
