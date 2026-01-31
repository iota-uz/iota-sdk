# Migration Guide: Ali/Shyona → BI-Chat

Step-by-step guide for migrating from the Ali/Shyona implementations to the BI-Chat foundation in IOTA SDK.

## Overview

The BI-Chat foundation is a ground-up rewrite of the Ali chat system with:
- Cleaner architecture (DDD boundaries)
- Provider-agnostic LLM interface
- Content-addressed context management
- Improved testability
- Better performance
- Standardized patterns

This guide maps old APIs to new APIs and provides migration examples.

## Breaking Changes Summary

### 1. Domain Models: Interfaces → Structs

**Old (Ali)**:
```go
// Domain models were interfaces
type Session interface {
    GetID() uuid.UUID
    GetTitle() string
    SetTitle(string)
}
```

**New (BI-Chat)**:
```go
// Domain models are structs
type Session struct {
    ID    uuid.UUID
    Title string
    // ... fields
}

// Construction via functional options
session := domain.NewSession(
    domain.WithTenantID(tenantID),
    domain.WithTitle("Analysis"),
)
```

**Migration Steps**:
1. Replace interface references with struct pointers: `Session` → `*domain.Session`
2. Replace getters with direct field access: `session.GetID()` → `session.ID`
3. Replace setters with new instance creation (immutability): `session.SetTitle()` → `session = session.WithTitle()`

### 2. Service Interfaces: Changed Signatures

**Old (Ali)**:
```go
type ChatService interface {
    SendMessage(sessionID uuid.UUID, content string) (*Message, error)
}
```

**New (BI-Chat)**:
```go
type ChatService interface {
    SendMessage(ctx context.Context, req SendMessageRequest) (*SendMessageResponse, error)
}
```

**Migration Steps**:
1. All methods now require `context.Context` first parameter
2. Request/response objects replace positional parameters
3. Response includes both user and assistant messages plus metadata

### 3. Agent Framework: Executor → Model + Tools

**Old (Shyona)**:
```go
executor := shyona.NewExecutor(llmClient, tools)
result := executor.Execute(prompt)
```

**New (BI-Chat)**:
```go
config := agents.Config{
    Model:    model,      // Provider-agnostic
    Tools:    tools,
    MaxTurns: 10,
}
generator := agent.ProcessMessage(ctx, config, userMessage)
```

**Migration Steps**:
1. Replace `LLMClient` with `agents.Model` implementation
2. Replace `Executor` with `AgentService` or direct ReAct loop
3. Use `Generator` pattern instead of blocking calls

### 4. Context Building: Manual → Builder API

**Old (Ali)**:
```go
systemPrompt := buildSystemPrompt(rules, schemas)
messages := buildMessages(history, userMessage)
```

**New (BI-Chat)**:
```go
builder := context.NewBuilder()
builder.
    System(systemCodec, rules).
    Reference(schemaCodec, schemas).
    History(historyCodec, history).
    Turn(turnCodec, userMessage)

compiled, _ := builder.Compile(renderer, policy)
```

**Migration Steps**:
1. Replace manual prompt building with `ContextBuilder`
2. Use codecs for type-safe block construction
3. Add token budgeting with `ContextPolicy`
4. Use provider-specific renderers

### 5. Knowledge Base: Custom → Bleve

**Old (Ali)**:
```go
// Custom full-text search implementation
results := kb.Search(query)
```

**New (BI-Chat)**:
```go
indexer, _ := kb.NewBleveIndexer("/path/to/index")
defer indexer.Close()

searcher := kb.NewBleveSearcher(indexer)
results, _ := searcher.Search(ctx, query, limit)
```

**Migration Steps**:
1. Replace custom search with Bleve indexer
2. Build index from document sources
3. Use `KBSearchTool` for agent integration

### 6. Streaming: Callbacks → Generator

**Old (Ali)**:
```go
chatService.StreamMessage(sessionID, content, func(chunk string) {
    fmt.Print(chunk)
})
```

**New (BI-Chat)**:
```go
chatService.SendMessageStream(ctx, req, func(chunk StreamChunk) {
    switch chunk.Type {
    case ChunkTypeContent:
        fmt.Print(chunk.Content)
    case ChunkTypeCitation:
        // Handle citation
    }
})
```

**Migration Steps**:
1. Replace simple callback with `StreamChunk` handler
2. Handle different chunk types (content, citation, usage)
3. Add error handling in callback

### 7. HITL: Not Implemented → Checkpoint/Resume

**Old (Ali)**:
```go
// HITL not implemented - agents couldn't ask questions
```

**New (BI-Chat)**:
```go
resp, _ := chatService.SendMessage(ctx, req)
if resp.Interrupt != nil {
    answers := getUserAnswers(resp.Interrupt.Questions)
    finalResp, _ := chatService.ResumeWithAnswer(ctx, ResumeRequest{
        SessionID:    sessionID,
        CheckpointID: resp.Interrupt.CheckpointID,
        Answers:      answers,
    })
}
```

**Migration Steps**:
1. Add interrupt handling in controllers
2. Save checkpoints to database
3. Implement resume endpoint

## Step-by-Step Migration Process

### Phase 1: Domain Layer Migration

**Step 1: Update Domain Models**

**Before (Ali)**:
```go
// modules/ali/domain/session/session.go
package session

import "github.com/google/uuid"

type Session interface {
    GetID() uuid.UUID
    GetTenantID() uuid.UUID
    GetUserID() int64
    GetTitle() string
    SetTitle(string)
    GetCreatedAt() time.Time
}

type sessionImpl struct {
    id        uuid.UUID
    tenantID  uuid.UUID
    userID    int64
    title     string
    createdAt time.Time
}
```

**After (BI-Chat)**:
```go
// Use pkg/bichat/domain directly
import "github.com/iota-uz/iota-sdk/pkg/bichat/domain"

// No custom types needed - use domain.Session directly
session := domain.NewSession(
    domain.WithTenantID(tenantID),
    domain.WithUserID(userID),
    domain.WithTitle("Analysis"),
)
```

**Step 2: Update Repository Interfaces**

**Before (Ali)**:
```go
type SessionRepository interface {
    Create(session Session) error
    FindByID(id uuid.UUID) (Session, error)
}
```

**After (BI-Chat)**:
```go
// Use pkg/bichat/domain repository interfaces
import "github.com/iota-uz/iota-sdk/pkg/bichat/domain"

// Repository interface already defined in domain package
var repo domain.SessionRepository
```

### Phase 2: Service Layer Migration

**Step 1: Update Service Interfaces**

**Before (Ali)**:
```go
// modules/ali/services/chat_service.go
type ChatService interface {
    SendMessage(sessionID uuid.UUID, content string) (*Message, error)
    GetHistory(sessionID uuid.UUID) ([]*Message, error)
}
```

**After (BI-Chat)**:
```go
// Use pkg/bichat/services directly
import "github.com/iota-uz/iota-sdk/pkg/bichat/services"

// Service interface already defined
var chatService services.ChatService

// Update method calls
resp, err := chatService.SendMessage(ctx, services.SendMessageRequest{
    SessionID:   sessionID,
    UserID:      userID,
    Content:     content,
    Attachments: nil,
})
```

**Step 2: Implement Service**

**Before (Ali)**:
```go
type chatServiceImpl struct {
    sessionRepo SessionRepository
    messageRepo MessageRepository
    executor    *Executor
}

func (s *chatServiceImpl) SendMessage(sessionID uuid.UUID, content string) (*Message, error) {
    // Manual context building
    systemPrompt := buildSystemPrompt()
    messages := buildMessages(sessionID)

    // Execute agent
    result := s.executor.Execute(systemPrompt, messages, content)

    // Save message
    msg := NewMessage(sessionID, RoleAssistant, result.Content)
    s.messageRepo.Create(msg)

    return msg, nil
}
```

**After (BI-Chat)**:
```go
type chatServiceImpl struct {
    sessionRepo   domain.SessionRepository
    messageRepo   domain.MessageRepository
    agentService  services.AgentService
    promptService services.PromptService
}

func (s *chatServiceImpl) SendMessage(ctx context.Context, req services.SendMessageRequest) (*services.SendMessageResponse, error) {
    op := serrors.Op("chatServiceImpl.SendMessage")

    // Save user message
    userMsg := domain.NewMessage(
        domain.WithSessionID(req.SessionID),
        domain.WithRole(domain.RoleUser),
        domain.WithContent(req.Content),
    )
    if err := s.messageRepo.Create(ctx, userMsg); err != nil {
        return nil, serrors.E(op, err)
    }

    // Process via agent
    generator, err := s.agentService.ProcessMessage(ctx, req.SessionID, req.Content, req.Attachments)
    if err != nil {
        return nil, serrors.E(op, err)
    }
    defer generator.Close()

    // Collect result
    var assistantMsg *domain.Message
    var interrupt *services.Interrupt

    for {
        event, err, hasMore := generator.Next()
        if err != nil {
            return nil, serrors.E(op, err)
        }
        if !hasMore {
            break
        }

        // Handle events (save message, citations, etc.)
        switch e := event.(type) {
        case *AgentCompleteEvent:
            assistantMsg = e.Message
        case *InterruptEvent:
            interrupt = e.Interrupt
        }
    }

    // Save assistant message
    if assistantMsg != nil {
        if err := s.messageRepo.Create(ctx, assistantMsg); err != nil {
            return nil, serrors.E(op, err)
        }
    }

    session, _ := s.sessionRepo.FindByID(ctx, req.SessionID)

    return &services.SendMessageResponse{
        UserMessage:      userMsg,
        AssistantMessage: assistantMsg,
        Session:          session,
        Interrupt:        interrupt,
    }, nil
}
```

### Phase 3: Agent Framework Migration

**Step 1: Replace LLMClient with Model**

**Before (Shyona)**:
```go
// Custom LLM client for each provider
type AnthropicClient struct {
    apiKey string
}

func (c *AnthropicClient) Complete(prompt string, messages []Message) (string, error) {
    // Provider-specific API call
}
```

**After (BI-Chat)**:
```go
// Use provider implementations of agents.Model
import "github.com/youraccount/llm-providers/anthropic"

model := anthropic.NewModel(client, anthropic.ModelConfig{
    Name:      "claude-3-5-sonnet-20241022",
    MaxTokens: 4096,
})

// Check capabilities
if model.HasCapability(agents.CapabilityThinking) {
    resp, _ := model.Generate(ctx, req,
        agents.WithReasoningEffort(agents.ReasoningHigh),
    )
}
```

**Step 2: Update Tool Definitions**

**Before (Shyona)**:
```go
type Tool struct {
    Name        string
    Description string
    Execute     func(input map[string]any) (string, error)
}

tool := Tool{
    Name:        "search_db",
    Description: "Search database",
    Execute: func(input map[string]any) (string, error) {
        query := input["query"].(string)
        return searchDB(query), nil
    },
}
```

**After (BI-Chat)**:
```go
tool := agents.NewTool(
    "search_db",
    "Search database by query",
    map[string]any{
        "type": "object",
        "properties": map[string]any{
            "query": map[string]any{
                "type":        "string",
                "description": "Search query",
            },
        },
        "required": []string{"query"},
    },
    func(ctx context.Context, input string) (string, error) {
        params, err := agents.ParseToolInput[SearchParams](input)
        if err != nil {
            return "", err
        }
        results := searchDB(ctx, params.Query)
        return agents.FormatToolOutput(results)
    },
)
```

**Step 3: Replace Executor with AgentService**

**Before (Shyona)**:
```go
executor := shyona.NewExecutor(llmClient, tools)
result := executor.Execute(systemPrompt, messages, userInput)
```

**After (BI-Chat)**:
```go
// Implement AgentService or use directly
agentService := NewAgentServiceImpl(model, tools)

generator, err := agentService.ProcessMessage(ctx, sessionID, content, attachments)
if err != nil {
    log.Fatal(err)
}
defer generator.Close()

for {
    event, err, hasMore := generator.Next()
    if err != nil {
        log.Fatal(err)
    }
    if !hasMore {
        break
    }
    handleEvent(event)
}
```

### Phase 4: Context Management Migration

**Step 1: Replace Manual Prompt Building**

**Before (Ali)**:
```go
func buildSystemPrompt(rules string, schemas []Schema) string {
    prompt := "You are a BI assistant.\n\n"
    prompt += "Rules:\n" + rules + "\n\n"
    prompt += "Schemas:\n"
    for _, schema := range schemas {
        prompt += fmt.Sprintf("- %s: %s\n", schema.Name, schema.Columns)
    }
    return prompt
}

func buildMessages(history []*Message, userMessage string) []Message {
    messages := make([]Message, 0, len(history)+1)
    for _, msg := range history {
        messages = append(messages, Message{
            Role:    msg.Role,
            Content: msg.Content,
        })
    }
    messages = append(messages, Message{
        Role:    "user",
        Content: userMessage,
    })
    return messages
}
```

**After (BI-Chat)**:
```go
import (
    "github.com/iota-uz/iota-sdk/pkg/bichat/context"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context/codecs"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context/renderers"
)

builder := context.NewBuilder()

// System rules
builder.System(
    codecs.NewSystemCodec(),
    codecs.SystemPayload{
        Content: "You are a BI assistant. Follow these rules: " + rules,
    },
)

// Database schemas
builder.Reference(
    codecs.NewDatabaseSchemaCodec(),
    codecs.DatabaseSchemaPayload{
        SchemaName: "public",
        Tables:     schemas,
    },
)

// Conversation history
builder.History(
    codecs.NewHistoryCodec(),
    codecs.HistoryPayload{
        Messages: history,
    },
)

// Current user message
builder.Turn(
    codecs.NewTurnCodec(),
    codecs.TurnPayload{
        Content: userMessage,
    },
)

// Compile with token budget
renderer := renderers.NewAnthropicRenderer()
policy := context.ContextPolicy{
    ContextWindow:     180000,
    CompletionReserve: 8000,
    OverflowStrategy:  context.OverflowTruncate,
}

compiled, err := builder.Compile(renderer, policy)
if err != nil {
    log.Fatal(err)
}

// Use compiled context with model
resp, _ := model.Generate(ctx, agents.Request{
    Messages: compiled.Messages,
})
```

**Step 2: Add Token Budgeting**

**Before (Ali)**:
```go
// No token budgeting - could exceed context window
```

**After (BI-Chat)**:
```go
policy := context.ContextPolicy{
    ContextWindow:     180000, // Claude 3.5 Sonnet
    CompletionReserve: 8000,
    OverflowStrategy:  context.OverflowCompact, // Summarize history on overflow
    KindPriorities:    context.DefaultKindPriorities(),
}

compiled, err := builder.Compile(renderer, policy)
if err != nil {
    // Handle overflow error or use compacted context
}
```

### Phase 5: Knowledge Base Migration

**Step 1: Replace Custom Search**

**Before (Ali)**:
```go
// Custom full-text search implementation
type KBService struct {
    documents map[string]Document
}

func (kb *KBService) Search(query string) []Document {
    // Simple string matching
    results := make([]Document, 0)
    for _, doc := range kb.documents {
        if strings.Contains(doc.Content, query) {
            results = append(results, doc)
        }
    }
    return results
}
```

**After (BI-Chat)**:
```go
import "github.com/iota-uz/iota-sdk/pkg/bichat/kb"

// Create Bleve indexer
indexer, err := kb.NewBleveIndexer("/path/to/index")
if err != nil {
    log.Fatal(err)
}
defer indexer.Close()

// Index documents
for _, doc := range documents {
    indexer.IndexDocument(ctx, kb.Document{
        ID:       doc.ID,
        Title:    doc.Title,
        Content:  doc.Content,
        Tags:     doc.Tags,
        Metadata: doc.Metadata,
    })
}

// Search
searcher := kb.NewBleveSearcher(indexer)
results, err := searcher.Search(ctx, query, 5)
```

**Step 2: Add KB Tool to Agent**

**Before (Ali)**:
```go
// No KB integration in agent
```

**After (BI-Chat)**:
```go
import "github.com/iota-uz/iota-sdk/pkg/bichat/tools"

tools := []agents.Tool{
    tools.NewKBSearchTool(searcher),
    // other tools...
}

// Agent can now search KB automatically
```

### Phase 6: Database Layer Migration

**Step 1: Update Repository Implementations**

**Before (Ali)**:
```go
type sessionRepositoryImpl struct {
    db *sql.DB
}

func (r *sessionRepositoryImpl) Create(session Session) error {
    query := `
        INSERT INTO ali_sessions (id, tenant_id, user_id, title, created_at)
        VALUES ($1, $2, $3, $4, $5)
    `
    _, err := r.db.Exec(query,
        session.GetID(),
        session.GetTenantID(),
        session.GetUserID(),
        session.GetTitle(),
        session.GetCreatedAt(),
    )
    return err
}
```

**After (BI-Chat)**:
```go
type sessionRepositoryImpl struct {
    db *sql.DB
}

func (r *sessionRepositoryImpl) Create(ctx context.Context, session *domain.Session) error {
    const op serrors.Op = "sessionRepositoryImpl.Create"

    tx, err := composables.UseTx(ctx)
    if err != nil {
        return serrors.E(op, err)
    }

    query := `
        INSERT INTO bichat_sessions (id, tenant_id, user_id, title, created_at)
        VALUES ($1, $2, $3, $4, $5)
    `
    _, err = tx.ExecContext(ctx, query,
        session.ID,
        session.TenantID,
        session.UserID,
        session.Title,
        session.CreatedAt,
    )
    if err != nil {
        return serrors.E(op, err)
    }

    return nil
}
```

**Step 2: Update Queries for Multi-Tenant Isolation**

**Before (Ali)**:
```go
query := `SELECT * FROM ali_sessions WHERE user_id = $1`
```

**After (BI-Chat)**:
```go
tenantID, err := composables.UseTenantID(ctx)
if err != nil {
    return nil, serrors.E(op, err)
}

query := `SELECT * FROM bichat_sessions WHERE tenant_id = $1 AND user_id = $2`
```

### Phase 7: Controller Layer Migration

**Step 1: Update HTTP Handlers**

**Before (Ali)**:
```go
func (c *ChatController) SendMessage(w http.ResponseWriter, r *http.Request) {
    sessionID := mux.Vars(r)["id"]
    content := r.FormValue("content")

    msg, err := c.chatService.SendMessage(uuid.MustParse(sessionID), content)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(msg)
}
```

**After (BI-Chat)**:
```go
func (c *ChatController) SendMessage(w http.ResponseWriter, r *http.Request) {
    const op serrors.Op = "ChatController.SendMessage"

    sessionID := uuid.MustParse(chi.URLParam(r, "id"))
    userID := composables.UseUserID(r.Context())

    type DTO struct {
        Content string `json:"content"`
    }

    dto, err := composables.UseForm(&DTO{}, r)
    if err != nil {
        htmx.Error(w, r, serrors.E(op, err))
        return
    }

    resp, err := c.chatService.SendMessage(r.Context(), services.SendMessageRequest{
        SessionID:   sessionID,
        UserID:      userID,
        Content:     dto.Content,
        Attachments: nil,
    })
    if err != nil {
        htmx.Error(w, r, serrors.E(op, err))
        return
    }

    // Handle interrupt (HITL)
    if resp.Interrupt != nil {
        // Render question form
        templates.QuestionForm(resp.Interrupt).Render(r.Context(), w)
        return
    }

    // Render assistant message
    templates.MessageRow(resp.AssistantMessage).Render(r.Context(), w)
}
```

**Step 2: Add Streaming Endpoint**

**Before (Ali)**:
```go
// No streaming support
```

**After (BI-Chat)**:
```go
func (c *ChatController) SendMessageStream(w http.ResponseWriter, r *http.Request) {
    const op serrors.Op = "ChatController.SendMessageStream"

    sessionID := uuid.MustParse(chi.URLParam(r, "id"))
    userID := composables.UseUserID(r.Context())

    type DTO struct {
        Content string `json:"content"`
    }

    dto, err := composables.UseForm(&DTO{}, r)
    if err != nil {
        htmx.Error(w, r, serrors.E(op, err))
        return
    }

    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "Streaming not supported", http.StatusInternalServerError)
        return
    }

    err = c.chatService.SendMessageStream(r.Context(),
        services.SendMessageRequest{
            SessionID:   sessionID,
            UserID:      userID,
            Content:     dto.Content,
            Attachments: nil,
        },
        func(chunk services.StreamChunk) {
            switch chunk.Type {
            case services.ChunkTypeContent:
                fmt.Fprintf(w, "data: %s\n\n", chunk.Content)
                flusher.Flush()
            case services.ChunkTypeDone:
                fmt.Fprintf(w, "event: done\ndata: \n\n")
                flusher.Flush()
            }
        },
    )
    if err != nil {
        log.Printf("Stream error: %v", err)
    }
}
```

### Phase 8: Migration Checklist

- [ ] **Phase 1: Domain Layer**
  - [ ] Replace domain interfaces with `pkg/bichat/domain` structs
  - [ ] Update repository interfaces to match `domain.SessionRepository`, `domain.MessageRepository`
  - [ ] Replace getters/setters with direct field access

- [ ] **Phase 2: Service Layer**
  - [ ] Replace service interfaces with `pkg/bichat/services` interfaces
  - [ ] Update method signatures (add `context.Context`, use request/response objects)
  - [ ] Implement services using new domain and repository interfaces

- [ ] **Phase 3: Agent Framework**
  - [ ] Replace `LLMClient` with `agents.Model` implementations
  - [ ] Update tool definitions to use `agents.Tool` interface
  - [ ] Replace `Executor` with `AgentService` or ReAct loop
  - [ ] Use `Generator` pattern for event streaming

- [ ] **Phase 4: Context Management**
  - [ ] Replace manual prompt building with `context.ContextBuilder`
  - [ ] Add token budgeting with `context.ContextPolicy`
  - [ ] Use provider-specific renderers (`AnthropicRenderer`, etc.)

- [ ] **Phase 5: Knowledge Base**
  - [ ] Replace custom search with `kb.BleveIndexer`
  - [ ] Index documents from sources
  - [ ] Add `KBSearchTool` to agent tools

- [ ] **Phase 6: Database Layer**
  - [ ] Rename tables: `ali_*` → `bichat_*`
  - [ ] Update repository implementations (use `composables.UseTx`, `composables.UseTenantID`)
  - [ ] Add tenant isolation to all queries

- [ ] **Phase 7: Controller Layer**
  - [ ] Update HTTP handlers (use `composables.UseForm`, `htmx` package)
  - [ ] Add streaming endpoints with SSE
  - [ ] Handle HITL interrupts

- [ ] **Phase 8: Testing**
  - [ ] Update unit tests (domain models, services)
  - [ ] Add integration tests (agent execution, context compilation)
  - [ ] Add E2E tests (full chat flow, streaming, HITL)

## Code Mapping Table

| Ali/Shyona | BI-Chat | Notes |
|------------|---------|-------|
| `ali.Session` (interface) | `domain.Session` (struct) | Domain models are now structs |
| `ali.Message` (interface) | `domain.Message` (struct) | Domain models are now structs |
| `ali.ChatService` | `services.ChatService` | Different method signatures |
| `shyona.Executor` | `agents.Model` + tools | Provider-agnostic model interface |
| `shyona.Tool` | `agents.Tool` | JSON Schema parameters, string I/O |
| `ali.KBService` | `kb.BleveIndexer` + `kb.BleveSearcher` | Bleve for full-text search |
| Manual prompt building | `context.ContextBuilder` | Content-addressed blocks |
| No token budgeting | `context.ContextPolicy` | Compile-time enforcement |
| No streaming | `chatService.SendMessageStream` | SSE with chunk callbacks |
| No HITL | `Interrupt` + `ResumeWithAnswer` | Checkpoint/resume pattern |
| No event system | `hooks.EventBus` | Observability and extensibility |

## FAQ

### Q: Can I use both Ali and BI-Chat in parallel during migration?

**A**: Yes. Keep Ali code in `modules/ali/` and add BI-Chat in a new module (e.g., `modules/bichat/`). Share infrastructure (DB, auth) and migrate incrementally.

### Q: Do I need to migrate my database schema?

**A**: Yes. Rename tables from `ali_*` to `bichat_*` and ensure `tenant_id` is indexed. Run migrations in parallel (read from old, write to both, switch reads, drop old).

### Q: How do I migrate custom tools?

**A**: Update tool signatures to match `agents.Tool` interface:
- Add JSON Schema for parameters
- Use string input/output (parse with `agents.ParseToolInput`, format with `agents.FormatToolOutput`)
- Add `context.Context` parameter for tenant/user context

### Q: What about custom LLM providers?

**A**: Implement `agents.Model` interface for your provider:
- `Generate(ctx, req, opts)` → `*Response`
- `Stream(ctx, req, opts)` → `Generator[Chunk]`
- `Info()` → `ModelInfo` (name, provider, capabilities)
- `HasCapability(cap)` → `bool`

### Q: How do I handle token budget overflows?

**A**: Configure `OverflowStrategy` in `ContextPolicy`:
- `OverflowError`: Fail compilation (safe, explicit)
- `OverflowTruncate`: Remove truncatable blocks from end (simple)
- `OverflowCompact`: Summarize history, prune tool outputs (sophisticated)

### Q: Can I keep my existing prompt templates?

**A**: Yes. Use `PromptService` to load templates from DB/files and inject into `System` blocks. Or use codecs to wrap existing prompt building logic.

### Q: How do I test the migration?

**A**: Start with unit tests (domain, services), then integration tests (agent execution), then E2E tests (HTTP endpoints). Run both old and new implementations in parallel with traffic mirroring.

## Support

If you encounter issues during migration:
- Check examples in [EXAMPLES.md](./EXAMPLES.md)
- Review architecture in [ARCHITECTURE.md](./ARCHITECTURE.md)
- Open GitHub issue: https://github.com/iota-uz/iota-sdk/issues
- API reference: `go doc github.com/iota-uz/iota-sdk/pkg/bichat`
