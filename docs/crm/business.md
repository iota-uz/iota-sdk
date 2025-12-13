---
layout: default
title: Business Requirements
parent: CRM Module
nav_order: 1
permalink: /docs/crm/business
---

# CRM Module - Business Requirements

## Problem Statement

Organizations need a centralized system to manage customer relationships, maintain communication history, and provide multi-channel support across various communication platforms. The current challenge involves:

- **Fragmented Communication**: Customer interactions scattered across different channels without unified history
- **Information Silos**: Disconnected customer data without comprehensive profiles
- **Manual Processes**: Time-consuming manual message sending without templates or automation
- **Channel Diversity**: Need to support multiple communication methods (SMS, WhatsApp, Telegram, Email, etc.)
- **Scalability**: Supporting message delivery across various third-party providers

## Target Audience

| User Type | Responsibilities |
|-----------|-----------------|
| **Sales Representatives** | Create client profiles, track communications, send messages |
| **Support Agents** | Monitor chats, respond to customer inquiries, manage conversations |
| **Team Leads** | Review communication history, ensure quality standards |
| **Administrators** | Configure message templates, manage team access, monitor communications |

## Business Objectives

### Primary Goals

1. **Unified Client Profile**: Centralize all customer information and communication channels
2. **Multi-Channel Communication**: Enable messaging across SMS, WhatsApp, Telegram, Email, Instagram
3. **Message Templates**: Accelerate communication with pre-defined responses
4. **Communication History**: Maintain audit trail of all customer interactions
5. **Scalable Messaging**: Support high-volume message delivery through providers

### Secondary Goals

1. Reduce response time to customer inquiries
2. Improve customer satisfaction through consistent communication
3. Enable data-driven customer insights
4. Support compliance and audit requirements
5. Automate routine customer notifications

## Entity Classifications

### Core Domain Entities

#### Client (Aggregate Root)
Represents a customer or business contact.

**Primary Attributes**:
- Personal Information: First/Last/Middle name, gender, date of birth
- Contact Details: Phone number, email address, physical address
- Identification: Passport number, Tax PIN (PIN)
- Communication: Multiple contact channels (email, phone, telegram, WhatsApp, etc.)
- Metadata: Comments, notes, creation/update timestamps

**Business Rules**:
- Client must have a first name
- Client can have multiple contact methods
- Contacts are organized by type (email, phone, telegram, WhatsApp, other)
- Client information is immutable except through explicit setter methods
- Deletion removes client profile and associated chats

#### Contact (Value Object)
Communication channel associated with a client.

**Contact Types**:
- Email: Electronic mail address
- Phone: Telephone number
- Telegram: Telegram messenger handle
- WhatsApp: WhatsApp phone number
- Other: Custom contact type

**Business Rules**:
- Each contact has a unique value within its type
- Contact type determines how messages are sent
- Contact creation/update tracked with timestamps

#### Chat (Aggregate Root)
Represents a conversation thread between organization and client.

**Participants**:
- Client: The customer being communicated with
- Members: Users and external parties involved in the chat
- Messages: Individual communications within the chat

**Business Rules**:
- Chat is created explicitly or on-demand when first message arrives
- One chat per client (unique relationship)
- Chat tracks last message timestamp
- Unread message count maintained per chat
- Chat creation timestamp immutable
- All participants tracked with membership records

#### Message (Entity)
Individual communication within a chat.

**Properties**:
- Content: Text message body
- Sender: Origin of message (User, Client, or external system)
- Attachments: Related file uploads
- Read Status: Tracked per message with read timestamp
- Transport: Channel used (Telegram, WhatsApp, SMS, Email, etc.)
- Timestamps: Creation, sent, and read times

**Business Rules**:
- Messages are immutable after creation
- Read status and timestamp tracked for audit
- Attachments optional, files stored separately
- Sender type determines handling (User internal, Client external, System automated)

#### Member (Entity)
Participant in a chat conversation.

**Member Types**:
- User: Internal system user
- Client: Customer via contact method
- Service: Automated system (bot, webhook)

**Business Rules**:
- Member tied to specific chat
- One member record per participant per chat
- Transport method determines communication capability
- Creation timestamp immutable

#### Sender (Polymorphic Interface)
Entity that originates a message.

**Sender Types**:
- **UserSender**: System user sending message
- **ClientSender**: Client sending via contact method
- **TelegramSender**: Telegram bot or user
- **WhatsAppSender**: WhatsApp account
- **SMSSender**: SMS recipient
- **EmailSender**: Email recipient
- **InstagramSender**: Instagram account
- **WebsiteSender**: Website visitor
- **OtherSender**: Generic external source

**Business Rules**:
- Each sender type carries specific metadata
- Sender type determines routing and formatting
- External senders can be created from provider callbacks

#### MessageTemplate (Entity)
Pre-defined message content for quick replies.

**Properties**:
- Template Text: Message content (may include variables)
- Creation Timestamp: When template was created
- Tenant Scope: Available to organization

**Business Rules**:
- Templates are static text content
- Can be used across all chat transports
- Tenant-specific (not shared across organizations)
- No template inheritance or composition

### Supporting Entities

#### Transport
Communication channel abstraction.

**Supported Transports**:
- Telegram: Telegram Bot API messaging
- WhatsApp: WhatsApp Business API
- SMS: Twilio SMS service
- Email: Email messaging
- Instagram: Instagram Direct Messages
- Website: Website visitor contact
- Phone: Voice calls
- Other: Generic fallback

**Business Rules**:
- Transport determines provider handling
- Provider implements Transport interface
- Each provider handles send/receive for specific channel

#### Provider
External service integration for message delivery.

**Current Providers**:
- Twilio: SMS and WhatsApp delivery
- Telegram Bot: Telegram messaging
- (Extensible for additional providers)

**Business Rules**:
- Provider registered for specific transport
- Provider handles protocol-specific details
- Callbacks integrated for inbound messages
- Configuration via environment variables

## Business Rules

### Client Rules

1. **Creation**: Client must have first name; other fields optional
2. **Identity**: Client ID unique within tenant
3. **Contacts**: Client can have multiple contacts of same/different types
4. **Updates**: Only explicit setter methods modify client state
5. **Deletion**: Soft delete pattern recommended (mark inactive vs physical delete)

### Chat Rules

1. **One-to-One**: Single chat per client relationship
2. **Implicit Creation**: Chat auto-created when first message arrives from new client contact
3. **Immutable Reference**: Client reference cannot change
4. **Message Ordering**: Messages ordered by creation timestamp
5. **Unread Tracking**: Automatic unread count based on read_at timestamp

### Message Rules

1. **Immutability**: Message content never modified after creation
2. **Sender Required**: Every message must identify sender type
3. **Read Tracking**: Read status and timestamp recorded when viewed
4. **Ordering**: Messages ordered by creation time
5. **Attachment Support**: Messages can include uploaded files

### Member Rules

1. **Unique Participation**: Single member record per participant per chat
2. **Transport Specific**: Member transport determines communication method
3. **Metadata**: Transport-specific metadata stored separately
4. **Immutable Creation**: Member creation timestamp never changes

### Template Rules

1. **Reusability**: Templates usable across all transport types
2. **Static Content**: No dynamic variable substitution in core template
3. **Tenant Scoped**: Templates not shared across organizations
4. **No Nesting**: Templates cannot reference other templates

## Communication Workflows

### Outbound Communication (User to Client)

```
User Initiates Message
    ↓
System Selects Transport
    ↓
Provider Sends via API (SMS/Telegram/etc)
    ↓
Message Stored with Sent Status
    ↓
Provider Callback Confirms Delivery
```

### Inbound Communication (Client to User)

```
Client Message Arrives via Provider
    ↓
Provider Webhook Received
    ↓
Chat Created or Located
    ↓
Message Created in Chat
    ↓
Member Created if New Participant
    ↓
System Publishes MessageReceivedEvent
    ↓
Notification Sent to Users
```

### Chat Initialization

```
User Opens Client Profile
    ↓
System Checks for Existing Chat
    ↓
If No Chat: Create New Chat + Initial Member
    ↓
If Chat Exists: Load Existing Chat + Members
    ↓
Display Chat UI with Message History
```

## Success Criteria

### Functional Criteria

- All client information centralized and accessible
- Messages send reliably across all supported transports
- Chat history complete and searchable
- Message templates reduce manual typing
- Unread message tracking prevents missed communications

### Performance Criteria

- Client lookup: < 100ms
- Chat list load: < 500ms
- Message send: < 2 seconds
- Multi-channel support: Handle 1000+ concurrent chats

### Quality Criteria

- 99.9% message delivery reliability
- Audit trail for all communications
- Data consistency across multi-tenant isolation
- Permission-based access control
- Event-driven integration capability

## Data Privacy & Compliance

1. **Tenant Isolation**: Strict data isolation between organizations
2. **Message Retention**: Configurable retention policies
3. **Audit Logging**: All message send/receive events logged
4. **User Consent**: Track client preferences and opt-in status
5. **GDPR Support**: Enable client data deletion workflows

## Future Enhancements

1. **AI Features**: Sentiment analysis, smart replies
2. **Analytics**: Communication metrics and insights
3. **Integration**: CRM connector to finance module
4. **Bulk Operations**: Send messages to client segments
5. **Custom Fields**: Extensible client profile attributes
6. **Conversation AI**: Chatbot integration
7. **Voice Support**: Call recording and transcription
