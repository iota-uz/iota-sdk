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
- `DecimalField` - High-precision decimal fields for financial data
- `DateField` - Date-only fields
- `TimeField` - Time-only fields
- `DateTimeField` - Combined date and time fields
- `TimestampField` - Unix timestamp fields
- `UUIDField` - UUID fields
- `SelectField` - Dropdown fields with options (static, searchable, or multi-select)

## Field Definition Examples

### String Field
```go
crud.NewStringField("name",
    crud.WithSearchable(),      // Enable text search
    crud.WithMinLen(3),         // Minimum length
    crud.WithMaxLen(100),       // Maximum length
    crud.WithPattern("^[A-Z]"), // Regex pattern
    crud.WithMultiline(),       // Textarea in forms
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

### Decimal Field
```go
crud.NewDecimalField("price",
    crud.WithPrecision(10),     // Total digits
    crud.WithScale(2),          // Decimal places
    crud.WithDecimalMin("0.00"),
    crud.WithDecimalMax("999999.99"),
    crud.WithRequired(),
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
    crud.WithReadonly(),
    crud.WithInitialValue(func() any { return time.Now() }),
)
```

### UUID Field
```go
crud.NewUUIDField("id",
    crud.WithKey(),          // Primary key
    crud.WithReadonly(),     // Auto-generated
    crud.WithInitialValue(func() any { return uuid.New() }),
)
```

### Select Field
```go
// Static select with string values
statusField := crud.NewSelectField("status").
    WithStaticOptions(
        crud.SelectOption{Value: "active", Label: "Active"},
        crud.SelectOption{Value: "inactive", Label: "Inactive"},
        crud.SelectOption{Value: "pending", Label: "Pending"},
    ).
    SetPlaceholder("Select status")

// Select with integer values
categoryField := crud.NewSelectField("category_id").
    AsIntSelect().
    WithStaticOptions(
        crud.SelectOption{Value: 1, Label: "Electronics"},
        crud.SelectOption{Value: 2, Label: "Clothing"},
        crud.SelectOption{Value: 3, Label: "Books"},
    )

// Boolean select
activeField := crud.NewSelectField("is_active").
    AsBoolSelect().
    WithStaticOptions(
        crud.SelectOption{Value: true, Label: "Yes"},
        crud.SelectOption{Value: false, Label: "No"},
    )

// Searchable select
productField := crud.NewSelectField("product_id").
    AsIntSelect().
    AsSearchable("/api/products/search").
    SetPlaceholder("Search products...")

// Multi-select combobox
tagsField := crud.NewSelectField("tags").
    WithCombobox("/api/tags/search", true).
    SetPlaceholder("Select tags")

// Dynamic options loader
departmentField := crud.NewSelectField("department_id").
    AsIntSelect().
    SetOptionsLoader(func(ctx context.Context) []crud.SelectOption {
        departments := departmentService.GetAll(ctx)
        options := make([]crud.SelectOption, len(departments))
        for i, dept := range departments {
            options[i] = crud.SelectOption{
                Value: dept.ID,      // Use actual int value
                Label: dept.Name,
            }
        }
        return options
    })
```

## Field Options

Common field options:

- `WithKey()` - Mark as primary key
- `WithReadonly()` - Read-only in forms
- `WithHidden()` - Hide from UI
- `WithSearchable()` - Enable text search (string fields only)
- `WithRequired()` - Add required validation
- `WithInitialValue(func() any)` - Set default value
- `WithRule(FieldRule)` - Add custom validation

## Select Field Features

### Select Types

1. **Static Select** - Fixed list of options
2. **Searchable Select** - Async search with API endpoint
3. **Combobox** - Multi-select with search capabilities

### Value Types

SelectField supports different underlying value types:
- String (default)
- Integer (`AsIntSelect()`)
- Boolean (`AsBoolSelect()`)
- Float (`AsFloatSelect()`)

### Label Display

The CRUD controller automatically displays select option labels instead of raw values in:
- **List views** - Table cells show labels
- **Detail views** - Field values show labels
- **Edit forms** - Dropdowns show labels with values

If no matching option is found for a value, the raw value is displayed as a fallback.

### API Endpoints for Searchable Selects

For searchable selects, create endpoints that:
1. Accept query parameter `?q=searchterm`
2. Return HTML rendered with `SearchOptions` component
3. Handle search logic server-side

```go
func (c *ProductController) SearchProducts(w http.ResponseWriter, r *http.Request) {
    query := r.URL.Query().Get("q")
    products := c.productService.Search(query)
    
    options := make([]*selects.Value, len(products))
    for i, product := range products {
        options[i] = &selects.Value{
            Value: strconv.Itoa(product.ID),
            Label: product.Name,
        }
    }
    
    props := &selects.SearchOptionsProps{
        Options: options,
        NothingFoundText: "No products found",
    }
    
    templ.Handler(selects.SearchOptions(props)).ServeHTTP(w, r)
}
```

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
        case "category_id":
            // For select fields with int values
            categoryID, _ := v.AsInt()
            product.CategoryID = categoryID
        case "status":
            // For select fields with string values
            status, _ := v.AsString()
            product.Status = status
        }
    }
    
    return product, nil
}

func (m *productMapper) ToFieldValues(ctx context.Context, entity Product) ([]crud.FieldValue, error) {
    return m.fields.FieldValues(map[string]any{
        "id":          entity.ID,
        "name":        entity.Name,
        "price":       entity.Price,
        "category_id": entity.CategoryID,
        "status":      entity.Status,
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
// Define fields with select field
fields := crud.NewFields([]crud.Field{
    crud.NewUUIDField("id", crud.WithKey(), crud.WithReadonly()),
    crud.NewStringField("name", 
        crud.WithRequired(),
        crud.WithSearchable(),
        crud.WithMinLen(3),
        crud.WithMaxLen(100),
    ),
    crud.NewSelectField("category_id").
        AsIntSelect().
        WithStaticOptions(
            crud.SelectOption{Value: 1, Label: "Electronics"},
            crud.SelectOption{Value: 2, Label: "Clothing"},
            crud.SelectOption{Value: 3, Label: "Books"},
        ),
    crud.NewSelectField("status").
        WithStaticOptions(
            crud.SelectOption{Value: "active", Label: "Active"},
            crud.SelectOption{Value: "inactive", Label: "Inactive"},
            crud.SelectOption{Value: "pending", Label: "Pending"},
        ),
    crud.NewDecimalField("price",
        crud.WithPrecision(10),
        crud.WithScale(2),
        crud.WithDecimalMin("0.00"),
    ),
    crud.NewBoolField("active",
        crud.WithInitialValue(func() any { return true }),
    ),
    crud.NewDateTimeField("created_at",
        crud.WithReadonly(),
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

// Select field validation
priorityField := crud.NewSelectField("priority").
    AsIntSelect().
    WithStaticOptions(/* options */).
    WithRequired().
    WithRule(func(fv crud.FieldValue) error {
        val, _ := fv.AsInt()
        if val > 10 {
            return errors.New("priority cannot exceed 10")
        }
        return nil
    })
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

## Field Labels and Localization

The CRUD controller automatically generates field labels using internationalization (i18n) patterns:

### Label Resolution Pattern

Field labels are resolved using the following pattern:
```
{SchemaName}.Fields.{FieldName}
```

For example, with a schema named "products" and a field named "name":
```
products.Fields.name
```

### Setting Up Custom Labels

Create translations in your module's locale files:

**English** (`modules/{module}/presentation/locales/en.json`):
```json
{
  "products": {
    "Fields": {
      "name": "Product Name",
      "price": "Unit Price",
      "category_id": "Category",
      "status": "Status",
      "active": "Is Active",
      "created_at": "Date Created"
    }
  }
}
```

**Russian** (`modules/{module}/presentation/locales/ru.json`):
```json
{
  "products": {
    "Fields": {
      "name": "Название товара",
      "price": "Цена за единицу",
      "category_id": "Категория",
      "status": "Статус",
      "active": "Активен",
      "created_at": "Дата создания"
    }
  }
}
```

### Label Fallback Behavior

If no translation is found, the system falls back to the field name itself:
1. First, tries to find translation using `{SchemaName}.Fields.{FieldName}`
2. If not found, uses the field name as-is (e.g., "name", "created_at")

### Boolean Field Special Labels

Boolean fields support additional custom labels for their values:

```go
crud.NewBoolField("active",
    crud.WithTrueLabel("Active"),    // Custom label for true value
    crud.WithFalseLabel("Inactive"), // Custom label for false value
)
```

These labels are used in:
- Table displays (showing "Active"/"Inactive" instead of "true"/"false")
- Form displays
- Detail views

### Select Field Label Display

Select fields automatically display option labels instead of raw values in all views:

1. **List View**: Table cells show the label of the selected option
2. **Details View**: Shows the label of the selected option
3. **Forms**: Dropdown shows labels with underlying values

Example:
```go
// Define select field
statusField := crud.NewSelectField("status").
    WithStaticOptions(
        crud.SelectOption{Value: "act", Label: "Active"},
        crud.SelectOption{Value: "inact", Label: "Inactive"},
    )

// In the database: status = "act"
// In list view: displays "Active"
// In details view: displays "Active"
// In edit form: dropdown shows "Active" as selected
```

For dynamic options, labels are resolved when the options loader is called:
```go
categoryField := crud.NewSelectField("category_id").
    AsIntSelect().
    SetOptionsLoader(func(ctx context.Context) []crud.SelectOption {
        // Load categories and return with labels
        categories := categoryService.List(ctx)
        options := make([]crud.SelectOption, len(categories))
        for i, cat := range categories {
            options[i] = crud.SelectOption{
                Value: cat.ID,   // int value
                Label: cat.Name, // displayed label
            }
        }
        return options
    })
```

### Example: Complete Localization Setup

```go
// Schema definition
schema := crud.NewSchema("products", fields, mapper)

// Locale files setup required:
// modules/mymodule/presentation/locales/en.json
{
  "products": {
    "Fields": {
      "id": "Product ID",
      "name": "Product Name", 
      "description": "Description",
      "price": "Price ($)",
      "category_id": "Category",
      "status": "Status",
      "active": "Status",
      "created_at": "Created Date"
    }
  }
}
```

## Best Practices

1. **Field Naming**: Use descriptive database column names that work as fallback labels
2. **Validation**: Combine field rules with entity validators
3. **Hooks**: Use hooks for timestamps and computed fields
4. **Events**: Subscribe to events for side effects
5. **Custom Logic**: Override repository/service for complex logic
6. **Localization**: Always provide translations for field labels in all supported languages
7. **Label Keys**: Use consistent naming patterns for translation keys across modules
8. **Select Fields**: 
   - Use appropriate value types (int for IDs, string for codes, bool for yes/no)
   - Provide meaningful labels that users understand
   - For large datasets, use searchable selects instead of static options
   - Consider using dynamic options loaders for data that changes frequently
9. **Type Safety**: Match SelectField value types to your domain model to avoid conversion errors
10. **Performance**: For select fields with many options, use searchable selects to reduce initial page load

## Migration Guide

### From Manual Select Implementation

If you have existing manual select implementations:

1. Replace manual option building with `WithStaticOptions`
2. Convert search endpoints to return `SearchOptions` component
3. Update form field creation to use `crud.NewSelectField`
4. Remove custom select handling from controllers
5. Update mappers to handle select field values with proper type conversion

### Example Migration

**Before**:
```go
// Manual select in form
<select name="category_id">
    <option value="1">Electronics</option>
    <option value="2">Clothing</option>
</select>

// Manual handling in controller
categoryID, _ := strconv.Atoi(r.FormValue("category_id"))
```

**After**:
```go
// Field definition
crud.NewSelectField("category_id").
    AsIntSelect().
    WithStaticOptions(
        crud.SelectOption{Value: 1, Label: "Electronics"},
        crud.SelectOption{Value: 2, Label: "Clothing"},
    )

// Automatic handling - no custom code needed
// The CRUD controller handles form parsing and type conversion
// Labels are automatically displayed in views
```

## Troubleshooting

### Select Field Values Not Displaying

1. **Check value types**: Ensure SelectOption.Value matches the field's ValueType
2. **Verify options**: Check that options are properly loaded (static or dynamic)
3. **Compare values**: The comparison is type-sensitive (1 != "1")

### Labels Not Showing

1. **Static options**: Verify Label field is set in SelectOption
2. **Dynamic options**: Ensure options loader returns options with labels
3. **Fallback**: If no matching option, raw value is displayed

### Form Submission Errors

1. **Type conversion**: Check that form values can be converted to field's ValueType
2. **Validation**: Ensure selected values pass field validation rules
3. **Required fields**: Check if select field is marked as required