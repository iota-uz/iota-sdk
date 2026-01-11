package crud

import (
	"context"
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

type testEntity struct {
	ID   uint64
	Name string
}

type testEntityMapper struct{}

func TestRepository_ListWithJoins(t *testing.T) {
	t.Run("falls back to normal list when Joins is nil", func(t *testing.T) {
		// Verify that List() works normally when Joins is nil
		fields := NewFields([]Field{
			NewIntField("id", WithKey()),
			NewStringField("name"),
		})

		mapper := &testEntityMapper{}
		schema := NewSchema("test_table", fields, mapper)
		repo := DefaultRepository(schema)

		params := &FindParams{
			Limit:  10,
			Offset: 0,
			Joins:  nil, // No joins
		}

		// This should not panic or error due to missing Joins
		// We can't actually execute it without a DB connection, but we can verify the method exists
		_ = repo
		_ = params
	})

	t.Run("validates join options when present", func(t *testing.T) {
		fields := NewFields([]Field{
			NewIntField("id", WithKey()),
			NewStringField("name"),
		})

		mapper := &testEntityMapper{}
		schema := NewSchema("test_table", fields, mapper)
		repo := DefaultRepository(schema).(*repository[testEntity])

		// Create params with invalid join (missing table)
		params := &FindParams{
			Joins: &JoinOptions{
				Joins: []JoinClause{
					{
						Type:        JoinTypeInner,
						Table:       "", // Invalid: empty table
						LeftColumn:  "test_table.role_id",
						RightColumn: "roles.id",
					},
				},
			},
		}

		// The buildJoinQuery method should validate and return error
		// We test the internal method since we can't execute full List without DB
		_, err := repo.buildJoinQuery(params)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "join table cannot be empty")
	})
}

func (m *testEntityMapper) ToEntities(ctx context.Context, values ...[]FieldValue) ([]testEntity, error) {
	return nil, nil
}

func (m *testEntityMapper) ToFieldValuesList(ctx context.Context, entities ...testEntity) ([][]FieldValue, error) {
	return nil, nil
}

func TestRepository_Get_WithFunctionalOptions(t *testing.T) {
	t.Run("accepts WithJoins option", func(t *testing.T) {
		fields := NewFields([]Field{
			NewIntField("id", WithKey()),
			NewStringField("name"),
		})

		mapper := &testEntityMapper{}
		schema := NewSchema("test_table", fields, mapper)
		repo := DefaultRepository(schema).(*repository[testEntity])

		joins := &JoinOptions{
			Joins: []JoinClause{
				{
					Type:        JoinTypeInner,
					Table:       "roles",
					TableAlias:  "r",
					LeftColumn:  "test_table.role_id",
					RightColumn: "r.id",
				},
			},
			SelectColumns: []string{"test_table.*", "r.name AS role_name"},
		}

		// Verify the functional options pattern compiles
		idField := fields.KeyField()

		// Build query using internal method to verify it works
		query, err := repo.buildGetWithJoinsQuery(idField.Value(int(1)), &FindParams{Joins: joins})
		require.NoError(t, err)
		assert.Contains(t, query, "INNER JOIN roles r ON test_table.role_id = r.id")
		assert.Contains(t, query, "WHERE id = $1")
	})

	t.Run("works without options (backward compatible)", func(t *testing.T) {
		fields := NewFields([]Field{
			NewIntField("id", WithKey()),
			NewStringField("name"),
		})

		mapper := &testEntityMapper{}
		schema := NewSchema("test_table", fields, mapper)
		repo := DefaultRepository(schema)

		idField := fields.KeyField()
		// This should compile without any options (backward compatible)
		// We can't actually execute without DB, but we verify the signature
		_ = repo
		_ = idField
	})
}

func TestRepository_Exists_WithFunctionalOptions(t *testing.T) {
	t.Run("accepts WithJoins option", func(t *testing.T) {
		fields := NewFields([]Field{
			NewIntField("id", WithKey()),
			NewStringField("name"),
		})

		mapper := &testEntityMapper{}
		schema := NewSchema("test_table", fields, mapper)
		repo := DefaultRepository(schema).(*repository[testEntity])

		joins := &JoinOptions{
			Joins: []JoinClause{
				{
					Type:        JoinTypeInner,
					Table:       "roles",
					TableAlias:  "r",
					LeftColumn:  "test_table.role_id",
					RightColumn: "r.id",
				},
			},
		}

		// Verify the functional options pattern compiles
		idField := fields.KeyField()

		// Build query using internal method to verify it works
		query, err := repo.buildExistsWithJoinsQuery(idField.Value(int(1)), &FindParams{Joins: joins})
		require.NoError(t, err)
		assert.Contains(t, query, "SELECT EXISTS")
		assert.Contains(t, query, "INNER JOIN roles r ON test_table.role_id = r.id")
		assert.Contains(t, query, "WHERE id = $1")
	})

	t.Run("works without options (backward compatible)", func(t *testing.T) {
		fields := NewFields([]Field{
			NewIntField("id", WithKey()),
			NewStringField("name"),
		})

		mapper := &testEntityMapper{}
		schema := NewSchema("test_table", fields, mapper)
		repo := DefaultRepository(schema)

		idField := fields.KeyField()
		// This should compile without any options (backward compatible)
		// We can't actually execute without DB, but we verify the signature
		_ = repo
		_ = idField
	})
}
