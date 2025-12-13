---
layout: default
title: Technical Architecture
parent: Warehouse Module
nav_order: 2
permalink: /docs/warehouse/technical
---

# Warehouse Module - Technical Architecture

## Directory Structure

```
modules/warehouse/
├── domain/                           # Domain Layer (Business Logic)
│   └── aggregates/
│       ├── position/
│       │   ├── position.go          # Position aggregate interface
│       │   ├── position_impl.go     # Position implementation
│       │   ├── repository.go        # Position repository interface
│       │   ├── events.go            # Domain events
│       │   └── types.go             # Position enums
│       ├── product/
│       │   ├── product.go           # Product aggregate interface
│       │   ├── product_impl.go      # Product implementation
│       │   ├── repository.go        # Product repository interface
│       │   ├── events.go            # Product events
│       │   ├── status.go            # Product status enum
│       │   └── value_objects.go     # Value objects (RFID, etc.)
│       └── order/
│           ├── order.go             # Order aggregate interface
│           ├── order_impl.go        # Order implementation
│           ├── item.go              # Item entity
│           ├── repository.go        # Order repository interface
│           ├── events.go            # Order events
│           ├── status.go            # Order status enum
│           ├── value_objects.go     # Order value objects
│           └── order_dto.go         # Data transfer objects
│
│   ├── entities/
│   │   └── inventory/
│   │       ├── inventory.go         # Inventory check entity
│   │       ├── check.go             # Check structure
│   │       ├── repository.go        # Inventory repository interface
│   │       ├── events.go            # Inventory events
│   │       └── types.go             # Inventory enums
│
│   └── value_objects/
│       └── unit.go                  # Unit value object
│
├── infrastructure/                   # Infrastructure Layer
│   ├── persistence/
│   │   ├── unit_repository.go       # Unit repository implementation
│   │   ├── product_repository.go    # Product repository implementation
│   │   ├── position_repository.go   # Position repository implementation
│   │   ├── order_repository.go      # Order repository implementation
│   │   ├── inventory_repository.go  # Inventory repository implementation
│   │   ├── models/
│   │   │   ├── models.go            # ORM models
│   │   │   └── mappers/
│   │   │       ├── warehouse_mappers.go      # Domain ↔ DB mapping
│   │   │       └── order_mappers.go          # Order specific mapping
│   │   └── schema/
│   │       └── warehouse-schema.sql # Database migrations
│   └── persistence/query/
│       └── queries.go               # Reusable query fragments
│
├── presentation/                     # Presentation Layer
│   ├── controllers/
│   │   ├── product_controller.go    # Product HTTP handlers
│   │   ├── position_controller.go   # Position HTTP handlers
│   │   ├── order_controller.go      # Order HTTP handlers
│   │   ├── inventory_controller.go  # Inventory HTTP handlers
│   │   ├── unit_controller.go       # Unit HTTP handlers
│   │   ├── position_import_config.go
│   │   ├── position_row_handler.go
│   │   ├── upload_service_adapter.go
│   │   └── dtos/
│   │       ├── product_dto.go
│   │       ├── position_dto.go
│   │       ├── order_dto.go
│   │       └── inventory_dto.go
│   ├── templates/
│   │   └── pages/
│   │       ├── products/            # Product pages
│   │       ├── positions/           # Position/catalog pages
│   │       ├── orders/              # Order pages
│   │       └── inventory/           # Inventory check pages
│   ├── viewmodels/
│   │   ├── product_viewmodel.go
│   │   ├── position_viewmodel.go
│   │   ├── order_viewmodel.go
│   │   └── inventory_viewmodel.go
│   ├── mappers/
│   │   └── warehouse_presenters.go  # ViewModel mappers
│   ├── locales/
│   │   ├── en.json                  # English translations
│   │   ├── ru.json                  # Russian translations
│   │   └── uz.json                  # Uzbek translations
│   └── assets/
│       └── css/                     # Module-specific styles
│
├── services/
│   ├── unit_service.go              # Unit service
│   ├── productservice/
│   │   └── product_service.go       # Product service
│   ├── positionservice/
│   │   ├── position_service.go      # Position service
│   │   └── position_validator.go    # Position validation
│   ├── orderservice/
│   │   ├── order_service.go         # Order service
│   │   └── order_validator.go       # Order validation
│   └── inventory_service.go         # Inventory check service
│
├── interfaces/
│   └── graph/
│       ├── schema.graphql           # GraphQL schema definition
│       ├── generated.go             # Generated resolver code
│       └── resolver.go              # Custom resolver implementations
│
├── permissions/
│   ├── constants.go                 # Permission definitions
│   └── module.go                    # Permission registration
│
├── gqlgen.yml                        # GraphQL code generation config
├── module.go                         # Module registration
└── README.md                         # Module documentation
```

## Layer Separation

### Domain Layer

**Location**: `modules/warehouse/domain/aggregates/*`

**Key Components**:
- Position interface and implementation
- Product interface and implementation
- Order aggregate with Item entities
- Inventory Check entity
- Unit value object
- Domain events
- Repository interfaces

**Characteristics**:
- Pure business logic, no external dependencies
- Interfaces for aggregates (not structs)
- Immutable setters returning new instances
- Business rules enforced in domain
- Status enums for lifecycle tracking

**Example - Order Aggregate**:
```go
// Domain interface
type Order interface {
    ID() uint
    Type() Type
    Status() Status
    Items() []Item

    SetStatus(status Status) Order
    AddItem(position position.Position, products ...product.Product) (Order, error)
    Complete() (Order, error)
}

// Status lifecycle
type Status string
const (
    Draft Status = "draft"
    Processing Status = "processing"
    Completed Status = "completed"
)
```

### Service Layer

**Location**: `modules/warehouse/services/*`

**Key Services**:
- `UnitService`: Unit CRUD and management
- `ProductService`: Product creation, status updates
- `PositionService`: Position catalog management
- `OrderService`: Order processing and fulfillment
- `InventoryService`: Inventory checks and verification

**Responsibilities**:
- Orchestrate between repositories and domain
- Validate business rules
- Manage transactions
- Publish domain events
- Check permissions via `composables.CanUser()`
- Coordinate with upload service for images

**Example**:
```go
type PositionService struct {
    repo      position.Repository
    uploadSvc *upload.Service
    publisher eventbus.EventBus
}

func (s *PositionService) Create(ctx context.Context, data *position.CreateDTO) (*position.Position, error) {
    if err := composables.CanUser(ctx, permissions.PositionCreate); err != nil {
        return nil, err
    }

    // Build domain entity
    p := domain.NewPosition(data.Title, data.Unit)

    // Save to repository
    var saved *position.Position
    err := composables.InTx(ctx, func(txCtx context.Context) error {
        var err error
        saved, err = s.repo.Save(txCtx, p)
        return err
    })

    if err != nil {
        return nil, err
    }

    // Publish event
    event, _ := position.NewCreatedEvent(ctx, *data, *saved)
    s.publisher.Publish(event)

    return saved, nil
}
```

### Repository Layer

**Location**: `modules/warehouse/infrastructure/persistence/*`

**Responsibilities**:
- Implement domain repository interfaces
- Map between domain entities and database models
- Handle tenant isolation
- Support pagination and filtering
- Query optimization

**Key Features**:
- Automatic tenant_id filtering via `composables.UseTenantID(ctx)`
- Transaction support via `composables.InTx(ctx, fn)`
- Batch operations for import
- Query fragments for complex queries

**Example**:
```go
// Repository interface (domain layer)
type Repository interface {
    Save(ctx context.Context, p Position) (Position, error)
    GetByID(ctx context.Context, id uint) (Position, error)
    GetPaginated(ctx context.Context, params *FindParams) ([]Position, error)
    Delete(ctx context.Context, id uint) error
}

// Repository implementation (infrastructure layer)
func (r *PositionRepository) GetByID(ctx context.Context, id uint) (*position.Position, error) {
    tenantID, _ := composables.UseTenantID(ctx)
    tx := composables.UseTx(ctx)

    // Query always includes tenant_id
    row := tx.QueryRowContext(ctx,
        `SELECT id, tenant_id, title, barcode, unit_id, created_at, updated_at
         FROM warehouse_positions
         WHERE id = $1 AND tenant_id = $2`,
        id, tenantID,
    )

    var model models.WarehousePosition
    if err := row.Scan(&model.ID, &model.TenantID, ...); err != nil {
        return nil, err
    }

    return mappers.ToPositionDomain(&model), nil
}
```

### Presentation Layer

**Location**: `modules/warehouse/presentation/*`

**Components**:

#### Controllers
HTTP request handlers:
- Accept requests and parse form data
- Call services for business logic
- Render templates or return JSON
- Check permissions via middleware
- Handle errors with proper status codes

**Example**:
```go
type ProductsController struct {
    app     application.Application
    service *productservice.ProductService
}

func (c *ProductsController) Create(w http.ResponseWriter, r *http.Request) {
    formData, err := composables.UseForm(&CreateProductDTO{}, r)
    if err != nil {
        http.Error(w, "Invalid form data", http.StatusBadRequest)
        return
    }

    product, err := c.service.Create(r.Context(), formData)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    vm := NewProductViewModel(product)
    templates.ProductCreated(r.Context(), vm).Render(r.Context(), w)
}
```

#### ViewModels
Transform domain entities to presentation:
```go
type ProductViewModel struct {
    ID       uint
    RFID     string
    Position PositionVM
    Status   string
    Created  string
}

func NewProductViewModel(p product.Product) ProductViewModel {
    return ProductViewModel{
        ID:     p.ID(),
        RFID:   p.Rfid(),
        Status: string(p.Status()),
        Created: p.CreatedAt().Format("2006-01-02"),
    }
}
```

#### Templates
Type-safe HTML templates using Templ:
```templ
templ ProductsList(ctx context.Context, products []ProductViewModel) {
    <table>
        <thead>
            <tr>
                <th>RFID</th>
                <th>Position</th>
                <th>Status</th>
                <th>Created</th>
            </tr>
        </thead>
        <tbody>
            for _, p := range products {
                <tr>
                    <td>{ p.RFID }</td>
                    <td>{ p.Position.Title }</td>
                    <td>{ p.Status }</td>
                    <td>{ p.Created }</td>
                </tr>
            }
        </tbody>
    </table>
}
```

## GraphQL API

**Schema Location**: `modules/warehouse/interfaces/graph/schema.graphql`

**Key Types**:
```graphql
type Product {
    id: ID!
    rfid: String!
    position: Position!
    status: ProductStatus!
    createdAt: Time!
}

type Position {
    id: ID!
    title: String!
    barcode: String
    unit: Unit!
    quantity: Int!
    images: [Upload!]!
    products: [Product!]!
}

type Order {
    id: ID!
    type: OrderType!
    status: OrderStatus!
    items: [OrderItem!]!
    createdAt: Time!
}

type InventoryCheck {
    id: ID!
    name: String!
    status: CheckStatus!
    results: [CheckResult!]!
    createdBy: User!
    createdAt: Time!
}
```

**Queries**:
- `products(limit, offset)` - List products
- `product(id)` - Get product by ID
- `positions(limit, offset)` - List positions
- `position(id)` - Get position with products
- `orders(limit, offset)` - List orders
- `order(id)` - Get order with items
- `inventoryChecks(limit, offset)` - List checks
- `inventoryCheck(id)` - Get check with results

**Mutations**:
- `createProduct` - Create product instance
- `updateProductStatus` - Change product status
- `createOrder` - Create warehouse order
- `completeOrder` - Mark order as completed
- `createInventoryCheck` - Initiate inventory count

## Data Flow

### Creating a Position

```
HTTP Request (form or API)
    ↓
Controller.Create
    ↓
Parse CreatePositionDTO
    ↓
Check Permission (PositionCreate)
    ↓
PositionService.Create
    ↓
Build domain.Position aggregate
    ↓
InTx() - start transaction
    ↓
PositionRepository.Save()
    ↓
Map domain to database model
    ↓
INSERT into warehouse_positions
    ↓
Return created position
    ↓
Publish PositionCreatedEvent
    ↓
Render response (HTML or JSON)
```

### Processing an Inbound Order

```
Create Order (type=inbound)
    ↓
Add Items (select positions + products)
    ↓
Update Product Status: Available
    ↓
Mark Order: Processing
    ↓
Verify all products accounted for
    ↓
Mark Order: Completed
    ↓
Update Position Quantities
    ↓
Publish OrderCompletedEvent
```

### Inventory Check

```
Create Inventory Check
    ↓
For each Position:
  └─ Record expected quantity
  └─ Count actual quantity
  └─ Calculate variance
    ↓
Complete Check
    ↓
Analyze Variances
    ↓
Flag issues > threshold
    ↓
Publish InventoryVarianceEvent
    ↓
Generate Report
```

## Key Design Patterns

### 1. Functional Options
Used for optional configuration:
```go
product.New(rfid, status,
    product.WithPosition(pos),
    product.WithID(123),
)
```

### 2. Immutable Aggregates
Setters return new instances:
```go
updatedOrder := order.SetStatus(newStatus).AddItem(pos, products...)
```

### 3. Repository Interface Injection
Services depend on interfaces, not implementations:
```go
type ProductService struct {
    repo product.Repository  // Interface
}
```

### 4. Value Objects
Encapsulate complex values:
```go
type Status string
func (s Status) IsValid() bool { ... }
func (s Status) CanTransitionTo(next Status) bool { ... }
```

### 5. Event-Driven Architecture
Publish events after state changes:
```go
event, _ := order.NewCompletedEvent(ctx, completedOrder)
publisher.Publish(event)
```

## Multi-Tenant Implementation

All warehouse operations enforce tenant isolation:

```go
// Repository automatically filters by tenant
func (r *PositionRepository) GetByID(ctx context.Context, id uint) (*Position, error) {
    tenantID, _ := composables.UseTenantID(ctx)

    // Query includes: WHERE id = $1 AND tenant_id = $2
    row := composables.UseTx(ctx).QueryRowContext(ctx,
        `SELECT ... FROM warehouse_positions
         WHERE id = $1 AND tenant_id = $2`,
        id, tenantID,
    )
    // Parse and return
}
```

## Error Handling

```go
const op serrors.Op = "PositionService.Create"

position, err := s.repo.GetByID(ctx, id)
if err != nil {
    if errors.Is(err, ErrNotFound) {
        return nil, serrors.E(op, serrors.KindNotFound, "position not found")
    }
    return nil, serrors.E(op, err)
}
```

## Testing Strategy

- **Domain Tests**: Entity behavior in isolation
- **Service Tests**: Service orchestration with mocked repositories
- **Repository Tests**: Database operations with test database
- **Controller Tests**: HTTP handling with mocked services
- **GraphQL Tests**: API queries and mutations
- **Integration Tests**: Full workflow with real database

See [Testing Guide](../../guides/backend/testing.md) for ITF framework details.

## Performance Optimization

### Indexes
```sql
CREATE INDEX idx_warehouse_products_rfid ON warehouse_products(tenant_id, rfid);
CREATE INDEX idx_warehouse_products_position ON warehouse_products(position_id);
CREATE INDEX idx_warehouse_positions_barcode ON warehouse_positions(tenant_id, barcode);
CREATE INDEX idx_warehouse_orders_status ON warehouse_orders(tenant_id, status);
CREATE INDEX idx_warehouse_inventory_checks_created ON warehouse_inventory_checks(tenant_id, created_at);
```

### Query Optimization
- Pagination for large result sets
- Batch loading of related entities
- Denormalization of frequently queried data
- Query caching where applicable
