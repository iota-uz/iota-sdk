package examples

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/crud"
)

// UserWithRole demonstrates an entity that includes joined data
type UserWithRole struct {
	ID       uint64
	Name     string
	Email    string
	RoleID   uint64
	RoleName string // Populated from JOIN
}

// ExampleSimpleInnerJoin demonstrates a simple INNER JOIN
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
				"users.*",
				"r.name as role_name",
			},
		},
	}

	return repo.List(ctx, params)
}

// ExampleMultipleJoins demonstrates multiple JOINs
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

	return repo.List(ctx, params)
}

// ExampleJoinWithFilters demonstrates JOINs combined with filters
func ExampleJoinWithFilters(ctx context.Context, repo crud.Repository[UserWithRole]) ([]UserWithRole, error) {
	params := &crud.FindParams{
		Query: "john", // Search term
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
