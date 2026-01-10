# JOIN Support in CRUD Package

This document explains how to use SQL JOIN functionality in the CRUD package.

## Overview

The CRUD package supports INNER JOIN, LEFT JOIN, and RIGHT JOIN operations through the `Repository.List()` method. JOINs are automatically applied when `FindParams.Joins` is provided.

## Quick Start

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

### LEFT JOIN
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

## Implementation Notes

- Uses `pkg/repo` query builders for type-safe SQL generation
- Automatically falls back to regular `List()` if no JOINs specified
- Maintains full backward compatibility
- Works with existing sorting, filtering, and pagination

## Get Single Entity with JOINs

### Using Functional Options Pattern (Recommended)

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

### Legacy Method (Deprecated)

The `GetWithJoins()` method is still available for backward compatibility:

```go
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
        SelectColumns: []string{"users.*", "r.name as role_name"},
    },
}

idField := schema.Fields().KeyField()
user, err := repo.GetWithJoins(ctx, idField.Value(123), params)
```

### Fallback Behavior

Both methods automatically fall back to the regular `Get()` query when:
- No JOIN options are provided
- `Joins` is `nil` or empty

This allows for flexible code that can conditionally include joins without separate logic branches.

## Check Existence with JOINs

### Using Functional Options Pattern (Recommended)

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

This is useful for checking if an entity exists with specific joined relationships, such as verifying a user has access to a resource through a role.

### Legacy Method (Deprecated)

The `ExistsWithJoins()` method is still available for backward compatibility:

```go
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
    },
}

idField := schema.Fields().KeyField()
exists, err := repo.ExistsWithJoins(ctx, idField.Value(123), params)
```

### Fallback Behavior

Both methods automatically fall back to the regular `Exists()` query when:
- No JOIN options are provided
- `Joins` is `nil` or empty

## Examples

See `pkg/crud/examples/join_example_test.go` for complete working examples.
