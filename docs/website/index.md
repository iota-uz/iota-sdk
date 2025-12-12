---
layout: default
title: Website
nav_order: 11
has_children: true
description: "Website Module - Public-facing pages and AI chatbot functionality"
---

# Website Module

The Website module provides public-facing functionality for IOTA SDK-powered applications, including static pages, landing pages, and an AI chatbot powered by Retrieval Augmented Generation (RAG) for knowledge-base answers.

## Overview

The Website module enables:

- **Public Pages**: Marketing and informational landing pages
- **AI Chatbot**: Intelligent chatbot with RAG for knowledge base retrieval
- **Chat Configuration**: Tenant-specific AI model and behavior settings
- **Chat History**: Persistent chat thread storage with message history
- **RAG Integration**: Dify or custom providers for context retrieval

## Module Location

```
modules/website/
├── domain/
│   ├── entities/
│   │   ├── aichatconfig/       # AI model configuration
│   │   ├── chatthread/         # Chat conversation threads
│   │   └── cache/              # Cache abstraction
│   └── repositories/           # Repository interfaces
├── infrastructure/
│   ├── persistence/            # Database layer
│   ├── rag/                    # RAG provider implementations
│   ├── cache/                  # Caching implementations
│   └── models/                 # Database models
├── services/
│   ├── website_chat_service.go # Chat management
│   ├── aichat_config_service.go # Configuration management
│   └── *_service_test.go
├── presentation/
│   ├── controllers/
│   │   ├── aichat_controller.go    # Chat API
│   │   ├── aichat_api_controller.go # WebSocket/API
│   │   └── *_controller_test.go
│   ├── templates/
│   │   └── pages/aichat/
│   │       └── configure_templ.go  # Config UI
│   ├── viewmodels/
│   │   └── aichat_viewmodel.go     # Data transformation
│   ├── mappers/
│   │   └── mappers.go              # Entity mapping
│   └── dtos/
│       └── dtos.go                 # Request/response DTOs
├── seed/
│   └── seed_aichatconfig.go        # Initial configuration
├── module.go                       # Module registration
├── links.go                        # Navigation links
└── nav_items.go                    # Navigation items
```

## Key Features

### AI Chatbot

- **Knowledge Base Integration**: RAG-powered answers from configured sources
- **Dify Provider**: Integration with Dify RAG service
- **Custom Providers**: Extensible provider architecture
- **Multi-tenant**: Each tenant can customize chatbot behavior
- **Chat History**: Persistent storage of chat threads

### Configuration Management

- **Per-tenant Settings**: Model selection, system prompts, temperature
- **Default Configuration**: Fallback settings for tenants
- **Secure Credentials**: API key management for LLM services
- **Model Selection**: Support for multiple LLM models

### Public Pages

- **Landing Pages**: Customizable public-facing pages
- **Static Content**: CMS-ready content management
- **SEO Optimization**: Meta tags and structured data
- **Responsive Design**: Mobile-friendly layouts

## Document Map

This Website documentation includes:

1. **[Business Requirements](./business.md)** - Use cases, problem statement, and business rules
2. **[Technical Architecture](./technical.md)** - Module structure, services, repositories, and RAG integration

## Integration Points

Website module integrates with:

- **Core Module**: User authentication (optional for public pages)
- **CRM Module**: Chat integration with customer support
- **Database**: PostgreSQL for configuration and chat history
- **RAG Providers**: Dify or custom knowledge base services
- **Frontend**: HTMX components for interactive chat

## Architecture Highlights

### RAG Architecture

```
User Query
    ↓
RAG Provider (Dify/Custom)
    ↓
Search Knowledge Base
    ↓
Retrieve Relevant Context
    ↓
LLM Processing with Context
    ↓
Generate Response
    ↓
Return to User
```

### Multi-tenant Configuration

Each tenant can customize:
- LLM model selection
- System prompts and behavior
- Temperature and token limits
- API credentials
- Default vs. custom configuration

### Chat Thread Management

- Each conversation is a thread
- Messages stored with timestamps
- Thread metadata (tenant, user, timestamp)
- Message filtering by time period

## Security Considerations

- Configuration encryption for API credentials
- Public page access without authentication
- Chat history optionally authenticated
- RAG provider security (API keys stored securely)
- Input validation and sanitization

## Related Documentation

- [Core Module](../core/index.md) - Authentication
- [CRM Module](../crm/index.md) - Chat integration
- [BiChat Module](../bichat/index.md) - Internal AI chatbot
