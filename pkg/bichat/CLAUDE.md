# BI-Chat Foundation Package

Agent framework for building LLM-powered BI chat modules in IOTA SDK.

## What It Is

BI-Chat provides:
- **Agent Framework**: ReAct loop execution with tools and LLM integration
- **Context Management**: Content-addressed blocks with token budgeting
- **Knowledge Base**: Bleve full-text search (pure Go, embedded)
- **Event System**: Pub/sub for observability and metrics
- **Domain Models**: Session, Message, Attachment, Citation entities
- **Common Tools**: SQL, schema, KB search, time, export, HITL

## Package Structure

```text
pkg/bichat/
├── agents/          # ReAct loop, Model interface, Tool interface, Generator pattern
├── context/         # Content-addressed blocks, token budgeting, renderers
├── kb/              # Bleve indexer, searcher, document sources
├── hooks/           # EventBus, event handlers, cost tracking
├── tools/           # Built-in tools (SQL, schema, KB, time, export, HITL)
└── domain/          # Session, Message, Attachment, Citation models
```

## Quick Start

### 1. Create Agent with Tools

```go
import (
    "github.com/iota-uz/iota-sdk/pkg/bichat/agents"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context"
    "github.com/iota-uz/iota-sdk/pkg/bichat/domain"
    "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
)

// Create LLM model (provider-specific)
model := anthropic.NewModel(client, anthropic.ModelConfig{
    Name:      "claude-3-5-sonnet-20241022",
    MaxTokens: 4096,
})

// Register tools
toolRegistry := []agents.Tool{
    tools.NewTimeTool(),
    tools.NewSQLExecuteTool(queryExecutor),
    tools.NewKBSearchTool(kbSearcher),
}

// Create session
session := domain.NewSession(
    domain.WithTenantID(tenantID),
    domain.WithUserID(userID),
    domain.WithTitle("BI Analysis"),
)
```

### 2. Build Context with Token Budget

```go
// Build context
builder := context.NewBuilder()
builder.
    System(context.SystemCodec, "You are a BI analyst.").
    Reference(context.SchemaCodec, dbSchemas).
    Memory(context.KBCodec, kbResults).
    History(context.HistoryCodec, messages).
    Turn(context.TurnCodec, userMessage)

// Compile with token budget
renderer := context.NewAnthropicRenderer()
policy := context.ContextPolicy{
    ContextWindow:     180000,
    CompletionReserve: 8000,
    OverflowStrategy:  context.OverflowTruncate,
}
compiled, err := builder.Compile(renderer, policy)
```

### 3. Execute ReAct Loop

```go
// Generate response
resp, err := model.Generate(ctx, agents.Request{
    Messages: compiled.Messages,
    Tools:    toolRegistry,
})

// Handle tool calls in ReAct loop
for _, toolCall := range resp.Message.ToolCalls {
    tool := findTool(toolRegistry, toolCall.Name)
    result, err := tool.Call(ctx, toolCall.Input)
    // Add result to context, continue loop
}
```

## Component Usage

### Agent Framework (`agents/`)

**Model Interface**: Provider-agnostic LLM abstraction
- `Generate(ctx, Request) (Response, error)` - Blocking generation
- `GenerateStream(ctx, Request) (Generator[Chunk], error)` - Streaming
- `HasCapability(cap) bool` - Feature detection (thinking, vision, JSON, tools)

**Tool Interface**: Agent capabilities
```go
tool := agents.NewTool(
    "search_db",
    "Search database for records",
    parameterSchema, // JSON Schema
    func(ctx context.Context, input string) (string, error) {
        params, _ := agents.ParseToolInput[SearchParams](input)
        results := search(params)
        return agents.FormatToolOutput(results)
    },
)
```

**Generator Pattern**: Lazy iteration for streaming
```go
gen, err := processMessage(ctx, req)
defer gen.Close()

for {
    event, err, hasMore := gen.Next()
    if err != nil || !hasMore {
        break
    }
    handleEvent(event)
}
```

### Token Estimation (`agents/`)

**Purpose**: Accurate token counting for cost tracking and budget management.

**Implementations**:
- **TiktokenEstimator**: Accurate token counting using tiktoken library (~10-50ms per message)
  - Supports provider-specific encodings (cl100k_base for GPT-4/Claude)
  - Includes overhead for message formatting and role tokens
- **CharacterBasedEstimator**: Fast approximation using ~4 chars/token (<1ms)
  - Configurable chars-per-token ratio (default: 4.0)
  - No external dependencies
- **NoOpTokenEstimator**: No-op implementation (returns 0 tokens)

**Usage**:
```go
// Tiktoken (accurate)
estimator := agents.NewTiktokenEstimator("cl100k_base")
tokens, _ := estimator.EstimateMessages(ctx, messages)

// Character-based (fast)
estimator := agents.NewCharacterBasedEstimator(4.0)
tokens, _ := estimator.EstimateTokens(ctx, "Hello world")

// Integrate with Executor
executor := agents.NewExecutor(agent, model,
    agents.WithTokenEstimator(estimator),
    agents.WithEventBus(eventBus),
)
```

**Features**:
- Estimates tokens for single text strings or full message arrays
- Accounts for tool calls, role tokens, and message overhead
- Published via LLM request events for cost tracking
- Optional - defaults to nil (no estimation)

### Context Management (`context/`)

**Content-Addressed Blocks**: SHA-256 hashing for deduplication
- Each block: `kind` + `codec` + `payload` → `hash`
- Same content = same hash (cache-friendly)
- Deterministic ordering: `kind` → `hash`

**Block Kinds** (priority order):
1. **KindPinned**: System rules (never removed)
2. **KindReference**: Schemas, docs (reference material)
3. **KindMemory**: RAG results, KB (knowledge)
4. **KindState**: Session state
5. **KindToolOutput**: Tool results
6. **KindHistory**: Conversation (truncatable)
7. **KindTurn**: Current user message (always last)

**Token Budgeting**: Compile-time enforcement
- `ContextWindow`: Model's max tokens
- `CompletionReserve`: Tokens for response
- `OverflowStrategy`: Error | Truncate | Compact

**Context Compaction with Summarization**:
```go
// Create LLM-based summarizer
summarizer := context.NewLLMHistorySummarizer(
    model,
    estimator,
    context.WithSystemPrompt("Condense this conversation..."),
)

// Configure policy with summarization
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

// Compile with intelligent compaction
compiled, _ := builder.Compile(renderer, policy)
```

**Features**:
- LLM-based conversation summarization to compress history
- Falls back to truncation if summarizer not configured
- Redacted stubs for restricted content (preserves structure)
- Configurable compaction strategies (prune tool outputs, summarize history)

**Renderers**: Provider-specific output
- `AnthropicRenderer`: Claude (Anthropic, Bedrock, Vertex)
- `OpenAIRenderer`: GPT (OpenAI, Azure)
- `GeminiRenderer`: Gemini (Google AI)

### Knowledge Base (`kb/`)

**Indexing**:
```go
indexer, _ := kb.NewBleveIndexer("/path/to/index")
defer indexer.Close()

doc := kb.Document{
    ID:      "doc-1",
    Title:   "Sales Report",
    Content: "Q1 revenue increased 25%...",
    Tags:    []string{"sales", "2024"},
}
indexer.IndexDocument(ctx, doc)
```

**Searching**:
```go
searcher := kb.NewBleveSearcher(indexer)
results, _ := searcher.Search(ctx, "revenue growth", 5)
// Results ranked by BM25, fuzzy matching enabled
```

### Event System (`hooks/`)

**EventBus**: Pub/sub for observability
```go
bus := hooks.NewEventBus()

// Subscribe to events
bus.Subscribe(hooks.EventLLMRequest, func(e hooks.Event) error {
    log.Printf("LLM request: %+v", e)
    return nil
})

bus.Subscribe(hooks.EventToolStart, metricsHandler)
bus.Subscribe(hooks.EventLLMResponse, costTracker)
```

**Event Types**:
- Agent: `agent.start`, `agent.complete`, `agent.error`
- LLM: `llm.request`, `llm.response`, `llm.stream`
- Tool: `tool.start`, `tool.complete`, `tool.error`
- Context: `context.compile`, `context.overflow`
- Session: `session.create`, `message.save`, `interrupt`

### Domain Models (`domain/`)

**Structs** (not interfaces) with functional options:
```go
session := domain.NewSession(
    domain.WithTenantID(tenantID),
    domain.WithUserID(userID),
    domain.WithTitle("Analysis"),
)

message := types.UserMessage(
    domain.WithSessionID(sessionID),
    domain.WithRole(types.RoleUser),
    domain.WithContent("Show revenue"),
)

attachment := domain.NewAttachment(
    domain.WithAttachmentMessageID(messageID),
    domain.WithFileName("data.csv"),
    domain.WithMimeType("text/csv"),
)
```

### Built-in Tools (`tools/`)

**SQL Tools**:
- `NewSchemaListTool(executor)` - List database tables
- `NewSchemaDescribeTool(executor)` - Describe table schema
- `NewSQLExecuteTool(executor)` - Execute SQL (read-only, validated)

**Knowledge Base**:
- `NewKBSearchTool(searcher)` - Full-text search

**Utilities**:
- `NewTimeTool()` - Current date/time
- `NewExportExcelTool()` - Export to Excel
- `NewExportPDFTool(gotenbergURL, storage, WithLogger(logger))` - Generate PDF reports with file storage
- `NewChartTool()` - Generate chart data

**HITL (Human-in-the-Loop)**:
- `NewQuestionTool()` - Ask user questions, trigger interrupt

### File Storage (`storage/`)

**Purpose**: Abstraction for saving and retrieving files (PDFs, exports, attachments).

**Implementations**:
- **LocalFileStorage**: Filesystem backend with unique file names (UUID + extension)
- **NoOpFileStorage**: No-op implementation for testing

**Usage**:
```go
// Create storage backend
storage, _ := storage.NewLocalFileStorage(
    "/var/lib/bichat/exports",
    "https://example.com/exports",
)

// Save file
metadata := storage.FileMetadata{
    ContentType: "application/pdf",
    Size:        int64(len(pdfData)),
}
url, _ := storage.Save(ctx, "report.pdf", bytes.NewReader(pdfData), metadata)
// Returns: https://example.com/exports/550e8400-e29b-41d4-a716-446655440000.pdf

// Retrieve file
reader, _ := storage.Get(ctx, url)
defer reader.Close()

// Use with PDF export tool
pdfTool := tools.NewExportToPDFTool(
    "http://gotenberg:3000",
    storage,
    tools.WithLogger(logger),
)
```

**Features**:
- Automatic unique file naming (UUID + original extension)
- Size validation
- Extension preservation
- Directory auto-creation
- Extensible for S3, GCS, Azure Blob, etc.

### Observability (`logging/`, `hooks/`)

**Logging**: Structured logging interface for BiChat components.

**Implementations**:
- **StdLogger**: Logs to stdout/stderr using fmt package
- **NoOpLogger**: No-op implementation (default)

**Usage**:
```go
// Create logger
logger := logging.NewStdLogger()

// Use with components
pdfTool := tools.NewExportToPDFTool(gotenbergURL, storage,
    tools.WithLogger(logger))

fsSource := sources.NewFileSystemSource(rootDir,
    sources.WithFSLogger(logger))

// Log messages
logger.Error(ctx, "failed to close watcher", map[string]any{
    "error": err.Error(),
    "root":  "/path/to/dir",
})
```

**Metrics**: Metric collection interface for observability.

**Implementations**:
- **StdMetricsRecorder**: Logs metrics to stdout (debugging only)
- **NoOpMetricsRecorder**: No-op implementation (default)

**Usage**:
```go
// Create metrics recorder
metrics := hooks.NewStdMetricsRecorder()

// Use with async handler
asyncHandler := handlers.NewAsyncHandler(
    baseHandler,
    bufferSize,
    handlers.WithMetrics(metrics),
)

// Metrics automatically recorded:
// - bichat.async_handler.queue_depth (gauge)
// - bichat.async_handler.dropped_events (counter)
```

**Integration with Production Systems**:
```go
// Example: Custom Prometheus metrics recorder
type PrometheusMetrics struct {
    counters   map[string]*prometheus.CounterVec
    gauges     map[string]*prometheus.GaugeVec
    histograms map[string]*prometheus.HistogramVec
}

func (m *PrometheusMetrics) IncrementCounter(name string, value int64, labels map[string]string) {
    counter := m.counters[name]
    counter.With(labels).Add(float64(value))
}

// Use with BiChat
metrics := NewPrometheusMetrics()
asyncHandler := handlers.NewAsyncHandler(handler, 100, handlers.WithMetrics(metrics))
```

## Common Patterns

### HITL Flow (ask_user_question)

```go
// Agent calls ask_user_question tool
// 1. Checkpoint saved
// 2. Interrupt yielded with questions
resp, _ := processMessage(ctx, req)
if resp.Interrupt != nil {
    // Display questions to user
    for _, q := range resp.Interrupt.Questions {
        fmt.Printf("%s: %s\n", q.ID, q.Text)
    }

    // Get answers
    answers := getUserAnswers(resp.Interrupt.Questions)

    // Resume from checkpoint
    finalResp, _ := resumeWithAnswer(ctx, ResumeRequest{
        SessionID:    sessionID,
        CheckpointID: resp.Interrupt.CheckpointID,
        Answers:      answers,
    })
}
```

### Streaming Responses

```go
gen, _ := model.GenerateStream(ctx, req)
defer gen.Close()

for {
    chunk, err, hasMore := gen.Next()
    if err != nil || !hasMore {
        break
    }
    fmt.Print(chunk.Content)
}
```

### Multi-Tenant Isolation

```go
// Repository implementations MUST use tenant isolation
tenantID, _ := composables.UseTenantID(ctx)

// Include tenant_id in WHERE clauses
query := "SELECT * FROM bichat_sessions WHERE tenant_id = $1 AND id = $2"
```

### Custom Tools

```go
myTool := agents.NewTool(
    "custom_search",
    "Search custom data source",
    map[string]any{
        "type": "object",
        "properties": map[string]any{
            "query": map[string]any{"type": "string"},
        },
        "required": []string{"query"},
    },
    func(ctx context.Context, input string) (string, error) {
        params, _ := agents.ParseToolInput[CustomParams](input)
        results := customSearch(ctx, params.Query)
        return agents.FormatToolOutput(results)
    },
)
```

### Custom Codecs

```go
type MyCodec struct {
    *context.BaseCodec
}

func (c *MyCodec) Validate(payload any) error {
    // Validate structure
    return nil
}

func (c *MyCodec) Canonicalize(payload any) ([]byte, error) {
    // Stable JSON serialization
    return json.Marshal(payload)
}
```

## Design Principles

1. **Domain models are structs** - Simpler, faster
2. **Services are interfaces** - DI and testing
3. **Functional options** - Clean construction
4. **Content-addressed context** - Deterministic, cache-friendly
5. **Provider-agnostic** - Swap LLM providers easily
6. **Token budgeting at compile time** - Predictable costs
7. **Generator pattern** - Backpressure control
8. **Multi-tenant by default** - Data isolation

## Integration Points

**Module Implementation** (`modules/bichat/`):
- Implements repository interfaces from `domain/`
- Uses services for business logic
- Creates agents with tools
- Renders UI with context
- Exposes GraphQL/HTTP APIs

**LLM Providers** (external):
- Implement `agents.Model` interface
- Provider-specific clients (Anthropic, OpenAI, Gemini)
- Token counting and streaming

**Database** (PostgreSQL):
- Multi-tenant tables with `tenant_id`
- Sessions, messages, attachments
- Checkpoint storage for HITL

## When to Use What

**Agent Framework**: Building conversational agents with tools
**Context Management**: Managing token budgets, context graphs
**Knowledge Base**: Full-text search, RAG, document indexing
**Event System**: Observability, metrics, logging, cost tracking
**Domain Models**: Data structures for sessions, messages
**Tools**: Extending agent capabilities (SQL, search, export, HITL)

## Testing

```bash
# Run all tests
go test ./pkg/bichat/...

# With coverage
go test -cover ./pkg/bichat/...

# Specific package
go test -v ./pkg/bichat/agents
```

## Related Files

- `modules/bichat/CLAUDE.md` - BiChat module implementation guide
- `~/.claude/plans/purrfect-coalescing-marshmallow.md` - Full implementation plan
