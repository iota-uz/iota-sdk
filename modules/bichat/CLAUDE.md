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

    // Optional: Token estimator for cost tracking
    bichat.WithTokenEstimator(tokenEstimator),

    // Optional: File storage for PDF exports
    bichat.WithFileStorage(fileStorage),

    // Optional: Logger for observability
    bichat.WithLogger(logger),

    // Optional: Metrics recorder
    bichat.WithMetrics(metrics),
)
```

### 3. Complete Setup with Observability

```go
import (
    "github.com/iota-uz/iota-sdk/modules/bichat"
    "github.com/iota-uz/iota-sdk/pkg/bichat/agents"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context"
    "github.com/iota-uz/iota-sdk/pkg/bichat/logging"
    "github.com/iota-uz/iota-sdk/pkg/bichat/storage"
    "github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
)

// Create token estimator
tokenEstimator := agents.NewTiktokenEstimator("cl100k_base")

// Create file storage
fileStorage, _ := storage.NewLocalFileStorage(
    "/var/lib/bichat/exports",
    "https://example.com/exports",
)

// Create logger
logger := logging.NewStdLogger()

// Create metrics recorder
metrics := hooks.NewStdMetricsRecorder()

// Create context policy with summarization
summarizer := context.NewLLMHistorySummarizer(llmModel, tokenEstimator)
policy := context.ContextPolicy{
    ContextWindow:     180000,
    CompletionReserve: 8000,
    OverflowStrategy:  context.OverflowCompact,
    Summarizer:        summarizer,
    Compaction: &context.CompactionConfig{
        SummarizeHistory:   true,
        MaxHistoryMessages: 50,
    },
}

// Configure module
cfg := bichat.NewModuleConfig(
    composables.UseTenantID,
    composables.UseUserID,
    chatRepo,
    llmModel,
    policy,
    parentAgent,
    bichat.WithTokenEstimator(tokenEstimator),
    bichat.WithFileStorage(fileStorage),
    bichat.WithLogger(logger),
    bichat.WithMetrics(metrics),
)

app.RegisterModule(bichat.NewModule(cfg))
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

**Required:**
```bash
# OpenAI API Key (REQUIRED)
OPENAI_API_KEY=sk-...

# Database connection (REQUIRED)
DATABASE_URL=postgres://user:pass@localhost/dbname
```

**Optional:**
```bash
# OpenAI Model (optional, defaults to gpt-4)
OPENAI_MODEL=gpt-4-turbo

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

## Setup & Configuration

### 1. Prerequisites

BiChat requires:
- **OpenAI API Key**: Set `OPENAI_API_KEY` environment variable
- **PostgreSQL Database**: Version 13+ with multi-tenant schema
- **Go 1.23+**: For SDK compatibility

### 2. Quick Start

```go
import (
    "github.com/iota-uz/iota-sdk/modules/bichat"
    "github.com/iota-uz/iota-sdk/modules/bichat/infrastructure"
    "github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/llmproviders"
    "github.com/iota-uz/iota-sdk/pkg/composables"
)

// Validate dependencies at startup
if err := bichat.ValidateBiChatDependencies(app.DB()); err != nil {
    log.Fatal("BiChat dependencies missing:", err)
}

// Create OpenAI model (reads OPENAI_API_KEY and OPENAI_MODEL from env)
model, err := llmproviders.NewOpenAIModel()
if err != nil {
    log.Fatal("Failed to create OpenAI model:", err)
}

// Create PostgreSQL query executor
executor := infrastructure.NewPostgresQueryExecutor(app.DB())

// Configure module with required dependencies
cfg := bichat.NewModuleConfig(
    composables.UseTenantID,
    composables.UseUserID,
    chatRepo,
    model,
    bichat.DefaultContextPolicy(),
    parentAgent,
    bichat.WithQueryExecutor(executor),
)

// Validate configuration
if err := cfg.Validate(); err != nil {
    log.Fatal("Invalid BiChat configuration:", err)
}
```

### 3. Production Deployment Checklist

Before deploying BiChat to production:

- [ ] **Environment Variables Set**:
  - `OPENAI_API_KEY` is configured
  - `DATABASE_URL` points to production database
  - `OPENAI_MODEL` is set if not using default gpt-4

- [ ] **Database Schema**:
  - Run migrations: `make db migrate up`
  - Verify tables: `bichat_sessions`, `bichat_messages`, `bichat_attachments`
  - Check indexes on `tenant_id` and `user_id`

- [ ] **Multi-Tenant Isolation**:
  - All queries include `tenant_id` in WHERE clauses
  - Context middleware injects tenant ID
  - Test cross-tenant data leakage

- [ ] **Query Executor Security**:
  - SQL validation blocks non-SELECT queries
  - Row limits enforced (max 1000 rows)
  - Query timeouts configured (default 30s)

- [ ] **Observability**:
  - Logging configured (see Observability section)
  - Metrics collection enabled
  - Event bus buffer sized appropriately

- [ ] **Cost Management**:
  - Token estimator enabled
  - Rate limiting configured
  - Monitor OpenAI API usage

## Troubleshooting

### Common Errors

**Error: "OPENAI_API_KEY environment variable is required"**
- **Cause**: OPENAI_API_KEY not set in environment
- **Fix**: Set environment variable: `export OPENAI_API_KEY=sk-...`
- **Production**: Add to `.env` file or container environment

**Error: "database connection is required for BiChat"**
- **Cause**: Database not initialized before BiChat setup
- **Fix**: Ensure `app.DB()` returns valid connection before calling `CreatePostgresQueryExecutor`
- **Check**: Verify `DATABASE_URL` is set correctly

**Error: "tenant ID required for query execution"**
- **Cause**: Context missing tenant ID when executing SQL
- **Fix**: Ensure tenant middleware is applied to BiChat routes
- **Verify**: `composables.UseTenantID(ctx)` returns valid UUID

**Error: "query contains disallowed keyword: INSERT"**
- **Cause**: Attempted to execute non-SELECT query via query executor
- **Fix**: Only SELECT and WITH (CTE) queries are allowed
- **Solution**: Use appropriate API for data modifications

**Error: "OpenAI API request failed"**
- **Cause**: Invalid API key, rate limit, or network issue
- **Fix**:
  - Verify API key is valid
  - Check OpenAI service status
  - Implement retry logic with backoff
  - Monitor rate limits

### Debugging Tips

**Enable Verbose Logging:**
```bash
LOG_LEVEL=debug
```

**Test OpenAI Connection:**
```bash
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"
```

**Verify Database Connection:**
```bash
psql $DATABASE_URL -c "SELECT 1"
```

**Check Tenant Context:**
```go
tenantID, err := composables.UseTenantID(ctx)
if err != nil {
    log.Printf("Missing tenant ID: %v", err)
}
```

## Observability & Cost Tracking

### Token Estimation

Track token usage for cost monitoring and budget enforcement:

```go
// Create estimator
estimator := agents.NewTiktokenEstimator("cl100k_base")

// Integrate with executor
executor := agents.NewExecutor(agent, model,
    agents.WithTokenEstimator(estimator),
    agents.WithEventBus(eventBus),
)

// Subscribe to LLM request events for cost tracking
eventBus.Subscribe(hooks.EventLLMRequest, func(e hooks.Event) error {
    reqEvent := e.(*events.LLMRequestEvent)
    log.Printf("LLM Request: model=%s, estimated_tokens=%d",
        reqEvent.ModelName, reqEvent.EstimatedTokens)

    // Track costs
    costPerToken := 0.00003 // $0.03 per 1K tokens (GPT-4)
    estimatedCost := float64(reqEvent.EstimatedTokens) * costPerToken
    metrics.RecordGauge("bichat.llm.estimated_cost", estimatedCost, labels)

    return nil
})
```

**Token Estimator Options**:
- **Tiktoken** (accurate, ~10-50ms): Use for production cost tracking
- **Character-based** (fast, <1ms): Use for rate limiting and quick estimates
- **NoOp** (instant): Disable estimation when not needed

### Logging

Add structured logging for debugging and monitoring:

```go
// Create logger
logger := logging.NewStdLogger()

// Use with components
pdfTool := tools.NewExportToPDFTool(
    gotenbergURL,
    fileStorage,
    tools.WithLogger(logger),
)

fsSource := sources.NewFileSystemSource(
    rootDir,
    sources.WithFSLogger(logger),
)

// Logs automatically capture errors:
// - Response body close failures
// - Filesystem watcher close errors
// - File storage errors
```

**Production Integration**:
```go
// Example: Custom slog-based logger
type SlogLogger struct {
    logger *slog.Logger
}

func (l *SlogLogger) Error(ctx context.Context, msg string, fields map[string]any) {
    l.logger.ErrorContext(ctx, msg, slogFields(fields)...)
}

// Use with BiChat
logger := &SlogLogger{logger: slog.Default()}
```

### Metrics

Monitor performance and health with metrics:

```go
// Create metrics recorder
metrics := hooks.NewStdMetricsRecorder()

// Use with async event handler
asyncHandler := handlers.NewAsyncHandler(
    baseHandler,
    100, // buffer size
    handlers.WithMetrics(metrics),
)

// Automatically tracked:
// - bichat.async_handler.queue_depth (gauge)
// - bichat.async_handler.dropped_events (counter with reason label)
```

**Production Integration with Prometheus**:
```go
import "github.com/prometheus/client_golang/prometheus"

type PrometheusMetrics struct {
    queueDepth   prometheus.Gauge
    droppedEvents *prometheus.CounterVec
}

func (m *PrometheusMetrics) RecordGauge(name string, value float64, labels map[string]string) {
    if name == "bichat.async_handler.queue_depth" {
        m.queueDepth.Set(value)
    }
}

func (m *PrometheusMetrics) IncrementCounter(name string, value int64, labels map[string]string) {
    if name == "bichat.async_handler.dropped_events" {
        m.droppedEvents.With(labels).Add(float64(value))
    }
}
```

### File Storage

Persist generated files (PDFs, exports) with storage abstraction:

```go
// Local filesystem storage
storage, _ := storage.NewLocalFileStorage(
    "/var/lib/bichat/exports",
    "https://example.com/exports",
)

// Use with PDF export tool
pdfTool := tools.NewExportToPDFTool(
    "http://gotenberg:3000",
    storage,
    tools.WithLogger(logger),
)

// Files automatically saved with unique names
// URL returned: https://example.com/exports/uuid.pdf
```

**Cloud Storage Integration**:
```go
// Example: Custom S3 storage backend
type S3Storage struct {
    bucket string
    client *s3.Client
}

func (s *S3Storage) Save(ctx context.Context, filename string, content io.Reader, metadata storage.FileMetadata) (string, error) {
    key := fmt.Sprintf("bichat/%s-%s", uuid.New(), filename)
    _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket:      aws.String(s.bucket),
        Key:         aws.String(key),
        Body:        content,
        ContentType: aws.String(metadata.ContentType),
    })
    if err != nil {
        return "", err
    }
    return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s.bucket, key), nil
}
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
