---
layout: default
title: BiChat
nav_order: 10
has_children: true
description: "BiChat Module - Enterprise multi-turn dialogue AI chatbot"
---

# BiChat Module

The BiChat module provides enterprise-grade AI chatbot capabilities for multi-tenant applications. It enables users to create intelligent dialogues with LLM-powered responses, maintain conversation history, and seamlessly integrate AI assistance into business workflows.

## Overview

BiChat enables:

- **Multi-turn Dialogues**: Maintain conversation context across multiple exchanges
- **LLM Integration**: Support for OpenAI and extensible provider architecture
- **User-centric Design**: Conversations scoped to individual users within tenants
- **Message History**: Complete conversation history with timestamps
- **Domain-driven Architecture**: Clean separation between domain logic and infrastructure

## Module Location

```
modules/bichat/
├── domain/
│   ├── entities/
│   │   ├── dialogue/          # Dialogue conversation entity
│   │   ├── llm/               # LLM abstraction layer
│   │   ├── embedding/         # Embedding vectors
│   │   └── prompt/            # Prompt management
│   └── repositories/          # Repository interfaces
├── infrastructure/
│   ├── persistence/           # Database layer
│   ├── llmproviders/          # LLM provider implementations
│   └── cache/                 # Caching layer
├── services/
│   ├── dialogue_service.go    # Conversation management
│   ├── embeddings_service.go  # Vector embeddings
│   └── prompt_service.go      # Prompt templates
├── presentation/
│   ├── controllers/           # HTTP handlers
│   ├── templates/             # UI templates
│   └── dtos/                  # Data transfer objects
└── module.go                  # Module registration
```

## Key Features

### Dialogue Management

- Create new conversations with initial user message
- Add and retrieve messages within conversations
- Set conversation labels for organization
- User-scoped access (tenant isolation enforced)

### LLM Provider Integration

- **OpenAI**: GPT-4, GPT-4 Turbo, GPT-3.5 Turbo support
- **Extensible Architecture**: Abstract provider interface for easy addition of other LLMs
- **Streaming Responses**: Real-time response streaming for better UX
- **Configurable Parameters**: Temperature, max_tokens, system prompts

### Message Management

- User and assistant message types
- Message timestamps and ordering
- Support for multi-modal content
- Complete conversation history

## Document Map

This BiChat documentation includes:

1. **[Business Requirements](./business.md)** - Use cases, problem statement, and business rules
2. **[Technical Architecture](./technical.md)** - Module structure, services, repositories, and LLM integration

## Integration Points

BiChat integrates with:

- **Core Module**: User authentication and tenant isolation
- **Database**: PostgreSQL for persistence
- **LLM Services**: OpenAI API for chat completions
- **Web Frontend**: HTMX-powered UI for interactions

## Architecture Highlights

### Domain-Driven Design

- Dialogue aggregate with immutable message handling
- LLM abstraction decoupled from infrastructure
- Repository interfaces for persistence
- Clean domain logic with no external dependencies

### Multi-tenant Isolation

- All queries filtered by tenant_id
- User-scoped conversations
- Database-level isolation enforced
- Secure access patterns via composables

### Streaming Architecture

- Real-time response streaming via OpenAI API
- Server-sent events (SSE) for frontend updates
- Progressive message rendering
- No blocking on long-running LLM calls

## Getting Started

- Review [Business Requirements](./business.md) to understand the domain
- Check [Technical Architecture](./technical.md) for implementation details
- Explore the module code at `modules/bichat/`

## Security Considerations

- All dialogues scoped to authenticated users
- Tenant isolation enforced at repository layer
- Secure API credential management
- Input validation on all user messages
- Rate limiting recommended for LLM API calls

## Related Documentation

- [Core Module](../core/index.md) - Authentication and user management
- [Website Module](../website/index.md) - Public-facing AI chat functionality
