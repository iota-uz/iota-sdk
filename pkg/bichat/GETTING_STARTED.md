# Getting Started with BI-Chat

A 5-minute guide to building your first BI-chat agent with the IOTA SDK BI-Chat foundation.

## Installation

BI-Chat is part of the IOTA SDK. Import the packages you need:

```go
import (
    "github.com/iota-uz/iota-sdk/pkg/bichat/agents"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context"
    "github.com/iota-uz/iota-sdk/pkg/bichat/domain"
    "github.com/iota-uz/iota-sdk/pkg/bichat/services"
    "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
)
```

## Quick Example: Simple Chat Agent

Here's a minimal example to get you started:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/google/uuid"
    "github.com/iota-uz/iota-sdk/pkg/bichat/agents"
    "github.com/iota-uz/iota-sdk/pkg/bichat/domain"
    "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
    // Your LLM provider implementation
    "github.com/youraccount/llm-providers/anthropic"
)

func main() {
    ctx := context.Background()

    // 1. Create LLM model
    model := anthropic.NewModel(client, anthropic.ModelConfig{
        Name:      "claude-3-5-sonnet-20241022",
        MaxTokens: 4096,
    })

    // 2. Define tools
    toolRegistry := []agents.Tool{
        tools.NewTimeTool(),
        tools.NewCalculatorTool(),
    }

    // 3. Create agent configuration
    config := agents.Config{
        Model:     model,
        Tools:     toolRegistry,
        MaxTurns:  10,
        SystemPrompt: "You are a helpful assistant.",
    }

    // 4. Create session
    session := domain.NewSession(
        domain.WithTenantID(uuid.New()),
        domain.WithUserID(123),
        domain.WithTitle("My First Chat"),
    )

    // 5. Build initial context
    builder := context.NewBuilder()
    builder.System(
        context.SystemCodec,
        "You are a helpful assistant with access to tools.",
    )

    // 6. Send user message
    userMessage := agents.NewUserMessage("What time is it?")

    // Process message (ReAct loop)
    generator, err := processMessage(ctx, config, builder, userMessage)
    if err != nil {
        log.Fatal(err)
    }
    defer generator.Close()

    // 7. Handle response
    for {
        event, err, hasMore := generator.Next()
        if err != nil {
            log.Fatal(err)
        }
        if !hasMore {
            break
        }

        // Process events (tool calls, LLM responses, etc.)
        fmt.Printf("Event: %+v\n", event)
    }
}
```

## Core Concepts

### 1. Domain Models

BI-Chat uses **structs** for domain models (not interfaces):

```go
// Create a session
session := domain.NewSession(
    domain.WithTenantID(tenantID),
    domain.WithUserID(userID),
    domain.WithTitle("Revenue Analysis"),
)

// Create a message
message := domain.NewMessage(
    domain.WithSessionID(session.ID),
    domain.WithRole(domain.RoleUser),
    domain.WithContent("Show me Q1 revenue"),
)
```

### 2. Service Layer

Services are **interfaces** for dependency injection:

```go
type ChatService interface {
    CreateSession(ctx context.Context, tenantID uuid.UUID, userID int64, title string) (*domain.Session, error)
    SendMessage(ctx context.Context, req SendMessageRequest) (*SendMessageResponse, error)
    SendMessageStream(ctx context.Context, req SendMessageRequest, onChunk func(StreamChunk)) error
    // ... more methods
}
```

### 3. Agent Framework

Agents execute ReAct loops with tools:

```go
// Define a custom tool
myTool := agents.NewTool(
    "search_database",
    "Search the database for records",
    map[string]any{
        "type": "object",
        "properties": map[string]any{
            "query": map[string]any{"type": "string"},
        },
        "required": []string{"query"},
    },
    func(ctx context.Context, input string) (string, error) {
        params, err := agents.ParseToolInput[SearchParams](input)
        if err != nil {
            return "", err
        }
        // Execute search
        results := searchDatabase(ctx, params.Query)
        return agents.FormatToolOutput(results)
    },
)
```

### 4. Context Management

Build context graphs with token budgeting:

```go
builder := context.NewBuilder()
builder.
    System(systemCodec, "System rules").
    Reference(schemaCodec, dbSchemas).
    Memory(ragCodec, kbResults).
    History(historyCodec, messages).
    Turn(turnCodec, userMessage)

// Compile with token budget
renderer := renderers.NewAnthropicRenderer()
policy := context.DefaultPolicy()
compiled, err := builder.Compile(renderer, policy)
```

### 5. Knowledge Base Integration

Index and search documents:

```go
// Create indexer
indexer, err := kb.NewBleveIndexer("/path/to/index")
if err != nil {
    log.Fatal(err)
}
defer indexer.Close()

// Index documents
doc := kb.Document{
    ID:      "doc-1",
    Title:   "Product Catalog",
    Content: "Our products include...",
    Tags:    []string{"products", "catalog"},
}
indexer.IndexDocument(ctx, doc)

// Search
searcher := kb.NewBleveSearcher(indexer)
results, err := searcher.Search(ctx, "product catalog", 5)
```

## Configuration Walkthrough

### Model Setup

Choose your LLM provider and configure the model:

```go
// Anthropic Claude
model := anthropic.NewModel(client, anthropic.ModelConfig{
    Name:      "claude-3-5-sonnet-20241022",
    MaxTokens: 4096,
})

// OpenAI GPT
model := openai.NewModel(client, openai.ModelConfig{
    Name:      "gpt-4-turbo-2024-04-09",
    MaxTokens: 4096,
})

// Check capabilities
if model.HasCapability(agents.CapabilityThinking) {
    resp, _ := model.Generate(ctx, req,
        agents.WithReasoningEffort(agents.ReasoningHigh),
    )
}
```

### Tools Configuration

Register tools available to the agent:

```go
tools := []agents.Tool{
    // Built-in tools
    tools.NewTimeTool(),
    tools.NewSchemaListTool(queryExecutor),
    tools.NewSchemaDescribeTool(queryExecutor),
    tools.NewSQLExecuteTool(queryExecutor),
    tools.NewKBSearchTool(kbSearcher),

    // HITL tool for asking user questions
    tools.NewQuestionTool(),

    // Custom tools
    myCustomTool,
}
```

### Token Budget Policy

Configure context window and overflow handling:

```go
policy := context.ContextPolicy{
    ContextWindow:     180000, // Claude 3.5 Sonnet context window
    CompletionReserve: 8000,   // Reserve tokens for completion
    OverflowStrategy:  context.OverflowTruncate,
    KindPriorities:    context.DefaultKindPriorities(),
}
```

## Next Steps

**Architecture Deep Dive**: Learn about the system architecture and design decisions.
→ See [ARCHITECTURE.md](./ARCHITECTURE.md)

**Common Use Cases**: Explore examples for BI agents, HITL, multi-agent orchestration.
→ See [EXAMPLES.md](./EXAMPLES.md)

**Migrating from Ali/Shyona**: Step-by-step migration guide.
→ See [MIGRATION.md](./MIGRATION.md)

**API Reference**: Complete API documentation in godoc.
→ Run `go doc github.com/iota-uz/iota-sdk/pkg/bichat`

## Common Patterns

### Streaming Responses

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

### Human-in-the-Loop (HITL)

```go
resp, err := chatService.SendMessage(ctx, req)
if resp.Interrupt != nil {
    // Agent has questions - get user answers
    answers := getUserAnswers(resp.Interrupt.Questions)

    // Resume with answers
    finalResp, err := chatService.ResumeWithAnswer(ctx, ResumeRequest{
        SessionID:    sessionID,
        CheckpointID: resp.Interrupt.CheckpointID,
        Answers:      answers,
    })
}
```

### Event Hooks

```go
// Create event bus
bus := hooks.NewEventBus()

// Register handlers
bus.Subscribe(hooks.EventLLMRequest, loggingHandler)
bus.Subscribe(hooks.EventLLMResponse, costTrackingHandler)
bus.Subscribe(hooks.EventToolStart, metricsHandler)

// Events are published automatically during agent execution
```

## Troubleshooting

**Token Budget Exceeded**
- Increase `ContextWindow` in policy
- Use `OverflowCompact` strategy to summarize history
- Reduce number of history messages

**Tool Call Failures**
- Check tool parameter validation
- Verify tool function error handling
- Enable debug logging for tool execution

**Multi-Tenant Isolation Issues**
- Ensure `tenant_id` is in context: `composables.UseTenantID(ctx)`
- Check repository WHERE clauses include tenant_id
- Verify session/message creation includes tenant_id

## Support

- GitHub Issues: https://github.com/iota-uz/iota-sdk/issues
- Documentation: https://iota-uz.github.io/iota-sdk/
- API Reference: `go doc github.com/iota-uz/iota-sdk/pkg/bichat`
