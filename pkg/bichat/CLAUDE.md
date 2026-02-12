# BI-Chat Foundation Package

Agent framework for building LLM-powered BI chat modules.

## Package Structure

```
pkg/bichat/
├── agents/           # ReAct loop, Model interface, Generator pattern
├── context/          # Content-addressed blocks, token budgeting
├── kb/               # Bleve full-text search
├── hooks/            # EventBus, observability
├── tools/            # Built-in tools (SQL, KB, export, HITL)
├── storage/          # File storage abstraction
├── logging/          # Structured logging
└── domain/           # Session, Message, Attachment entities
```

## Core Concepts

### Agent Framework (`agents/`)

**Model Interface**: Provider-agnostic LLM abstraction
```go
type Model interface {
    Generate(ctx context.Context, req Request) (Response, error)
    GenerateStream(ctx context.Context, req Request) (Generator[Chunk], error)
    HasCapability(cap Capability) bool
}
```

**Tool Interface**: Agent capabilities
```go
tool := agents.NewTool("search_db", "Search database", schema, handler)
```

**Generator Pattern**: Lazy iteration for streaming
```go
gen, _ := model.GenerateStream(ctx, req)
defer gen.Close()  // CRITICAL: Always defer Close()

for {
    chunk, err, hasMore := gen.Next()
    if !hasMore { break }
    // process chunk...
}
```

### Context Management (`context/`)

**Content-Addressed Blocks**: SHA-256 hashing for deduplication
- Block kinds (priority order): Pinned → Reference → Memory → State → ToolOutput → History → Turn
- Deterministic ordering by `kind` then `hash`

**Token Budgeting**: Compile-time enforcement
```go
policy := context.ContextPolicy{
    ContextWindow:     180000,  // Model max tokens
    CompletionReserve: 8000,    // Response tokens
    OverflowStrategy:  context.OverflowCompact,
    Summarizer:        summarizer,  // Optional LLM summarization
}
compiled, _ := builder.Compile(renderer, policy)
```

**Block Kinds**:
1. `KindPinned` - System rules (never removed)
2. `KindReference` - Schemas, docs
3. `KindMemory` - RAG results
4. `KindState` - Session state
5. `KindToolOutput` - Tool results
6. `KindHistory` - Conversation (truncatable/compacted)
7. `KindTurn` - Current user message (always last)

### Event System (`hooks/`)

```go
bus := hooks.NewEventBus()
bus.Subscribe(hooks.EventLLMRequest, costTracker)
bus.Subscribe(hooks.EventToolStart, metricsHandler)
```

Events: `agent.start/complete/error`, `llm.request/response/stream`, `tool.start/complete`, `context.overflow`, `interrupt`

### Domain Models (`domain/`)

**Structs** (not interfaces) with functional options:
```go
session := domain.NewSession(
    domain.WithTenantID(tenantID),
    domain.WithUserID(userID),
    domain.WithTitle("Analysis"),
)
```

### Built-in Tools (`tools/`)

**SQL**: `SchemaListTool`, `SchemaDescribeTool`, `SQLExecuteTool`
**Search**: `KBSearchTool`
**Export**: `ExportExcelTool`, `ExportPDFTool(gotenbergURL, storage)`
**HITL**: `QuestionTool` (triggers interrupt for user input)
**Utility**: `TimeTool`, `ChartTool`

## Common Patterns

### HITL Flow

```go
resp, _ := executor.Execute(ctx, "Show sales")
if resp.Interrupt != nil {
    // Display questions to user
    checkpointID := resp.Interrupt.CheckpointID
    
    // Get answers, then resume
    final, _ := executor.Resume(ctx, checkpointID, answers)
}
```

### Token Estimation

```go
estimator := agents.NewTiktokenEstimator("cl100k_base")
tokens, _ := estimator.EstimateMessages(ctx, messages)
```

**Implementations**:
- `TiktokenEstimator` - Accurate, ~10-50ms per message
- `CharacterBasedEstimator` - Fast approximation, <1ms
- `NoOpTokenEstimator` - No estimation

### File Storage

```go
storage, _ := storage.NewLocalFileStorage("/var/lib/bichat", "https://cdn.example.com")
url, _ := storage.Save(ctx, "report.pdf", reader, metadata)
// Returns: https://cdn.example.com/550e8400-e29b-41d4-a716-446655440000.pdf
```

### Observability

```go
// Logging
logger := logging.NewStdLogger()
pdfTool := tools.NewExportToPDFTool(gotenbergURL, storage, tools.WithLogger(logger))

// Metrics
metrics := hooks.NewStdMetricsRecorder()
asyncHandler := handlers.NewAsyncHandler(baseHandler, bufferSize, handlers.WithMetrics(metrics))
```

## Design Principles

1. **Domain models are structs** - Simpler than interfaces
2. **Content-addressed context** - Deterministic, cache-friendly
3. **Token budgeting at compile time** - Predictable costs
4. **Generator pattern** - Backpressure control, cancellation support
5. **Multi-tenant by default** - Data isolation built-in

## Critical Gotchas

1. **Generator Close**: Always `defer gen.Close()` to prevent resource leaks
2. **Context Cancellation**: SSE streams must handle disconnect gracefully
3. **Token Budget**: ContextWindow must include CompletionReserve. Overflow triggers compaction or error.
4. **Checkpointer**: Required for HITL. Without it, `ask_user_question` fails.
5. **Multi-Tenant**: Repository queries MUST filter by `tenant_id`
6. **Overflow Strategy**:
   - `OverflowError` - Return error if over budget
   - `OverflowTruncate` - Remove history blocks
   - `OverflowCompact` - Summarize history with LLM

## Integration Points

- **Module**: `modules/bichat/` implements repository interfaces, creates agents
- **LLM Providers**: Implement `agents.Model` (OpenAI, Anthropic, etc.)
- **Database**: Multi-tenant PostgreSQL with `tenant_id` isolation
