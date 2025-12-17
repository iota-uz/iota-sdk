---
layout: default
title: Technical Architecture
parent: Website
nav_order: 2
description: "Website Module Technical Architecture"
---

# Technical Architecture

## Module Structure

```
modules/website/
├── domain/
│   ├── entities/
│   │   ├── aichatconfig/
│   │   │   ├── aichatconfig.go         # AI config interface
│   │   │   └── aichatconfig_test.go
│   │   ├── chatthread/
│   │   │   ├── chatthread.go           # Chat thread interface
│   │   │   └── chatthread_test.go
│   │   └── cache/
│   │       └── cache.go                # Cache abstraction
│   └── repositories/
│       ├── aichatconfig_repository.go  # Config repository interface
│       └── chatthread_repository.go    # Thread repository interface
├── infrastructure/
│   ├── persistence/
│   │   ├── aichatconfig_repository.go  # Config database impl
│   │   ├── thread_repository.go        # Thread database impl
│   │   ├── inmem_thread_repository.go  # In-memory impl (optional)
│   │   ├── website_mappers.go          # Entity mappers
│   │   └── models/
│   │       └── models.go               # Database models
│   ├── rag/
│   │   ├── rag.go                      # RAG provider interface
│   │   └── dify_provider.go            # Dify RAG implementation
│   └── cache/
│       └── redis_cache.go              # Redis cache implementation
├── services/
│   ├── aichat_config_service.go        # Configuration management
│   ├── website_chat_service.go         # Chat/thread management
│   └── *_service_test.go               # Service tests
├── presentation/
│   ├── controllers/
│   │   ├── aichat_controller.go        # HTTP handlers
│   │   ├── aichat_api_controller.go    # API endpoints
│   │   └── *_controller_test.go
│   ├── viewmodels/
│   │   └── aichat_viewmodel.go         # Data transformation
│   ├── mappers/
│   │   └── mappers.go                  # DTO mapping
│   ├── templates/
│   │   └── pages/aichat/
│   │       └── configure_templ.go      # Configuration UI
│   └── dtos/
│       └── dtos.go                     # Request/response DTOs
├── seed/
│   └── seed_aichatconfig.go            # Default configuration
├── module.go                           # Module registration
├── links.go                            # Navigation links
└── nav_items.go                        # Navigation items
```

## Domain Model

### AIConfig Entity

```go
type AIConfig interface {
    ID() uuid.UUID
    TenantID() uuid.UUID
    ModelName() string
    ModelType() AIModelType
    SystemPrompt() string
    Temperature() float32
    MaxTokens() int
    BaseURL() string
    AccessToken() string
    IsDefault() bool
    CreatedAt() time.Time
    UpdatedAt() time.Time

    // Immutable updates (return new instance)
    SetSystemPrompt(prompt string) AIConfig
    WithTemperature(temp float32) (AIConfig, error)
    WithMaxTokens(tokens int) (AIConfig, error)
    WithModelName(modelName string) (AIConfig, error)
    WithBaseURL(baseURL string) (AIConfig, error)
    SetAccessToken(accessToken string) AIConfig
    WithIsDefault(isDefault bool) (AIConfig, error)
}
```

**Key Properties**:
- **ID**: Unique configuration identifier
- **TenantID**: Multi-tenant configuration
- **ModelName**: LLM model selection (e.g., "gpt-4", "gpt-3.5-turbo")
- **ModelType**: Provider type (e.g., "openai")
- **SystemPrompt**: Custom behavior definition
- **Temperature**: Response creativity (0.0-2.0)
- **MaxTokens**: Response length limit
- **BaseURL**: API endpoint URL
- **AccessToken**: Secure API credentials
- **IsDefault**: Default configuration flag

**Business Rules**:
- Temperature must be 0.0-2.0
- Model name and base URL required
- Access token required and secure
- Immutable updates (functional programming style)

### ChatThread Entity

```go
type ChatThread interface {
    ID() uuid.UUID
    Timestamp() time.Time
    ChatID() uint
    Messages() []chat.Message
}
```

**Properties**:
- **ID**: Unique thread identifier
- **Timestamp**: Thread creation time
- **ChatID**: Reference to parent chat
- **Messages**: All messages in thread (chronologically ordered)

**Features**:
- Message filtering by timestamp
- Complete history preservation
- Time-windowed message retrieval

## Services

### AIChatConfigService

**File**: `modules/website/services/aichat_config_service.go`

Configuration management for AI chatbot:

```go
type AIChatConfigService struct {
    configRepository aichatconfig.Repository
    // ... dependencies
}

// GetDefault retrieves default config for tenant
func (s *AIChatConfigService) GetDefault(
    ctx context.Context,
) (aichatconfig.AIConfig, error)

// GetByID retrieves specific configuration
func (s *AIChatConfigService) GetByID(
    ctx context.Context,
    id uuid.UUID,
) (aichatconfig.AIConfig, error)

// Save creates or updates configuration
func (s *AIChatConfigService) Save(
    ctx context.Context,
    config aichatconfig.AIConfig,
) (aichatconfig.AIConfig, error)

// SetDefault marks configuration as default
func (s *AIChatConfigService) SetDefault(
    ctx context.Context,
    id uuid.UUID,
) error

// List lists all configurations for tenant
func (s *AIChatConfigService) List(
    ctx context.Context,
) ([]aichatconfig.AIConfig, error)
```

**Responsibilities**:
- Configuration CRUD operations
- Default configuration management
- Validation of settings
- Secure credential handling

### WebsiteChatService

**File**: `modules/website/services/website_chat_service.go`

Chat and thread management:

```go
type WebsiteChatService struct {
    threadRepository chatthread.Repository
    ragProvider      rag.Provider
    llmProvider      LLMProvider
    configService    *AIChatConfigService
    // ... other dependencies
}

// StartChat begins new conversation thread
func (s *WebsiteChatService) StartChat(
    ctx context.Context,
    initialMessage string,
) (chatthread.ChatThread, error)

// SendMessage processes user message and generates response
func (s *WebsiteChatService) SendMessage(
    ctx context.Context,
    threadID uuid.UUID,
    userMessage string,
) (string, error)

// GetThread retrieves chat thread with history
func (s *WebsiteChatService) GetThread(
    ctx context.Context,
    threadID uuid.UUID,
) (chatthread.ChatThread, error)

// SearchRAG searches knowledge base for relevant context
func (s *WebsiteChatService) SearchRAG(
    ctx context.Context,
    query string,
) ([]string, error)
```

**Responsibilities**:
- Thread lifecycle management
- Message processing and LLM integration
- RAG-based context retrieval
- Response generation with context

## RAG Integration

### RAG Provider Interface

**File**: `modules/website/infrastructure/rag/rag.go`

```go
type Provider interface {
    // SearchRelevantContext searches knowledge base
    // Returns relevant document chunks for query
    SearchRelevantContext(
        ctx context.Context,
        query string,
    ) ([]string, error)
}
```

### Dify Provider

**File**: `modules/website/infrastructure/rag/dify_provider.go`

Integration with Dify RAG service:

```go
type DifyProvider struct {
    apiKey      string
    baseURL     string
    datasetID   string
    httpClient  *http.Client
}

func NewDifyProvider(
    apiKey string,
    baseURL string,
    datasetID string,
) *DifyProvider {
    return &DifyProvider{
        apiKey:     apiKey,
        baseURL:    baseURL,
        datasetID:  datasetID,
        httpClient: &http.Client{},
    }
}

// SearchRelevantContext queries Dify knowledge base
func (p *DifyProvider) SearchRelevantContext(
    ctx context.Context,
    query string,
) ([]string, error) {
    // Call Dify API
    // Extract relevant documents
    // Return document chunks
}
```

### Custom Provider Example

Extensible architecture allows custom providers:

```go
type CustomProvider struct {
    // ... custom fields
}

func (p *CustomProvider) SearchRelevantContext(
    ctx context.Context,
    query string,
) ([]string, error) {
    // Custom RAG logic
    return results, nil
}
```

## Chat Message Flow

### Starting New Chat

```
User Query (Public/No Auth)
    ↓
WebsiteChatService.StartChat()
    ↓
Get AI Config (default for tenant)
    ↓
Create new ChatThread
    ↓
Process initial message through RAG
    ↓
Search knowledge base via RAG Provider
    ↓
Prepare LLM request with context
    ↓
Call LLM API (OpenAI)
    ↓
Add user and assistant messages to thread
    ↓
Save thread to repository
    ↓
Return thread with response
    ↓
Render in UI
```

### Continuing Chat

```
User Message
    ↓
WebsiteChatService.SendMessage()
    ↓
Load existing ChatThread
    ↓
Process message through RAG
    ↓
Retrieve relevant context from knowledge base
    ↓
Prepare LLM request with full history + context
    ↓
Call LLM API with messages
    ↓
Add messages to thread
    ↓
Save updated thread
    ↓
Return response
```

## Database Schema

### AIConfig Table

```sql
CREATE TABLE website_aichatconfigs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    model_name VARCHAR(255) NOT NULL,
    model_type VARCHAR(50) NOT NULL,
    system_prompt TEXT,
    temperature FLOAT DEFAULT 0.7,
    max_tokens INT DEFAULT 1024,
    base_url VARCHAR(500),
    access_token VARCHAR(500) ENCRYPTED,
    is_default BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

-- Indexes
CREATE INDEX idx_aichatconfig_tenant_id ON website_aichatconfigs(tenant_id);
CREATE INDEX idx_aichatconfig_is_default ON website_aichatconfigs(is_default);
CREATE UNIQUE INDEX idx_aichatconfig_tenant_default
    ON website_aichatconfigs(tenant_id)
    WHERE is_default = TRUE;
```

### ChatThread Table

```sql
CREATE TABLE website_chatthreads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    chat_id BIGINT NOT NULL,
    messages JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

-- Indexes
CREATE INDEX idx_chatthread_tenant_id ON website_chatthreads(tenant_id);
CREATE INDEX idx_chatthread_chat_id ON website_chatthreads(chat_id);
CREATE INDEX idx_chatthread_created_at ON website_chatthreads(created_at);
```

## Controllers

### AIChatController

**File**: `modules/website/presentation/controllers/aichat_controller.go`

Configuration UI and management:

**Routes**:
- `GET /website/ai-chat` - Display chat interface
- `GET /website/ai-chat/configure` - Configuration page
- `POST /website/ai-chat/config` - Update configuration

```go
func (c *AIChatController) Configure(
    r *http.Request,
    w http.ResponseWriter,
    logger *logrus.Entry,
    configService *websiteServices.AIChatConfigService,
) {
    ctx := r.Context()

    // Get default configuration
    config, err := configService.GetDefault(ctx)
    if err != nil {
        // Handle error
        return
    }

    // Create viewmodel
    vm := viewmodels.NewAIChatViewModel(config)

    // Render template
    templ.Handler(aichat.Configure(vm), templ.WithStreaming()).ServeHTTP(w, r)
}
```

### AIChatAPIController

**File**: `modules/website/presentation/controllers/aichat_api_controller.go`

Chat API endpoints:

**Routes**:
- `POST /api/website/chat/start` - Start new chat
- `POST /api/website/chat/{threadId}/message` - Send message
- `GET /api/website/chat/{threadId}` - Get chat history

## ViewModels

### AIChatViewModel

**File**: `modules/website/presentation/viewmodels/aichat_viewmodel.go`

Transform domain entities for presentation:

```go
type AIChatViewModel struct {
    ID           uuid.UUID
    ModelName    string
    SystemPrompt string
    Temperature  float32
    MaxTokens    int
}

func NewAIChatViewModel(config aichatconfig.AIConfig) AIChatViewModel {
    return AIChatViewModel{
        ID:           config.ID(),
        ModelName:    config.ModelName(),
        SystemPrompt: config.SystemPrompt(),
        Temperature:  config.Temperature(),
        MaxTokens:    config.MaxTokens(),
    }
}
```

**Separation of Concerns**:
- ViewModels: Data transformation only
- Controllers: Request/response handling
- Services: Business logic
- Repositories: Data access

## Caching Strategy

### Configuration Cache

- Cache AI config for 1 hour (rarely changes)
- Invalidate on update
- Reduce database queries

```go
type CachedConfigService struct {
    configService *AIChatConfigService
    cache         cache.Cache
    ttl           time.Duration
}

func (s *CachedConfigService) GetDefault(ctx context.Context) (aichatconfig.AIConfig, error) {
    // Check cache first
    if cached := s.cache.Get("config:default"); cached != nil {
        return cached.(aichatconfig.AIConfig), nil
    }

    // Load from service
    config, err := s.configService.GetDefault(ctx)
    if err != nil {
        return nil, err
    }

    // Cache result
    s.cache.Set("config:default", config, s.ttl)
    return config, nil
}
```

### Chat History Cache

- Keep recent threads in memory
- Reduces database queries for active conversations
- Automatic cleanup of old threads

## Error Handling

Structured error handling:

```go
const op serrors.Op = "AIChatConfigService.GetDefault"

config, err := s.configRepository.GetDefault(ctx)
if err != nil {
    if errors.Is(err, aichatconfig.ErrConfigNotFound) {
        return nil, serrors.E(op, serrors.KindNotFound, err)
    }
    return nil, serrors.E(op, err)
}
```

## Security

### Credential Management

- API keys encrypted at rest
- Never logged or exposed in errors
- Secure transmission (TLS required)
- Rotation mechanism

### Access Control

- Public chat endpoints (no auth required)
- Configuration endpoints protected (admin only)
- Tenant isolation enforced

### Input Validation

- Message length limits
- Prompt injection prevention
- SQL injection prevention (parameterized queries)
- XSS prevention in responses

## Performance Optimization

### RAG Optimization

- Cache knowledge base search results
- Batch index updates
- Efficient vector similarity search

### LLM Optimization

- Batch API requests during off-peak
- Cache common responses
- Streaming responses for UX
- Token usage tracking and limits

### Database Optimization

- Indexes on frequently queried columns
- JSONB queries optimized
- Connection pooling
- Read replicas for analytics

## Testing

### Service Tests

Cover configuration and chat workflows:
- Configuration CRUD
- Default config logic
- Chat message processing
- RAG integration

### Repository Tests

Cover data persistence:
- Configuration save/load
- Chat thread storage
- Message ordering

### Controller Tests

Cover HTTP interfaces:
- Configuration endpoints
- Chat API endpoints
- Error handling

## Multi-tenant Isolation

All queries include tenant isolation:

```go
tenantID := composables.UseTenantID(ctx)

query := `
    SELECT * FROM website_aichatconfigs
    WHERE tenant_id = $1
      AND is_default = true
`
```

## Integration with CRM

Website chat can integrate with CRM:

```
Website Chat Thread
    ↓
Link to CRM Chat when needed
    ↓
Hand off to support team
    ↓
CRM chat continues conversation
```

## Deployment Considerations

- Separate config for each environment
- Knowledge base URL per environment
- API keys in secrets management
- Rate limiting configuration
- Response time SLAs
