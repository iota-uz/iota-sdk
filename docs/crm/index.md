---
layout: default
title: CRM Module
nav_order: 4
has_children: true
permalink: /docs/crm
---

# CRM Module

Customer Relationship Management module for IOTA SDK providing comprehensive client management, multi-channel communication, and message templating capabilities.

## Overview

The CRM module enables businesses to:

- **Client Management**: Maintain detailed client profiles with contact information, personal details, and communication preferences
- **Multi-Channel Communication**: Support conversations across Telegram, WhatsApp, SMS, Email, Instagram, and website channels
- **Message Templates**: Create and manage reusable message templates for consistent communication
- **Chat History**: Maintain organized chat threads with message history and read status tracking
- **Automated Notifications**: Send messages through various transport providers

## Key Features

| Feature | Description |
|---------|-------------|
| **Client Profiles** | Store comprehensive client information including contacts, passport, PIN, gender, date of birth |
| **Multi-Transport Chat** | Handle conversations from multiple channels (Telegram, WhatsApp, SMS, Email, etc.) |
| **Chat Members** | Support multiple participants (users and clients) in chat threads |
| **Message Attachments** | Include file uploads with messages |
| **Message Templates** | Pre-defined message templates for quick replies |
| **Unread Tracking** | Track and manage unread messages per chat |
| **Event System** | Publish events for client and chat changes |

## Architecture Boundaries

```
┌─────────────────────────────────────────────────┐
│           Presentation Layer                     │
│  Controllers → ViewModels → Templates → DTOs    │
├─────────────────────────────────────────────────┤
│           Service Layer                          │
│  ClientService → ChatService → MessageTemplate  │
│                   ↓ Event Publishing            │
├─────────────────────────────────────────────────┤
│           Repository Layer                       │
│  ClientRepository → ChatRepository →             │
│  MessageTemplateRepository                      │
├─────────────────────────────────────────────────┤
│           Domain Layer                           │
│  Client ─→ Contact                              │
│  Chat ─→ Message ─→ Member ─→ Sender           │
│  MessageTemplate                                 │
└─────────────────────────────────────────────────┘
```

## Core Entities

| Entity | Purpose | Relationships |
|--------|---------|---------------|
| **Client** | Individual customer/contact | Contains multiple Contacts |
| **Contact** | Communication channel for a client | Belongs to Client (email, phone, telegram, etc.) |
| **Chat** | Conversation thread | Contains Messages and Members |
| **Message** | Individual message in a chat | References Chat and Sender |
| **Member** | Participant in a chat | References Chat and Sender |
| **Sender** | Message originator (User or Client) | Implements polymorphic interface |
| **MessageTemplate** | Pre-defined message content | Standalone entity |

## Integration Points

| Component | Integration | Purpose |
|-----------|-------------|---------|
| **Twilio Provider** | SMS/WhatsApp | Send/receive messages via Twilio |
| **Event Bus** | Event Publishing | Publish domain events for clients and chats |
| **Upload Service** | Message Attachments | Handle file uploads with messages |
| **Passport Service** | Client Identification | Store client passport information |
| **User Service** | Chat Members | Reference system users in chat membership |
| **Telegram Bot** | Notifications | Send automated Telegram notifications |

## Document Map

- [Business Requirements](./business.md) - Problem statement, use cases, and business rules
- [Technical Architecture](./technical.md) - Code structure, layer separation, and implementation patterns
- [Data Model](./data-model.md) - Database schema, entity relationships, and constraints
- [User Experience](./ux.md) - UI workflows, page structure, and interaction patterns

## Quick Links

- **Package**: `github.com/iota-uz/iota-sdk/modules/crm`
- **Routes**: `/crm/clients`, `/crm/chats`, `/crm/instant-messages`
- **Services**: `ClientService`, `ChatService`, `MessageTemplateService`
- **Repositories**: `ClientRepository`, `ChatRepository`, `MessageTemplateRepository`
- **Permissions**: `ClientRead`, `ClientCreate`, `ClientUpdate`, `ClientDelete`, `ChatRead`, `ChatCreate`, etc.

## Multi-Tenant Support

All CRM entities include tenant isolation:

- **Client**: Scoped to tenant via `tenant_id`
- **Chat**: Scoped to tenant via `tenant_id`
- **Message**: Inherited from parent Chat
- **Contact**: Inherited from parent Client
- **Member**: Scoped to tenant via `tenant_id`
- **MessageTemplate**: Scoped to tenant via `tenant_id`

Query filters automatically include tenant isolation via repository implementations.

## Event System

The CRM module publishes the following domain events:

- `ClientCreatedEvent` - Published when a new client is created
- `ClientUpdatedEvent` - Published when a client profile is updated
- `ClientDeletedEvent` - Published when a client is deleted
- `ChatCreatedEvent` - Published when a new chat is initiated
- `MessageSentEvent` - Published when a message is sent
- `MessageReceivedEvent` - Published when a message is received from external transport

Events include full entity data and context information for integration with other modules.
