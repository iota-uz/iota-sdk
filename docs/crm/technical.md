---
layout: default
title: Technical Architecture
parent: CRM Module
nav_order: 2
permalink: /docs/crm/technical
---

# CRM Module - Technical Architecture

## Directory Structure

```
modules/crm/
├── domain/                           # Domain Layer (Business Logic)
│   └── aggregates/
│       ├── client/
│       │   ├── client.go            # Client aggregate interface
│       │   ├── client_impl.go       # Client implementation
│       │   ├── contact.go           # Contact value object
│       │   ├── repository.go        # Client repository interface
│       │   ├── events.go            # Domain events
│       │   └── types.go             # Client enums (ContactType)
│       └── chat/
│           ├── chat.go              # Chat aggregate interface
│           ├── chat_impl.go         # Chat implementation
│           ├── message.go           # Message entity interface
│           ├── member.go            # Member entity interface
│           ├── sender.go            # Sender polymorphic interface
│           ├── provider.go          # Transport provider interface
│           ├── repository.go        # Chat repository interface
│           ├── events.go            # Domain events
│           └── types.go             # Chat enums (Transport, SenderType)
│
├── infrastructure/                   # Infrastructure Layer (Persistence & External)
│   ├── persistence/
│   │   ├── client_repository.go     # Client repository implementation
│   │   ├── chat_repository.go       # Chat repository implementation
│   │   ├── message_template_repository.go
│   │   ├── models/
│   │   │   ├── models.go            # ORM/database models
│   │   │   └── mappers.go           # Domain → Database mapping
│   │   └── schema/
│   │       └── crm-schema.sql       # Database migrations
│   ├── cpass-providers/             # Communication Provider Implementations
│   │   ├── provider.go              # Abstract provider base
│   │   ├── twilio.go                # Twilio SMS/WhatsApp provider
│   │   └── config.go                # Provider configuration
│   └── telegram/                     # Telegram Bot Integration
│       └── bot.go                    # Telegram bot handler
│
├── presentation/                     # Presentation Layer (HTTP & UI)
│   ├── controllers/
│   │   ├── client_controller.go     # Client HTTP handlers
│   │   ├── chat_controller.go       # Chat HTTP handlers
│   │   ├── message_template_controller.go
│   │   ├── twilio_controller.go     # Webhook handlers
│   │   └── dtos/                    # Data Transfer Objects
│   │       ├── client_dto.go
│   │       └── chat_dto.go
│   ├── templates/
│   │   └── pages/
│   │       ├── clients/             # Client list/detail pages
│   │       ├── chats/               # Chat interface pages
│   │       └── message-templates/   # Template management pages
│   ├── viewmodels/
│   │   ├── client_viewmodel.go      # Client presentation logic
│   │   └── chat_viewmodel.go        # Chat presentation logic
│   ├── locales/
│   │   ├── en.json                  # English translations
│   │   ├── ru.json                  # Russian translations
│   │   └── uz.json                  # Uzbek translations
│   └── assets/
│       └── css/                     # Module-specific styles
│
├── services/
│   ├── client_service.go            # Client business logic
│   ├── chat_service.go              # Chat business logic
│   └── messagetemplate_service.go   # Message template logic
│
├── handlers/
│   ├── client_handler.go            # Event handlers for clients
│   ├── sms_handler.go               # SMS event handlers
│   └── notification_handler.go      # Telegram notification handlers
│
├── permissions/
│   ├── constants.go                 # Permission definitions
│   └── module.go                    # Permission registration
│
├── datasource.go                     # Search/spotlight data source
├── links.go                          # Navigation links
├── module.go                         # Module registration
└── README.md                         # Module documentation
```

## Layer Separation

### Domain Layer (Pure Business Logic)

**Location**: `modules/crm/domain/aggregates/*`

**Key Components**:
- Client interface and implementation
- Chat interface and implementation
- Message and Member entities
- Polymorphic Sender interface
- Provider interface for message delivery
- Domain events (CreatedEvent, UpdatedEvent, DeletedEvent)

**Characteristics**:
- No external dependencies (no database, HTTP, or messaging)
- Business rules enforced in aggregate setters
- Immutable aggregate returns (setters return new instances)
- Events triggered on state changes
- Interfaces represent contracts, implementations are private structs

**Example**:
```go
// Domain interface
type Client interface {
    ID() uint
    FirstName() string
    SetPhone(number phone.Phone) Client  // Returns new instance
    AddContact(contact Contact) Client
    RemoveContact(contactID uint) Client
}

// Setter creates new instance (immutability)
func (c *client) SetPhone(number phone.Phone) Client {
    result := *c
    result.phone = number
    result.updatedAt = time.Now()
    return &result
}
```

### Service Layer (Business Logic Orchestration)

**Location**: `modules/crm/services/*`

**Key Services**:
- `ClientService`: CRUD operations, event publishing
- `ChatService`: Chat management, message delivery, provider routing
- `MessageTemplateService`: Template CRUD operations

**Responsibilities**:
- Coordinate between repositories and domain
- Validate business rules
- Manage transactions
- Publish domain events
- Check permissions via `composables.CanUser()`

**Example**:
```go
type ClientService struct {
    repo      client.Repository
    publisher eventbus.EventBus
}

func (s *ClientService) Create(ctx context.Context, data client.Client) error {
    var createdClient client.Client
    err := composables.InTx(ctx, func(txCtx context.Context) error {
        created, err := s.repo.Save(ctx, data)
        if err != nil {
            return err
        }
        createdClient = created
        return nil
    })
    if err != nil {
        return err
    }
    createdEvent, _ := client.NewCreatedEvent(ctx, data)
    createdEvent.Result = createdClient
    s.publisher.Publish(createdEvent)
    return nil
}
```

### Repository Layer (Data Persistence)

**Location**: `modules/crm/infrastructure/persistence/*`

**Responsibilities**:
- Implement domain repository interfaces
- Handle database queries with tenant isolation
- Map between domain entities and database models
- Support pagination and filtering

**Key Features**:
- Interfaces defined in domain layer
- Implementations in infrastructure layer
- Automatic tenant_id filtering via `composables.UseTenantID(ctx)`
- Transaction support via `composables.InTx(ctx, fn)`

**Example**:
```go
// Domain interface (crm/domain/aggregates/client/repository.go)
type Repository interface {
    Save(ctx context.Context, client Client) (Client, error)
    GetByID(ctx context.Context, id uint) (Client, error)
    GetPaginated(ctx context.Context, params *FindParams) ([]Client, error)
    Count(ctx context.Context, params *FindParams) (int64, error)
    Delete(ctx context.Context, id uint) error
}

// Repository implementation
func (r *ClientRepository) Save(ctx context.Context, client client.Client) (client.Client, error) {
    tenantID, _ := composables.UseTenantID(ctx)
    tx := composables.UseTx(ctx)

    // Build model and insert/update
    // Always include tenant_id in WHERE clauses
    return mapped_client, nil
}
```

### Presentation Layer (HTTP & UI)

**Location**: `modules/crm/presentation/*`

**Components**:

#### Controllers
HTTP request handlers that:
- Accept HTTP requests and parse form data
- Call services for business logic
- Render templates or return JSON
- Check permissions via middleware
- Handle errors with proper status codes

**Example**:
```go
type ClientController struct {
    app     application.Application
    service *services.ClientService
}

func (c *ClientController) List(w http.ResponseWriter, r *http.Request) {
    params := composables.UsePaginated(r)
    clients, err := c.service.GetPaginated(r.Context(), &client.FindParams{
        Limit:  params.Limit,
        Offset: params.Offset,
    })
    // Render template with data
}
```

#### ViewModels
Transform domain entities to presentation structures:
```go
type ClientViewModel struct {
    ID        uint
    FirstName string
    LastName  string
    Phone     string
    Email     string
    Contacts  []ContactVM
}

func NewClientViewModel(c client.Client) ClientViewModel {
    return ClientViewModel{
        ID:        c.ID(),
        FirstName: c.FirstName(),
        LastName:  c.LastName(),
        // ... map other fields
    }
}
```

#### Templates (Templ)
HTML templates with type safety using Templ framework:
```templ
templ ClientProfile(ctx context.Context, client ClientViewModel) {
    <div class="client-profile">
        <h1>{ client.FirstName } { client.LastName }</h1>
        <p>Phone: { client.Phone }</p>
        <p>Email: { client.Email }</p>
    </div>
}
```

## Data Flow

### Creating a Client

```
HTTP Request
    ↓
Controller.Create (validates permission)
    ↓
Parse ClientDTO from form data
    ↓
Domain.New() - create client aggregate
    ↓
ClientService.Create()
    ↓
InTx() - wrap in transaction
    ↓
ClientRepository.Save() - persist to database
    ↓
Publish ClientCreatedEvent
    ↓
Return HTTP response
```

### Sending a Message

```
HTTP Request to send message
    ↓
Controller validates permissions
    ↓
ChatService.SendMessage()
    ↓
Locate Chat and Client
    ↓
Validate Message Content
    ↓
Get Transport Provider
    ↓
Provider.Send() - call external API
    ↓
Message persisted to database
    ↓
Event published: MessageSentEvent
    ↓
Provider callback on delivery confirmation
```

### Receiving a Message (Webhook)

```
Provider Webhook Request (Twilio, Telegram, etc)
    ↓
TwilioController.WebhookHandler()
    ↓
Parse provider payload
    ↓
ChatService.OnMessageReceived()
    ↓
Locate or create Chat
    ↓
Locate or create Member
    ↓
Message persisted
    ↓
Publish MessageReceivedEvent
```

## Key Design Patterns

### 1. Functional Options Pattern
Used for entity creation with optional fields:
```go
client.New("John",
    client.WithLastName("Doe"),
    client.WithEmail(email),
    client.WithPhone(phone),
)
```

### 2. Immutable Aggregates
Setter methods return new instances, never modify in-place:
```go
updatedClient := client.SetPhone(newPhone).SetEmail(newEmail)
```

### 3. Repository Interface Injection
Services depend on interfaces, not implementations:
```go
type ClientService struct {
    repo client.Repository  // Interface, not *ClientRepository
}
```

### 4. Polymorphic Senders
Multiple sender types implementing common interface:
```go
type Sender interface {
    Type() SenderType
}

type UserSender interface {
    Sender
    UserID() uint
    FirstName() string
}

type ClientSender interface {
    Sender
    ClientID() uint
    ContactID() uint
}
```

### 5. Transport Provider Abstraction
Pluggable message delivery providers:
```go
type Provider interface {
    Transport() Transport
    Send(ctx context.Context, msg Message) error
    OnReceived(callback func(msg Message) error)
}
```

### 6. Event-Driven Architecture
Domain events published after state changes:
```go
event, _ := client.NewCreatedEvent(ctx, data)
event.Result = createdClient
publisher.Publish(event)
```

## API Contracts

### Client Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/crm/clients` | List clients with pagination |
| GET | `/crm/clients/:id` | Get client details |
| POST | `/crm/clients` | Create new client |
| PUT | `/crm/clients/:id` | Update client |
| DELETE | `/crm/clients/:id` | Delete client |
| GET | `/crm/clients/:id/contacts` | List client contacts |
| POST | `/crm/clients/:id/contacts` | Add contact to client |

### Chat Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/crm/chats` | List chats |
| GET | `/crm/chats/:id` | Get chat details with messages |
| POST | `/crm/chats/:id/messages` | Send message |
| PUT | `/crm/chats/:id/messages/:msgId/read` | Mark message as read |
| GET | `/crm/chats/:id/messages` | List chat messages (paginated) |

### Message Template Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/crm/instant-messages` | List templates |
| POST | `/crm/instant-messages` | Create template |
| PUT | `/crm/instant-messages/:id` | Update template |
| DELETE | `/crm/instant-messages/:id` | Delete template |

### Webhook Endpoints

| Provider | Endpoint | Purpose |
|----------|----------|---------|
| Twilio | `/crm/webhooks/twilio` | Inbound SMS/WhatsApp messages |
| Telegram | `/tg/webhook` | Telegram bot messages |

## Multi-Tenant Implementation

All CRM operations enforce tenant isolation:

```go
// Repository automatically adds tenant filter
func (r *ClientRepository) GetByID(ctx context.Context, id uint) (client.Client, error) {
    tenantID, _ := composables.UseTenantID(ctx)

    // Query always includes: WHERE id = $1 AND tenant_id = $2
    row := composables.UseTx(ctx).QueryRowContext(ctx,
        "SELECT ... FROM crm_clients WHERE id = $1 AND tenant_id = $2",
        id, tenantID,
    )
    // Parse and return
}
```

## Error Handling

CRM module uses standard error handling pattern:

```go
const op serrors.Op = "ClientService.GetByID"

client, err := r.GetByID(ctx, id)
if err != nil {
    if errors.Is(err, ErrNotFound) {
        return nil, serrors.E(op, serrors.KindNotFound, "client not found")
    }
    return nil, serrors.E(op, err)
}
```

## Testing Strategy

- **Domain Tests**: Test entity behavior in isolation
- **Service Tests**: Test service orchestration with mocked repositories
- **Repository Tests**: Test database operations with test database
- **Controller Tests**: Test HTTP handling with mocked services
- **Integration Tests**: Test full flow with real database

See the [Testkit module](../testkit/) for ITF framework details.
