package controllers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iota-uz/iota-sdk/pkg/repo"
)

// Helper function to verify SQL generation (this would be unit test for repository)
func TestSortByToSQL(t *testing.T) {
	sortBy := repo.SortBy[string]{
		Fields: []repo.SortByField[string]{
			{Field: "name", Ascending: true},
		},
	}

	fieldMap := map[string]string{
		"name":        "name",
		"description": "description",
	}

	sql := sortBy.ToSQL(fieldMap)
	assert.Contains(t, sql, "ORDER BY name ASC")

	// Test descending
	sortBy.Fields[0].Ascending = false
	sql = sortBy.ToSQL(fieldMap)
	assert.Contains(t, sql, "ORDER BY name DESC")

	// Test multiple fields
	sortBy.Fields = append(sortBy.Fields, repo.SortByField[string]{
		Field: "description", Ascending: true,
	})
	sql = sortBy.ToSQL(fieldMap)
	// Check that both fields are in the SQL with proper ordering
	assert.Contains(t, sql, "ORDER BY")
	assert.Contains(t, sql, "name DESC")
	assert.Contains(t, sql, "description ASC")
}
