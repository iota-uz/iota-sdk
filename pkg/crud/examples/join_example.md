# CRUD Package - JOIN Usage Examples

This document provides comprehensive examples of using JOIN functionality in the IOTA SDK CRUD package.

## Table of Contents

1. [UserWithRole Entity](#userwrole-entity)
2. [Simple INNER JOIN](#simple-inner-join)
3. [Multiple JOINs](#multiple-joins)
4. [JOINs with Filters](#joins-with-filters)
5. [Get Single Entity with JOINs](#get-single-entity-with-joins)
6. [Check Existence with JOINs](#check-existence-with-joins)

## UserWithRole Entity

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

## Simple INNER JOIN

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
                "users.*",           // All columns from users table
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

## Multiple JOINs

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

## JOINs with Filters

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

**Explanation:**
- The `Query` parameter applies search filters defined in the schema
- JOINs are applied before filters
- Limit and offset work as expected with JOINs

## Get Single Entity with JOINs

Fetch a single entity by ID with joined data:

```go
func ExampleGetWithJoins(ctx context.Context, repo crud.Repository[UserWithRole], schema crud.Schema[UserWithRole], userID int) (UserWithRole, error) {
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

    // Get the ID field from the schema
    idField := schema.Fields().KeyField()

    // Fetch the user with role information
    return repo.GetWithJoins(ctx, idField.Value(userID), params)
}
```

**Explanation:**
- `GetWithJoins` is used for fetching a single entity
- The primary key field is obtained from the schema
- Returns an error if the entity is not found

## Check Existence with JOINs

Check if an entity exists with specific JOIN conditions:

```go
func ExampleExistsWithJoins(ctx context.Context, repo crud.Repository[UserWithRole], schema crud.Schema[UserWithRole], userID int) (bool, error) {
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

    // Get the ID field from the schema
    idField := schema.Fields().KeyField()

    // Check if user exists with a role (INNER JOIN means only users with roles will exist)
    return repo.ExistsWithJoins(ctx, idField.Value(userID), params)
}
```

**Explanation:**
- `ExistsWithJoins` returns `true` if an entity exists matching both the ID and JOIN conditions
- With `JoinTypeInner`, existence requires a matching joined record
- More efficient than fetching the full entity when you only need to check existence

## Best Practices

1. **Always use table aliases** for clarity and to avoid column name conflicts
2. **Match aliased columns to struct fields** using `as` in `SelectColumns`
3. **Use INNER JOIN** when the joined record must exist
4. **Use LEFT JOIN** when the joined record is optional
5. **Validate your JoinOptions** - the package will return errors for invalid configurations
6. **Consider performance** - JOINs can be expensive on large tables
7. **Index your JOIN columns** - ensure foreign key columns are indexed for optimal performance
