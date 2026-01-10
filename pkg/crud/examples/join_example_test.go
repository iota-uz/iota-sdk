package examples

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/crud"
)

// TestExampleCompiles verifies that the examples compile
func TestExampleCompiles(t *testing.T) {
	t.Run("join clause construction", func(t *testing.T) {
		join := crud.JoinClause{
			Type:        crud.JoinTypeInner,
			Table:       "roles",
			TableAlias:  "r",
			LeftColumn:  "users.role_id",
			RightColumn: "r.id",
		}

		if err := join.Validate(); err != nil {
			t.Errorf("Valid join clause failed validation: %v", err)
		}
	})

	t.Run("join options construction", func(t *testing.T) {
		opts := &crud.JoinOptions{
			Joins: []crud.JoinClause{
				{
					Type:        crud.JoinTypeLeft,
					Table:       "roles",
					LeftColumn:  "users.role_id",
					RightColumn: "roles.id",
				},
			},
			SelectColumns: []string{"users.*", "roles.name"},
		}

		if err := opts.Validate(); err != nil {
			t.Errorf("Valid join options failed validation: %v", err)
		}
	})
}
