---
layout: default
title: CRUD Package
parent: Advanced
nav_order: 2
description: "Generic CRUD operations and schema-driven development in IOTA SDK"
---

# CRUD Package

The CRUD Package (`pkg/crud`) provides a generic, type-safe framework for building Create, Read, Update, and Delete operations with minimal boilerplate code.

## Overview

The CRUD package enables:

- **Schema-Driven Development**: Define entities using structured schemas
- **Automatic CRUD Operations**: Generate repositories and services
- **Type-Safe Fields**: Field definitions with built-in validation
- **Customizable Validators**: Define custom validation rules
- **Event Publishing**: Automatic domain event integration
- **UI Generation**: Automatic form and table generation

## Core Concepts

### Schema
A schema defines the structure, fields, and behavior of an entity.

```go
schema := crud.NewSchema(
    "users",
    []crud.Field{
        crud.NewStringField("firstName"),
        crud.NewStringField("email"),
        crud.NewBoolField("active"),
    },
)
```

### Fields
Type-safe field definitions with validation and configuration.

```go
// String field with constraints
crud.NewStringField("name",
    crud.WithRequired(),
    crud.WithMinLen(3),
    crud.WithMaxLen(100),
    crud.WithPattern("^[A-Za-z ]+$"),
    crud.WithSearchable(),
)

// Integer field with range
crud.NewIntField("age",
    crud.WithRequired(),
    crud.WithMin(0),
    crud.WithMax(150),
)

// Decimal field for money
crud.NewDecimalField("price",
    crud.WithRequired(),
    crud.WithPrecision(10),
    crud.WithScale(2),
    crud.WithDecimalMin("0.00"),
)

// Date field
crud.NewDateField("birthDate",
    crud.WithRequired(),
    crud.WithMaxDate(time.Now()),
)

// Select field with options
crud.NewSelectField("status",
    crud.WithStaticOptions(
        crud.SelectOption{Value: "active", Label: "Active"},
        crud.SelectOption{Value: "inactive", Label: "Inactive"},
    ),
)
```

### Repositories
Auto-generated repositories handle persistence:

```go
// Interface generated from schema
type UserRepository interface {
    GetByID(ctx context.Context, id uuid.UUID) (User, error)
    Create(ctx context.Context, user User) (User, error)
    Update(ctx context.Context, user User) (User, error)
    Delete(ctx context.Context, id uuid.UUID) error
    GetPaginated(ctx context.Context, params *FindParams) ([]User, error)
}

// Usage
user, err := repo.GetByID(ctx, userID)
created, err := repo.Create(ctx, newUser)
```

### Services
Auto-generated services provide business logic:

```go
// Service with event publishing
type UserService interface {
    Create(ctx context.Context, dto *CreateUserDTO) (User, error)
    Update(ctx context.Context, id uuid.UUID, dto *UpdateUserDTO) (User, error)
    Delete(ctx context.Context, id uuid.UUID) error
    GetByID(ctx context.Context, id uuid.UUID) (User, error)
    GetPaginated(ctx context.Context, params *FindParams) ([]User, error)
}

// Usage
user, err := service.Create(ctx, &CreateUserDTO{
    FirstName: "John",
    Email: "john@example.com",
})
```

## Field Types Reference

### Text Fields
```go
// Simple string
crud.NewStringField("name")

// Email with validation
crud.NewStringField("email",
    crud.WithPattern("^[^@]+@[^@]+\\.[^@]+$"),
)

// Textarea
crud.NewStringField("description",
    crud.WithMultiline(),
)

// URL
crud.NewStringField("website",
    crud.WithPattern("^https?://"),
)

// UUID
crud.NewUUIDField("id",
    crud.WithKey(),
    crud.WithReadonly(),
)
```

### Numeric Fields
```go
// Integer
crud.NewIntField("quantity",
    crud.WithMin(0),
)

// Floating point
crud.NewFloatField("rating",
    crud.WithMin(0),
    crud.WithMax(5),
)

// High precision
crud.NewDecimalField("accountBalance",
    crud.WithPrecision(15),
    crud.WithScale(2),
)
```

### Boolean Field
```go
crud.NewBoolField("active",
    crud.WithTrueLabel("Active"),
    crud.WithFalseLabel("Inactive"),
)
```

### Date/Time Fields
```go
// Date only
crud.NewDateField("orderDate",
    crud.WithRequired(),
)

// Time only
crud.NewTimeField("deliveryTime")

// Date and time
crud.NewDateTimeField("createdAt",
    crud.WithReadonly(),
    crud.WithInitialValue(func(ctx context.Context) any {
        return time.Now()
    }),
)

// Unix timestamp
crud.NewTimestampField("lastModified")
```

### Select Fields
```go
// Static options
statusField := crud.NewSelectField("status",
    crud.WithStaticOptions(
        crud.SelectOption{Value: "draft", Label: "Draft"},
        crud.SelectOption{Value: "published", Label: "Published"},
    ),
)

// Searchable select
categoryField := crud.NewSelectField("category",
    crud.WithSearchable(),
    crud.WithFetch(func(ctx context.Context, query string) ([]SelectOption, error) {
        // Fetch options from database
        return categoryService.Search(ctx, query)
    }),
)

// Multi-select
tagsField := crud.NewSelectField("tags",
    crud.WithMultiple(),
    crud.WithStaticOptions(
        crud.SelectOption{Value: "featured", Label: "Featured"},
        crud.SelectOption{Value: "sale", Label: "On Sale"},
    ),
)
```

## Usage Patterns

### Basic Schema Definition

```go
package users

import "github.com/iota-uz/iota-sdk/pkg/crud"

func NewUserSchema() crud.Schema {
    return crud.NewSchema(
        "users",
        []crud.Field{
            crud.NewStringField("firstName",
                crud.WithRequired(),
                crud.WithMinLen(2),
            ),
            crud.NewStringField("lastName",
                crud.WithRequired(),
                crud.WithMinLen(2),
            ),
            crud.NewStringField("email",
                crud.WithRequired(),
                crud.WithUnique(),
            ),
            crud.NewBoolField("active",
                crud.WithInitialValue(func(ctx context.Context) any {
                    return true
                }),
            ),
        },
    )
}
```

### Schema with Custom Validation

```go
schema := crud.NewSchema(
    "products",
    fields,
    crud.WithValidator(func(ctx context.Context, entity interface{}) error {
        product := entity.(*Product)

        // Custom business logic validation
        if product.Price() <= 0 {
            return errors.New("price must be positive")
        }

        if product.StockLevel() < 0 {
            return errors.New("stock cannot be negative")
        }

        return nil
    }),
)
```

### Schema with Hooks

```go
schema := crud.NewSchema(
    "invoices",
    fields,
    crud.WithCreateHook(func(ctx context.Context, entity interface{}) error {
        invoice := entity.(*Invoice)
        // Generate invoice number
        invoice.SetNumber(generateInvoiceNumber(ctx))
        return nil
    }),
    crud.WithUpdateHook(func(ctx context.Context, entity interface{}) error {
        invoice := entity.(*Invoice)
        // Update modified timestamp
        invoice.SetModifiedAt(time.Now())
        return nil
    }),
    crud.WithDeleteHook(func(ctx context.Context, id uuid.UUID) error {
        // Archive instead of delete
        return archiveInvoice(ctx, id)
    }),
)
```

### Using Builder Pattern

```go
builder := crud.NewBuilder(
    schema,
    eventPublisher,
    crud.WithRepository(customRepository), // optional
    crud.WithService(customService),       // optional
)

repository := builder.Repository()
service := builder.Service()
controller := builder.Controller()
```

## Advanced Features

### Custom Field Rendering

```go
customField := crud.NewStringField("color",
    crud.WithRenderer(func(value interface{}) string {
        color := value.(string)
        return fmt.Sprintf(
            `<input type="color" value="%s">`,
            color,
        )
    }),
)
```

### Computed Fields

```go
schema := crud.NewSchema(
    "invoices",
    append(fields,
        crud.NewDecimalField("total",
            crud.WithReadonly(),
            crud.WithComputed(func(ctx context.Context, entity interface{}) interface{} {
                invoice := entity.(*Invoice)
                return invoice.CalculateTotal()
            }),
        ),
    ),
)
```

### Conditional Fields

```go
priorityField := crud.NewSelectField("priority",
    crud.WithCondition(func(ctx context.Context, entity interface{}) bool {
        order := entity.(*Order)
        return order.Status() == "urgent"
    }),
)
```

### Custom Query Filters

```go
repo.WithFilter(crud.Filter{
    Field: "status",
    Operator: "eq",
    Value: "active",
})

results, err := repo.GetPaginated(ctx, &crud.FindParams{
    Limit: 20,
    Offset: 0,
    Filters: []crud.Filter{
        {Field: "status", Operator: "eq", Value: "active"},
        {Field: "created_at", Operator: "gte", Value: time.Now().AddDate(0, -1, 0)},
    },
})
```

## Integration with Services

### Service Layer Usage

```go
// Services automatically have event publishing
service := builder.Service()

// Create triggers domain event
user, err := service.Create(ctx, &CreateUserDTO{
    FirstName: "John",
    Email: "john@example.com",
})

// Event published:
// {
//     type: "user.created",
//     aggregate_id: user.ID(),
//     data: createDTO,
//     timestamp: now,
// }
```

### Permission Checking

```go
// Services respect RBAC
service.Create(ctx, dto) // Checks 'users.create' permission

// Custom permission checks
crud.WithPermissionCheck(func(ctx context.Context, action string) error {
    user, _ := composables.UseUser(ctx)
    if !user.HasPermission(fmt.Sprintf("users.%s", action)) {
        return errors.New("permission denied")
    }
    return nil
})
```

## Database Migration

The CRUD package can auto-generate migrations:

```bash
# Generate migration from schema
go run ./cmd/migrate schema users > migrations/20240101_create_users.sql
```

Generated SQL:

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    first_name VARCHAR(255) NOT NULL,
    last_name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID REFERENCES users(id),
    CONSTRAINT unique_user_email_per_tenant UNIQUE (tenant_id, email)
);

CREATE INDEX idx_users_tenant ON users(tenant_id);
```

## Form Generation

The CRUD package auto-generates forms:

```templ
// Auto-generated from schema
templ CreateUserForm(schema crud.Schema) {
    <form method="post" action="/users">
        @FormField(schema.Field("firstName"))
        @FormField(schema.Field("lastName"))
        @FormField(schema.Field("email"))
        @FormField(schema.Field("active"))
        <button type="submit">Create User</button>
    </form>
}
```

## Table Generation

Auto-generate data tables:

```templ
templ UsersTable(users []User, schema crud.Schema) {
    <table>
        <thead>
            <tr>
                @for _, field := range schema.Fields() {
                    <th>{ field.Label() }</th>
                }
            </tr>
        </thead>
        <tbody>
            @for _, user := range users {
                <tr>
                    @for _, field := range schema.Fields() {
                        <td>{ field.Render(user.Field(field.Name())) }</td>
                    }
                </tr>
            }
        </tbody>
    </table>
}
```

## Performance Considerations

- **Pagination**: Use `Limit` and `Offset` for large datasets
- **Indexes**: Automatically created for searchable fields
- **Caching**: Consider caching frequently accessed entities
- **Batch Operations**: Process large datasets in batches

## Testing CRUD Operations

```go
func TestUserCRUD(t *testing.T) {
    schema := NewUserSchema()
    repo := builder.Repository()

    // Create
    user := repo.Create(ctx, newUser)

    // Read
    fetched, err := repo.GetByID(ctx, user.ID())
    assert.Equal(t, fetched.Email(), user.Email())

    // Update
    updated := fetched.WithEmail("newemail@example.com")
    repo.Update(ctx, updated)

    // Delete
    repo.Delete(ctx, user.ID())
}
```

---

For more information, see the [Advanced Features Overview](./index.md) or the [CRUD Package documentation](../crud-package.md).
