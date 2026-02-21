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
// The delegation tool follows the "agent-as-tool" pattern: the parent agent
// delegates a task to a child agent, which executes independently and returns
// its final result. The parent agent retains control and continues execution
// with the child's result.
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
//   - The tool executes the child agent independently
//   - Recursion is prevented (child doesn't get delegation tool)
//   - Child receives the delegated task and executes in isolation
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
			"subagent_type": map[string]any{
				"type":        "string",
				"description": "The type of specialized agent (e.g., 'editor', 'debugger', 'e2e-tester', 'general-purpose', 'Plan', 'Explore', etc.). Use the agent list from the tool description.",
			},
			"prompt": map[string]any{
				"type":        "string",
				"description": "The detailed task description for the agent to perform. Be clear and specific about requirements, context, and expected outcomes.",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "A short 3-5 word summary of what the agent will do (e.g., 'Fix payment controller bug', 'Add user registration tests').",
			},
		},
		"required": []string{"subagent_type", "prompt", "description"},
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

// Call implements Tool.Call by delegating to CallStreaming with a no-op emitter.
func (t *DelegationTool) Call(ctx context.Context, input string) (string, error) {
	return t.CallStreaming(ctx, input, func(ExecutorEvent) bool { return true })
}

// CallStreaming executes the delegation by invoking the child agent.
// The input should be a JSON string with subagent_type, prompt, and description fields.
//
// The child agent receives only the delegated task as input and executes
// independently. The final result is returned as a JSON string.
// Intermediate child events (tool calls, thinking) are forwarded via emit.
//
// Returns the final result from the child agent as a JSON string.
func (t *DelegationTool) CallStreaming(ctx context.Context, input string, emit EventEmitter) (string, error) {
	// Parse input arguments
	var args struct {
		SubagentType string `json:"subagent_type"`
		Prompt       string `json:"prompt"`
		Description  string `json:"description"`
	}

	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("parse delegation input: %w", err)
	}

	if args.SubagentType == "" {
		return "", fmt.Errorf("subagent_type is required")
	}

	if args.Prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	if args.Description == "" {
		return "", fmt.Errorf("description is required")
	}

	// Get child agent from registry
	childAgent, exists := t.registry.Get(args.SubagentType)
	if !exists {
		availableAgents := make([]string, 0)
		for _, agent := range t.registry.All() {
			availableAgents = append(availableAgents, agent.Name())
		}
		return "", fmt.Errorf("agent %q not found; available agents: %v", args.SubagentType, availableAgents)
	}

	// Get returns ExtendedAgent; use directly.
	extendedAgent := childAgent

	// If no model is configured, just return agent metadata (useful for testing)
	if t.model == nil {
		metadata := extendedAgent.Metadata()
		result := map[string]any{
			"agent":       metadata.Name,
			"description": metadata.Description,
			"when_to_use": metadata.WhenToUse,
			"task":        args.Prompt,
			"summary":     args.Description,
			"note":        "No model configured - returning agent metadata only",
		}
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return "", fmt.Errorf("marshal result: %w", err)
		}
		return string(resultJSON), nil
	}

	// Prepare child messages
	metadata := extendedAgent.Metadata()
	taskMessage := types.UserMessage(args.Prompt)
	childMessages := []types.Message{taskMessage}

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

	// Collect final result from generator, forwarding child events to parent.
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
				finalContent = event.Result.Message.Content()
				finalUsage = event.Result.Usage
			}
		case EventTypeError:
			return "", fmt.Errorf("child agent %q error: %w", args.SubagentType, event.Error)
		case EventTypeInterrupt:
			// Child agent requested user interaction - not supported in delegation
			return "", fmt.Errorf("child agent %q requested interrupt (not supported in delegation)", args.SubagentType)
		case EventTypeToolStart, EventTypeToolEnd:
			// Forward child tool events to parent stream with agent identity.
			if event.Tool != nil {
				forwarded := event
				forwarded.Tool = &ToolEvent{
					CallID:     event.Tool.CallID,
					Name:       event.Tool.Name,
					AgentName:  metadata.Name,
					Arguments:  event.Tool.Arguments,
					Result:     event.Tool.Result,
					Error:      event.Tool.Error,
					DurationMs: event.Tool.DurationMs,
					Artifacts:  event.Tool.Artifacts,
				}
				emit(forwarded)
			}
		case EventTypeThinking:
			// Forward child thinking events to parent stream.
			emit(event)
		case EventTypeChunk:
			// Content events from child are not forwarded — only the final
			// result matters to the parent agent.
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
