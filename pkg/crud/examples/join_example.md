# CRUD Package - JOIN and Relations Examples

This document provides comprehensive examples of using JOIN functionality and the Relation Builder in the IOTA SDK CRUD package.

> **PostgreSQL Only:** All examples in this document are for PostgreSQL databases using the `pgx/v5` driver. The SQL snippets use PostgreSQL-specific functions and syntax:
> - `JSON_AGG()` and `json_build_object()` for HasMany aggregation
> - `COALESCE(..., '[]'::json)` type casts for null handling
>
> These queries will **not work** on non-PostgreSQL databases.

## Table of Contents

1. [Manual JOINs](#manual-joins)
2. [Relation Builder](#relation-builder)
3. [BelongsTo Relations](#belongsto-relations)
4. [HasMany Relations](#hasmany-relations)
5. [Nested Relations](#nested-relations)
6. [Security: SelectColumns Validation](#security-selectcolumns-validation)
7. [Best Practices](#best-practices)

---

## Manual JOINs

### UserWithRole Entity

First, define an entity struct that includes fields from joined tables:

```go
package examples

// UserWithRole demonstrates an entity that includes joined data
type UserWithRole struct {
    ID       uint64
    Name     string
    Email    string
    RoleID   uint64
    RoleName string // Populated from JOIN with roles table
}
```

**Key Points:**
- The `RoleName` field will be populated from the `roles` table via JOIN
- The struct must have fields matching the aliased columns in `SelectColumns`
- Use `as` aliases in SQL to map joined columns to struct fields

### Simple INNER JOIN

Fetch users with their role information using an INNER JOIN:

```go
func ExampleSimpleInnerJoin(ctx context.Context, repo crud.Repository[UserWithRole]) ([]UserWithRole, error) {
    params := &crud.FindParams{
        Limit:  10,
        Offset: 0,
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
                "users.*",             // All columns from users table
                "r.name as role_name", // Role name aliased to match RoleName field
            },
        },
    }

    return repo.List(ctx, params)
}
```

**Explanation:**
- `JoinTypeInner` ensures only users with matching roles are returned
- `TableAlias` ("r") provides a shorthand for the joined table
- `LeftColumn` and `RightColumn` specify the JOIN condition
- `SelectColumns` controls which columns are returned and their aliases

### Multiple JOINs

Combine data from multiple tables using multiple LEFT JOINs:

```go
func ExampleMultipleJoins(ctx context.Context, repo crud.Repository[UserWithRole]) ([]UserWithRole, error) {
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
            },
            SelectColumns: []string{
                "users.*",
                "r.name as role_name",
            },
        },
    }

    return repo.List(ctx, params)
}
```

**Explanation:**
- `JoinTypeLeft` returns all users even if they don't have a matching role
- Multiple JOIN clauses are processed in order
- Each joined column must have a corresponding field in the struct

### JOINs with Filters

Combine JOINs with search filters for more refined queries:

```go
func ExampleJoinWithFilters(ctx context.Context, repo crud.Repository[UserWithRole]) ([]UserWithRole, error) {
    params := &crud.FindParams{
        Query: "john", // Search term applied to searchable fields
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
        },
        Limit: 10,
    }

    return repo.List(ctx, params)
}
```

### Get Single Entity with JOINs

Fetch a single entity by ID with joined data:

```go
func ExampleGetWithJoins(ctx context.Context, repo crud.Repository[UserWithRole], schema crud.Schema[UserWithRole], userID int) (UserWithRole, error) {
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
        SelectColumns: []string{
            "users.*",
            "r.name as role_name",
        },
    }

    idField := schema.Fields().KeyField()
    return repo.Get(ctx, idField.Value(userID), crud.WithJoins(joins))
}
```

### Check Existence with JOINs

Check if an entity exists with specific JOIN conditions:

```go
func ExampleExistsWithJoins(ctx context.Context, repo crud.Repository[UserWithRole], schema crud.Schema[UserWithRole], userID int) (bool, error) {
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
    return repo.Exists(ctx, idField.Value(userID), crud.WithJoins(joins))
}
```

---

## Relation Builder

The `RelationBuilder` provides a fluent, declarative API for defining relationships between entities.

### Basic Setup

```go
import "github.com/iota-uz/iota-sdk/pkg/crud"

// Define schemas for related entities
var RoleSchema = crud.NewSchema[Role](
    "roles",
    crud.NewFields(
        crud.NewIntField("id").Key(),
        crud.NewStringField("name"),
    ),
    roleMapper,
)

// Declare relations using the builder
relations := crud.NewRelationBuilder().
    BelongsTo("role", RoleSchema).
        LocalKey("role_id").
        RemoteKey("id").
        EntityField("role_entity").
        Mapper(roleMapper).
        SetOnParent(func(parent, child any) any {
            user := parent.(User)
            if role, ok := child.(Role); ok {
                return user.WithRole(role)
            }
            return parent
        }).
    Build()

// Create schema with relations
var UserSchema = crud.NewSchemaWithRelations[User](
    "users",
    crud.NewFields(
        crud.NewUUIDField("id").Key(),
        crud.NewStringField("name"),
        crud.NewIntField("role_id"),
    ),
    userMapper,
    relations,
)
```

---

## BelongsTo Relations

BelongsTo represents a many-to-one relationship where the foreign key is on the current table.

### Vehicle -> VehicleType Example

```go
// VehicleType schema
var VehicleTypeSchema = crud.NewSchema[VehicleType](
    "reference.vehicle_types",
    crud.NewFields(
        crud.NewIntField("id").Key(),
        crud.NewStringField("name"),
        crud.NewIntField("group_id"),
    ),
    vehicleTypeMapper,
)

// Vehicle relations
vehicleRelations := crud.NewRelationBuilder().
    BelongsTo("vt", VehicleTypeSchema).
        LocalKey("vehicle_type_id").
        RemoteKey("id").
        EntityField("vehicle_type_entity").
        Mapper(vehicleTypeMapper).
        SetOnParent(func(parent, child any) any {
            v := parent.(Vehicle)
            if vt, ok := child.(VehicleType); ok {
                return v.SetVehicleType(vt)
            }
            return parent
        }).
    Build()
```

**Generated SQL:**
```sql
SELECT v.*, vt.id AS vt__id, vt.name AS vt__name, vt.group_id AS vt__group_id
FROM insurance.vehicles v
LEFT JOIN reference.vehicle_types vt ON v.vehicle_type_id = vt.id
```

---

## HasMany Relations

HasMany represents a one-to-many relationship. These are handled via JSON subqueries to avoid row multiplication.

### Person -> Documents Example

```go
// Document schema
var DocumentSchema = crud.NewSchema[Document](
    "insurance.person_documents",
    crud.NewFields(
        crud.NewUUIDField("id").Key(),
        crud.NewUUIDField("person_id"),
        crud.NewStringField("type"),
        crud.NewStringField("seria"),
        crud.NewStringField("number"),
    ),
    documentMapper,
)

// Person relations with HasMany
personRelations := crud.NewRelationBuilder().
    HasMany("docs", DocumentSchema).
        LocalKey("id").           // PK on persons table
        RemoteKey("person_id").   // FK on documents table
        EntityField("documents_entity").
        Mapper(documentMapper).
        SetOnParent(func(parent, child any) any {
            p := parent.(Person)
            if docs, ok := child.([]Document); ok {
                return p.SetDocuments(docs)
            }
            return parent
        }).
    Build()
```

**Generated SQL:**
```sql
SELECT p.*,
    (SELECT COALESCE(JSON_AGG(json_build_object(
        'id', docs.id,
        'person_id', docs.person_id,
        'type', docs.type,
        'seria', docs.seria,
        'number', docs.number
    )), '[]'::json)
    FROM insurance.person_documents docs
    WHERE docs.person_id = p.id) AS docs__json
FROM insurance.persons p
```

### Key Differences from BelongsTo

| Aspect | BelongsTo | HasMany |
|--------|-----------|---------|
| FK Location | On current table | On related table |
| SQL Strategy | JOIN | JSON subquery |
| Result Type | Single entity | Slice of entities |
| Row Multiplication | No | No (avoided via subquery) |

### JSON Unmarshaling Requirement

**Important:** HasMany child entities must be JSON-deserializable. The SDK uses `json.Unmarshal` to parse the aggregated JSON.

```go
// Child entity MUST have JSON tags matching database column names
type Document struct {
    ID       uuid.UUID `json:"id"`
    PersonID uuid.UUID `json:"person_id"`
    Type     string    `json:"type"`
    Seria    string    `json:"seria"`
    Number   string    `json:"number"`
}
```

Alternatively, implement `json.Unmarshaler` for custom parsing logic.

---

## Nested Relations

### Nested BelongsTo (Vehicle -> VehicleType -> VehicleGroup)

```go
// VehicleGroup schema
var VehicleGroupSchema = crud.NewSchema[VehicleGroup](
    "reference.vehicle_groups",
    crud.NewFields(
        crud.NewIntField("id").Key(),
        crud.NewStringField("name"),
    ),
    vehicleGroupMapper,
)

// VehicleType with nested BelongsTo to VehicleGroup
vehicleTypeRelations := crud.NewRelationBuilder().
    BelongsTo("vg", VehicleGroupSchema).
        LocalKey("group_id").
        RemoteKey("id").
        EntityField("group_entity").
        Mapper(vehicleGroupMapper).
        SetOnParent(func(parent, child any) any {
            vt := parent.(VehicleType)
            if vg, ok := child.(VehicleGroup); ok {
                return vt.SetGroup(vg)
            }
            return parent
        }).
    Build()

var VehicleTypeSchema = crud.NewSchemaWithRelations[VehicleType](
    "reference.vehicle_types",
    vehicleTypeFields,
    vehicleTypeMapper,
    vehicleTypeRelations,
)

// Vehicle -> VehicleType (automatically includes nested VehicleGroup)
vehicleRelations := crud.NewRelationBuilder().
    BelongsTo("vt", VehicleTypeSchema).
        LocalKey("vehicle_type_id").
        // ... rest of config
    Build()
```

**Generated SQL:**
```sql
SELECT v.*,
    vt.id AS vt__id, vt.name AS vt__name, vt.group_id AS vt__group_id,
    vt__vg.id AS vt__vg__id, vt__vg.name AS vt__vg__name
FROM insurance.vehicles v
LEFT JOIN reference.vehicle_types vt ON v.vehicle_type_id = vt.id
LEFT JOIN reference.vehicle_groups vt__vg ON vt.group_id = vt__vg.id
```

### Nested HasMany (Vehicle -> Owner -> Documents)

```go
// Person schema with HasMany documents
personRelations := crud.NewRelationBuilder().
    HasMany("docs", DocumentSchema).
        LocalKey("id").
        RemoteKey("person_id").
        // ... config
    Build()

var PersonSchema = crud.NewSchemaWithRelations[Person](
    "insurance.persons",
    personFields,
    personMapper,
    personRelations,
)

// Vehicle -> Owner (Person with nested Documents)
vehicleRelations := crud.NewRelationBuilder().
    BelongsTo("owner", PersonSchema).
        LocalKey("owner_id").
        // ... config
    Build()
```

**Generated SQL:**
```sql
SELECT v.*,
    owner.id AS owner__id, owner.name AS owner__name,
    (SELECT COALESCE(JSON_AGG(json_build_object(
        'id', docs.id,
        'person_id', docs.person_id,
        'type', docs.type
    )), '[]'::json)
    FROM insurance.person_documents docs
    WHERE docs.person_id = owner.id) AS owner__docs__json
FROM insurance.vehicles v
LEFT JOIN insurance.persons owner ON v.owner_id = owner.id
```

---

## Security: SelectColumns Validation

The CRUD package **automatically validates** `SelectColumns` to prevent SQL injection attacks.

### Allowed Column Specifications

Valid column specifications include:
- Simple columns: `"id"`, `"name"`, `"email"`
- Table-qualified columns: `"users.id"`, `"users.email"`
- Aliased columns: `"users.name AS user_name"`, `"roles.name as role_name"`
- Wildcards: `"*"`, `"users.*"`, `"roles.*"`

### Blocked Patterns

The validation **rejects** column specifications containing:
- SQL keywords: `UNION`, `SELECT`, `INSERT`, `UPDATE`, `DELETE`, `DROP`, `CREATE`, `ALTER`, `EXEC`, `EXECUTE`
- SQL comments: `--`, `/*`, `*/`
- Statement terminators: `;`
- Function calls: `COUNT(*)`, `SUM(amount)` (use raw SQL queries for aggregations)

### Example of Safe Usage

```go
// SAFE - All valid column specifications
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
            "users.*",             // Wildcard
            "r.name AS role_name", // Aliased column
            "users.id",            // Table-qualified
            "email",               // Simple column
        },
    },
}

// REJECTED - SQL injection attempts will be caught
params := &crud.FindParams{
    Joins: &crud.JoinOptions{
        SelectColumns: []string{
            "users.id; DROP TABLE users;",    // Contains semicolon
            "users.id UNION SELECT password", // Contains UNION
            "COUNT(users.id)",                // Contains function call
        },
    },
}
```

---

## Best Practices

1. **Use RelationBuilder for complex relationships** - It handles JOINs, column selection, and nested relations automatically

2. **Always use table aliases** for clarity and to avoid column name conflicts

3. **Match aliased columns to struct fields** using `as` in `SelectColumns`

4. **Use INNER JOIN** when the joined record must exist

5. **Use LEFT JOIN** (default) when the joined record is optional

6. **Validate your JoinOptions** - the package will return errors for invalid configurations

7. **Consider performance** - JOINs can be expensive on large tables

8. **Index your JOIN columns** - ensure foreign key columns are indexed for optimal performance

9. **Trust HasMany subqueries** - They prevent row multiplication and handle empty arrays gracefully

10. **Implement SetOnParent correctly** - Always type-check the child parameter before casting:

```go
SetOnParent(func(parent, child any) any {
    p := parent.(Person)

    // Type-check before casting
    if docs, ok := child.([]Document); ok {
        return p.SetDocuments(docs)
    }

    return parent // Return unchanged if type doesn't match
})
```

11. **For aggregations** - use raw SQL queries instead of trying to include functions in SelectColumns
