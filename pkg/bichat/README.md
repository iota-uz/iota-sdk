# BI-Chat Foundation

A foundational library for building BI-chat agent modules in iota-sdk.

## Quick Links

- **[Getting Started Guide](./GETTING_STARTED.md)** - 5-minute quick start with installation and basic examples
- **[Architecture Guide](./ARCHITECTURE.md)** - High-level architecture, design decisions, and system components
- **[Examples](./EXAMPLES.md)** - Complete, runnable examples for common use cases
- **[Migration Guide](./MIGRATION.md)** - Step-by-step migration from Ali/Shyona to BI-Chat
- **[API Reference](https://pkg.go.dev/github.com/iota-uz/iota-sdk/pkg/bichat)** - Complete API documentation

## What's in the Box

BI-Chat provides a complete foundation for building intelligent BI chat agents:

### Core Components

**Domain Layer** (`pkg/bichat/domain`)
- Session, Message, Attachment, Citation models (structs, not interfaces)
- Repository interfaces for data persistence
- Clean DDD boundaries

**Service Layer** (`pkg/bichat/services`)
- `ChatService`: Session management, message sending, streaming
- `AgentService`: Framework bridge for agent interactions
- `QueryExecutorService`: Safe SQL execution with validation
- `PromptService`: Dynamic prompt rendering

**Agent Framework** (`pkg/bichat/agents`)
- Provider-agnostic `Model` interface (Anthropic, OpenAI, Gemini)
- ReAct loop execution with tool calling
- Built-in tools: `ask_user_question`, `final_answer`, `task` (sub-agent)
- Custom tool support via simple `Tool` interface
- Generator pattern for lazy event streaming

**Context Management** (`pkg/bichat/context`)
- Content-addressed blocks with SHA-256 hashing
- Token budgeting with overflow strategies
- Provider-specific renderers (Anthropic, OpenAI, Gemini)
- Query DSL for block filtering
- BI-specific codecs (schemas, queries, charts, KB results)

**Knowledge Base** (`pkg/bichat/kb`)
- Bleve full-text search (pure Go, no external dependencies)
- Document indexing from multiple sources (filesystem, database, custom)
- BM25 ranking with fuzzy matching
- Thread-safe concurrent access

**Event Hooks** (`pkg/bichat/hooks`)
- Pub/sub event bus for observability
- Built-in handlers: logging, metrics, cost tracking
- Custom handler support
- Async and sync event processing

**Common Tools** (`pkg/bichat/tools`)
- Time, schema list/describe, SQL execute
- KB search, chart generation
- Excel/PDF export
- HITL question tool

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        Application Layer                        │
│  (Your BI module: controllers, ViewModels, templates)          │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                       Service Layer                              │
│  ChatService | AgentService | QueryExecutor | PromptService     │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                     Agent Framework                              │
│  ReAct Loop | Model Interface | Tools | Generator               │
└─────┬────────────────────┬────────────────────┬─────────────────┘
      │                    │                    │
┌─────▼──────┐  ┌──────────▼─────────┐  ┌──────▼──────────────────┐
│  Context   │  │   Knowledge Base   │  │   Event Hooks           │
│  Manager   │  │   (Bleve Index)    │  │   (Observability)       │
└────────────┘  └────────────────────┘  └─────────────────────────┘
      │                    │                    │
┌─────▼────────────────────▼────────────────────▼─────────────────┐
│                      Domain Layer                                │
│  Session | Message | Attachment | Citation | Repository Ifaces  │
└──────────────────────────────────────────────────────────────────┘
```

See [ARCHITECTURE.md](./ARCHITECTURE.md) for detailed architecture documentation.

## Quick Example

```go
package main

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "github.com/iota-uz/iota-sdk/pkg/bichat/agents"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context"
    "github.com/iota-uz/iota-sdk/pkg/bichat/domain"
    "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
)

func main() {
    ctx := context.Background()

    // Create model
    model := anthropic.NewModel(client, anthropic.ModelConfig{
        Name:      "claude-3-5-sonnet-20241022",
        MaxTokens: 4096,
    })

    // Create session
    session := domain.NewSession(
        domain.WithTenantID(uuid.New()),
        domain.WithUserID(123),
        domain.WithTitle("My Chat"),
    )

    // Build context
    builder := context.NewBuilder()
    builder.System(systemCodec, "You are a helpful assistant.")
    builder.Turn(turnCodec, "What time is it?")

    // Compile and generate
    compiled, _ := builder.Compile(renderer, policy)
    resp, _ := model.Generate(ctx, agents.Request{
        Messages: compiled.Messages,
        Tools:    []agents.Tool{tools.NewTimeTool()},
    })

    fmt.Println(resp.Message.Content)
}
```

See [GETTING_STARTED.md](./GETTING_STARTED.md) for complete examples.

## Domain Layer

Domain models are **structs** (not interfaces) following idiomatic Go patterns.

### Session

```go
session := domain.NewSession(
    domain.WithTenantID(tenantID),
    domain.WithUserID(userID),
    domain.WithTitle("Q1 Revenue Analysis"),
)
```

### Message

```go
message := domain.NewMessage(
    domain.WithSessionID(sessionID),
    domain.WithRole(domain.RoleUser),
    domain.WithContent("Show me revenue for Q1"),
)
```

### Attachment

```go
attachment := domain.NewAttachment(
    domain.WithAttachmentMessageID(messageID),
    domain.WithFileName("chart.png"),
    domain.WithMimeType("image/png"),
    domain.WithSizeBytes(int64(len(data))),
)
```

## Service Layer

Services are **interfaces** for dependency injection and testability.

### ChatService

Primary public API for chat functionality:

```go
type ChatService interface {
    CreateSession(ctx context.Context, tenantID uuid.UUID, userID int64, title string) (*domain.Session, error)
    SendMessage(ctx context.Context, req SendMessageRequest) (*SendMessageResponse, error)
    SendMessageStream(ctx context.Context, req SendMessageRequest, onChunk func(StreamChunk)) error
    ResumeWithAnswer(ctx context.Context, req ResumeRequest) (*SendMessageResponse, error)
    // ... more methods
}
```

### AgentService

Framework bridge for agent interactions:

```go
type AgentService interface {
    ProcessMessage(ctx context.Context, sessionID uuid.UUID, content string, attachments []domain.Attachment) (Generator[Event], error)
    ResumeWithAnswer(ctx context.Context, sessionID uuid.UUID, checkpointID string, answers map[string]string) (Generator[Event], error)
}
```

### QueryExecutorService

BI-specific SQL execution:

```go
type QueryExecutorService interface {
    SchemaList(ctx context.Context) ([]TableInfo, error)
    SchemaDescribe(ctx context.Context, tableName string) (*TableSchema, error)
    ExecuteQuery(ctx context.Context, sql string, params []any, timeoutMs int) (*QueryResult, error)
    ValidateQuery(sql string) error
}
```

### PromptService

Dynamic prompt rendering:

```go
type PromptService interface {
    GetPromptData(ctx context.Context) (*PromptData, error)
    RenderPrompt(ctx context.Context, templateName string, data *PromptData) (string, error)
}
```

## Generator Pattern

Lazy iteration pattern for streaming results:

```go
gen, err := agentService.ProcessMessage(ctx, sessionID, content, attachments)
if err != nil {
    return err
}
defer gen.Close()

for {
    event, err, hasMore := gen.Next()
    if err != nil {
        return err
    }
    if !hasMore {
        break
    }
    handleEvent(event)
}
```

## Streaming Pattern

Real-time response streaming:

```go
err := chatService.SendMessageStream(ctx, req, func(chunk StreamChunk) {
    switch chunk.Type {
    case ChunkTypeContent:
        fmt.Print(chunk.Content)
    case ChunkTypeCitation:
        fmt.Printf("\n[Source: %s]", chunk.Citation.Source)
    case ChunkTypeUsage:
        fmt.Printf("\nTokens: %d", chunk.Usage.TotalTokens)
    }
})
```

## HITL (Human-in-the-Loop) Pattern

Agent can ask questions and wait for user answers:

```go
// Send message
resp, err := chatService.SendMessage(ctx, req)
if err != nil {
    return err
}

// Check for interrupt
if resp.Interrupt != nil {
    // Display questions to user
    for _, q := range resp.Interrupt.Questions {
        fmt.Printf("%s: %s\n", q.ID, q.Text)
    }

    // Get user answers
    answers := getUserAnswers(resp.Interrupt.Questions)

    // Resume with answers
    resumeReq := ResumeRequest{
        SessionID:    sessionID,
        CheckpointID: resp.Interrupt.CheckpointID,
        Answers:      answers,
    }
    finalResp, err := chatService.ResumeWithAnswer(ctx, resumeReq)
}
```

## Multi-Tenant Support

All operations are tenant-scoped:

```go
// Repository implementations MUST use tenant isolation
tenantID, err := composables.UseTenantID(ctx)
if err != nil {
    return err
}

// Include tenant_id in WHERE clauses
query := "SELECT * FROM bichat_sessions WHERE tenant_id = $1 AND id = $2"
```

## Design Principles

1. **Domain models are structs** - Simpler, more performant
2. **Repository and services are interfaces** - Testability, DI
3. **Functional options pattern** - Clean, extensible construction
4. **UUID for IDs** - Distributed-system friendly
5. **time.Time for timestamps** - Standard library
6. **Comprehensive godoc** - API documentation ready
7. **Content-addressed context** - Deterministic, cache-friendly
8. **Provider-agnostic abstractions** - Swap LLM providers easily
9. **Token budgeting at compile time** - Predictable costs
10. **Generator pattern for streaming** - Backpressure control

## Key Features

### Content-Addressed Context Blocks
- Each block has SHA-256 hash based on content
- Deterministic ordering (kind → hash)
- Efficient deduplication and caching

### Provider-Agnostic Model Interface
- Swap Anthropic, OpenAI, Gemini without code changes
- Capability detection (thinking, vision, JSON mode, tools)
- Unified streaming and blocking APIs

### Token Budgeting
- Compile-time enforcement with overflow strategies
- Provider-specific token counting
- Configurable context window and completion reserve

### Knowledge Base Integration
- Bleve full-text search (pure Go, embedded)
- Document indexing from multiple sources
- BM25 ranking with fuzzy matching

### Event System
- Pub/sub for observability
- Built-in handlers: logging, metrics, cost tracking
- Custom handler support

### Multi-Agent Orchestration
- Parent agent delegates to child agents via `task` tool
- Checkpoint/resume for HITL
- Generator pattern for event streaming

## Common Use Cases

See [EXAMPLES.md](./EXAMPLES.md) for complete code examples:

1. **Simple Chat Agent** - Basic conversational agent with tools
2. **BI Agent with SQL Tools** - Database querying and analysis
3. **Multi-Agent Orchestration** - Parent/child agent delegation
4. **HITL with Interrupts** - Agent asks questions, waits for answers
5. **Custom Tools and Codecs** - Extend with custom capabilities
6. **Knowledge Base Integration** - Index and search documents

## Implementation Status

- ✅ Phase 1: Agent Framework (Complete)
- ✅ Phase 2: Hooks & Events (Complete)
- ✅ Phase 3: Context Management (Complete)
- ✅ Phase 4: Knowledge Base (Complete)
- ✅ Phase 5: Domain & Services (Complete)
- ✅ Phase 6: Common Tools (Complete)
- 📋 Phase 7: Ready-to-Use Module (Planned)
- 📋 Phase 8: React UI (Planned)
- 📋 Phase 9: Interop Layer (Planned)

## Contributing

Contributions are welcome! Please follow these guidelines:

1. **Read the documentation** - Start with [ARCHITECTURE.md](./ARCHITECTURE.md)
2. **Write tests** - All new code must have unit tests
3. **Follow patterns** - Match existing code style and patterns
4. **Update docs** - Keep documentation in sync with code
5. **Run checks** - `go vet ./...` and `go test ./...` must pass

### Development Setup

```bash
# Clone repository
git clone https://github.com/iota-uz/iota-sdk.git
cd iota-sdk/pkg/bichat

# Install dependencies
go mod download

# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Generate documentation
go doc -all > API.txt
```

### Code Style

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` and `goimports`
- Comprehensive godoc comments
- Interfaces for services, structs for domain
- Functional options for construction
- Error wrapping with `serrors` package

## License

IOTA SDK is licensed under the MIT License. See [LICENSE](../../LICENSE) for details.

## Support

- **GitHub Issues**: https://github.com/iota-uz/iota-sdk/issues
- **Documentation**: https://iota-uz.github.io/iota-sdk/
- **API Reference**: https://pkg.go.dev/github.com/iota-uz/iota-sdk/pkg/bichat

## References

- **Full Plan**: `~/.claude/plans/purrfect-coalescing-marshmallow.md`
- **Domain Patterns**: Based on `eai/back/modules/ali/domain/`
- **Service Patterns**: Based on `eai/back/modules/ali/services/`
- **Context Management**: Port of `@diyor28/context` with BI improvements
