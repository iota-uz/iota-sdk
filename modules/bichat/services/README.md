# BiChat Services Layer

This directory contains the service layer implementations for the BiChat module. Services bridge the domain layer (entities, business logic) with the Agent Framework (execution, tools, context management).

## Architecture

```
modules/bichat/services/
├── agent_service_impl.go       # Production AgentService implementation
├── agent_service_impl_test.go  # Comprehensive tests
├── agent_service_stub.go        # Stub implementation (deprecated, use agent_service_impl.go)
└── README.md                    # This file
```

**Key Interfaces:**
- `pkg/bichat/services/agent_service.go` - AgentService interface definition
- `pkg/bichat/services/chat_service.go` - ChatService interface definition (TODO)

## AgentService Implementation

The `AgentService` is the bridge between the HTTP layer and the Agent Framework. It orchestrates:

1. **Context Building** - Constructs conversation context from messages
2. **Agent Execution** - Runs the agent's ReAct loop with tools
3. **Event Streaming** - Returns streaming events (content, tools, interrupts, completion)
4. **HITL Support** - Handles human-in-the-loop interrupts via checkpoints

### Usage

```go
// Create service
service := services.NewAgentService(services.AgentServiceConfig{
    Agent:        baseAgent,
    Model:        model,
    Policy:       policy,
    Renderer:     renderer,
    Checkpointer: checkpointer,
    EventBus:     eventBus, // Optional
})

// Process a message
gen, err := service.ProcessMessage(ctx, sessionID, content, attachments)
if err != nil {
    return err
}
defer gen.Close()

// Stream events
for {
    event, err, hasMore := gen.Next()
    if err != nil {
        return err
    }
    if !hasMore {
        break
    }

    switch event.Type {
    case services.EventTypeContent:
        fmt.Print(event.Content)
    case services.EventTypeToolStart:
        fmt.Printf("Tool: %s\n", event.Tool.Name)
    case services.EventTypeInterrupt:
        // Handle HITL
        answers := askUser(event.Interrupt.Questions)
        resumeGen, _ := service.ResumeWithAnswer(ctx, sessionID, event.Interrupt.CheckpointID, answers)
        // Continue streaming from resumeGen
    case services.EventTypeDone:
        fmt.Printf("Tokens used: %d\n", event.Usage.TotalTokens)
    }
}
```

### Event Types

- **EventTypeContent** - Streaming text chunks from LLM
- **EventTypeCitation** - Source citations (e.g., web search results)
- **EventTypeUsage** - Token usage for cost tracking
- **EventTypeToolStart** - Tool execution started
- **EventTypeToolEnd** - Tool execution completed
- **EventTypeInterrupt** - HITL interrupt (needs user input)
- **EventTypeDone** - Execution complete
- **EventTypeError** - Error during execution

### Dependencies

**Agent Framework** (`pkg/bichat/agents/`):
- `agents.ExtendedAgent` - Agent interface with tools and system prompt
- `agents.Model` - LLM model interface for generation
- `agents.Executor` - ReAct loop executor
- `agents.Checkpointer` - Checkpoint persistence for HITL

**Context Management** (`pkg/bichat/context/`):
- `context.Builder` - Builds context graphs from messages
- `context.Policy` - Token budget and overflow strategies
- `context.Renderer` - Converts context to provider-specific format

**Observability** (`pkg/bichat/hooks/`):
- `hooks.EventBus` - Publishes events for monitoring and analytics

### Implementation Details

#### Context Building

Current implementation uses a simplified context builder (`compileSimpleContext`). Future versions will use the full `context.Builder` with codecs:

```go
// Future implementation (when codecs are available)
builder := context.NewBuilder()
builder.System(systemCodec, agent.SystemPrompt(ctx))
builder.History(historyCodec, sessionMessages)
builder.Turn(turnCodec, userMessage, attachments)

compiled, err := builder.Compile(renderer, policy)
```

#### Event Conversion

The service layer converts `agents.ExecutorEvent` to `services.Event` via `convertExecutorEvent()`. This decouples the service layer from the agent framework's internal event structure.

#### Generator Pattern

Uses the Generator pattern for lazy, streaming iteration:

```go
type Generator[T any] interface {
    Next() (value T, err error, hasMore bool)
    Close()
}
```

Benefits:
- **Memory Efficient** - Events are processed as they arrive
- **Cancellable** - Consumer can stop early via `Close()`
- **Error Handling** - Errors are propagated through the generator

#### Multi-Tenant Isolation

All operations validate `tenant_id` via `composables.UseTenantID(ctx)`:

```go
tenantID, err := composables.UseTenantID(ctx)
if err != nil {
    return nil, serrors.E(op, err)
}
```

This ensures data isolation in multi-tenant environments.

### Testing

Comprehensive test coverage in `agent_service_impl_test.go`:

- **Unit Tests** - Test each method in isolation with mocks
- **Event Conversion Tests** - Verify all event types convert correctly
- **Error Cases** - Test validation errors, missing context, etc.
- **Generator Tests** - Test streaming and cleanup behavior

Run tests:
```bash
go test -v ./modules/bichat/services -run ^Test -count=1
```

### TODOs

1. **Session Repository Integration** - Load message history from database
2. **Full Context Builder** - Use `context.Builder` with codecs when available
3. **Enhanced HITL** - Support multi-question interrupts with structured answers
4. **Citation Support** - Parse and expose citations from agent responses
5. **Usage Tracking** - Persist token usage for billing/analytics

## Migration from Stub

The stub implementation (`agent_service_stub.go`) is now **deprecated**. To migrate:

1. Replace `NewAgentServiceStub()` with `NewAgentService(config)`
2. Provide required dependencies (Agent, Model, Policy, Renderer, Checkpointer)
3. Update tests to use the real implementation with mocks

Example:

```go
// Before (stub)
service := services.NewAgentServiceStub()

// After (production)
service := services.NewAgentService(services.AgentServiceConfig{
    Agent:        agent,
    Model:        model,
    Policy:       policy,
    Renderer:     renderer,
    Checkpointer: checkpointer,
})
```

## Related Documentation

- [Agent Framework](../../../pkg/bichat/agents/README.md) - Core agent execution
- [Context Management](../../../pkg/bichat/context/README.md) - Context building and compilation
- [BiChat Domain](../domain/README.md) - Domain entities and value objects
- [BiChat Plan](../../../docs/bichat/plan.md) - Overall architecture and phases
