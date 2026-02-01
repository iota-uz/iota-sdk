# BiChat Module

Production-ready BI chat module using the BiChat foundation (`pkg/bichat`).

## Module Structure

```
modules/bichat/
├── module.go                    # Module registration, DI wiring
├── applet.go                    # Applet implementation (pkg/applet integration)
├── config.go                    # Configuration types with feature flags
├── infrastructure/
│   ├── persistence/
│   │   ├── chat_repository.go   # PostgreSQL implementation
│   │   └── schema/              # SQL migrations
│   └── llmproviders/
│       └── openai_provider.go   # LLM provider
├── services/
│   └── agent_service_impl.go    # Agent orchestration with event streaming
├── presentation/
│   ├── assets/
│   │   ├── embed.go             # Embedded React build (dist/)
│   │   └── dist/                # React build output (created by npm build)
│   ├── controllers/
│   │   ├── chat_controller.go   # GraphQL endpoints
│   │   ├── stream_controller.go # SSE streaming (line ~65 critical)
│   │   └── web_controller.go    # DEPRECATED: Use applet system instead
│   ├── graphql/
│   │   └── schema.graphql       # GraphQL schema
│   └── templates/pages/bichat/  # HTMX templates (for legacy routes)
└── agents/
    └── default_agent.go         # Default BI agent
```

## Applet System Integration

BiChat uses the `pkg/applet` system for React app integration. This provides:

**Context Injection**:
- Server context passed to React via `window.__BICHAT_CONTEXT__`
- Includes: user, tenant, locale, config, session, feature flags
- Built by `pkg/applet.ContextBuilder` (no manual implementation needed)

**Asset Serving**:
- React build artifacts embedded via `presentation/assets/embed.go`
- Served at `/bichat/assets/*` by `AppletController`
- Build process outputs to `presentation/assets/dist/`

**Feature Flags**:
- Configured via `ModuleConfig` (code-based, no env vars)
- Passed to React via `InitialContext.Extensions.features`
- Flags: `vision`, `webSearch`, `codeInterpreter`, `multiAgent`

**Usage Example**:
```go
cfg := bichat.NewModuleConfig(
    composables.UseTenantID,
    composables.UseUserID,
    chatRepo,
    llmModel,
    bichat.DefaultContextPolicy(),
    parentAgent,
    bichat.WithVision(true),                  // Enable vision
    bichat.WithCodeInterpreter(true),         // Enable code interpreter
    bichat.WithWebSearch(false),              // Disable web search
    bichat.WithMultiAgent(false),             // Disable multi-agent
)

module := bichat.NewModuleWithConfig(cfg)
app.RegisterModule(module)
```

**React Integration**:
```typescript
// Access context in React
const context = window.__BICHAT_CONTEXT__;
const { user, tenant, locale, config, extensions } = context;

// Check feature flags
if (extensions.features.vision) {
  // Enable vision UI
}
```

## Critical Implementation Details

### Repository Pattern (Multi-Tenant)

```go
// CRITICAL: Always use tenant isolation
tenantID, err := composables.UseTenantID(ctx)
query := "SELECT * FROM bichat_sessions WHERE tenant_id = $1 AND id = $2"
```

### SSE Streaming (Critical)

**Location**: `stream_controller.go:65-95`

```go
w.Header().Set("Content-Type", "text/event-stream")
w.Header().Set("Cache-Control", "no-cache")
flusher := w.(http.Flusher)

for {
    event, err, hasMore := gen.Next()
    if !hasMore { break }
    fmt.Fprintf(w, "data: %s\n\n", toJSON(event))
    flusher.Flush()
}
```

**GOTCHA**: Event generators must handle context cancellation properly. Always defer `gen.Close()`.

### Module Configuration

```go
cfg := bichat.NewModuleConfig(
    composables.UseTenantID,
    composables.UseUserID,
    chatRepo,
    llmModel,
    bichat.DefaultContextPolicy(),
    parentAgent,
    bichat.WithQueryExecutor(executor),        // Optional: SQL execution
    bichat.WithKBSearcher(kbSearcher),         // Optional: KB search
    bichat.WithCheckpointer(checkpointer),     // Optional: HITL
    bichat.WithTokenEstimator(estimator),      // Optional: Cost tracking
)
```

## Database Schema

See: `infrastructure/persistence/schema/bichat-schema.sql`

**Key Tables:**
- `bichat_sessions` - Chat sessions (tenant_id, user_id, status, pinned)
- `bichat_messages` - Messages (session_id, role, content, tool_calls, citations)
- `bichat_attachments` - File attachments (message_id, file_name, storage_path)
- `bichat_checkpoints` - HITL checkpoints (thread_id, expires_at)

**Note**: `citations` column in `bichat_messages` stores JSONB array of web search citations with fields: Type, Title, URL, Excerpt, StartIndex, EndIndex.

**Critical Indexes:**
- `idx_bichat_sessions_tenant_user` - Multi-tenant queries
- `idx_bichat_messages_session` - Message listing
- `idx_bichat_checkpoints_thread` - Checkpoint lookup

## API Endpoints

```graphql
type Query {
  sessions(limit: Int, offset: Int): [Session!]!
  session(id: ID!): Session
  messages(sessionId: ID!, limit: Int, offset: Int): [Message!]!
}

type Mutation {
  createSession(title: String): Session!
  sendMessage(sessionId: ID!, content: String!, attachments: [Upload!]): SendMessageResponse!
  resumeWithAnswer(sessionId: ID!, checkpointId: ID!, answers: JSON!): SendMessageResponse!
}

type Subscription {
  messageStream(sessionId: ID!): MessageChunk!
}
```

HTTP Routes:
- `GET /bichat` - React chat app
- `POST /bichat/stream` - SSE streaming
- `POST /bichat/graphql` - GraphQL endpoint

## Default BI Agent Tools

Core tools (always available):
- `ask_user_question` - HITL questions (triggers interrupt)
- `final_answer` - End conversation
- `schema_list` - List database tables
- `schema_describe` - Describe table schema
- `sql_execute` - Execute read-only SQL (max 1000 rows, 30s timeout)
- `time` - Current date/time

Optional tools (configured via agent options):
- `kb_search` - Search knowledge base (requires `WithKBSearcher()`)
- `web_search` - Search the web for real-time information (requires `WithWebSearch(true)`)
- `export_data_to_excel` - Export data to Excel (requires `WithDataExportTool()`)
- `export_query_to_excel` - Execute SQL and export to Excel (requires `WithQueryExportTool()`)
- `task` - Delegate to sub-agents (requires `WithDelegationTool()`)

## HITL (Human-in-the-Loop) Flow

```go
// Agent interrupts with questions
resp, _ := agentService.ProcessMessage(ctx, sessionID, content, nil)
if resp.Interrupt != nil {
    // UI displays questions, saves checkpointID
    checkpointID := resp.Interrupt.CheckpointID
}

// Later: Resume with answers
finalResp, _ := agentService.ResumeWithAnswer(ctx, sessionID, checkpointID, answers)
```

## Events

Module emits via EventBus:
- `agent.start/complete/error` - Agent lifecycle
- `llm.request/response/stream` - LLM calls
- `tool.start/complete` - Tool execution
- `interrupt` - HITL trigger

## Environment Variables

```bash
# Required
OPENAI_API_KEY=sk-...
DATABASE_URL=postgres://...

# Optional
OPENAI_MODEL=gpt-4-turbo          # default: gpt-4
BICHAT_CONTEXT_WINDOW=180000      # default: 200k
BICHAT_COMPLETION_RESERVE=8000    # default: 8k
```

## Multi-Agent Orchestration

BiChat supports multi-agent orchestration where the parent BI agent can delegate complex tasks to specialized sub-agents.

### Enabling Multi-Agent Mode

There are two ways to set up multi-agent orchestration:

**Option 1: Automatic setup (Recommended)**

```go
// 1. Create agent registry and register sub-agents
registry := agents.NewAgentRegistry()
sqlAgent, _ := bichatagents.NewSQLAgent(queryExecutor)
registry.Register(sqlAgent)

// 2. Create parent agent with registry
parentAgent, _ := bichatagents.NewDefaultBIAgent(
    queryExecutor,
    bichatagents.WithKBSearcher(kbSearcher),
    bichatagents.WithAgentRegistry(registry),  // Pass registry to parent
    bichatagents.WithCodeInterpreter(true),
)

// 3. Create config - parent agent already knows about sub-agents
cfg := bichat.NewModuleConfig(
    composables.UseTenantID,
    composables.UseUserID,
    chatRepo,
    model,
    bichat.DefaultContextPolicy(),
    parentAgent,  // Agent with registry included
    bichat.WithQueryExecutor(queryExecutor),
)

// 4. Create service with registry for dynamic tool creation
service := services.NewAgentService(services.AgentServiceConfig{
    Agent:         cfg.ParentAgent,
    Model:         cfg.Model,
    AgentRegistry: registry,  // Service uses this for delegation tool
    // ... other config
})
```

**Option 2: Lazy setup (config-time)**

```go
// Create parent agent without registry
parentAgent, _ := bichatagents.NewDefaultBIAgent(queryExecutor)

// Config creates registry and sub-agents automatically
cfg := bichat.NewModuleConfig(
    composables.UseTenantID,
    composables.UseUserID,
    chatRepo,
    model,
    bichat.DefaultContextPolicy(),
    parentAgent,
    bichat.WithQueryExecutor(queryExecutor),  // Required for SQLAgent
    bichat.WithMultiAgent(true),              // Enable multi-agent orchestration
)

// Config automatically:
// 1. Creates AgentRegistry
// 2. Registers SQLAgent
// 3. Makes registry available via cfg.AgentRegistry
// 4. Parent agent system prompt won't include sub-agents (limitation)

// Service still needs registry for delegation tool
service := services.NewAgentService(services.AgentServiceConfig{
    Agent:         cfg.ParentAgent,
    Model:         cfg.Model,
    AgentRegistry: cfg.AgentRegistry,  // Use registry from config
    // ... other config
})
```

**Recommendation**: Use Option 1 for full control and better system prompts.

### Available Sub-Agents

**SQLAgent** (`sql-analyst`):
- Specializes in SQL query generation and database analysis
- Tools: `schema_list`, `schema_describe`, `sql_execute`
- Use when: Complex multi-step queries, schema exploration, data analysis
- Isolated from parent context (no HITL, no charting)

### Usage at Execution Time

The delegation tool is **automatically** added by `AgentService` when the registry is configured. The service dynamically creates the delegation tool at execution time with runtime session/tenant IDs.

**Implementation** (already done in `services/agent_service_impl.go`):

```go
// Create AgentService with registry
service := services.NewAgentService(services.AgentServiceConfig{
    Agent:         parentAgent,
    Model:         model,
    Policy:        policy,
    Renderer:      renderer,
    Checkpointer:  checkpointer,
    EventBus:      eventBus,
    ChatRepo:      chatRepo,
    AgentRegistry: cfg.AgentRegistry,  // Pass registry from config
})

// The service automatically adds delegation tool during execution:
// 1. Gets agent's default tools
// 2. Creates delegation tool with session/tenant IDs
// 3. Appends to tools list
// 4. Creates executor with extended tools
```

**No manual tool wiring needed** - just pass the registry to the service config.

### Delegation Workflow

User: "Find top 10 customers by total sales and generate a chart"

1. **Parent agent** receives request
2. Parent uses `task` tool to delegate SQL analysis to `sql-analyst`
3. **SQLAgent** executes independently:
   - `schema_list` to find tables
   - `schema_describe` to understand schema
   - `sql_execute` to query data
   - `final_answer` to return results
4. **Parent agent** receives SQLAgent's result
5. Parent uses `draw_chart` to visualize
6. Parent calls `final_answer` with chart + insights

### Recursion Prevention

The delegation tool is automatically filtered from child agent tool lists to prevent infinite delegation chains. SQLAgent never receives the `task` tool.

## Common Gotchas

1. **Generator Pattern**: Always defer `gen.Close()`. Check `hasMore` before processing events.
2. **Context Cancellation**: SSE streams must handle client disconnect gracefully.
3. **Token Budgeting**: Context window = prompt + completion reserve. Exceeding causes overflow errors.
4. **Multi-Tenant**: Every repository query MUST filter by `tenant_id`.
5. **SQL Validation**: Query executor blocks non-SELECT queries. Use `WITH ... SELECT` for CTEs.
6. **Checkpointer**: Required for HITL. Without it, `ask_user_question` tool fails.
7. **Delegation Tool**: Created at execution time, not config time (needs session/tenant IDs)

## Testing

```bash
go test -v ./modules/bichat/infrastructure/persistence/  # Repository
go test -v ./modules/bichat/services/                    # Service
go test -v ./modules/bichat/presentation/controllers/    # Controller
```

**Pattern**: Use ITF framework for integration tests
```go
func TestChatRepo(t *testing.T) {
    env := itf.Setup(t, itf.WithPermissions("bichat.access"))
    defer env.Teardown()
    repo := env.Repository().(*ChatRepository)
    // test...
}
```
