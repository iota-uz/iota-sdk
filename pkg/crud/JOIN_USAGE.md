# JOIN Support in CRUD Package

This document explains how to use SQL JOIN functionality in the CRUD package.

## Overview

The CRUD package supports two approaches for loading related data:

1. **Manual JOINs** - Direct JOIN configuration via `FindParams.Joins`
2. **Relation-Based JOINs** - Declarative relations using `RelationBuilder` (recommended)

Both approaches support INNER JOIN, LEFT JOIN, and RIGHT JOIN operations.

## Table of Contents

- [Quick Start (Manual JOINs)](#quick-start-manual-joins)
- [Relation Builder (Recommended)](#relation-builder-recommended)
- [BelongsTo Relations](#belongsto-relations)
- [HasMany Relations](#hasmany-relations)
- [Nested Relations](#nested-relations)
- [Mapping Related Entities](#mapping-related-entities)
- [JOIN Types](#join-types)
- [Table Aliases](#table-aliases)
- [Multiple JOINs](#multiple-joins)
- [Selecting Columns](#selecting-columns)
- [Combining with Filters](#combining-with-filters)
- [Get Single Entity with JOINs](#get-single-entity-with-joins)
- [Check Existence with JOINs](#check-existence-with-joins)
- [Implementation Notes](#implementation-notes)

---

## Quick Start (Manual JOINs)

```go
import "github.com/iota-uz/iota-sdk/pkg/crud"

// Define your entity with joined fields
type UserWithRole struct {
    ID       uint64
    Name     string
    RoleID   uint64
    RoleName string // Populated from JOIN
}

// Use JOINs in List()
params := &crud.FindParams{
    Joins: &crud.JoinOptions{
        Joins: []crud.JoinClause{
            {
                Type:        crud.JoinTypeInner,
                Table:       "roles",
                TableAlias:  "r",
                LeftColumn:  "users.role_id",
                RightColumn: "r.id",
            },
        },
        SelectColumns: []string{
            "users.*",
            "r.name as role_name",
        },
    },
}

users, err := repo.List(ctx, params)
```

---

## Relation Builder (Recommended)

The `RelationBuilder` provides a fluent API for declaring relationships between schemas. This approach:

- Automatically generates JOINs and SELECT columns
- Handles nested relations recursively
- Provides type-safe entity mapping
- Supports both BelongsTo (many-to-one) and HasMany (one-to-many) relationships

### Basic Usage

```go
import "github.com/iota-uz/iota-sdk/pkg/crud"

// Define schemas for related entities
var RoleSchema = crud.NewSchema[Role](
    "roles",
    crud.NewFields(...),
    roleMapper,
)

// Declare relations
relations := crud.NewRelationBuilder().
    BelongsTo("role", RoleSchema).
        LocalKey("role_id").
        RemoteKey("id").
        EntityField("role_entity").
        Mapper(roleMapper).
        SetOnParent(func(parent, child any) any {
            user := parent.(User)
            role := child.(Role)
            return user.WithRole(role)
        }).
    Build()

// Create schema with relations
var UserSchema = crud.NewSchemaWithRelations[User](
    "users",
    crud.NewFields(...),
    userMapper,
    relations,
)
```

---

## BelongsTo Relations

BelongsTo represents a many-to-one relationship where the foreign key is on the current table.

**Example: Vehicle belongs to VehicleType**

```go
relations := crud.NewRelationBuilder().
    BelongsTo("vt", VehicleTypeSchema).
        LocalKey("vehicle_type_id").   // FK column in vehicles table
        RemoteKey("id").               // PK column in vehicle_types table
        EntityField("vehicle_type").   // Name for entity field in mapper
        Mapper(vehicleTypeMapper).
        SetOnParent(func(parent, child any) any {
            v := parent.(Vehicle)
            vt := child.(VehicleType)
            return v.SetVehicleType(vt)
        }).
    Build()
```

**Generated SQL:**
```sql
SELECT vehicles.*, vt.id AS vt__id, vt.name AS vt__name
FROM vehicles
LEFT JOIN vehicle_types vt ON vehicles.vehicle_type_id = vt.id
```

### Chaining BelongsTo Relations

```go
relations := crud.NewRelationBuilder().
    BelongsTo("vt", VehicleTypeSchema).
        LocalKey("vehicle_type_id").
        EntityField("vehicle_type").
        // ... config
    BelongsTo("owner", PersonSchema).
        LocalKey("owner_id").
        EntityField("owner").
        // ... config
    Build()
```

---

## HasMany Relations

HasMany represents a one-to-many relationship where the foreign key is on the related table. HasMany relations are **automatically handled via JSON subqueries** (not JOINs) to avoid row multiplication.

**Example: Person has many Documents**

```go
relations := crud.NewRelationBuilder().
    HasMany("docs", DocumentSchema).
        LocalKey("id").                // PK column in persons table
        RemoteKey("person_id").        // FK column in documents table
        EntityField("documents").
        Mapper(documentMapper).
        SetOnParent(func(parent, child any) any {
            p := parent.(Person)
            docs := child.([]Document)
            return p.SetDocuments(docs)
        }).
    Build()
```

**Generated SQL:**
```sql
SELECT persons.*,
    (SELECT COALESCE(JSON_AGG(json_build_object(
        'id', docs.id,
        'person_id', docs.person_id,
        'type', docs.type,
        'number', docs.number
    )), '[]'::json)
    FROM documents docs
    WHERE docs.person_id = persons.id) AS docs__json
FROM persons
```

### Key Points for HasMany

1. **No JOINs** - HasMany relations generate JSON subqueries, not JOINs
2. **Automatic filtering** - The SDK automatically skips HasMany relations when building JOINs
3. **JSON aggregation** - Results are aggregated as JSON arrays using `JSON_AGG`
4. **Null handling** - Empty arrays return `'[]'::json` instead of NULL

### JSON Unmarshaling Requirement

**Important:** The child entity type must be JSON-deserializable. The SDK uses `json.Unmarshal` to parse the aggregated JSON array into a slice of entities.

**Option 1: Use JSON struct tags (recommended)**

```go
type Document struct {
    ID       uuid.UUID `json:"id"`
    PersonID uuid.UUID `json:"person_id"`
    Type     string    `json:"type"`
    Seria    string    `json:"seria"`
    Number   string    `json:"number"`
}
```

**Option 2: Implement `json.Unmarshaler` for custom parsing**

```go
func (d *Document) UnmarshalJSON(data []byte) error {
    // Custom unmarshaling logic
    var raw map[string]any
    if err := json.Unmarshal(data, &raw); err != nil {
        return err
    }
    // Map fields...
    return nil
}
```

**Note:** The JSON keys in your struct tags must match the column names in the database exactly (as specified in `json_build_object`). The SDK builds JSON with column names as keys.

---

## Nested Relations

Relations can be nested to load deeply related data. The SDK handles nested relations through the `Through` configuration and recursive relation discovery.

### Nested BelongsTo

```go
// VehicleType -> VehicleGroup (nested BelongsTo)
vehicleTypeRelations := crud.NewRelationBuilder().
    BelongsTo("vg", VehicleGroupSchema).
        LocalKey("group_id").
        EntityField("group").
        // ...
    Build()

var VehicleTypeSchema = crud.NewSchemaWithRelations[VehicleType](
    "vehicle_types",
    ...,
    vehicleTypeRelations,
)

// Vehicle -> VehicleType (includes nested VehicleGroup)
vehicleRelations := crud.NewRelationBuilder().
    BelongsTo("vt", VehicleTypeSchema).
        LocalKey("vehicle_type_id").
        EntityField("vehicle_type").
        // ...
    Build()
```

**Generated SQL:**
```sql
SELECT vehicles.*,
    vt.id AS vt__id, vt.name AS vt__name,
    vt__vg.id AS vt__vg__id, vt__vg.name AS vt__vg__name
FROM vehicles
LEFT JOIN vehicle_types vt ON vehicles.vehicle_type_id = vt.id
LEFT JOIN vehicle_groups vt__vg ON vt.group_id = vt__vg.id
```

### Nested HasMany (Inside BelongsTo)

HasMany relations nested inside BelongsTo are handled via subqueries within the parent JSON object.

```go
// Person has many Documents (nested in Vehicle's owner relation)
personRelations := crud.NewRelationBuilder().
    HasMany("docs", DocumentSchema).
        LocalKey("id").
        RemoteKey("person_id").
        // ...
    Build()

var PersonSchema = crud.NewSchemaWithRelations[Person](
    "persons",
    ...,
    personRelations,
)

// Vehicle -> Owner (Person with nested Documents)
vehicleRelations := crud.NewRelationBuilder().
    BelongsTo("owner", PersonSchema).
        LocalKey("owner_id").
        // ...
    Build()
```

---

## Mapping Related Entities

Use `RelationMapper` to automatically map related entities from query results.

### Creating a RelationMapper

```go
relationMapper := crud.NewRelationMapper[Vehicle](
    vehicleSchema.Fields(),
    vehicleMapper,
)

// Add relation mappings (called in schema setup)
for _, rel := range relations {
    relationMapper.AddRelation(rel)
}
```

### Using MapWithRelations

```go
// In repository
func (r *Repository) GetWithRelations(ctx context.Context, id uuid.UUID) (Vehicle, error) {
    // Execute query with JOINs...

    // Map result including relations
    entity, err := relationMapper.ToEntity(ctx, fieldValues)
    return entity, err
}
```

### SetOnParent Pattern

The `SetOnParent` function is called for each relation to attach the child entity to the parent:

```go
SetOnParent(func(parent, child any) any {
    p := parent.(Person)

    // For BelongsTo (single entity)
    if role, ok := child.(Role); ok {
        return p.WithRole(role)
    }

    // For HasMany (slice)
    if docs, ok := child.([]Document); ok {
        return p.WithDocuments(docs)
    }

    return parent
})
```

---

## JOIN Types

### INNER JOIN
Returns only rows where there's a match in both tables.

```go
crud.JoinClause{
    Type:        crud.JoinTypeInner,
    Table:       "roles",
    LeftColumn:  "users.role_id",
    RightColumn: "roles.id",
}
```

### LEFT JOIN (Default)
Returns all rows from the left table, with matched rows from right table (or NULL).

```go
crud.JoinClause{
    Type:        crud.JoinTypeLeft,
    Table:       "roles",
    LeftColumn:  "users.role_id",
    RightColumn: "roles.id",
}
```

### RIGHT JOIN
Returns all rows from the right table, with matched rows from left table (or NULL).

```go
crud.JoinClause{
    Type:        crud.JoinTypeRight,
    Table:       "roles",
    LeftColumn:  "users.role_id",
    RightColumn: "roles.id",
}
```

---

## Table Aliases

Use aliases to make queries clearer and avoid column name conflicts:

```go
crud.JoinClause{
    Type:        crud.JoinTypeLeft,
    Table:       "roles",
    TableAlias:  "r",  // Use 'r' instead of 'roles' in query
    LeftColumn:  "users.role_id",
    RightColumn: "r.id",
}
```

---

## Multiple JOINs

Chain multiple JOINs together:

```go
params := &crud.FindParams{
    Joins: &crud.JoinOptions{
        Joins: []crud.JoinClause{
            {
                Type:        crud.JoinTypeLeft,
                Table:       "roles",
                TableAlias:  "r",
                LeftColumn:  "users.role_id",
                RightColumn: "r.id",
            },
            {
                Type:        crud.JoinTypeLeft,
                Table:       "departments",
                TableAlias:  "d",
                LeftColumn:  "users.department_id",
                RightColumn: "d.id",
            },
        },
        SelectColumns: []string{
            "users.*",
            "r.name as role_name",
            "d.name as department_name",
        },
    },
}
```

---

## Selecting Columns

By default, `SELECT *` is used. Override with `SelectColumns`:

```go
crud.JoinOptions{
    SelectColumns: []string{
        "users.id",
        "users.name",
        "roles.name as role_name",
    },
}
```

---

## Combining with Filters

JOINs work seamlessly with existing filter functionality:

```go
params := &crud.FindParams{
    Query: "john",  // Search term
    Filters: []crud.Filter{
        {Column: "active", Filter: repo.Eq(true)},
    },
    Joins: &crud.JoinOptions{
        Joins: []crud.JoinClause{
            {
                Type:        crud.JoinTypeInner,
                Table:       "roles",
                LeftColumn:  "users.role_id",
                RightColumn: "roles.id",
            },
        },
    },
    Limit:  10,
    Offset: 0,
}
```

---

## Get Single Entity with JOINs

Use the `WithJoins()` option with the `Get()` method:

```go
// Get user by ID with role information
joins := &crud.JoinOptions{
    Joins: []crud.JoinClause{
        {
            Type:        crud.JoinTypeInner,
            Table:       "roles",
            TableAlias:  "r",
            LeftColumn:  "users.role_id",
            RightColumn: "r.id",
        },
    },
    SelectColumns: []string{"users.*", "r.name as role_name"},
}

idField := schema.Fields().KeyField()
user, err := repo.Get(ctx, idField.Value(123), crud.WithJoins(joins))
```

### Fallback Behavior

The `Get()` method automatically falls back to a regular query (without JOINs) when:
- No JOIN options are provided
- `Joins` is `nil` or empty

---

## Check Existence with JOINs

Use the `WithJoins()` option with the `Exists()` method:

```go
// Check if user exists with a specific role
joins := &crud.JoinOptions{
    Joins: []crud.JoinClause{
        {
            Type:        crud.JoinTypeInner,
            Table:       "roles",
            TableAlias:  "r",
            LeftColumn:  "users.role_id",
            RightColumn: "r.id",
        },
    },
}

idField := schema.Fields().KeyField()
exists, err := repo.Exists(ctx, idField.Value(123), crud.WithJoins(joins))
```

---

## Implementation Notes

- Uses `pkg/repo` query builders for type-safe SQL generation
- Automatically falls back to regular `List()` if no JOINs specified
- Maintains full backward compatibility
- Works with existing sorting, filtering, and pagination
- HasMany relations are always handled via JSON subqueries (never JOINs)
- Nested relations are discovered recursively via `BuildRelationsRecursive()`
- Column prefixes follow the pattern: `alias__field` (e.g., `vt__name`)
- Nested prefixes chain: `parent__child__field` (e.g., `vt__vg__name`)

## Examples

See `pkg/crud/examples/join_example_test.go` for complete working examples.
