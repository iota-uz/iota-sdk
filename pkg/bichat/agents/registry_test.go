package agents

import (
	"context"
	"sync"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// mockAgent is a test implementation of the ExtendedAgent interface.
type mockAgent struct {
	name        string
	description string
	tools       []Tool
}

func (m *mockAgent) Name() string        { return m.name }
func (m *mockAgent) Description() string { return m.description }
func (m *mockAgent) Metadata() AgentMetadata {
	return AgentMetadata{
		Name:        m.name,
		Description: m.description,
	}
}
func (m *mockAgent) Tools() []Tool                           { return m.tools }
func (m *mockAgent) SystemPrompt(ctx context.Context) string { return "" }
func (m *mockAgent) OnToolCall(ctx context.Context, toolName, input string) (string, error) {
	return "", nil
}

// TestAgentRegistry_CRUD tests basic CRUD operations for AgentRegistry.
func TestAgentRegistry_CRUD(t *testing.T) {
	t.Parallel()

	t.Run("Register and Get", func(t *testing.T) {
		t.Parallel()

		registry := NewAgentRegistry()
		agent := &mockAgent{
			name:        "test-agent",
			description: "A test agent",
		}

		// Register agent
		err := registry.Register(agent)
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}

		// Get agent
		got, exists := registry.Get("test-agent")
		if !exists {
			t.Fatal("Get() agent not found")
		}
		if got.Name() != agent.Name() {
			t.Errorf("Get() name = %q, want %q", got.Name(), agent.Name())
		}
	})

	t.Run("Get non-existent agent", func(t *testing.T) {
		t.Parallel()

		registry := NewAgentRegistry()
		_, exists := registry.Get("missing")
		if exists {
			t.Error("Get() found non-existent agent")
		}
	})

	t.Run("Register duplicate agent", func(t *testing.T) {
		t.Parallel()

		registry := NewAgentRegistry()
		agent := &mockAgent{name: "duplicate", description: "Test"}

		err := registry.Register(agent)
		if err != nil {
			t.Fatalf("First Register() error = %v", err)
		}

		err = registry.Register(agent)
		if err == nil {
			t.Fatal("Register() duplicate agent should return error")
		}
	})

	t.Run("Register nil agent", func(t *testing.T) {
		t.Parallel()

		registry := NewAgentRegistry()
		err := registry.Register(nil)
		if err == nil {
			t.Fatal("Register(nil) should return error")
		}
	})

	t.Run("Register agent with empty name", func(t *testing.T) {
		t.Parallel()

		registry := NewAgentRegistry()
		agent := &mockAgent{name: "", description: "Test"}

		err := registry.Register(agent)
		if err == nil {
			t.Fatal("Register() with empty name should return error")
		}
	})

	t.Run("All agents", func(t *testing.T) {
		t.Parallel()

		registry := NewAgentRegistry()
		agents := []*mockAgent{
			{name: "agent1", description: "First"},
			{name: "agent2", description: "Second"},
			{name: "agent3", description: "Third"},
		}

		for _, agent := range agents {
			if err := registry.Register(agent); err != nil {
				t.Fatalf("Register() error = %v", err)
			}
		}

		all := registry.All()
		if len(all) != len(agents) {
			t.Errorf("All() count = %d, want %d", len(all), len(agents))
		}

		// Verify all agents are present
		names := make(map[string]bool)
		for _, agent := range all {
			names[agent.Name()] = true
		}
		for _, agent := range agents {
			if !names[agent.Name()] {
				t.Errorf("All() missing agent %q", agent.Name())
			}
		}
	})

	t.Run("All on empty registry", func(t *testing.T) {
		t.Parallel()

		registry := NewAgentRegistry()
		all := registry.All()
		if len(all) != 0 {
			t.Errorf("All() on empty registry = %d agents, want 0", len(all))
		}
	})

	t.Run("Describe", func(t *testing.T) {
		t.Parallel()

		registry := NewAgentRegistry()
		agent1 := &mockAgent{name: "sql-agent", description: "Executes SQL queries"}
		agent2 := &mockAgent{name: "chart-agent", description: "Creates charts"}

		registry.Register(agent1)
		registry.Register(agent2)

		desc := registry.Describe()
		if desc == "" {
			t.Fatal("Describe() returned empty string")
		}

		// Check that description contains agent names and descriptions
		if !containsString(desc, "sql-agent") {
			t.Error("Describe() missing agent1 name")
		}
		if !containsString(desc, "Executes SQL queries") {
			t.Error("Describe() missing agent1 description")
		}
		if !containsString(desc, "chart-agent") {
			t.Error("Describe() missing agent2 name")
		}
		if !containsString(desc, "Creates charts") {
			t.Error("Describe() missing agent2 description")
		}
	})

	t.Run("Describe empty registry", func(t *testing.T) {
		t.Parallel()

		registry := NewAgentRegistry()
		desc := registry.Describe()
		if desc == "" {
			t.Fatal("Describe() on empty registry returned empty string")
		}
		if !containsString(desc, "No agents registered") {
			t.Error("Describe() should indicate no agents")
		}
	})
}

// TestAgentRegistry_Concurrent tests thread safety of AgentRegistry.
func TestAgentRegistry_Concurrent(t *testing.T) {
	t.Parallel()

	registry := NewAgentRegistry()
	const numGoroutines = 100

	// Test concurrent Register and Get
	t.Run("Concurrent Register and Get", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(numGoroutines * 2) // Register + Get for each goroutine

		// Concurrent Register
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				agent := &mockAgent{
					name:        stringID("agent", id),
					description: stringID("Description", id),
				}
				registry.Register(agent) // May fail for duplicates, that's OK
			}(i)
		}

		// Concurrent Get
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				registry.Get(stringID("agent", id))
			}(i)
		}

		wg.Wait()
	})

	// Test concurrent All
	t.Run("Concurrent All", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				registry.All()
			}()
		}

		wg.Wait()
	})

	// Test concurrent Describe
	t.Run("Concurrent Describe", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				registry.Describe()
			}()
		}

		wg.Wait()
	})
}

// TestToolRegistry_CRUD tests basic CRUD operations for ToolRegistry.
func TestToolRegistry_CRUD(t *testing.T) {
	t.Parallel()

	t.Run("Register and Get", func(t *testing.T) {
		t.Parallel()

		registry := NewToolRegistry()
		tool := NewTool("test-tool", "A test tool", map[string]any{}, func(ctx context.Context, input string) (string, error) {
			return input, nil
		})

		err := registry.Register(tool)
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}

		got, exists := registry.Get("test-tool")
		if !exists {
			t.Fatal("Get() tool not found")
		}
		if got.Name() != tool.Name() {
			t.Errorf("Get() name = %q, want %q", got.Name(), tool.Name())
		}
	})

	t.Run("Register duplicate tool", func(t *testing.T) {
		t.Parallel()

		registry := NewToolRegistry()
		tool := NewTool("duplicate", "Test", map[string]any{}, func(ctx context.Context, input string) (string, error) {
			return "", nil
		})

		err := registry.Register(tool)
		if err != nil {
			t.Fatalf("First Register() error = %v", err)
		}

		err = registry.Register(tool)
		if err == nil {
			t.Fatal("Register() duplicate tool should return error")
		}
	})

	t.Run("Register nil tool", func(t *testing.T) {
		t.Parallel()

		registry := NewToolRegistry()
		err := registry.Register(nil)
		if err == nil {
			t.Fatal("Register(nil) should return error")
		}
	})

	t.Run("RegisterMany success", func(t *testing.T) {
		t.Parallel()

		registry := NewToolRegistry()
		tools := []Tool{
			NewTool("tool1", "First", map[string]any{}, func(ctx context.Context, input string) (string, error) {
				return "", nil
			}),
			NewTool("tool2", "Second", map[string]any{}, func(ctx context.Context, input string) (string, error) {
				return "", nil
			}),
			NewTool("tool3", "Third", map[string]any{}, func(ctx context.Context, input string) (string, error) {
				return "", nil
			}),
		}

		err := registry.RegisterMany(tools...)
		if err != nil {
			t.Fatalf("RegisterMany() error = %v", err)
		}

		// Verify all tools registered
		for _, tool := range tools {
			if _, exists := registry.Get(tool.Name()); !exists {
				t.Errorf("RegisterMany() tool %q not found", tool.Name())
			}
		}
	})

	t.Run("RegisterMany with duplicate in batch", func(t *testing.T) {
		t.Parallel()

		registry := NewToolRegistry()
		tools := []Tool{
			NewTool("duplicate", "First", map[string]any{}, func(ctx context.Context, input string) (string, error) {
				return "", nil
			}),
			NewTool("duplicate", "Second", map[string]any{}, func(ctx context.Context, input string) (string, error) {
				return "", nil
			}),
		}

		err := registry.RegisterMany(tools...)
		if err == nil {
			t.Fatal("RegisterMany() with duplicates should return error")
		}

		// Verify no tools registered (atomic)
		if _, exists := registry.Get("duplicate"); exists {
			t.Error("RegisterMany() should not register any tools on error")
		}
	})

	t.Run("RegisterMany with existing conflict", func(t *testing.T) {
		t.Parallel()

		registry := NewToolRegistry()
		existing := NewTool("existing", "Existing", map[string]any{}, func(ctx context.Context, input string) (string, error) {
			return "", nil
		})
		registry.Register(existing)

		tools := []Tool{
			NewTool("new1", "New 1", map[string]any{}, func(ctx context.Context, input string) (string, error) {
				return "", nil
			}),
			NewTool("existing", "Conflict", map[string]any{}, func(ctx context.Context, input string) (string, error) {
				return "", nil
			}),
		}

		err := registry.RegisterMany(tools...)
		if err == nil {
			t.Fatal("RegisterMany() with conflict should return error")
		}

		// Verify new tools not registered (atomic)
		if _, exists := registry.Get("new1"); exists {
			t.Error("RegisterMany() should not register any tools on conflict")
		}
	})

	t.Run("RegisterMany with nil tool", func(t *testing.T) {
		t.Parallel()

		registry := NewToolRegistry()
		tools := []Tool{
			NewTool("tool1", "First", map[string]any{}, func(ctx context.Context, input string) (string, error) {
				return "", nil
			}),
			nil,
		}

		err := registry.RegisterMany(tools...)
		if err == nil {
			t.Fatal("RegisterMany() with nil tool should return error")
		}
	})

	t.Run("RegisterMany empty", func(t *testing.T) {
		t.Parallel()

		registry := NewToolRegistry()
		err := registry.RegisterMany()
		if err != nil {
			t.Fatalf("RegisterMany() with no tools error = %v", err)
		}
	})

	t.Run("All tools", func(t *testing.T) {
		t.Parallel()

		registry := NewToolRegistry()
		tools := []Tool{
			NewTool("tool1", "First", map[string]any{}, func(ctx context.Context, input string) (string, error) {
				return "", nil
			}),
			NewTool("tool2", "Second", map[string]any{}, func(ctx context.Context, input string) (string, error) {
				return "", nil
			}),
		}

		for _, tool := range tools {
			if err := registry.Register(tool); err != nil {
				t.Fatalf("Register() error = %v", err)
			}
		}

		all := registry.All()
		if len(all) != len(tools) {
			t.Errorf("All() count = %d, want %d", len(all), len(tools))
		}
	})
}

// TestModelRegistry_Default tests default model handling.
func TestModelRegistry_Default(t *testing.T) {
	t.Parallel()

	t.Run("Default model not set", func(t *testing.T) {
		t.Parallel()

		registry := NewModelRegistry()
		_, err := registry.Default()
		if err == nil {
			t.Fatal("Default() with no default set should return error")
		}
	})

	t.Run("SetDefault and get", func(t *testing.T) {
		t.Parallel()

		registry := NewModelRegistry()
		model := &mockModel{name: "test-model"}

		err := registry.Register("test", model)
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}

		err = registry.SetDefault("test")
		if err != nil {
			t.Fatalf("SetDefault() error = %v", err)
		}

		got, err := registry.Default()
		if err != nil {
			t.Fatalf("Default() error = %v", err)
		}
		if got != model {
			t.Error("Default() returned wrong model")
		}
	})

	t.Run("SetDefault non-existent model", func(t *testing.T) {
		t.Parallel()

		registry := NewModelRegistry()
		err := registry.SetDefault("missing")
		if err == nil {
			t.Fatal("SetDefault() for non-existent model should return error")
		}
	})

	t.Run("Default after model removal", func(t *testing.T) {
		t.Parallel()

		registry := NewModelRegistry()
		model := &mockModel{name: "test"}

		registry.Register("test", model)
		registry.SetDefault("test")

		// Simulate removal by creating new registry
		// (In real scenario, you'd need a Remove method)
		registry2 := NewModelRegistry()
		registry2.defaultModel = "test"

		_, err := registry2.Default()
		if err == nil {
			t.Fatal("Default() for removed model should return error")
		}
	})
}

// TestModelRegistry_Capability tests capability checking.
func TestModelRegistry_Capability(t *testing.T) {
	t.Parallel()

	t.Run("HasCapability true", func(t *testing.T) {
		t.Parallel()

		registry := NewModelRegistry()
		model := &mockModel{
			name: "test",
			capabilities: []Capability{
				CapabilityStreaming,
				CapabilityTools,
			},
		}

		registry.Register("test", model)

		if !registry.HasCapability("test", CapabilityStreaming) {
			t.Error("HasCapability() should return true for streaming")
		}
		if !registry.HasCapability("test", CapabilityTools) {
			t.Error("HasCapability() should return true for tools")
		}
	})

	t.Run("HasCapability false", func(t *testing.T) {
		t.Parallel()

		registry := NewModelRegistry()
		model := &mockModel{
			name:         "test",
			capabilities: []Capability{CapabilityStreaming},
		}

		registry.Register("test", model)

		if registry.HasCapability("test", CapabilityVision) {
			t.Error("HasCapability() should return false for vision")
		}
	})

	t.Run("HasCapability for non-existent model", func(t *testing.T) {
		t.Parallel()

		registry := NewModelRegistry()
		if registry.HasCapability("missing", CapabilityStreaming) {
			t.Error("HasCapability() for non-existent model should return false")
		}
	})
}

// TestModelRegistry_CRUD tests basic CRUD operations for ModelRegistry.
func TestModelRegistry_CRUD(t *testing.T) {
	t.Parallel()

	t.Run("Register and Get", func(t *testing.T) {
		t.Parallel()

		registry := NewModelRegistry()
		model := &mockModel{name: "test-model"}

		err := registry.Register("test", model)
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}

		got, exists := registry.Get("test")
		if !exists {
			t.Fatal("Get() model not found")
		}
		if got != model {
			t.Error("Get() returned wrong model")
		}
	})

	t.Run("Register duplicate", func(t *testing.T) {
		t.Parallel()

		registry := NewModelRegistry()
		model := &mockModel{name: "test"}

		err := registry.Register("test", model)
		if err != nil {
			t.Fatalf("First Register() error = %v", err)
		}

		err = registry.Register("test", model)
		if err == nil {
			t.Fatal("Register() duplicate should return error")
		}
	})

	t.Run("Register with empty name", func(t *testing.T) {
		t.Parallel()

		registry := NewModelRegistry()
		model := &mockModel{name: "test"}

		err := registry.Register("", model)
		if err == nil {
			t.Fatal("Register() with empty name should return error")
		}
	})

	t.Run("Register nil model", func(t *testing.T) {
		t.Parallel()

		registry := NewModelRegistry()
		err := registry.Register("test", nil)
		if err == nil {
			t.Fatal("Register(nil) should return error")
		}
	})
}

// mockModel is a test implementation of the Model interface.
type mockModel struct {
	name         string
	capabilities []Capability
}

func (m *mockModel) Generate(ctx context.Context, req Request, opts ...GenerateOption) (*Response, error) {
	return nil, nil
}

func (m *mockModel) Stream(ctx context.Context, req Request, opts ...GenerateOption) types.Generator[Chunk] {
	return nil
}

func (m *mockModel) Info() ModelInfo {
	return ModelInfo{
		Name:         m.name,
		Provider:     "mock",
		Capabilities: m.capabilities,
	}
}

func (m *mockModel) HasCapability(capability Capability) bool {
	for _, c := range m.capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// Helper functions

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func stringID(prefix string, id int) string {
	return prefix + string(rune('0'+id%10))
}
