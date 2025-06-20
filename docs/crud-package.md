# CRUD Package Documentation

The `pkg/crud` package provides a generic, type-safe framework for building Create, Read, Update, and Delete operations in the IOTA SDK. It follows a builder pattern and integrates seamlessly with the module system.

## Overview

The CRUD package consists of several key components:

- **Builder**: Orchestrates the creation of CRUD schemas, repositories, and services
- **Schema**: Defines the structure and validation rules for entities
- **Repository**: Handles database operations
- **Service**: Implements business logic with event publishing
- **Controller**: Provides HTTP endpoints with automatic UI generation
- **Fields**: Type-safe field definitions with validation

## Core Components

### Builder

The builder pattern simplifies CRUD setup:

```go
builder := crud.NewBuilder(
    schema,
    eventPublisher,
    crud.WithRepository(customRepo), // optional
    crud.WithService(customService),  // optional
)

schema := builder.Schema()
repository := builder.Repository()
service := builder.Service()
```

### Schema Definition

Schemas define entity structure and behavior:

```go
schema := crud.NewSchema(
    "table_name",
    fields,
    mapper,
    crud.WithValidator(entityValidator),
    crud.WithCreateHook(onCreate),
    crud.WithUpdateHook(onUpdate),
    crud.WithDeleteHook(onDelete),
)
```

### Field Types

The package supports various field types:

- `StringField` - Text fields with length constraints
- `IntField` - Integer fields with min/max values
- `BoolField` - Boolean fields with custom labels
- `FloatField` - Floating-point fields with precision
- `DateField` - Date-only fields
- `TimeField` - Time-only fields
- `DateTimeField` - Combined date and time fields
- `TimestampField` - Unix timestamp fields
- `UUIDField` - UUID fields

## Field Definition Examples

### String Field
```go
crud.NewStringField("name",
    crud.WithSearchable(true),  // Enable text search
    crud.WithMinLen(3),         // Minimum length
    crud.WithMaxLen(100),       // Maximum length
    crud.WithPattern("^[A-Z]"), // Regex pattern
    crud.WithMultiline(true),   // Textarea in forms
)
```

### Integer Field
```go
crud.NewIntField("age",
    crud.WithMin(0),
    crud.WithMax(150),
    crud.WithRequired(),
    crud.WithInitialValue(func() any { return 18 }),
)
```

### Boolean Field
```go
crud.NewBoolField("active",
    crud.WithTrueLabel("Active"),
    crud.WithFalseLabel("Inactive"),
    crud.WithInitialValue(func() any { return true }),
)
```

### Date/Time Fields
```go
// Date field
crud.NewDateField("birth_date",
    crud.WithMinDate(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)),
    crud.WithMaxDate(time.Now()),
)

// DateTime field
crud.NewDateTimeField("created_at",
    crud.WithReadonly(true),
    crud.WithInitialValue(func() any { return time.Now() }),
)
```

### UUID Field
```go
crud.NewUUIDField("id",
    crud.WithKey(true),      // Primary key
    crud.WithReadonly(true), // Auto-generated
    crud.WithInitialValue(func() any { return uuid.New() }),
)
```

## Field Options

Common field options:

- `WithKey(bool)` - Mark as primary key
- `WithReadonly(bool)` - Read-only in forms
- `WithHidden(bool)` - Hide from UI
- `WithSearchable(bool)` - Enable text search (string fields only)
- `WithRequired()` - Add required validation
- `WithInitialValue(func() any)` - Set default value
- `WithRule(FieldRule)` - Add custom validation

## Mapper Implementation

Mappers convert between entities and field values:

```go
type productMapper struct {
    fields crud.Fields
}

func (m *productMapper) ToEntity(ctx context.Context, values []crud.FieldValue) (Product, error) {
    product := Product{}
    
    for _, v := range values {
        switch v.Field().Name() {
        case "id":
            id, _ := v.AsUUID()
            product.ID = id
        case "name":
            name, _ := v.AsString()
            product.Name = name
        case "price":
            price, _ := v.AsFloat64()
            product.Price = price
        }
    }
    
    return product, nil
}

func (m *productMapper) ToFieldValues(ctx context.Context, entity Product) ([]crud.FieldValue, error) {
    return m.fields.FieldValues(map[string]any{
        "id":    entity.ID,
        "name":  entity.Name,
        "price": entity.Price,
    })
}
```

## Repository Operations

The default repository provides these operations:

```go
type Repository[T any] interface {
    GetAll(ctx context.Context) ([]T, error)
    Get(ctx context.Context, value FieldValue) (T, error)
    Exists(ctx context.Context, value FieldValue) (bool, error)
    Count(ctx context.Context, filters *FindParams) (int64, error)
    List(ctx context.Context, params *FindParams) ([]T, error)
    Create(ctx context.Context, values []FieldValue) (T, error)
    Update(ctx context.Context, values []FieldValue) (T, error)
    Delete(ctx context.Context, value FieldValue) (T, error)
}
```

### Query Parameters

```go
params := &crud.FindParams{
    Query:   "search text",     // Text search
    Filters: []crud.Filter{     // Field filters
        {Column: "status", Filter: repo.Equal("active")},
        {Column: "price", Filter: repo.GreaterThan(100)},
    },
    Limit:   20,
    Offset:  0,
    SortBy:  crud.SortBy{Field: "created_at", Desc: true},
}
```

## Service Layer

Services add business logic and event publishing:

```go
type Service[T any] interface {
    GetAll(ctx context.Context) ([]T, error)
    Get(ctx context.Context, value FieldValue) (T, error)
    Exists(ctx context.Context, value FieldValue) (bool, error)
    Count(ctx context.Context, params *FindParams) (int64, error)
    List(ctx context.Context, params *FindParams) ([]T, error)
    Save(ctx context.Context, entity T) (T, error)  // Create or Update
    Delete(ctx context.Context, value FieldValue) (T, error)
}
```

The service automatically:
- Validates entities before save
- Executes lifecycle hooks
- Publishes domain events
- Manages transactions

## HTTP Controller

The CRUD controller generates complete web interfaces:

```go
controller := controllers.NewCrudController(
    "/products",
    app,
    builder,
    controllers.WithoutDelete(),  // Disable delete
    controllers.WithoutCreate(),  // Disable create
    controllers.WithoutEdit(),    // Disable edit
)
```

Generated endpoints:
- `GET /products` - List with pagination and search
- `GET /products/{id}/details` - View details
- `GET /products/new` - Create form
- `POST /products` - Create entity
- `GET /products/{id}/edit` - Edit form
- `POST /products/{id}` - Update entity
- `DELETE /products/{id}` - Delete entity

## Complete Example

```go
// Define fields
fields := crud.NewFields([]crud.Field{
    crud.NewUUIDField("id", crud.WithKey(true), crud.WithReadonly(true)),
    crud.NewStringField("name", 
        crud.WithRequired(),
        crud.WithSearchable(true),
        crud.WithMinLen(3),
        crud.WithMaxLen(100),
    ),
    crud.NewFloatField("price",
        crud.WithFloatMin(0),
        crud.WithPrecision(2),
    ),
    crud.NewBoolField("active",
        crud.WithInitialValue(func() any { return true }),
    ),
    crud.NewDateTimeField("created_at",
        crud.WithReadonly(true),
        crud.WithInitialValue(func() any { return time.Now() }),
    ),
})

// Create schema
schema := crud.NewSchema(
    "products",
    fields,
    NewProductMapper(fields),
    crud.WithValidator(validateProduct),
)

// Build CRUD components
builder := crud.NewBuilder(schema, eventBus)

// Register in module
app.RegisterServices(builder.Service())
app.RegisterControllers(
    controllers.NewCrudController("/products", app, builder),
)
```

## Validation

### Field-Level Rules

```go
crud.NewStringField("email",
    crud.WithRule(crud.EmailRule()),
    crud.WithRequired(),
)
```

### Entity-Level Validation

```go
func validateProduct(product Product) error {
    if product.Price < product.Cost {
        return errors.New("price must be greater than cost")
    }
    return nil
}
```

## Event Handling

The service publishes events automatically:

```go
// Subscribe to events
eventBus.Subscribe(func(event *crud.CreatedEvent[Product]) {
    log.Printf("Product created: %v", event.Result)
})

eventBus.Subscribe(func(event *crud.UpdatedEvent[Product]) {
    log.Printf("Product updated: %v", event.Result)
})

eventBus.Subscribe(func(event *crud.DeletedEvent[Product]) {
    log.Printf("Product deleted: %v", event.Data)
})
```

## Custom Repository

To provide a custom repository implementation:

```go
type customProductRepo struct {
    schema crud.Schema[Product]
    // Add any additional fields needed
}

func NewCustomProductRepo(schema crud.Schema[Product]) crud.Repository[Product] {
    return &customProductRepo{
        schema: schema,
    }
}

// Implement all Repository interface methods
func (r *customProductRepo) Create(ctx context.Context, values []crud.FieldValue) (Product, error) {
    // Custom creation logic
    // You can use crud.DefaultRepository as a reference implementation
    return Product{}, nil
}

// ... implement other methods ...

// Use with builder
builder := crud.NewBuilder(
    schema,
    eventPublisher,
    crud.WithRepository(NewCustomProductRepo(schema)),
)
```

## Best Practices

1. **Field Naming**: Use database column names for field names
2. **Validation**: Combine field rules with entity validators
3. **Hooks**: Use hooks for timestamps and computed fields
4. **Events**: Subscribe to events for side effects
5. **Custom Logic**: Override repository/service for complex logic
6. **Localization**: Provide translations for field labels and errors