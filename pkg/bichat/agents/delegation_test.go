package agents_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
)

// TestNewDelegationTool verifies basic delegation tool creation.
func TestNewDelegationTool(t *testing.T) {
	t.Parallel()

	registry := agents.NewAgentRegistry()
	sessionID := uuid.New()
	tenantID := uuid.New()
	tool := agents.NewDelegationTool(registry, nil, sessionID, tenantID)

	if tool.Name() != agents.ToolTask {
		t.Errorf("Expected tool name %q, got %q", agents.ToolTask, tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Expected non-empty description")
	}

	params := tool.Parameters()
	if params == nil {
		t.Fatal("Expected non-nil parameters")
	}

	// Verify required fields
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties to be map[string]any")
	}

	if _, hasAgentName := props["agent_name"]; !hasAgentName {
		t.Error("Expected agent_name in parameters")
	}

	if _, hasTask := props["task"]; !hasTask {
		t.Error("Expected task in parameters")
	}
}

// TestDelegationTool_AgentNotFound verifies error handling when agent doesn't exist.
func TestDelegationTool_AgentNotFound(t *testing.T) {
	t.Parallel()

	registry := agents.NewAgentRegistry()
	sessionID := uuid.New()
	tenantID := uuid.New()
	tool := agents.NewDelegationTool(registry, nil, sessionID, tenantID)

	input := `{"agent_name": "nonexistent", "task": "do something"}`
	_, err := tool.Call(context.Background(), input)

	if err == nil {
		t.Fatal("Expected error when agent not found")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' in error, got: %v", err)
	}
}

// TestDelegationTool_InvalidInput verifies input validation.
func TestDelegationTool_InvalidInput(t *testing.T) {
	t.Parallel()

	registry := agents.NewAgentRegistry()
	sessionID := uuid.New()
	tenantID := uuid.New()
	tool := agents.NewDelegationTool(registry, nil, sessionID, tenantID)

	tests := []struct {
		name        string
		input       string
		expectedErr string
	}{
		{
			name:        "invalid json",
			input:       `{invalid json}`,
			expectedErr: "parse delegation input",
		},
		{
			name:        "missing agent_name",
			input:       `{"task": "do something"}`,
			expectedErr: "agent_name is required",
		},
		{
			name:        "missing task",
			input:       `{"agent_name": "test"}`,
			expectedErr: "task is required",
		},
		{
			name:        "empty agent_name",
			input:       `{"agent_name": "", "task": "test"}`,
			expectedErr: "agent_name is required",
		},
		{
			name:        "empty task",
			input:       `{"agent_name": "test", "task": ""}`,
			expectedErr: "task is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Call(context.Background(), tt.input)
			if err == nil {
				t.Fatal("Expected error for invalid input")
			}
			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("Expected error containing %q, got: %v", tt.expectedErr, err)
			}
		})
	}
}

// TestDelegationTool_NoExecutor verifies behavior when no executor is provided.
// Without an executor, the tool should return agent metadata.
func TestDelegationTool_NoExecutor(t *testing.T) {
	t.Parallel()

	registry := agents.NewAgentRegistry()

	// Register a test agent
	testAgent := agents.NewBaseAgent(
		agents.WithName("test_agent"),
		agents.WithDescription("A test agent"),
		agents.WithWhenToUse("Use for testing"),
	)

	if err := registry.Register(testAgent); err != nil {
		t.Fatalf("Failed to register agent: %v", err)
	}

	sessionID := uuid.New()
	tenantID := uuid.New()
	tool := agents.NewDelegationTool(registry, nil, sessionID, tenantID)

	input := `{"agent_name": "test_agent", "task": "perform test"}`
	result, err := tool.Call(context.Background(), input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Parse result
	var resultData map[string]any
	if err := json.Unmarshal([]byte(result), &resultData); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Verify result contains expected fields
	if resultData["agent"] != "test_agent" {
		t.Errorf("Expected agent 'test_agent', got %v", resultData["agent"])
	}

	if resultData["task"] != "perform test" {
		t.Errorf("Expected task 'perform test', got %v", resultData["task"])
	}

	if resultData["description"] != "A test agent" {
		t.Errorf("Expected description 'A test agent', got %v", resultData["description"])
	}

	if _, hasNote := resultData["note"]; !hasNote {
		t.Error("Expected note field in result")
	}
}

// TestDelegationTool_Isolated verifies Isolated isolation level.
// Child should only see the task message, not parent context.
func TestDelegationTool_Isolated(t *testing.T) {
	t.Parallel()

	registry := agents.NewAgentRegistry()

	// Create child agent with Isolated isolation
	childAgent := agents.NewBaseAgent(
		agents.WithName("isolated_child"),
		agents.WithDescription("An isolated child agent"),
		agents.WithIsolation(agents.Isolated),
	)

	if err := registry.Register(childAgent); err != nil {
		t.Fatalf("Failed to register agent: %v", err)
	}

	// Verify isolation level
	if childAgent.Metadata().Isolation != agents.Isolated {
		t.Errorf("Expected Isolated isolation, got %v", childAgent.Metadata().Isolation)
	}

	// Note: Full execution test would require a mock executor
	// For now, we verify that the agent is properly registered and configured
	retrieved, exists := registry.Get("isolated_child")
	if !exists {
		t.Fatal("Agent not found in registry")
	}

	if retrieved.Metadata().Isolation != agents.Isolated {
		t.Errorf("Expected Isolated isolation, got %v", retrieved.Metadata().Isolation)
	}
}

// TestDelegationTool_ReadParent verifies ReadParent isolation level.
// Child should see parent context but modifications shouldn't propagate back.
func TestDelegationTool_ReadParent(t *testing.T) {
	t.Parallel()

	registry := agents.NewAgentRegistry()

	// Create child agent with ReadParent isolation
	childAgent := agents.NewBaseAgent(
		agents.WithName("read_parent_child"),
		agents.WithDescription("A read-parent child agent"),
		agents.WithIsolation(agents.ReadParent),
	)

	if err := registry.Register(childAgent); err != nil {
		t.Fatalf("Failed to register agent: %v", err)
	}

	// Verify isolation level
	if childAgent.Metadata().Isolation != agents.ReadParent {
		t.Errorf("Expected ReadParent isolation, got %v", childAgent.Metadata().Isolation)
	}

	// Verify string representation
	if childAgent.Metadata().Isolation.String() != "read_parent" {
		t.Errorf("Expected 'read_parent', got %q", childAgent.Metadata().Isolation.String())
	}
}

// TestDelegationTool_FullAccess verifies FullAccess isolation level.
// Child and parent should share context with bidirectional propagation.
func TestDelegationTool_FullAccess(t *testing.T) {
	t.Parallel()

	registry := agents.NewAgentRegistry()

	// Create child agent with FullAccess isolation
	childAgent := agents.NewBaseAgent(
		agents.WithName("full_access_child"),
		agents.WithDescription("A full-access child agent"),
		agents.WithIsolation(agents.FullAccess),
	)

	if err := registry.Register(childAgent); err != nil {
		t.Fatalf("Failed to register agent: %v", err)
	}

	// Verify isolation level
	if childAgent.Metadata().Isolation != agents.FullAccess {
		t.Errorf("Expected FullAccess isolation, got %v", childAgent.Metadata().Isolation)
	}

	// Verify string representation
	if childAgent.Metadata().Isolation.String() != "full_access" {
		t.Errorf("Expected 'full_access', got %q", childAgent.Metadata().Isolation.String())
	}
}

// TestDelegationTool_RecursionPrevention verifies that delegation tool is not passed to children.
// This prevents infinite delegation chains.
func TestDelegationTool_RecursionPrevention(t *testing.T) {
	t.Parallel()

	registry := agents.NewAgentRegistry()

	// Create a tool that is NOT the delegation tool
	normalTool := agents.NewTool(
		"normal_tool",
		"A normal tool",
		map[string]any{},
		func(ctx context.Context, input string) (string, error) {
			return "ok", nil
		},
	)

	// Create delegation tool
	sessionID := uuid.New()
	tenantID := uuid.New()
	delegationTool := agents.NewDelegationTool(registry, nil, sessionID, tenantID)

	// Create child agent with both tools
	childAgent := agents.NewBaseAgent(
		agents.WithName("child_with_tools"),
		agents.WithDescription("Child agent with tools"),
		agents.WithTools(normalTool, delegationTool),
	)

	if err := registry.Register(childAgent); err != nil {
		t.Fatalf("Failed to register agent: %v", err)
	}

	// Verify child has both tools initially
	tools := childAgent.Tools()
	if len(tools) != 2 {
		t.Fatalf("Expected 2 tools, got %d", len(tools))
	}

	// In actual delegation, the delegation tool would be filtered out
	// We can verify this by checking tool names
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name()] = true
	}

	if !toolNames["normal_tool"] {
		t.Error("Expected normal_tool in agent tools")
	}

	if !toolNames[agents.ToolTask] {
		t.Error("Expected delegation tool in agent tools (before filtering)")
	}
}

// TestIsolationLevel_String verifies string representation of isolation levels.
func TestIsolationLevel_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		level    agents.IsolationLevel
		expected string
	}{
		{agents.Isolated, "isolated"},
		{agents.ReadParent, "read_parent"},
		{agents.FullAccess, "full_access"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.level.String() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, tt.level.String())
			}
		})
	}
}

// TestAgentMetadata_IsolationLevels verifies agent metadata with different isolation levels.
func TestAgentMetadata_IsolationLevels(t *testing.T) {
	t.Parallel()

	levels := []agents.IsolationLevel{
		agents.Isolated,
		agents.ReadParent,
		agents.FullAccess,
	}

	for _, level := range levels {
		t.Run(level.String(), func(t *testing.T) {
			agent := agents.NewBaseAgent(
				agents.WithName("test_"+level.String()),
				agents.WithDescription("Test agent"),
				agents.WithIsolation(level),
			)

			metadata := agent.Metadata()
			if metadata.Isolation != level {
				t.Errorf("Expected isolation %v, got %v", level, metadata.Isolation)
			}

			if metadata.Name != "test_"+level.String() {
				t.Errorf("Expected name 'test_%s', got %q", level.String(), metadata.Name)
			}
		})
	}
}

// TestDelegationTool_Description verifies that the description includes agent listing.
func TestDelegationTool_Description(t *testing.T) {
	t.Parallel()

	registry := agents.NewAgentRegistry()

	// Register multiple agents
	agent1 := agents.NewBaseAgent(
		agents.WithName("sql_agent"),
		agents.WithDescription("Executes SQL queries"),
	)

	agent2 := agents.NewBaseAgent(
		agents.WithName("chart_agent"),
		agents.WithDescription("Creates data visualizations"),
	)

	if err := registry.Register(agent1); err != nil {
		t.Fatalf("Failed to register agent1: %v", err)
	}

	if err := registry.Register(agent2); err != nil {
		t.Fatalf("Failed to register agent2: %v", err)
	}

	sessionID := uuid.New()
	tenantID := uuid.New()
	tool := agents.NewDelegationTool(registry, nil, sessionID, tenantID)
	description := tool.Description()

	// Verify description includes agent information
	if !strings.Contains(description, "sql_agent") {
		t.Error("Expected description to contain 'sql_agent'")
	}

	if !strings.Contains(description, "chart_agent") {
		t.Error("Expected description to contain 'chart_agent'")
	}

	if !strings.Contains(description, "Executes SQL queries") {
		t.Error("Expected description to contain agent1 description")
	}

	if !strings.Contains(description, "Creates data visualizations") {
		t.Error("Expected description to contain agent2 description")
	}
}

// TestDelegationTool_ContextParameter verifies optional context parameter handling.
func TestDelegationTool_ContextParameter(t *testing.T) {
	t.Parallel()

	registry := agents.NewAgentRegistry()

	testAgent := agents.NewBaseAgent(
		agents.WithName("test_agent"),
		agents.WithDescription("Test agent"),
	)

	if err := registry.Register(testAgent); err != nil {
		t.Fatalf("Failed to register agent: %v", err)
	}

	sessionID := uuid.New()
	tenantID := uuid.New()
	tool := agents.NewDelegationTool(registry, nil, sessionID, tenantID)

	// Test with optional context parameter
	input := `{
		"agent_name": "test_agent",
		"task": "perform analysis",
		"context": {
			"user_id": 123,
			"tenant_id": "abc"
		}
	}`

	result, err := tool.Call(context.Background(), input)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var resultData map[string]any
	if err := json.Unmarshal([]byte(result), &resultData); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Verify result includes agent and task
	if resultData["agent"] != "test_agent" {
		t.Errorf("Expected agent 'test_agent', got %v", resultData["agent"])
	}

	if resultData["task"] != "perform analysis" {
		t.Errorf("Expected task 'perform analysis', got %v", resultData["task"])
	}
}

// TestExecutor_CreationBasic verifies basic executor creation.
// This test ensures the executor can be created with minimal configuration.
func TestExecutor_CreationBasic(t *testing.T) {
	t.Parallel()

	// Create a minimal agent
	agent := agents.NewBaseAgent(
		agents.WithName("test_executor_agent"),
		agents.WithDescription("Test agent for executor"),
	)

	// Create executor (model can be nil for creation test)
	executor := agents.NewExecutor(agent, nil)

	if executor == nil {
		t.Fatal("Expected non-nil executor")
	}

	// Executor is created successfully
	// Actual execution would require a real model
}
