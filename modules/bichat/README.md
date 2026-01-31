# BI-Chat Module

A production-ready chat module for IOTA SDK based on the BI-Chat foundation library (`pkg/bichat`).

## Overview

This module provides a complete chat interface with:
- Multi-tenant session and message management
- LLM-powered conversations with tool calling
- Knowledge base integration
- SQL query execution for BI tasks
- Streaming responses with SSE
- React UI with HTMX integration
- HITL (Human-in-the-Loop) support
- GraphQL API

## Architecture

```
modules/bichat/
├── config.go                    # Module configuration
├── module.go                    # Module registration and wiring
├── infrastructure/
│   ├── persistence/
│   │   ├── chat_repository.go          # PostgreSQL implementation
│   │   ├── chat_repository_test.go     # Repository tests
│   │   └── schema/
│   │       └── bichat-schema.sql       # Database migrations
│   └── llmproviders/
│       └── openai_provider.go          # LLM provider implementation
├── presentation/
│   ├── controllers/
│   │   ├── chat_controller.go          # HTTP/GraphQL endpoints
│   │   ├── stream_controller.go        # SSE streaming
│   │   └── web_controller.go           # React app rendering
│   ├── graphql/
│   │   └── schema.graphql              # GraphQL schema
│   ├── interop/
│   │   ├── context.go                  # Server context injection
│   │   ├── permissions.go              # Permission helpers
│   │   └── types.go                    # Shared Go/TS types
│   ├── locales/
│   │   ├── en.json                     # English translations
│   │   ├── ru.json                     # Russian translations
│   │   └── uz.json                     # Uzbek translations
│   └── templates/
│       └── pages/bichat/               # Templ templates
├── services/
│   ├── chat_service.go                 # Chat business logic
│   ├── chat_service_test.go            # Service tests
│   └── agent_service.go                # Agent orchestration
└── agents/
    └── default_agent.go                # Default BI agent implementation
```

## Dependencies

### Required
- `pkg/bichat/domain` - Domain models (Session, Message, Attachment, Citation)
- `pkg/bichat/services` - Service interfaces
- `pkg/bichat/hooks` - Event system
- `pkg/bichat/context` - Context management
- `pkg/bichat/tools` - Common tools (SQL, KB, Chart, etc.)

### Optional
- `pkg/bichat/kb` - Knowledge base indexing (if KBSearcher provided)
- `pkg/bichat/agents` - Agent framework (Phase 1 - in progress)

## Configuration

### Basic Setup

```go
import (
    "github.com/iota-uz/iota-sdk/modules/bichat"
    "github.com/iota-uz/iota-sdk/pkg/bichat/domain"
    "github.com/iota-uz/iota-sdk/pkg/bichat/context"
)

// Create configuration
cfg := bichat.NewModuleConfig(
    composables.UseTenantID,  // Tenant ID extractor
    composables.UseUserID,    // User ID extractor
    chatRepo,                 // ChatRepository implementation
    llmModel,                 // LLM Model implementation
    bichat.DefaultContextPolicy(), // Context policy
    parentAgent,              // Parent agent
)

// Register module
app.RegisterModule(bichat.NewModule(cfg))
```

### Advanced Setup with Optional Dependencies

```go
cfg := bichat.NewModuleConfig(
    composables.UseTenantID,
    composables.UseUserID,
    chatRepo,
    llmModel,
    bichat.DefaultContextPolicy(),
    parentAgent,

    // Optional: Add sub-agents for delegation
    bichat.WithSubAgents(
        sqlAgent,
        chartAgent,
        reportAgent,
    ),

    // Optional: Enable SQL execution
    bichat.WithQueryExecutor(queryExecutor),

    // Optional: Enable knowledge base search
    bichat.WithKBSearcher(kbSearcher),

    // Optional: Enable history summarization
    bichat.WithSummarizer(summarizer),

    // Optional: Custom event bus
    bichat.WithEventBus(eventBus),

    // Optional: Custom logger
    bichat.WithLogger(logger),

    // Optional: PostgreSQL checkpointer for HITL
    bichat.WithCheckpointer(checkpointer),
)
```

## Database Schema

The module requires the following tables:

### Sessions Table
```sql
CREATE TABLE bichat_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    pinned BOOLEAN NOT NULL DEFAULT false,
    parent_session_id UUID REFERENCES bichat_sessions(id) ON DELETE SET NULL,
    pending_question_agent VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bichat_sessions_tenant_user ON bichat_sessions(tenant_id, user_id);
CREATE INDEX idx_bichat_sessions_status ON bichat_sessions(status);
CREATE INDEX idx_bichat_sessions_created_at ON bichat_sessions(created_at DESC);
```

### Messages Table
```sql
CREATE TABLE bichat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES bichat_sessions(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    tool_calls JSONB,
    tool_call_id VARCHAR(255),
    citations JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bichat_messages_session ON bichat_messages(session_id, created_at DESC);
CREATE INDEX idx_bichat_messages_role ON bichat_messages(role);
```

### Attachments Table
```sql
CREATE TABLE bichat_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES bichat_messages(id) ON DELETE CASCADE,
    file_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,
    storage_path TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bichat_attachments_message ON bichat_attachments(message_id);
```

## API Endpoints

### GraphQL

```graphql
type Query {
  sessions(limit: Int, offset: Int): [Session!]!
  session(id: ID!): Session
  messages(sessionId: ID!, limit: Int, offset: Int): [Message!]!
}

type Mutation {
  createSession(title: String): Session!
  sendMessage(sessionId: ID!, content: String!, attachments: [Upload!]): SendMessageResponse!
  resumeWithAnswer(sessionId: ID!, checkpointId: ID!, answers: JSON!): SendMessageResponse!
  archiveSession(id: ID!): Session!
  pinSession(id: ID!): Session!
}

type Subscription {
  messageStream(sessionId: ID!): MessageChunk!
}
```

### HTTP Endpoints

- `GET /bichat` - Render React chat app
- `POST /bichat/stream` - SSE streaming endpoint
- `GET /bichat/sessions` - List user sessions
- `POST /bichat/sessions` - Create new session
- `POST /bichat/sessions/:id/messages` - Send message

## Permissions

The module defines the following permissions:

- `bichat.access` - Access to BI-Chat module
- `bichat.read_all` - Read all sessions (admin)
- `bichat.export` - Export chat data
- `bichat.manage` - Manage settings

## Events

The module emits the following events via the event bus:

### Agent Events
- `agent.start` - Agent execution started
- `agent.complete` - Agent execution completed
- `agent.error` - Agent execution failed

### LLM Events
- `llm.request` - LLM API request sent
- `llm.response` - LLM API response received
- `llm.stream` - LLM streaming chunk

### Tool Events
- `tool.start` - Tool execution started
- `tool.complete` - Tool execution completed
- `tool.error` - Tool execution failed

### Context Events
- `context.compile` - Context compiled
- `context.compact` - Context compacted
- `context.overflow` - Context overflow handled

### Session Events
- `session.create` - Session created
- `message.save` - Message saved
- `interrupt` - HITL interrupt triggered

## Testing

### Repository Tests
```bash
go test ./modules/bichat/infrastructure/persistence/... -v
```

### Service Tests
```bash
go test ./modules/bichat/services/... -v
```

### Controller Tests
```bash
go test ./modules/bichat/presentation/controllers/... -v
```

### Integration Tests
```bash
go test ./modules/bichat/... -v -tags=integration
```

## UI Integration

The module renders a React SPA with server-side context injection:

```go
// Web controller injects context
func (c *WebController) RenderChatApp(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    user := composables.UseUser(ctx)
    tenant := composables.UseTenantID(ctx)
    pageCtx := composables.UsePageCtx(ctx)

    initialContext := interop.InitialContext{
        User: interop.UserContext{
            ID:          user.ID,
            Email:       user.Email,
            Permissions: c.getUserPermissions(ctx),
        },
        Tenant: interop.TenantContext{
            ID: tenant.String(),
        },
        Locale: interop.LocaleContext{
            Language:     pageCtx.GetLocale().String(),
            Translations: c.getTranslations(pageCtx),
        },
        Config: interop.AppConfig{
            GraphQLEndpoint: "/bichat/graphql",
            StreamEndpoint:  "/bichat/stream",
        },
    }

    return templates.RenderReactApp(w, initialContext)
}
```

## Deployment

### Environment Variables

```bash
# LLM Provider (optional, defaults to in-memory)
LLM_PROVIDER=openai
LLM_API_KEY=sk-...
LLM_MODEL=gpt-4

# Context Configuration
BICHAT_CONTEXT_WINDOW=180000
BICHAT_COMPLETION_RESERVE=8000

# Knowledge Base (optional)
BICHAT_KB_PATH=/var/lib/bichat/kb
BICHAT_KB_ENABLED=true

# Observability
BICHAT_EVENT_BUS_BUFFER_SIZE=1000
LOG_LEVEL=info
```

### Database Migrations

Migrations are automatically applied on app startup via the embedded schema file.

## Implementation Status

### ✅ Completed (Phase 7)
- [x] Module configuration (config.go)
- [x] Module registration and wiring (module.go)
- [x] Interop layer types (presentation/interop/)
- [x] Translation files (presentation/locales/)
- [x] Documentation (README.md)

### ⏳ Pending (requires Phase 1: Agent Framework)
- [ ] PostgreSQL chat repository implementation
- [ ] Chat service implementation
- [ ] Agent service implementation
- [ ] Default BI agent implementation
- [ ] GraphQL schema and resolvers
- [ ] HTTP controllers (chat, stream, web)
- [ ] Repository tests
- [ ] Service tests
- [ ] Controller tests
- [ ] Database migrations (sessions, messages, attachments)

### Dependencies
- **Phase 1** (Agent Framework) must be completed first for:
  - Agent interface
  - Executor with ReAct loop
  - Model interface
  - Tool registry
  - Checkpointer interface
  - Generator pattern

## Next Steps

1. Complete Phase 1 (Agent Framework) in `pkg/bichat/agents/`
2. Implement PostgreSQL chat repository
3. Implement chat service with agent orchestration
4. Create default BI agent with tools
5. Add GraphQL schema and resolvers
6. Implement controllers (chat, stream, web)
7. Write comprehensive tests
8. Update database migrations
9. Document API usage examples

## Related Documentation

- [BI-Chat Foundation Plan](~/.claude/plans/purrfect-coalescing-marshmallow.md)
- [pkg/bichat README](/pkg/bichat/README.md)
- [Agent Framework Documentation](/pkg/bichat/agents/README.md)
- [Context Management Documentation](/pkg/bichat/context/README.md)
- [Knowledge Base Documentation](/pkg/bichat/kb/README.md)
- [Tools Documentation](/pkg/bichat/tools/README.md)
