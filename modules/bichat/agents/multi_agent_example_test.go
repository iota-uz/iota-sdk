package agents

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This example demonstrates the complete multi-agent orchestration workflow.
// It shows how to:
// 1. Create an agent registry
// 2. Register specialized sub-agents (SQLAgent)
// 3. Create a parent agent
// 4. Create a delegation tool with runtime session/tenant IDs
// 5. Execute with delegation support

// mockServicesExecutor implements bichatservices.QueryExecutorService for testing.
type mockServicesExecutor struct {
	schemaListFn     func(ctx context.Context) ([]bichatservices.TableInfo, error)
	schemaDescribeFn func(ctx context.Context, tableName string) (*bichatservices.TableSchema, error)
	executeQueryFn   func(ctx context.Context, sql string, params []any, timeoutMs int) (*bichatservices.QueryResult, error)
	validateQueryFn  func(sql string) error
}

func (m *mockServicesExecutor) SchemaList(ctx context.Context) ([]bichatservices.TableInfo, error) {
	if m.schemaListFn != nil {
		return m.schemaListFn(ctx)
	}
	return []bichatservices.TableInfo{
		{Name: "users", Schema: "public", RowCount: 100, Description: "User accounts"},
		{Name: "orders", Schema: "public", RowCount: 1000, Description: "Customer orders"},
	}, nil
}

func (m *mockServicesExecutor) SchemaDescribe(ctx context.Context, tableName string) (*bichatservices.TableSchema, error) {
	if m.schemaDescribeFn != nil {
		return m.schemaDescribeFn(ctx, tableName)
	}
	return &bichatservices.TableSchema{
		Name:   tableName,
		Schema: "public",
		Columns: []bichatservices.ColumnInfo{
			{Name: "id", Type: "integer", IsPrimaryKey: true},
			{Name: "name", Type: "varchar(255)"},
		},
	}, nil
}

func (m *mockServicesExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeoutMs int) (*bichatservices.QueryResult, error) {
	if m.executeQueryFn != nil {
		return m.executeQueryFn(ctx, sql, params, timeoutMs)
	}
	return &bichatservices.QueryResult{
		Columns:  []string{"id", "name", "total"},
		Rows:     [][]any{{1, "John", 1000}, {2, "Jane", 2000}},
		RowCount: 2,
	}, nil
}

func (m *mockServicesExecutor) ValidateQuery(sql string) error {
	if m.validateQueryFn != nil {
		return m.validateQueryFn(sql)
	}
	return nil
}

// Example_MultiAgentOrchestration demonstrates the complete setup and usage of multi-agent orchestration.
func Example_multiAgentOrchestration() {
	// 1. Create query executor (in real app, this would be PostgreSQL connection)
	executor := &mockServicesExecutor{}

	// 2. Create agent registry
	registry := agents.NewAgentRegistry()

	// 3. Create and register SQLAgent
	sqlAgent, _ := NewSQLAgent(executor)
	_ = registry.Register(sqlAgent)

	// 4. Verify agent is registered
	agent, exists := registry.Get("sql-analyst")
	if exists {
		fmt.Printf("Registered agent: %s\n", agent.Name())
		fmt.Printf("Description: %s\n", agent.Description())
	}

	// 5. Create delegation tool with runtime session/tenant IDs
	sessionID := uuid.New()
	tenantID := uuid.New()

	delegationTool := agents.NewDelegationTool(registry, nil, sessionID, tenantID)

	fmt.Printf("Delegation tool: %s\n", delegationTool.Name())
	fmt.Printf("Available agents in registry: %d\n", len(registry.All()))

	// Output:
	// Registered agent: sql-analyst
	// Description: Specialized agent for SQL query generation and database analysis
	// Delegation tool: task
	// Available agents in registry: 1
}

func TestMultiAgentSetup(t *testing.T) {
	t.Parallel()

	// Create query executor
	executor := &mockServicesExecutor{}

	// Create agent registry
	registry := agents.NewAgentRegistry()

	// Create SQLAgent
	sqlAgent, err := NewSQLAgent(executor)
	require.NoError(t, err)
	require.NotNil(t, sqlAgent)

	// Register SQLAgent
	err = registry.Register(sqlAgent)
	require.NoError(t, err)

	// Verify registration
	retrievedAgent, exists := registry.Get("sql-analyst")
	assert.True(t, exists, "sql-analyst should be registered")
	assert.Equal(t, "sql-analyst", retrievedAgent.Name())

	// Verify registry description
	description := registry.Describe()
	assert.Contains(t, description, "sql-analyst")
	assert.Contains(t, description, "SQL query generation")

	// Create delegation tool
	sessionID := uuid.New()
	tenantID := uuid.New()

	delegationTool := agents.NewDelegationTool(registry, nil, sessionID, tenantID)
	assert.Equal(t, "task", delegationTool.Name())
	assert.Contains(t, delegationTool.Description(), "sql-analyst")
}

func TestDelegationToolCreation(t *testing.T) {
	t.Parallel()

	executor := &mockServicesExecutor{}
	registry := agents.NewAgentRegistry()

	// Register SQLAgent
	sqlAgent, err := NewSQLAgent(executor)
	require.NoError(t, err)
	err = registry.Register(sqlAgent)
	require.NoError(t, err)

	// Create delegation tool with session/tenant IDs
	sessionID := uuid.New()
	tenantID := uuid.New()

	delegationTool := agents.NewDelegationTool(registry, nil, sessionID, tenantID)

	// Verify tool properties
	assert.Equal(t, "task", delegationTool.Name())
	assert.NotEmpty(t, delegationTool.Description())
	assert.NotNil(t, delegationTool.Parameters())

	// Verify parameters include required fields
	params := delegationTool.Parameters()
	assert.Equal(t, "object", params["type"])

	properties := params["properties"].(map[string]any)
	assert.Contains(t, properties, "subagent_type")
	assert.Contains(t, properties, "prompt")
	assert.Contains(t, properties, "description")

	required := params["required"].([]string)
	assert.Contains(t, required, "subagent_type")
	assert.Contains(t, required, "prompt")
	assert.Contains(t, required, "description")
}

func TestAgentRegistryDescribe(t *testing.T) {
	t.Parallel()

	executor := &mockServicesExecutor{}
	registry := agents.NewAgentRegistry()

	// Register SQLAgent
	sqlAgent, err := NewSQLAgent(executor)
	require.NoError(t, err)
	err = registry.Register(sqlAgent)
	require.NoError(t, err)

	// Get registry description
	description := registry.Describe()

	// Verify description format
	assert.Contains(t, description, "# Available Agents")
	assert.Contains(t, description, "## sql-analyst")
	assert.Contains(t, description, "Specialized agent for SQL query generation")
	assert.Contains(t, description, "database analysis")
}
