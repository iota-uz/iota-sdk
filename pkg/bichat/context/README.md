# Context Management

Content-addressed context management for BI-Chat agents. A Go port of [@diyor28/context](https://github.com/diyor28/context) with improvements for BI use cases.

## Features

- **Content-addressed blocks** with SHA-256 hashing for deduplication
- **Immutable context graphs** with deterministic ordering
- **Provider-agnostic rendering** for Anthropic, OpenAI, Gemini
- **Token budget enforcement** with overflow handling (truncate, compact, error)
- **Early validation** in builder for fail-fast behavior
- **Query DSL** for powerful block filtering
- **BI-specific codecs** for schemas, queries, charts, KB results

## Quick Start

```go
import (
    "github.com/iota-uz/iota-sdk/pkg/bichat/context"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context/codecs"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context/renderers"
)

// Create codecs
systemCodec := codecs.NewSystemRulesCodec()
schemaCodec := codecs.NewDatabaseSchemaCodec()

// Build context
builder := context.NewBuilder()
builder.
    System(systemCodec, codecs.SystemRulesPayload{
        Text: "You are a helpful BI assistant.",
    }).
    Reference(schemaCodec, dbSchemas).
    History(historyCodec, messages).
    Turn(turnCodec, userMessage)

// Compile for provider
renderer := renderers.NewAnthropicRenderer()
policy := context.DefaultPolicy()
compiled, err := builder.Compile(renderer, policy)

// Use with LLM
response, err := llm.Generate(compiled.SystemPrompt, compiled.Messages)
```

## Block Kinds

Blocks are ordered by kind in compiled context:

| Kind | Description | Position |
|------|-------------|----------|
| `KindPinned` | System rules, instructions | First |
| `KindReference` | Tool schemas, docs | After pinned |
| `KindMemory` | RAG results, long-term memory | After reference |
| `KindState` | Workflow/session state | After memory |
| `KindToolOutput` | Tool execution results | After state |
| `KindHistory` | Conversation history | After tool outputs |
| `KindTurn` | Current user message | Last |

## Token Budgeting

Configure token limits and overflow handling:

```go
policy := context.ContextPolicy{
    ContextWindow:     180000, // Claude 3.5 Sonnet
    CompletionReserve: 8000,
    OverflowStrategy:  context.OverflowTruncate,
    KindPriorities:    context.DefaultKindPriorities(),
}
```

**Overflow strategies:**
- `OverflowError`: Return error when budget exceeded
- `OverflowTruncate`: Remove truncatable blocks from end
- `OverflowCompact`: Summarize history, prune old tool outputs

## Provider Renderers

Built-in renderers for major providers:

```go
// Anthropic Claude (also works with Bedrock, Vertex AI)
anthropic := renderers.NewAnthropicRenderer()

// OpenAI GPT (also works with Azure OpenAI)
openai := renderers.NewOpenAIRenderer()

// Google Gemini
gemini := renderers.NewGeminiRenderer()

// Custom tokenizer (optional)
anthropic := renderers.NewAnthropicRenderer(
    renderers.WithAnthropicTokenizer(myTokenizer),
)
```

## BI-Specific Codecs

### Database Schema

```go
schemaCodec := codecs.NewDatabaseSchemaCodec()
builder.Reference(schemaCodec, codecs.DatabaseSchemaPayload{
    SchemaName: "public",
    Tables: []codecs.TableSchema{
        {
            Name: "users",
            Columns: []codecs.TableColumn{
                {Name: "id", Type: "integer", Nullable: false},
                {Name: "email", Type: "text", Nullable: false},
            },
        },
    },
})
```

### Query Results

```go
// Auto-truncates to max 100 rows
queryCodec := codecs.NewQueryResultCodec(codecs.WithMaxRows(100))
builder.ToolOutput(queryCodec, codecs.QueryResultPayload{
    Query:   "SELECT * FROM users",
    Columns: []string{"id", "name", "email"},
    Rows:    queryRows,
})
```

### Chart Data

```go
chartCodec := codecs.NewChartDataCodec()
builder.ToolOutput(chartCodec, codecs.ChartDataPayload{
    ChartType: "bar",
    Title:     "Sales by Region",
    Data:      chartData,
})
```

### Knowledge Base Results

```go
kbCodec := codecs.NewKBSearchResultsCodec()
builder.Memory(kbCodec, codecs.KBSearchResultsPayload{
    Query: "user authentication",
    Results: kbResults,
})
```

## Query DSL

Filter blocks with powerful query DSL:

```go
// Find all public history blocks
blocks := graph.Select(
    context.Kind(context.KindHistory).
        And(context.Sensitivity(context.SensitivityPublic)),
)

// Find blocks with specific tag
recentBlocks := graph.Select(context.HasTag("recent"))

// Complex query
importantBlocks := graph.Select(
    context.Kind(context.KindReference).
        And(context.HasTag("important")).
        Or(context.Kind(context.KindPinned)),
)
```

## Custom Codecs

Implement the `Codec` interface for custom block types:

```go
type MyCodec struct {
    *context.BaseCodec
}

func NewMyCodec() *MyCodec {
    return &MyCodec{
        BaseCodec: context.NewBaseCodec("my-codec", "1.0.0"),
    }
}

func (c *MyCodec) Validate(payload any) error {
    // Validate payload structure
    return nil
}

func (c *MyCodec) Canonicalize(payload any) ([]byte, error) {
    // Convert to canonical form for deterministic hashing
    return context.SortedJSONBytes(payload)
}
```

## Sensitivity Levels

Control content filtering with sensitivity levels:

| Level | Description |
|-------|-------------|
| `SensitivityPublic` | Safe to fork to any model |
| `SensitivityInternal` | Contains business logic/PII |
| `SensitivityRestricted` | Contains credentials/secrets |

Filter by sensitivity:

```go
policy := context.ContextPolicy{
    MaxSensitivity:   context.SensitivityPublic,
    RedactRestricted: true,
}
```

## Thread Safety

- `ContextGraph`: Thread-safe for concurrent reads and writes
- `ContextBuilder`: Not thread-safe, use from single goroutine
- `Renderer`: Stateless, safe for concurrent use

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                   ContextBuilder                        │
│  (Fluent API for building context)                      │
└───────────────────┬─────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────┐
│                   ContextGraph                          │
│  (Immutable storage with deterministic ordering)        │
└───────────────────┬─────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────┐
│                   Compiler                              │
│  (Token budgeting + overflow handling)                  │
└───────────────────┬─────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────┐
│                   Renderer                              │
│  (Provider-specific format + token estimation)          │
└─────────────────────────────────────────────────────────┘
```

## Design Inspirations

- Original TypeScript implementation: [@diyor28/context](https://github.com/diyor28/context)
- Anthropic context engineering: [Link](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents)
- Shyona prompt handling: `shy-trucks/core/modules/shyona/services/prompt_service.go`

## Testing

See `example_test.go` for comprehensive examples.

Run tests:
```bash
go test ./pkg/bichat/context/...
```
