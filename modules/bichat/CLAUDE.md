# BiChat Module

Production-ready BI chat module for IOTA SDK using the BI-Chat foundation (`pkg/bichat`).

## What It Is

Complete chat module with:
- Multi-tenant session and message management
- LLM-powered BI agent with SQL tools
- GraphQL API and HTTP/SSE endpoints
- React UI with HTMX integration
- Knowledge base search
- HITL (Human-in-the-Loop) support

## Module Structure

```
modules/bichat/
├── module.go                           # Module registration, DI wiring
├── config.go                           # Configuration types
├── infrastructure/
│   ├── persistence/
│   │   ├── chat_repository.go         # PostgreSQL implementation
│   │   └── chat_repository_test.go    # Repository tests
│   └── llmproviders/
│       └── openai_provider.go         # LLM provider implementation
├── services/
│   ├── agent_service_impl.go          # Agent orchestration
│   └── agent_service_impl_test.go     # Service tests
├── presentation/
│   ├── controllers/
│   │   ├── chat_controller.go         # HTTP/GraphQL endpoints
│   │   ├── stream_controller.go       # SSE streaming
│   │   └── web_controller.go          # React app rendering
│   ├── graphql/
│   │   └── schema.graphql             # GraphQL schema
│   ├── interop/
│   │   ├── context.go                 # Server context injection
│   │   ├── permissions.go             # Permission helpers
│   │   └── types.go                   # Go/TS shared types
│   ├── locales/
│   │   ├── en.json                    # Translations
│   │   ├── ru.json
│   │   └── uz.json
│   └── templates/
│       └── pages/bichat/              # Templ templates
└── agents/
    └── default_agent.go               # Default BI agent
```

## Quick Setup

### 1. Module Configuration

```go
import (
    "github.com/iota-uz/iota-sdk/modules/bichat"
    "github.com/iota-uz/iota-sdk/pkg/bichat/agents"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context"
)

// Create config
cfg := bichat.NewModuleConfig(
    composables.UseTenantID,  // Tenant ID extractor
    composables.UseUserID,    // User ID extractor
    chatRepo,                 // ChatRepository implementation
    llmModel,                 // LLM Model implementation
    bichat.DefaultContextPolicy(), // Token budget policy
    parentAgent,              // Parent agent
)

// Register module
app.RegisterModule(bichat.NewModule(cfg))
```

### 2. With Optional Dependencies

```go
cfg := bichat.NewModuleConfig(
    composables.UseTenantID,
    composables.UseUserID,
    chatRepo,
    llmModel,
    bichat.DefaultContextPolicy(),
    parentAgent,

    // Optional: SQL execution
    bichat.WithQueryExecutor(queryExecutor),

    // Optional: Knowledge base
    bichat.WithKBSearcher(kbSearcher),

    // Optional: Sub-agents
    bichat.WithSubAgents(sqlAgent, chartAgent),

    // Optional: Custom event bus
    bichat.WithEventBus(eventBus),

    // Optional: PostgreSQL checkpointer for HITL
    bichat.WithCheckpointer(checkpointer),
)
```

## Database Schema

### Sessions Table

```sql
CREATE TABLE bichat_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    pinned BOOLEAN NOT NULL DEFAULT false,
    parent_session_id UUID REFERENCES bichat_sessions(id),
    pending_question_agent VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bichat_sessions_tenant_user ON bichat_sessions(tenant_id, user_id);
```

### Messages Table

```sql
CREATE TABLE bichat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES bichat_sessions(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    tool_calls JSONB,
    tool_call_id VARCHAR(255),
    citations JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bichat_messages_session ON bichat_messages(session_id, created_at DESC);
```

### Attachments Table

```sql
CREATE TABLE bichat_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES bichat_messages(id) ON DELETE CASCADE,
    file_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,
    storage_path TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bichat_attachments_message ON bichat_attachments(message_id);
```

## API Endpoints

### GraphQL Schema

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
  archiveSession(id: ID!): Session!
  pinSession(id: ID!): Session!
}

type Subscription {
  messageStream(sessionId: ID!): MessageChunk!
}
```

### HTTP Routes

- `GET /bichat` - React chat app
- `POST /bichat/stream` - SSE streaming
- `GET /bichat/sessions` - List sessions
- `POST /bichat/sessions` - Create session
- `POST /bichat/sessions/:id/messages` - Send message

## Service Implementations

### ChatRepository (PostgreSQL)

```go
type ChatRepository interface {
    CreateSession(ctx context.Context, session *domain.Session) error
    GetSession(ctx context.Context, id uuid.UUID) (*domain.Session, error)
    ListSessions(ctx context.Context, userID int64, limit, offset int) ([]*domain.Session, error)
    SaveMessage(ctx context.Context, message *domain.Message) error
    GetMessages(ctx context.Context, sessionID uuid.UUID, limit, offset int) ([]*domain.Message, error)
    SaveAttachment(ctx context.Context, attachment *domain.Attachment) error
}
```

**Implementation Pattern**:
- Use `composables.UseTenantID(ctx)` for tenant isolation
- Include `tenant_id` in all WHERE clauses
- Parameterized queries ($1, $2) - no concatenation
- Error wrapping with `serrors.E(op, err)`

### AgentService

```go
type AgentService interface {
    ProcessMessage(ctx context.Context, sessionID uuid.UUID, content string, attachments []domain.Attachment) (Generator[Event], error)
    ResumeWithAnswer(ctx context.Context, sessionID uuid.UUID, checkpointID string, answers map[string]string) (Generator[Event], error)
}
```

**Responsibilities**:
- Build context graph (system, schemas, KB, history, user turn)
- Execute ReAct loop with tools
- Yield events (LLM requests, tool calls, results)
- Handle interrupts (HITL questions)
- Save checkpoints for resume

## Default BI Agent

Configured with tools:
- `ask_user_question` - HITL questions
- `final_answer` - End conversation
- `task` - Delegate to sub-agents
- `time` - Current date/time
- `schema_list` - List database tables
- `schema_describe` - Describe table schema
- `sql_execute` - Execute SQL (read-only, validated)
- `kb_search` - Search knowledge base

**System Prompt Pattern**:
```go
systemPrompt := `You are a BI analyst assistant.

Tools available:
- schema_list: List all database tables
- schema_describe: Get table schema and columns
- sql_execute: Execute SQL queries (SELECT only)
- kb_search: Search documentation and knowledge base
- ask_user_question: Ask clarifying questions
- final_answer: Provide final response

When analyzing data:
1. Understand the question
2. Explore schema with schema_list/schema_describe
3. Write and execute SQL queries
4. Analyze results
5. Provide insights with final_answer

If unsure, use ask_user_question.`
```

## React UI Integration

Server injects context for React app:

```go
func (c *WebController) RenderChatApp(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    user := composables.UseUser(ctx)
    tenant := composables.UseTenantID(ctx)
    pageCtx := composables.UsePageCtx(ctx)

    initialContext := interop.InitialContext{
        User: interop.UserContext{
            ID:          user.ID,
            Email:       user.Email,
            Permissions: c.getUserPermissions(ctx),
        },
        Tenant: interop.TenantContext{
            ID: tenant.String(),
        },
        Locale: interop.LocaleContext{
            Language:     pageCtx.GetLocale().String(),
            Translations: c.getTranslations(pageCtx),
        },
        Config: interop.AppConfig{
            GraphQLEndpoint: "/bichat/graphql",
            StreamEndpoint:  "/bichat/stream",
        },
    }

    return templates.RenderReactApp(w, initialContext)
}
```

## SSE Streaming Pattern

```go
func (c *StreamController) StreamMessages(w http.ResponseWriter, r *http.Request) {
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    // Get event generator
    gen, err := c.agentService.ProcessMessage(ctx, sessionID, content, attachments)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer gen.Close()

    // Stream events
    flusher := w.(http.Flusher)
    for {
        event, err, hasMore := gen.Next()
        if err != nil || !hasMore {
            break
        }

        // Send SSE event
        fmt.Fprintf(w, "data: %s\n\n", toJSON(event))
        flusher.Flush()
    }
}
```

## HITL Flow

### 1. Agent Asks Questions

```go
// Agent calls ask_user_question tool during ReAct loop
resp, err := chatService.SendMessage(ctx, req)

// Check for interrupt
if resp.Interrupt != nil {
    // Display questions to user in UI
    for _, q := range resp.Interrupt.Questions {
        renderQuestion(q.ID, q.Text, q.Options)
    }
    // Save checkpoint ID for resume
    checkpointID := resp.Interrupt.CheckpointID
}
```

### 2. User Provides Answers

```go
// User submits answers via form/UI
answers := map[string]string{
    "question_1": "Q1 2024",
    "question_2": "revenue",
}

// Resume from checkpoint
resumeReq := ResumeRequest{
    SessionID:    sessionID,
    CheckpointID: checkpointID,
    Answers:      answers,
}
finalResp, err := chatService.ResumeWithAnswer(ctx, resumeReq)
```

## Permissions

```go
const (
    PermBiChatAccess   = "bichat.access"      // Access module
    PermBiChatReadAll  = "bichat.read_all"    // Read all sessions (admin)
    PermBiChatExport   = "bichat.export"      // Export chat data
    PermBiChatManage   = "bichat.manage"      // Manage settings
)
```

**Controller Pattern**:
```go
func (c *ChatController) RegisterRoutes(r *mux.Router) {
    // Require bichat.access permission
    r.Use(middleware.RequirePermission(PermBiChatAccess))

    r.HandleFunc("/sessions", c.ListSessions).Methods("GET")
    r.HandleFunc("/sessions", c.CreateSession).Methods("POST")
    r.HandleFunc("/sessions/{id}/messages", c.SendMessage).Methods("POST")
}
```

## Events

Module emits events via EventBus:

- `agent.start` - Agent execution started
- `agent.complete` - Agent execution completed
- `agent.error` - Agent execution failed
- `llm.request` - LLM API request sent
- `llm.response` - LLM API response received
- `llm.stream` - LLM streaming chunk
- `tool.start` - Tool execution started
- `tool.complete` - Tool execution completed
- `context.compile` - Context compiled
- `context.overflow` - Token budget overflow
- `session.create` - Session created
- `message.save` - Message saved
- `interrupt` - HITL interrupt triggered

## Testing

### Repository Tests

```bash
go test -v ./modules/bichat/infrastructure/persistence/
```

**Pattern**: Use ITF framework
```go
func TestChatRepository_CreateSession(t *testing.T) {
    t.Parallel()
    env := itf.Setup(t, itf.WithPermissions("bichat.access"))
    defer env.Teardown()

    repo := env.Repository().(*ChatRepository)
    session := domain.NewSession(
        domain.WithTenantID(env.TenantID),
        domain.WithUserID(env.UserID),
        domain.WithTitle("Test Session"),
    )

    err := repo.CreateSession(env.Context(), session)
    assert.NoError(t, err)
    assert.NotEmpty(t, session.ID)
}
```

### Service Tests

```bash
go test -v ./modules/bichat/services/
```

**Pattern**: Mock dependencies
```go
func TestAgentService_ProcessMessage(t *testing.T) {
    t.Parallel()
    env := itf.Setup(t, itf.WithPermissions("bichat.access"))
    defer env.Teardown()

    mockModel := &MockModel{}
    service := NewAgentService(mockModel, toolRegistry, repo, eventBus)

    gen, err := service.ProcessMessage(env.Context(), sessionID, "Show revenue", nil)
    assert.NoError(t, err)
    defer gen.Close()

    // Validate events
    for {
        event, err, hasMore := gen.Next()
        if !hasMore {
            break
        }
        assert.NoError(t, err)
    }
}
```

### Controller Tests

```bash
go test -v ./modules/bichat/presentation/controllers/
```

## Environment Variables

```bash
# LLM Provider (defaults to OpenAI)
LLM_PROVIDER=openai
LLM_API_KEY=sk-...
LLM_MODEL=gpt-4

# Context Configuration
BICHAT_CONTEXT_WINDOW=180000
BICHAT_COMPLETION_RESERVE=8000

# Knowledge Base (optional)
BICHAT_KB_PATH=/var/lib/bichat/kb
BICHAT_KB_ENABLED=true

# Observability
BICHAT_EVENT_BUS_BUFFER_SIZE=1000
LOG_LEVEL=info
```

## Common Patterns

### Creating Custom Agent

```go
type CustomAgent struct {
    model         agents.Model
    tools         []agents.Tool
    systemPrompt  string
}

func (a *CustomAgent) Execute(ctx context.Context, input string) (agents.Response, error) {
    // Build context
    builder := context.NewBuilder()
    builder.System(context.SystemCodec, a.systemPrompt)
    builder.Turn(context.TurnCodec, input)

    // Compile
    compiled, _ := builder.Compile(renderer, policy)

    // Generate
    return a.model.Generate(ctx, agents.Request{
        Messages: compiled.Messages,
        Tools:    a.tools,
    })
}
```

### Adding Custom Tool

```go
customTool := agents.NewTool(
    "custom_analysis",
    "Perform custom BI analysis",
    parameterSchema,
    func(ctx context.Context, input string) (string, error) {
        params, _ := agents.ParseToolInput[AnalysisParams](input)
        results := performAnalysis(ctx, params)
        return agents.FormatToolOutput(results)
    },
)

// Add to tool registry
toolRegistry = append(toolRegistry, customTool)
```

### Multi-Tenant Isolation

```go
// Repository implementation
func (r *ChatRepository) GetSession(ctx context.Context, id uuid.UUID) (*domain.Session, error) {
    const op serrors.Op = "ChatRepository.GetSession"

    // CRITICAL: Get tenant ID from context
    tenantID, err := composables.UseTenantID(ctx)
    if err != nil {
        return nil, serrors.E(op, err)
    }

    // CRITICAL: Include tenant_id in WHERE clause
    query := "SELECT * FROM bichat_sessions WHERE tenant_id = $1 AND id = $2"
    var session domain.Session
    err = r.db.GetContext(ctx, &session, query, tenantID, id)
    if err != nil {
        return nil, serrors.E(op, err)
    }

    return &session, nil
}
```

## Migration from Ali/Shyona

**Key Differences**:
- Ali: `modules/ali/` (legacy)
- BiChat: `modules/bichat/` (new)

**Changes**:
1. Domain models are structs (not interfaces)
2. Content-addressed context (not time-based)
3. Generator pattern (not channels)
4. Provider-agnostic Model interface
5. Token budgeting at compile time
6. HITL via checkpoint/resume

**Migration Steps**:
1. Update imports: `pkg/ali` → `pkg/bichat`
2. Replace interfaces with structs for domain models
3. Update context building to use `ContextBuilder`
4. Replace channel loops with Generator pattern
5. Update HITL flow to use checkpoint/resume
6. Test thoroughly with ITF framework

## Related Files

- `pkg/bichat/CLAUDE.md` - BI-Chat foundation package guide
- `~/.claude/plans/purrfect-coalescing-marshmallow.md` - Implementation plan
