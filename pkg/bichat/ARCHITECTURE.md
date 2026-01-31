# BI-Chat Architecture

A comprehensive guide to the BI-Chat foundation architecture, design decisions, and system components.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Application Layer                        │
│  (Your BI module: controllers, ViewModels, templates)          │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                       Service Layer (pkg/bichat/services)        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │ ChatService  │  │AgentService  │  │PromptService │         │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
│  ┌──────────────────────────────────────────────────────┐      │
│  │          QueryExecutorService                        │      │
│  └──────────────────────────────────────────────────────┘      │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                     Agent Framework (pkg/bichat/agents)          │
│  ┌──────────────────────────────────────────────────────┐      │
│  │  ReAct Loop Executor (Model + Tools + Generator)     │      │
│  └──────────────────────────────────────────────────────┘      │
└─────┬────────────────────┬────────────────────┬─────────────────┘
      │                    │                    │
┌─────▼──────┐  ┌──────────▼─────────┐  ┌──────▼──────────────────┐
│  Context   │  │   Knowledge Base   │  │   Event Hooks           │
│  Manager   │  │   (Bleve Index)    │  │   (Observability)       │
│            │  │                    │  │                         │
│ - Builder  │  │ - Indexer          │  │ - EventBus              │
│ - Compiler │  │ - Searcher         │  │ - Cost Tracking         │
│ - Renderer │  │ - Sources          │  │ - Logging               │
│ - Policy   │  │ - Documents        │  │ - Metrics               │
└────────────┘  └────────────────────┘  └─────────────────────────┘
      │                    │                    │
┌─────▼────────────────────▼────────────────────▼─────────────────┐
│                      Domain Layer (pkg/bichat/domain)            │
│  ┌──────────┐  ┌──────────┐  ┌──────────────┐  ┌──────────┐   │
│  │ Session  │  │ Message  │  │ Attachment   │  │Citation  │   │
│  └──────────┘  └──────────┘  └──────────────┘  └──────────┘   │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │             Repository Interfaces                        │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────┬────────────────────────────────────┘
                              │
┌─────────────────────────────▼────────────────────────────────────┐
│              Infrastructure Layer (Your Implementation)           │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  PostgreSQL Repositories (SessionRepo, MessageRepo)      │   │
│  └──────────────────────────────────────────────────────────┘   │
└───────────────────────────────────────────────────────────────────┘
```

## Component Overview

### 1. Domain Layer (`pkg/bichat/domain`)

**Purpose**: Pure business entities and contracts.

**Key Components**:
- **Session**: Chat session aggregate (tenant-scoped, user-scoped)
- **Message**: Individual messages (user, assistant, system, tool)
- **Attachment**: File attachments for messages
- **Citation**: Source references for knowledge-grounded responses
- **Repository Interfaces**: Contracts for data persistence

**Design Principles**:
- Domain models are **structs** (not interfaces) for simplicity and performance
- Immutable construction via functional options
- No external dependencies (pure business logic)
- Repository interfaces live in domain (DDD boundary)

**Example**:
```go
session := domain.NewSession(
    domain.WithTenantID(tenantID),
    domain.WithUserID(userID),
    domain.WithTitle("Q1 Analysis"),
)
```

### 2. Service Layer (`pkg/bichat/services`)

**Purpose**: Application services orchestrating business logic.

**Key Services**:

**ChatService**: Primary public API for chat functionality
- Session lifecycle management
- Message sending (blocking and streaming)
- HITL resume operations
- Title generation

**AgentService**: Framework bridge for agent interactions
- Process messages through ReAct loop
- Resume with user answers after interrupts
- Event generation for observability

**QueryExecutorService**: BI-specific SQL execution
- Schema listing and description
- Safe query execution with validation
- Timeout enforcement
- Result formatting

**PromptService**: Dynamic prompt rendering
- Tenant-specific prompt templates
- Context data injection
- Multi-language support

**Design Principles**:
- Services are **interfaces** for DI and testability
- Tenant isolation via context (`composables.UseTenantID(ctx)`)
- Error wrapping with `serrors.E(op, err)`
- Transaction coordination when needed

### 3. Agent Framework (`pkg/bichat/agents`)

**Purpose**: ReAct loop execution with LLM and tools.

**Key Components**:

**Model Interface**: LLM abstraction
- Provider-agnostic generation API
- Capability-aware (thinking, vision, JSON mode, tools)
- Streaming and blocking modes
- Token usage tracking

**Tool Interface**: Agent capabilities
- Simple string I/O pattern (JSON input/output)
- Built-in tools: `ask_user_question`, `final_answer`, `task` (sub-agent)
- Custom tools via `agents.NewTool()` or `agents.ToolFunc`
- Parameter validation via JSON Schema

**Generator Pattern**: Lazy iteration for events
- Backpressure-aware streaming
- Error propagation
- Resource cleanup

**Message Types**:
- `UserMessage`: User input
- `AssistantMessage`: LLM response
- `ToolCallMessage`: Tool invocation request
- `ToolResultMessage`: Tool execution result

**ReAct Loop Flow**:
```
1. User sends message
2. Build context (system, tools, history, user turn)
3. LLM generates response
   ├─ Text response? → Save and return
   ├─ Tool call? → Execute tool → Add result → Loop back to 3
   └─ ask_user_question? → Save checkpoint → Yield interrupt
4. User answers questions
5. Resume from checkpoint → Loop back to 3
```

### 4. Context Management (`pkg/bichat/context`)

**Purpose**: Content-addressed context building with token budgeting.

**Key Concepts**:

**Content-Addressed Blocks**:
- Each block has SHA-256 hash based on metadata + canonicalized payload
- Same content = same hash (deduplication)
- Deterministic ordering by kind → hash
- Efficient caching

**Block Kinds** (ordered in compiled context):
1. **KindPinned**: System rules (always first, never removed)
2. **KindReference**: Tool schemas, documentation (reference material)
3. **KindMemory**: Long-term memory, RAG results (knowledge base)
4. **KindState**: Current workflow/session state
5. **KindToolOutput**: Tool execution results
6. **KindHistory**: Conversation history (truncatable)
7. **KindTurn**: Current user message (always last)

**ContextBuilder**: Fluent API for constructing context
```go
builder := context.NewBuilder()
builder.
    System(systemCodec, "System rules").
    Reference(schemaCodec, dbSchemas).
    Memory(ragCodec, kbResults).
    History(historyCodec, messages).
    Turn(turnCodec, userMessage)
```

**Codec Interface**: Serialization for block types
- `Validate(payload)`: Check payload is valid
- `Canonicalize(payload)`: Convert to canonical form for hashing
- Built-in codecs: System, Tool, Query, Chart, History, KB

**Compiler**: Transforms graph to provider format
- Computes token usage
- Enforces token budget
- Handles overflow (error, truncate, compact)
- Provider-specific rendering

**Renderers**: Provider-specific output
- `AnthropicRenderer`: Claude models (Anthropic, Bedrock, Vertex)
- `OpenAIRenderer`: GPT models (OpenAI, Azure)
- `GeminiRenderer`: Gemini models (Google AI)

**ContextPolicy**: Token budget configuration
```go
policy := context.ContextPolicy{
    ContextWindow:     180000, // Model's context window
    CompletionReserve: 8000,   // Tokens reserved for output
    OverflowStrategy:  context.OverflowTruncate,
    KindPriorities:    context.DefaultKindPriorities(),
}
```

**Overflow Strategies**:
- `OverflowError`: Fail compilation if budget exceeded
- `OverflowTruncate`: Remove truncatable blocks from end
- `OverflowCompact`: Summarize history, prune old tool outputs

### 5. Knowledge Base (`pkg/bichat/kb`)

**Purpose**: Full-text search for knowledge-grounded responses.

**Key Components**:

**KBIndexer**: Build and maintain search index
- `IndexDocument()`: Add single document
- `IndexDocuments()`: Batch indexing
- `Rebuild()`: Full index rebuild from source
- `GetStats()`: Index statistics

**KBSearcher**: Query the index
- Full-text search with BM25 ranking
- Fuzzy matching
- Result highlighting
- Relevance scoring

**Document**: Indexed content unit
- `ID`: Unique identifier
- `Title`, `Content`: Searchable fields
- `Metadata`: Key-value pairs
- `Tags`: Categorical labels
- `Source`: Reference to original

**DocumentSource**: Batch document providers
- `DatabaseSource`: Index from SQL tables
- `FilesystemSource`: Index from files
- Custom sources via interface

**Bleve Implementation**:
- Pure Go full-text search (no external dependencies)
- Thread-safe concurrent access
- Efficient disk storage
- Rich query syntax

**Example**:
```go
indexer, _ := kb.NewBleveIndexer("/path/to/index")
defer indexer.Close()

doc := kb.Document{
    ID:      "doc-1",
    Title:   "Sales Report Q1 2024",
    Content: "Revenue increased by 25%...",
    Tags:    []string{"sales", "report", "2024"},
}
indexer.IndexDocument(ctx, doc)

searcher := kb.NewBleveSearcher(indexer)
results, _ := searcher.Search(ctx, "revenue growth Q1", 5)
```

### 6. Event Hooks (`pkg/bichat/hooks`)

**Purpose**: Extensibility and observability.

**Key Components**:

**EventBus**: Publish-subscribe pattern
- `Subscribe(eventType, handler)`: Register handler
- `Publish(event)`: Dispatch to handlers
- Thread-safe concurrent dispatch
- Async and sync handlers

**Event Types**:
- **Agent**: `agent.start`, `agent.complete`, `agent.error`
- **LLM**: `llm.request`, `llm.response`, `llm.stream`
- **Tool**: `tool.start`, `tool.complete`, `tool.error`
- **Context**: `context.compile`, `context.compact`, `context.overflow`
- **Session**: `session.create`, `message.save`, `interrupt`

**Built-in Handlers**:
- `LoggingHandler`: Structured logging
- `MetricsHandler`: Prometheus metrics
- `CostTrackingHandler`: Token cost aggregation
- `AsyncHandler`: Non-blocking event processing

**Example**:
```go
bus := hooks.NewEventBus()

bus.Subscribe(hooks.EventLLMRequest, func(e hooks.Event) error {
    llmEvent := e.(*hooks.LLMRequestEvent)
    log.Printf("LLM request: model=%s, tokens=%d",
        llmEvent.ModelName, llmEvent.PromptTokens)
    return nil
})

bus.Subscribe(hooks.EventLLMResponse, costTracker.TrackCost)
```

### 7. Tools (`pkg/bichat/tools`)

**Purpose**: Reusable agent capabilities.

**Built-in Tools**:

**TimeTool**: Current date/time
- Returns formatted timestamp
- Timezone-aware

**SchemaListTool**: List database tables
- Queries information_schema
- Returns table names and descriptions

**SchemaDescribeTool**: Describe table schema
- Returns columns, types, constraints
- Foreign key relationships

**SQLExecuteTool**: Execute SQL queries
- Read-only validation
- Timeout enforcement
- Result truncation
- Error handling

**KBSearchTool**: Search knowledge base
- Full-text search
- Result ranking
- Citation generation

**QuestionTool** (HITL): Ask user questions
- Triggers interrupt
- Saves checkpoint
- Waits for user answers

**ExportExcelTool**: Export data to Excel
- Tabular data formatting
- Multi-sheet support
- Auto-width columns

**ExportPDFTool**: Generate PDF reports
- Template-based rendering
- Chart embedding
- Custom styling

**ChartTool**: Generate chart data
- Bar, line, pie, scatter charts
- JSON output for frontend rendering

**Example Custom Tool**:
```go
searchTool := agents.NewTool(
    "search_products",
    "Search product catalog by name or category",
    map[string]any{
        "type": "object",
        "properties": map[string]any{
            "query": map[string]any{
                "type":        "string",
                "description": "Search query",
            },
            "category": map[string]any{
                "type":        "string",
                "description": "Product category filter (optional)",
            },
        },
        "required": []string{"query"},
    },
    func(ctx context.Context, input string) (string, error) {
        params, err := agents.ParseToolInput[SearchParams](input)
        if err != nil {
            return "", err
        }

        results := productRepo.Search(ctx, params.Query, params.Category)
        return agents.FormatToolOutput(results)
    },
)
```

## Data Flow

### User Message to Response

```
1. User Input
   │
   ├─ HTTP Request (POST /bichat/sessions/{id}/messages)
   │
2. Controller Layer
   │
   ├─ Parse request body
   ├─ Validate session ownership
   ├─ Call ChatService.SendMessage()
   │
3. ChatService
   │
   ├─ Save user message to DB
   ├─ Call AgentService.ProcessMessage()
   │
4. AgentService
   │
   ├─ Build context graph
   │   ├─ System prompt
   │   ├─ Database schemas
   │   ├─ KB search results
   │   ├─ Conversation history
   │   └─ User message
   │
   ├─ Compile context (with token budget)
   │
   ├─ Execute ReAct loop
   │   ├─ LLM Generate
   │   ├─ Tool call? → Execute → Add result → Loop
   │   ├─ HITL interrupt? → Save checkpoint → Return
   │   └─ Final answer? → Return
   │
5. ChatService
   │
   ├─ Save assistant message to DB
   ├─ Publish events to EventBus
   │
6. Controller Layer
   │
   ├─ Format response
   └─ Return HTTP response (JSON or SSE stream)
```

### Context Compilation Flow

```
1. ContextBuilder
   │
   ├─ Add blocks (System, Reference, Memory, History, Turn)
   │
2. ContextGraph
   │
   ├─ Store blocks with content-addressed hashing
   ├─ Sort by kind → hash (deterministic ordering)
   │
3. Compiler.Compile(graph, renderer, policy)
   │
   ├─ Render blocks to provider format
   │   ├─ AnthropicRenderer → {system: string, messages: [...]}
   │   ├─ OpenAIRenderer → {messages: [...]}
   │   └─ GeminiRenderer → {contents: [...]}
   │
   ├─ Count tokens (provider-specific tokenizer)
   │
   ├─ Check budget (ContextWindow - CompletionReserve)
   │
   ├─ Overflow?
   │   ├─ OverflowError → Return error
   │   ├─ OverflowTruncate → Remove truncatable blocks from end
   │   └─ OverflowCompact → Summarize history, prune tool outputs
   │
   └─ Return CompiledContext (ready for LLM API)
```

### ReAct Loop Execution

```
1. Initial Request
   │
   ├─ context = Build context graph
   ├─ compiled = Compile(context, renderer, policy)
   │
2. Loop (max MaxTurns iterations)
   │
   ├─ response = model.Generate(compiled)
   │
   ├─ response contains text?
   │   └─ Break loop, return response
   │
   ├─ response contains tool_calls?
   │   ├─ For each tool call:
   │   │   ├─ Find tool by name
   │   │   ├─ Execute: result = tool.Call(ctx, input)
   │   │   ├─ Special tools:
   │   │   │   ├─ ask_user_question → Save checkpoint, yield interrupt
   │   │   │   ├─ final_answer → Break loop, return result
   │   │   │   └─ task → Spawn sub-agent, execute, return result
   │   │   └─ Add ToolResultMessage to context
   │   │
   │   ├─ Recompile context with new tool results
   │   └─ Loop back to step 2
   │
   └─ Max turns exceeded? → Return timeout error
```

## Key Design Decisions

### 1. Domain Models as Structs

**Decision**: Use structs for domain models instead of interfaces.

**Rationale**:
- Simpler, more idiomatic Go
- Better performance (no vtable lookups)
- Easier to serialize/deserialize
- Clearer ownership semantics
- Repository and services remain interfaces for DI

**Trade-off**: Less polymorphism, but BI domain is stable and benefits from simplicity.

### 2. Content-Addressed Context Blocks

**Decision**: Hash blocks based on content, not time/sequence.

**Rationale**:
- Deduplication: Same content = same hash
- Deterministic: Same inputs = same context
- Cache-friendly: Hash as cache key
- Efficient comparison: Hash equality check

**Trade-off**: Requires canonical serialization (stable JSON, sorted keys).

### 3. Generator Pattern for Events

**Decision**: Use lazy generators instead of callbacks or channels.

**Rationale**:
- Backpressure: Consumer controls flow
- Resource safety: Defer cleanup
- Error handling: No silent drops
- Composable: Map, filter, buffer

**Trade-off**: Manual iteration (no `range` loops), but explicit control.

### 4. Provider-Agnostic Model Interface

**Decision**: Abstract LLM providers behind common interface.

**Rationale**:
- Swap providers without code changes
- Test with mock models
- Multi-provider support (fallback, A/B testing)
- Capability-based feature detection

**Trade-off**: Lowest common denominator for features.

### 5. Token Budgeting at Compile Time

**Decision**: Enforce token limits during context compilation, not runtime.

**Rationale**:
- Early failure detection
- Consistent behavior across providers
- Predictable costs
- Overflow strategies as policy

**Trade-off**: Requires accurate token counting (provider-specific).

### 6. HITL via Checkpoint/Resume

**Decision**: Interrupt execution and save checkpoint for user input.

**Rationale**:
- Stateless controllers (no long-lived connections)
- Multi-turn conversations
- User can answer at leisure
- Fault-tolerant (resume after restart)

**Trade-off**: More complex state management (checkpoints in DB).

### 7. Multi-Tenant by Default

**Decision**: All operations are tenant-scoped via context.

**Rationale**:
- Security: Data isolation
- Performance: Index partitioning
- Compliance: GDPR, data residency
- Scalability: Shard by tenant

**Trade-off**: Must pass tenant_id everywhere (enforced by `composables.UseTenantID(ctx)`).

### 8. Bleve for Knowledge Base

**Decision**: Use Bleve (pure Go) instead of Elasticsearch.

**Rationale**:
- Zero external dependencies
- Embedded (same process)
- Fast full-text search
- Easy deployment

**Trade-off**: Less scalable than distributed search (fine for single-tenant use cases).

## Extension Points

### Custom Codecs

Implement `Codec` interface for custom block types:

```go
type MyCodec struct {
    *context.BaseCodec
}

func (c *MyCodec) Validate(payload any) error {
    // Validate payload structure
}

func (c *MyCodec) Canonicalize(payload any) ([]byte, error) {
    // Convert to canonical JSON
}
```

### Custom Renderers

Implement `Renderer` interface for new LLM providers:

```go
type MyRenderer struct{}

func (r *MyRenderer) Render(blocks []context.ContextBlock) (context.RenderedContext, error) {
    // Convert blocks to provider-specific format
}

func (r *MyRenderer) CountTokens(rendered context.RenderedContext) (int, error) {
    // Count tokens using provider's tokenizer
}
```

### Custom Tools

Extend agent capabilities with custom tools:

```go
tool := agents.NewTool(
    "my_tool",
    "Description for LLM",
    parameterSchema,
    func(ctx context.Context, input string) (string, error) {
        // Tool logic
    },
)
```

### Custom Event Handlers

Process events for custom integrations:

```go
bus.Subscribe(hooks.EventLLMRequest, func(e hooks.Event) error {
    // Custom handling (logging, metrics, alerts)
})
```

### Custom Document Sources

Index from custom data sources:

```go
type MySource struct{}

func (s *MySource) FetchDocuments(ctx context.Context) ([]kb.Document, error) {
    // Fetch from API, DB, filesystem, etc.
}

indexer.Rebuild(ctx, mySource)
```

## Performance Considerations

### Token Budget Overhead

- Context compilation is O(n) in number of blocks
- Token counting is provider-specific (some require API calls)
- Cache compiled contexts when possible (hash as key)

### Knowledge Base Scaling

- Bleve index grows with document count
- Full rebuild is O(n) in documents
- Incremental updates are O(1)
- Search is O(log n) with BM25 ranking

### Event Bus Performance

- Synchronous handlers block event loop
- Use `AsyncHandler` for slow operations (DB writes, API calls)
- Event bus is thread-safe (mutex-protected)

### Database Queries

- Session/message queries MUST include tenant_id (indexed)
- Paginate history queries (default 50 messages)
- Use prepared statements for repeated queries
- Index on (tenant_id, user_id, created_at) for list queries

### Memory Management

- Close generators to release resources
- Close indexer when done
- Clear builder between requests (reuse instance)
- Stream large responses (don't buffer in memory)

## Security Considerations

### Multi-Tenant Isolation

- ALWAYS include tenant_id in WHERE clauses
- Validate tenant ownership before operations
- Use `composables.UseTenantID(ctx)` for consistency

### SQL Injection Prevention

- QueryExecutor validates queries (read-only, no DDL/DML)
- Use parameterized queries ($1, $2, never concatenation)
- Timeout enforcement (max 30 seconds)
- Result size limits (max 10k rows)

### Sensitive Data

- Mark blocks with `SensitivityLevel` (Public, Internal, Confidential, Secret)
- Redact secrets in logs and events
- Encrypt attachments at rest
- Sanitize error messages (no SQL details to users)

### Rate Limiting

- Implement token rate limiting per tenant
- Limit concurrent sessions per user
- Throttle LLM API calls (provider limits)
- Prevent abuse (max message length, attachment size)

## Testing Strategy

### Unit Tests

- Domain models: Construction, validation, immutability
- Codecs: Canonicalization, hashing, validation
- Tools: Parameter parsing, execution, error handling
- Generators: Iteration, error propagation, cleanup

### Integration Tests

- Context compilation: Token counting, overflow handling
- Knowledge base: Indexing, searching, relevance
- Event bus: Publishing, dispatching, handler execution
- ReAct loop: Tool calls, interrupts, final answers

### End-to-End Tests

- Full chat flow: User message → agent → response
- HITL flow: Interrupt → user answers → resume
- Streaming: Chunk delivery, backpressure
- Multi-agent: Parent → child → result

### Performance Tests

- Token budget with large history (1000+ messages)
- Knowledge base search (10k+ documents)
- Concurrent sessions (100+ users)
- Event bus throughput (1000+ events/sec)

## Further Reading

- **Getting Started Guide**: [GETTING_STARTED.md](./GETTING_STARTED.md)
- **Usage Examples**: [EXAMPLES.md](./EXAMPLES.md)
- **Migration Guide**: [MIGRATION.md](./MIGRATION.md)
- **API Reference**: `go doc github.com/iota-uz/iota-sdk/pkg/bichat`
