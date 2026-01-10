package crud

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindParams_WithJoins(t *testing.T) {
	params := &FindParams{
		Query:  "test",
		Limit:  10,
		Offset: 0,
		Joins: &JoinOptions{
			Joins: []JoinClause{
				{
					Type:        JoinTypeInner,
					Table:       "roles",
					TableAlias:  "r",
					LeftColumn:  "users.role_id",
					RightColumn: "r.id",
				},
			},
		},
	}

	require.NotNil(t, params.Joins)
	assert.Len(t, params.Joins.Joins, 1)
	assert.Equal(t, JoinTypeInner, params.Joins.Joins[0].Type)
	assert.Equal(t, "roles", params.Joins.Joins[0].Table)
}

func TestFindParams_WithJoinsAndSelectColumns(t *testing.T) {
	params := &FindParams{
		Joins: &JoinOptions{
			Joins: []JoinClause{
				{
					Type:        JoinTypeLeft,
					Table:       "roles",
					LeftColumn:  "users.role_id",
					RightColumn: "roles.id",
				},
			},
			SelectColumns: []string{"users.*", "roles.name as role_name"},
		},
	}

	require.NotNil(t, params.Joins)
	require.NotNil(t, params.Joins.SelectColumns)
	assert.Len(t, params.Joins.SelectColumns, 2)
	assert.Equal(t, "users.*", params.Joins.SelectColumns[0])
}

func TestFindParams_NilJoins(t *testing.T) {
	params := &FindParams{
		Query:  "test",
		Limit:  10,
		Offset: 0,
		Joins:  nil,
	}

	assert.Nil(t, params.Joins)
}
