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
│   │   ├── postgres_chat_repository_test.go
│   │   └── schema/              # SQL migrations
│   ├── llmproviders/
│   │   ├── openai_model.go      # OpenAI implementation of agents.Model
│   │   └── openai_model_test.go
│   ├── logs/
│   └── postgres_executor.go     # SQL execution tool implementation
├── services/
│   ├── agent_service_impl.go    # Agent orchestration with event streaming
│   ├── chat_service_impl.go     # Chat session management
│   ├── attachment_service.go    # File upload handling
│   └── title_generation_service.go # AI session title generator
├── presentation/
│   ├── assets/
│   │   ├── embed.go             # Embedded React build (dist/)
│   │   └── dist/                # React build output (created by pnpm build)
│   ├── controllers/
│   │   ├── chat_controller.go   # Legacy REST endpoints (not registered by default)
│   │   ├── stream_controller.go # SSE streaming
│   │   └── web_controller_test.go
│   └── locales/                 # i18n files
└── agents/
    ├── default_agent.go         # Default BI agent
    ├── sql_agent.go             # Specialized SQL agent
    └── query_executor_adapter.go # Tool adapter
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
query := "SELECT * FROM bichat.sessions WHERE tenant_id = $1 AND id = $2"
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

### Project Prompt Extension

Use repository-scoped extensions to add domain instructions while keeping the SDK vendor prompt intact.

```go
cfg := bichat.NewModuleConfig(
    composables.UseTenantID,
    composables.UseUserID,
    chatRepo,
    llmModel,
    bichat.DefaultContextPolicy(),
    parentAgent,
    bichat.WithProjectPromptExtension(`
You are operating in insurance BI domain.
Prioritize policy lifecycle, claims fraud signals, reserve adequacy, and underwriting KPIs.
`),
)
```

Provider-based extension (resolved once during `BuildServices()`):

```go
cfg := bichat.NewModuleConfig(
    composables.UseTenantID,
    composables.UseUserID,
    chatRepo,
    llmModel,
    bichat.DefaultContextPolicy(),
    parentAgent,
    bichat.WithProjectPromptExtensionProvider(
        prompts.ProjectPromptExtensionProviderFunc(func() (string, error) {
            return loadProjectPromptFromStore(), nil
        }),
    ),
)
```

Behavior:
- Vendor/base system prompt is always preserved.
- Extension is appended in parent agent flow (`AgentService.ProcessMessage`).
- Provider output takes precedence over static extension when non-empty.

## Skills Tree (Codex/Claude-Style)

BiChat supports a startup-loaded skills tree from markdown files. Skills are injected as a
`KindReference` context block per turn (not merged into the base system prompt).

### Directory Structure

- Configure a global skills root directory.
- Each skill must be a directory containing `SKILL.md`.
- Skill slug is the directory path relative to root.

Example:

```text
/opt/bichat/skills/
  analytics/
    sql-debug/
      SKILL.md
  finance/
    month-end/
      SKILL.md
```

### SKILL.md Frontmatter Schema

```md
---
name: SQL Debugging
description: Recover quickly from SQL errors in BI workflows.
when_to_use:
  - query error
  - wrong column
tags:
  - sql
  - debugging
---

# Optional heading

Skill instructions body...
```

Required fields:
- `name`
- `description`
- `when_to_use`
- `tags`

### Runtime Behavior

- Skills are loaded and validated once during `BuildServices()`.
- On each turn, BiChat auto-selects relevant skills by lexical overlap.
- Users can force a skill with mention syntax: `@skill-slug` (example: `@finance/month-end`).
- Default budget: top 3 skills and max 8000 rendered characters.
- If skills loading fails, startup fails fast.

### Configuration

```go
cfg := bichat.NewModuleConfig(
    composables.UseTenantID,
    composables.UseUserID,
    chatRepo,
    llmModel,
    bichat.DefaultContextPolicy(),
    parentAgent,
    bichat.WithSkillsDir("/opt/bichat/skills"),
    bichat.WithSkillsSelectionLimit(3),
    bichat.WithSkillsMaxChars(8000),
)
```

## Database Schema

See: `infrastructure/persistence/schema/bichat-schema.sql`

**Key Tables:**
- `bichat.sessions` - Chat sessions (tenant_id, user_id, status, pinned)
- `bichat.messages` - Messages (session_id, role, content, tool_calls, citations)
- `bichat.attachments` - File attachments (message_id, file_name, storage_path)
- `bichat.checkpoints` - HITL checkpoints (thread_id, expires_at)
- `bichat.learnings` - Learned SQL gotchas and fixes
- `bichat.validated_queries` - Reusable validated SQL patterns

**Note**: `citations` column in `bichat_messages` stores JSONB array of web search citations with fields: Type, Title, URL, Excerpt, StartIndex, EndIndex.

**Critical Indexes:**
- `idx_bichat_sessions_tenant_user` - Multi-tenant queries
- `idx_bichat_messages_session` - Message listing
- `idx_bichat_checkpoints_thread` - Checkpoint lookup

## API Endpoints

BiChat UI access is implemented via applet RPC + streaming.

HTTP Routes:
- `GET /bi-chat` - React chat applet
- `POST /bi-chat/rpc` - Applet RPC (session CRUD, artifacts, etc.)
- `POST /bi-chat/stream` - SSE streaming (assistant response chunks)

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
OPENAI_MODEL=gpt-5.2-2025-12-11         # default: gpt-5.2-2025-12-11
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

## Migration Guide: Fail-Fast Refactoring

The BiChat module now uses a **fail-fast** approach - configuration errors are caught at startup instead of allowing degraded functionality at runtime.

### Breaking Changes

1. **Attachment storage now required** - Must provide paths or explicitly disable
2. **Module registration can fail** - Must check error from `app.RegisterModule()`
3. **AnthropicRenderer default** - Proper context rendering (was stub)
4. **TiktokenEstimator default** - Accurate token counting (~10-50ms overhead)

### Old Code (Silent Failures)

```go
cfg := bichat.NewModuleConfig(
    composables.UseTenantID,
    composables.UseUserID,
    chatRepo,
    model,
    bichat.DefaultContextPolicy(),
    parentAgent,
)

module := bichat.NewModuleWithConfig(cfg)
app.RegisterModule(module) // No error check - failures logged but hidden
```

**Problems**:
- Attachments silently discarded (NoOp fallback)
- Title generation fails silently
- RPC becomes unavailable without indication
- Token counting disabled (returns 0)

### New Code (Fail Fast)

```go
cfg := bichat.NewModuleConfig(
    composables.UseTenantID,
    composables.UseUserID,
    chatRepo,
    model,
    bichat.DefaultContextPolicy(),
    parentAgent,
    // REQUIRED: Attachment storage configuration
    bichat.WithAttachmentStorage(
        "/var/lib/bichat/attachments",
        "https://example.com/bichat/attachments",
    ),
)

module := bichat.NewModuleWithConfig(cfg)
if err := app.RegisterModule(module); err != nil {
    log.Fatalf("Failed to register BiChat: %v", err)
}
```

### Optional Configurations

**Disable Attachments** (testing only):
```go
cfg := bichat.NewModuleConfig(
    ...,
    bichat.WithNoOpAttachmentStorage(), // Explicit disable
)
```

**Auto-Titles**:
Session title generation is always enabled and runs automatically after assistant responses.

**Custom Renderer**:
```go
customRenderer := myrenderers.NewCustomRenderer()
cfg := bichat.NewModuleConfig(
    ...,
    bichat.WithRenderer(customRenderer),
)
```

### Error Messages

All errors are descriptive and actionable:

```
"AttachmentStorageBasePath required - use WithAttachmentStorage(path, url) or WithNoOpAttachmentStorage()"
"OverflowStrategy=compact requires accurate TokenEstimator (not NoOp)"
"failed to create title generation service: model returned error"
"failed to create attachment storage: directory /var/lib/bichat/attachments is not writable"
```

### Troubleshooting

**Error: "AttachmentStorageBasePath required"**
- Solution: Add `bichat.WithAttachmentStorage("/path", "https://url")` to config
- Or: Use `bichat.WithNoOpAttachmentStorage()` for testing

**Error: "failed to create attachment storage: permission denied"**
- Solution: Ensure directory exists and is writable by application user
- Check: `mkdir -p /var/lib/bichat/attachments && chown app:app /var/lib/bichat/attachments`

**Error: "failed to build BiChat services"**
- Solution: Check logs for underlying error (title generation, storage, etc.)
- Enable debug logging: `cfg.Logger.SetLevel(logrus.DebugLevel)`

**Module registration fails at startup**
- This is expected behavior - fix configuration and restart
- Old behavior: Module loaded with broken features
- New behavior: Application fails fast with clear error message
