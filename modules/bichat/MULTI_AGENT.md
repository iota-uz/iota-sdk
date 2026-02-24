# Multi-Agent Orchestration Implementation

This document describes the multi-agent orchestration system implemented for the BiChat module.

## Overview

The multi-agent system enables the parent BI agent to delegate complex tasks to specialized sub-agents. This provides:

- **Task Specialization**: Sub-agents focus on specific domains (e.g., SQL analysis)
- **Isolation**: Sub-agents execute independently without access to parent context
- **Recursion Prevention**: Delegation tools are filtered from child agent tool lists
- **Runtime Configuration**: Delegation tools created at execution time with session/tenant IDs

## Architecture

### Components

1. **AgentRegistry** (`pkg/bichat/agents/registry.go`)
   - Thread-safe agent registration and discovery
   - Provides `Describe()` method for LLM consumption
   - Already existed in the framework

2. **DelegationTool** (`pkg/bichat/agents/delegation.go`)
   - Creates delegation tool with access to agent registry
   - Requires runtime session/tenant IDs
   - Automatically filters itself from child agent tool lists
   - Already existed in the framework

3. **SQLAgent** (`modules/bichat/agents/sql_agent.go`) **NEW**
   - Specialized agent for SQL query generation and database analysis
   - Tools: `schema_list`, `schema_describe`, `sql_execute`
   - Isolated from parent (no HITL, no charting, no KB search)
   - System prompt optimized for SQL workflows

4. **ExcelAgent** (`modules/bichat/agents/excel_agent.go`) **NEW**
   - Specialized spreadsheet attachment analyst for large files
   - Tools: `artifact_reader`, `ask_user_question`
   - Isolated from parent context and SQL tools
   - Prompts for paginated reads and data-quality checks

5. **Module Configuration** (`modules/bichat/config.go`)
   - `Capabilities.MultiAgent` controls multi-agent orchestration
   - `AgentRegistry` field stores registered sub-agents
   - `setupMultiAgentSystem()` automatically creates and registers SQLAgent
   - `setupExcelSubAgent()` registers ExcelAgent in `BuildServices` when storage is available

## Implementation

### 1. SQLAgent Creation

**File**: `modules/bichat/agents/sql_agent.go`

```go
agent, err := NewSQLAgent(queryExecutor)
```

Features:
- Accepts `bichatservices.QueryExecutorService` (module-level interface)
- Uses adapter to work with `tools.QueryExecutorService`
- System prompt from embedded file `sql_agent.prompt`
- Only SQL-related tools (no HITL, no charting)

### 2. Module Configuration

**File**: `modules/bichat/config.go`

```go
cfg := bichat.NewModuleConfig(
    ...,
    bichat.WithCapabilities(bichat.Capabilities{MultiAgent: true}), // Enable multi-agent
)
```

When `Capabilities.MultiAgent` is true:
1. Creates `AgentRegistry`
2. Creates `SQLAgent` if `QueryExecutor` is available
3. Registers SQLAgent in registry
4. Registers ExcelAgent in `BuildServices` when attachment storage is available
5. Registry available at `cfg.AgentRegistry`

### 3. Runtime Delegation

**At execution time** (in service layer):

```go
// Create delegation tool with runtime IDs
delegationTool := agents.NewDelegationTool(
    cfg.AgentRegistry,
    cfg.Model,
    sessionID,
    tenantID,
)

// Add to parent agent tools
allTools := append(parentAgent.Tools(), delegationTool)

// Create executor with extended tools
executor := agents.NewExecutor(
    parentAgent,
    model,
    agents.WithExecutorTools(allTools),
)
```

## Files Created

1. **`modules/bichat/agents/sql_agent.go`**
   - SQLAgent implementation
   - Uses functional options pattern
   - Supports custom model configuration

2. **`modules/bichat/agents/excel_agent.go`**
   - ExcelAgent implementation
   - Uses functional options pattern
   - Uses artifact_reader for attachment-driven workflows

3. **`modules/bichat/agents/sql_agent.prompt`**
   - System prompt for SQL analyst agent
   - Focused on SQL workflow (EXPLORE → WRITE → EXECUTE → RETURN)
   - Emphasizes safety and best practices

4. **`modules/bichat/agents/excel_agent.prompt`**
   - System prompt for spreadsheet analysis workflows
   - Focused on large attachment processing

5. **`modules/bichat/agents/sql_agent_test.go`**
   - Unit tests for SQLAgent
   - Mock implementation of `bichatservices.QueryExecutorService`
   - Tests agent creation, tools, system prompt, and tool routing

6. **`modules/bichat/agents/excel_agent_test.go`**
   - Unit tests for ExcelAgent
   - Verifies required dependencies, default tools, and options

7. **`modules/bichat/agents/sql_agent_test.go`**
   - Adapter between service and tool interfaces
   - Converts `bichatservices.QueryResult` (Rows [][]any) to `tools.QueryResult` (Rows []map[string]interface{})

8. **`modules/bichat/agents/multi_agent_example_test.go`**
   - Complete multi-agent workflow examples
   - Demonstrates registry creation, agent registration, delegation tool creation
   - Example function showing end-to-end setup

## Files Modified

1. **`modules/bichat/config.go`**
   - Added `AgentRegistry *agents.AgentRegistry` field
   - Added `setupMultiAgentSystem()` method
   - Automatic SQLAgent creation when `EnableMultiAgent` is true

2. **`modules/bichat/CLAUDE.md`**
   - Added "Multi-Agent Orchestration" section
   - Documented EnableMultiAgent flag
   - Explained delegation workflow with examples
   - Updated Common Gotchas with delegation tool note

## Usage Example

### Configuration

```go
// Create module config with multi-agent enabled
cfg := bichat.NewModuleConfig(
    composables.UseTenantID,
    composables.UseUserID,
    chatRepo,
    model,
    bichat.DefaultContextPolicy(),
    parentAgent,
    bichat.WithQueryExecutor(queryExecutor),  // Required for SQLAgent
    bichat.WithMultiAgent(true),              // Enable multi-agent
)
```

### Execution

```go
// In service layer
func (s *AgentService) ProcessMessage(ctx context.Context, sessionID uuid.UUID, content string) {
    // Create delegation tool at runtime
    delegationTool := agents.NewDelegationTool(
        s.config.AgentRegistry,
        s.config.Model,
        sessionID,
        composables.UseTenantID(ctx),
    )

    // Add to tools
    allTools := append(s.config.ParentAgent.Tools(), delegationTool)

    // Create executor with delegation support
    executor := agents.NewExecutor(
        s.config.ParentAgent,
        s.config.Model,
        agents.WithExecutorTools(allTools),
    )

    // Execute
    gen := executor.Execute(ctx, agents.Input{...})
    defer gen.Close()

    // Process events...
}
```

### Delegation Workflow

User: "Find top 10 customers by total sales"

1. **Parent BI agent** receives request
2. Parent delegates to `sql-analyst` using `task` tool:
   ```json
   {
     "subagent_type": "sql-analyst",
     "prompt": "Find top 10 customers by total sales",
     "description": "Analyze top customers"
   }
   ```
3. **SQLAgent** executes:
   - `schema_list` → Find sales/customers tables
   - `schema_describe` → Understand schema structure
   - `sql_execute` → Run query with JOIN and ORDER BY
   - Returns structured results in response (implicit stop)
4. **Parent agent** receives SQLAgent result
5. Parent uses `draw_chart` to visualize
6. Parent returns chart + insights in response (implicit stop)

## Testing

```bash
# Run SQL agent tests
go test -v ./modules/bichat/agents -run "^TestSQLAgent" -count=1

# Run multi-agent orchestration tests
go test -v ./modules/bichat/agents -run "^TestMultiAgent" -count=1

# Run all agent tests
go test -v ./modules/bichat/agents -count=1
```

## Key Design Decisions

1. **Runtime Delegation Tool Creation**: Delegation tool needs session/tenant IDs, so it's created at execution time, not configuration time

2. **QueryExecutorAdapter**: Two different QueryExecutorService interfaces exist:
   - `bichatservices.QueryExecutorService` (richer, module-level)
   - `tools.QueryExecutorService` (minimal, tool-level)

   Adapter bridges them to reuse existing tool implementations

3. **Automatic SQLAgent Registration**: When `EnableMultiAgent` is true, SQLAgent is automatically created and registered. No manual setup required

4. **Tool Filtering**: `DelegationTool.filterDelegationTool()` prevents recursion by removing delegation tool from child agent tool lists

5. **Isolation**: SQLAgent has no access to parent context, HITL tools, or charting. It focuses solely on SQL analysis

## Future Extensions

Potential sub-agents to add:

- **ChartAgent**: Specialized in data visualization and chart configuration
- **ReportAgent**: Generates formatted reports with multiple sections
- **ValidationAgent**: Validates data quality and business rules
- **ExportAgent**: Handles complex export formats and transformations

Each sub-agent follows the same pattern:
1. Create agent implementation in `modules/bichat/agents/`
2. Add registration logic in `config.setupMultiAgentSystem()`
3. Update system prompts to inform parent about new capabilities

## References

- BiChat Foundation: `pkg/bichat/agents/`
- Agent Framework: `pkg/bichat/agents/agent.go`
- Tool Interface: `pkg/bichat/agents/tool.go`
- Delegation Tool: `pkg/bichat/agents/delegation.go`
- Agent Registry: `pkg/bichat/agents/registry.go`
