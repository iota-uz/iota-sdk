---
layout: default
title: Technical Architecture
parent: BiChat
nav_order: 2
description: "BiChat Module Technical Architecture"
---

# Technical Architecture

## Module Structure

```
modules/bichat/
├── domain/
│   ├── entities/
│   │   ├── dialogue/
│   │   │   ├── dialogue.go               # Dialogue interface
│   │   │   ├── dialogue_impl.go          # Implementation
│   │   │   ├── dialogue_repository.go    # Repository interface
│   │   │   ├── dialogue_reply.go         # Reply structure
│   │   │   ├── dialogue_events.go        # Domain events
│   │   │   ├── dialogue_start.go         # Dialogue creation
│   │   │   └── dialogue_*_test.go        # Tests
│   │   ├── llm/
│   │   │   ├── llm.go                    # LLM abstraction
│   │   │   └── message.go                # Chat message types
│   │   ├── embedding/
│   │   │   └── embedding.go              # Vector embeddings
│   │   └── prompt/
│   │       ├── prompt.go                 # Prompt interface
│   │       └── prompt_repository.go      # Repository interface
│   └── repositories/
│       ├── dialogue_repository.go        # Repository interface
│       └── prompt_repository.go
├── infrastructure/
│   ├── persistence/
│   │   ├── dialogue_repository.go        # Database implementation
│   │   ├── bichat_mappers.go             # Entity mappers
│   │   └── models/
│   │       └── models.go                 # Database models
│   ├── llmproviders/
│   │   ├── openai_provider.go            # OpenAI integration
│   │   └── mappers.go                    # Request/response mapping
│   └── cache/
│       └── cache.go                      # Caching layer
├── services/
│   ├── dialogue_service.go               # Dialogue management
│   ├── embeddings_service.go             # Vector embeddings
│   ├── prompt_service.go                 # Prompt templates
│   ├── chatfuncs/                        # Chat functions
│   └── *_service_test.go
├── presentation/
│   ├── controllers/
│   │   ├── bichat_controller.go          # HTTP handlers
│   │   └── *_controller_test.go
│   ├── templates/
│   │   └── pages/bichat/
│   │       └── bichat_templ.go           # Chat UI
│   └── dtos/
│       └── bichat_dto.go                 # Request/response DTOs
├── module.go                             # Module registration
├── links.go                              # Navigation links
└── nav_items.go                          # Navigation items
```

## Domain Model

### Dialogue Aggregate

```go
type Dialogue interface {
    ID() uint
    TenantID() uuid.UUID
    UserID() uint
    Label() string
    Messages() Messages
    LastMessage() llm.ChatCompletionMessage
    CreatedAt() time.Time
    UpdatedAt() time.Time

    // Immutable operations (return new instance)
    AddMessages(messages ...llm.ChatCompletionMessage) Dialogue
    SetMessages(messages Messages) Dialogue
    SetLastMessage(msg llm.ChatCompletionMessage) Dialogue
}
```

**Key Properties**:
- **ID**: Unique dialogue identifier
- **TenantID**: Multi-tenant isolation
- **UserID**: User who owns dialogue
- **Label**: User-provided dialogue name
- **Messages**: Full conversation history (ordered by timestamp)
- **LastMessage**: Most recent message for quick access

**Business Rules**:
- Immutable design (updates return new instance)
- Tenant isolation enforced
- Complete message history maintained
- Timestamps auto-managed

### Message Types

```go
type ChatCompletionMessage struct {
    Role    string  // "user", "assistant", "system"
    Content string  // Message text
    Name    string  // Optional name for function calls
}
```

### LLM Abstraction

```go
type ChatCompletionRequest struct {
    Model               string
    Messages            []ChatCompletionMessage
    MaxTokens           int
    Temperature         float32
    TopP                float32
    // ... other parameters
}
```

## Services

### DialogueService

**File**: `modules/bichat/services/dialogue_service.go`

**Key Methods**:

```go
type DialogueService struct {
    dialogueRepository     domain.DialogueRepository
    llmProvider           LLMProvider
    // ... other dependencies
}

// StartDialogue creates a new dialogue with initial message
func (s *DialogueService) StartDialogue(
    ctx context.Context,
    userMessage string,
    model string,
) (Dialogue, error)

// GetDialogueByID retrieves existing dialogue
func (s *DialogueService) GetDialogueByID(
    ctx context.Context,
    dialogueID uint,
) (Dialogue, error)

// AddMessage adds user message and gets AI response
func (s *DialogueService) AddMessage(
    ctx context.Context,
    dialogueID uint,
    message string,
) (Dialogue, error)

// ListUserDialogues lists all dialogues for current user
func (s *DialogueService) ListUserDialogues(
    ctx context.Context,
) ([]Dialogue, error)

// DeleteDialogue soft-deletes dialogue
func (s *DialogueService) DeleteDialogue(
    ctx context.Context,
    dialogueID uint,
) error
```

**Responsibilities**:
- Dialogue lifecycle management
- LLM request/response handling
- Message ordering and history
- Tenant isolation enforcement
- Error handling and recovery

### EmbeddingsService

**File**: `modules/bichat/services/embeddings_service.go`

Handles vector embeddings for semantic search:

```go
func (s *EmbeddingsService) GenerateEmbedding(
    ctx context.Context,
    text string,
) ([]float32, error)

func (s *EmbeddingsService) FindSimilar(
    ctx context.Context,
    embedding []float32,
    topK int,
) ([]string, error)
```

### PromptService

**File**: `modules/bichat/services/prompt_service.go`

Manages system prompts and templates:

```go
func (s *PromptService) GetSystemPrompt(
    ctx context.Context,
) (string, error)

func (s *PromptService) RenderPrompt(
    template string,
    data map[string]interface{},
) (string, error)
```

## Repositories

### DialogueRepository Interface

**File**: `modules/bichat/domain/entities/dialogue/dialogue_repository.go`

```go
type Repository interface {
    // Save persists a dialogue
    Save(ctx context.Context, dialogue Dialogue) (Dialogue, error)

    // GetByID retrieves dialogue by ID
    GetByID(ctx context.Context, id uint) (Dialogue, error)

    // GetByUserID lists dialogues for user
    GetByUserID(
        ctx context.Context,
        limit, offset int,
    ) ([]Dialogue, int, error)

    // Update updates existing dialogue
    Update(ctx context.Context, dialogue Dialogue) error

    // Delete soft-deletes dialogue
    Delete(ctx context.Context, id uint) error

    // Restore recovers soft-deleted dialogue
    Restore(ctx context.Context, id uint) error
}
```

### DialogueRepository Implementation

**File**: `modules/bichat/infrastructure/persistence/dialogue_repository.go`

**Key Features**:
- Parameterized SQL queries (no string concatenation)
- Tenant isolation via `composables.UseTenantID(ctx)`
- JSON storage for message arrays
- Efficient indexing on user_id, tenant_id, created_at
- Soft delete support with recovery

```go
func (r *DialogueRepository) Save(
    ctx context.Context,
    dialogue Dialogue,
) (Dialogue, error) {
    const op = "DialogueRepository.Save"

    tx := composables.UseTx(ctx)
    tenantID := composables.UseTenantID(ctx)

    // Parameterized query with tenant_id filter
    query := `
        INSERT INTO bichat_dialogues
        (tenant_id, user_id, label, messages, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id
    `

    // ... execution and mapping
    return dialogue, nil
}
```

## LLM Integration

### Provider Architecture

Abstract provider interface allows multiple LLM implementations:

```go
type LLMProvider interface {
    CreateChatCompletionStream(
        ctx context.Context,
        request ChatCompletionRequest,
    ) (*ChatCompletionStream, error)
}
```

### OpenAI Provider

**File**: `modules/bichat/infrastructure/llmproviders/openai_provider.go`

```go
type OpenAIProvider struct {
    client *openai.Client
}

func NewOpenAIProvider(authToken string) *OpenAIProvider {
    return &OpenAIProvider{
        client: openai.NewClient(authToken),
    }
}

func (p *OpenAIProvider) CreateChatCompletionStream(
    ctx context.Context,
    request ChatCompletionRequest,
) (*ChatCompletionStream, error) {
    // Map domain request to OpenAI format
    openaiRequest := DomainToOpenAIChatCompletionRequest(request)

    // Call OpenAI API
    return p.client.CreateChatCompletionStream(ctx, openaiRequest)
}
```

### Response Streaming

Real-time response streaming for better UX:

```go
// Controller streams responses to client
stream, err := provider.CreateChatCompletionStream(ctx, request)
if err != nil {
    return errors.E(op, err)
}
defer stream.Close()

// Stream chunks back to client
w.Header().Set("Content-Type", "text/event-stream")
w.Header().Set("Cache-Control", "no-cache")
w.Header().Set("Connection", "keep-alive")

for {
    response, err := stream.Recv()
    if errors.Is(err, io.EOF) {
        break
    }
    // Write chunk to response
    fmt.Fprintf(w, "data: %s\n\n", response.Choices[0].Delta.Content)
    w.Flush()
}
```

## Database Schema

### Dialogues Table

```sql
CREATE TABLE bichat_dialogues (
    id SERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL,
    user_id BIGINT NOT NULL,
    label VARCHAR(255),
    messages JSONB NOT NULL DEFAULT '[]',
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Indexes for performance
CREATE INDEX idx_bichat_dialogues_tenant_id ON bichat_dialogues(tenant_id);
CREATE INDEX idx_bichat_dialogues_user_id ON bichat_dialogues(user_id);
CREATE INDEX idx_bichat_dialogues_created_at ON bichat_dialogues(created_at);
CREATE INDEX idx_bichat_dialogues_deleted_at ON bichat_dialogues(deleted_at);
CREATE INDEX idx_bichat_dialogues_tenant_user ON bichat_dialogues(tenant_id, user_id);
```

### Messages Storage (JSONB)

Messages stored as JSON array within dialogue:

```json
{
  "messages": [
    {
      "role": "user",
      "content": "What is IOTA?"
    },
    {
      "role": "assistant",
      "content": "IOTA is a distributed ledger..."
    }
  ]
}
```

## Controllers

### BiChatController

**File**: `modules/bichat/presentation/controllers/bichat_controller.go`

**Routes**:
- `GET /bi-chat` - Display chat interface
- `POST /bi-chat/new` - Start new dialogue
- `DELETE /bi-chat/{id}` - Delete dialogue

```go
func (c *BiChatController) Create(w http.ResponseWriter, r *http.Request) {
    dto, err := composables.UseForm(&dtos.MessageDTO{}, r)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Start new dialogue with initial message
    _, err = c.dialogueService.StartDialogue(r.Context(), dto.Message, "gpt-4o")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    shared.Redirect(w, r, c.basePath)
}
```

## Message Flow

### Creating New Dialogue

```
User Input
    ↓
Controller.Create()
    ↓
DialogueService.StartDialogue()
    ↓
Create Dialogue aggregate with user message
    ↓
Call OpenAI API with user message
    ↓
Receive AI response
    ↓
Add AI response to dialogue messages
    ↓
DialogueRepository.Save()
    ↓
Return updated dialogue with response
    ↓
Render updated UI
```

### Adding Message to Existing Dialogue

```
User Input
    ↓
Controller.AddMessage()
    ↓
DialogueService.AddMessage()
    ↓
Load existing dialogue from repository
    ↓
Add user message to dialogue
    ↓
Prepare LLM request with full message history
    ↓
Call OpenAI API with context
    ↓
Receive AI response
    ↓
Add AI response to dialogue
    ↓
Save updated dialogue
    ↓
Stream response to client via SSE
```

## Multi-tenant Isolation

All queries enforce tenant isolation:

```go
// Repository automatically adds tenant_id filter
tenantID := composables.UseTenantID(ctx)

query := `
    SELECT * FROM bichat_dialogues
    WHERE tenant_id = $1  -- Tenant isolation
      AND user_id = $2    -- User isolation
      AND deleted_at IS NULL
`
```

## Error Handling

Structured error handling using `serrors`:

```go
const op serrors.Op = "DialogueService.StartDialogue"

if err != nil {
    return nil, serrors.E(op, err)
}
```

## Performance Considerations

### Optimization Strategies

1. **Message Storage**:
   - Store messages as JSONB for efficient queries
   - Index on user_id and tenant_id for quick lookups
   - Archive old conversations separately

2. **LLM Caching**:
   - Cache embeddings for semantic search
   - Cache system prompts
   - Rate limiting on API calls

3. **Query Optimization**:
   - Use indexes on frequently filtered columns
   - Pagination for large result sets
   - Connection pooling for database

### Scaling Strategies

- Separate read/write databases for analytics
- Message archival for very long conversations
- Caching layer (Redis) for hot conversations
- Batch API calls during off-peak hours

## Testing

### Service Tests

**File**: `modules/bichat/services/*_service_test.go`

Tests cover:
- Happy path: create dialogue, add messages
- Error cases: API failures, validation errors
- Tenant isolation: verify cross-tenant access blocked
- Message ordering: ensure chronological order

### Repository Tests

**File**: `modules/bichat/infrastructure/persistence/*_test.go`

Tests cover:
- CRUD operations
- Message persistence
- Soft deletes and recovery
- Query performance

### Controller Tests

**File**: `modules/bichat/presentation/controllers/*_test.go`

Tests cover:
- Authentication required
- Tenant isolation in responses
- Form parsing
- Error responses

## Security

### Access Control

- All routes require authentication
- Tenant isolation enforced at repository layer
- User can only access own dialogues

### Input Validation

- Message length limits enforced
- Content sanitization to prevent injection
- API request validation

### Data Protection

- Messages stored securely in database
- Soft deletes for recovery
- Audit logging of important operations
- TLS for API communications
