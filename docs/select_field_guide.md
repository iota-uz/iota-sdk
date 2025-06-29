# SelectField Implementation Guide

## Overview

The SelectField in IOTA SDK provides a flexible way to create dropdown/select fields in CRUD forms with support for static options, searchable selects, and multi-select comboboxes.

## Features

- **Multiple Select Types**: Static, Searchable, and Combobox
- **Type-Safe Values**: Support for string, int, and bool value types with `any` type for SelectOption.Value
- **Dynamic Options**: Load options from functions or API endpoints
- **Integration**: Seamless integration with crud_controller and form rendering

## Value Types

The SelectField supports different value types while maintaining type safety:

- **SelectOption.Value** is of type `any`, allowing you to use the actual type (int, bool, etc.)
- **Field Type** is determined by the ValueType (IntFieldType, BoolFieldType, etc.)
- **HTML Rendering** automatically converts values to strings for form rendering
- **Form Parsing** automatically converts string values back to the correct type

## Basic Usage

### 1. Static Select Field

```go
// Simple string select
statusField := crud.NewSelectField("Status").
    WithStaticOptions(
        crud.SelectOption{Value: "active", Label: "Active"},
        crud.SelectOption{Value: "inactive", Label: "Inactive"},
    ).
    SetPlaceholder("Select status")
```

### 2. Integer Value Select

```go
// Select with integer values (Value is now int, not string)
categoryField := crud.NewSelectField("CategoryID").
    AsIntSelect().
    WithStaticOptions(
        crud.SelectOption{Value: 1, Label: "Category 1"},
        crud.SelectOption{Value: 2, Label: "Category 2"},
    )
```

### 3. Boolean Select

```go
// Yes/No select for boolean fields (Value is now bool, not string)
activeField := crud.NewSelectField("IsActive").
    AsBoolSelect().
    WithStaticOptions(
        crud.SelectOption{Value: true, Label: "Yes"},
        crud.SelectOption{Value: false, Label: "No"},
    )
```

### 4. Searchable Select

```go
// Async search select
productField := crud.NewSelectField("ProductID").
    AsIntSelect().
    AsSearchable("/api/products/search").
    SetPlaceholder("Search products...")
```

### 5. Multi-Select Combobox

```go
// Multiple selection with search
tagsField := crud.NewSelectField("Tags").
    WithCombobox("/api/tags/search", true).
    SetPlaceholder("Select tags")
```

### 6. Dynamic Options

```go
// Load options dynamically
departmentField := crud.NewSelectField("DepartmentID").
    SetOptionsLoader(func() []crud.SelectOption {
        // Load from service/database
        departments := departmentService.GetAll()
        options := make([]crud.SelectOption, len(departments))
        for i, dept := range departments {
            options[i] = crud.SelectOption{
                Value: strconv.Itoa(dept.ID),
                Label: dept.Name,
            }
        }
        return options
    })
```

## API Endpoints for Searchable Selects

For searchable selects, the endpoint should:
1. Accept a query parameter `?q=searchterm`
2. Return HTML rendered with `SearchOptions` component
3. Handle the search logic server-side

Example endpoint:
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

## Integration with CRUD Schema

```go
type ProductSchema struct {
    fields crud.Fields
}

func NewProductSchema() *ProductSchema {
    fields := crud.NewFields([]crud.Field{
        crud.NewUUIDField("ID", crud.WithKey()),
        crud.NewStringField("Name", crud.WithRequired()),
        
        // Category select with integer values
        crud.NewSelectField("CategoryID").
            AsIntSelect().
            WithStaticOptions(
                crud.SelectOption{Value: 1, Label: "Electronics"},
                crud.SelectOption{Value: 2, Label: "Clothing"},
            ),
        
        // Status select
        crud.NewSelectField("Status").
            WithStaticOptions(
                crud.SelectOption{Value: "active", Label: "Active"},
                crud.SelectOption{Value: "inactive", Label: "Inactive"},
            ),
    })
    
    return &ProductSchema{fields: fields}
}
```

## Validation

Select fields support standard CRUD validation rules:

```go
priorityField := crud.NewSelectField("Priority").
    AsIntSelect().
    WithStaticOptions(/* options */).
    With(crud.WithRequired()).
    With(crud.WithRules(func(fv crud.FieldValue) error {
        val, _ := fv.AsInt()
        if val > 10 {
            return errors.New("priority cannot exceed 10")
        }
        return nil
    }))
```

## Best Practices

1. **Use appropriate value types**: Match the SelectField value type to your domain model
2. **Provide placeholders**: Help users understand what to select
3. **Limit options**: For large datasets, use searchable selects instead of static options
4. **Validate selections**: Ensure selected values are valid and authorized
5. **Handle empty values**: Consider if the field is required or optional

## Migration from Manual Implementation

If you have existing manual select implementations:

1. Replace manual option building with `WithStaticOptions`
2. Convert search endpoints to return `SearchOptions` component
3. Update form field creation to use `crud.NewSelectField`
4. Remove custom select handling from controllers