package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEqFilter(t *testing.T) {
	filter := Eq("test")
	assert.Equal(t, "column = $1", filter.String("column", 1))
	assert.Equal(t, []any{"test"}, filter.Value())
}

func TestNotEqFilter(t *testing.T) {
	filter := NotEq("test")
	assert.Equal(t, "column != $1", filter.String("column", 1))
	assert.Equal(t, []any{"test"}, filter.Value())
}

func TestGtFilter(t *testing.T) {
	filter := Gt(10)
	assert.Equal(t, "column > $1", filter.String("column", 1))
	assert.Equal(t, []any{10}, filter.Value())
}

func TestGteFilter(t *testing.T) {
	filter := Gte(10)
	assert.Equal(t, "column >= $1", filter.String("column", 1))
	assert.Equal(t, []any{10}, filter.Value())
}

func TestLtFilter(t *testing.T) {
	filter := Lt(10)
	assert.Equal(t, "column < $1", filter.String("column", 1))
	assert.Equal(t, []any{10}, filter.Value())
}

func TestLteFilter(t *testing.T) {
	filter := Lte(10)
	assert.Equal(t, "column <= $1", filter.String("column", 1))
	assert.Equal(t, []any{10}, filter.Value())
}

func TestInFilter(t *testing.T) {
	t.Run("with values", func(t *testing.T) {
		filter := In([]string{"a", "b", "c"})
		assert.Equal(t, "column IN ($1, $2, $3)", filter.String("column", 1))
		assert.Equal(t, []any{"a", "b", "c"}, filter.Value())
	})

	t.Run("panic on non-slice", func(t *testing.T) {
		assert.Panics(t, func() {
			In("not a slice")
		})
	})
}

func TestNotInFilter(t *testing.T) {
	t.Run("with values", func(t *testing.T) {
		filter := NotIn([]int{1, 2, 3})
		assert.Equal(t, "column NOT IN ($1, $2, $3)", filter.String("column", 1))
		assert.Equal(t, []any{1, 2, 3}, filter.Value())
	})

	t.Run("panic on non-slice", func(t *testing.T) {
		assert.Panics(t, func() {
			NotIn("not a slice")
		})
	})
}

func TestLikeFilter(t *testing.T) {
	filter := Like("%test%")
	assert.Equal(t, "column LIKE $1", filter.String("column", 1))
	assert.Equal(t, []any{"%test%"}, filter.Value())
}

func TestNotLikeFilter(t *testing.T) {
	filter := NotLike("%test%")
	assert.Equal(t, "column NOT LIKE $1", filter.String("column", 1))
	assert.Equal(t, []any{"%test%"}, filter.Value())
}

func TestOrFilter(t *testing.T) {
	t.Run("simple OR", func(t *testing.T) {
		filter := Or(Eq("test"), Like("%sample%"))
		assert.Equal(t, "(column = $1 OR column LIKE $1)", filter.String("column", 1))
		assert.Equal(t, []any{"test", "%sample%"}, filter.Value())
	})

	t.Run("nested OR in OR", func(t *testing.T) {
		filter := Or(
			Eq("value1"),
			Or(
				Eq("value2"),
				Eq("value3"),
			),
		)
		assert.Equal(t, "(column = $1 OR (column = $1 OR column = $1))", filter.String("column", 1))
		assert.Equal(t, []any{"value1", "value2", "value3"}, filter.Value())
	})

	t.Run("nested AND in OR", func(t *testing.T) {
		filter := Or(
			Eq("value1"),
			And(
				Gt(5),
				Lt(10),
			),
		)
		assert.Equal(t, "(column = $1 OR (column > $1 AND column < $1))", filter.String("column", 1))
		assert.Equal(t, []any{"value1", 5, 10}, filter.Value())
	})
}

func TestAndFilter(t *testing.T) {
	t.Run("simple AND", func(t *testing.T) {
		filter := And(Gt(5), Lt(10))
		assert.Equal(t, "(column > $1 AND column < $1)", filter.String("column", 1))
		assert.Equal(t, []any{5, 10}, filter.Value())
	})

	t.Run("nested AND in AND", func(t *testing.T) {
		filter := And(
			Eq("value1"),
			And(
				Gt(5),
				Lt(10),
			),
		)
		assert.Equal(t, "(column = $1 AND (column > $1 AND column < $1))", filter.String("column", 1))
		assert.Equal(t, []any{"value1", 5, 10}, filter.Value())
	})

	t.Run("nested OR in AND", func(t *testing.T) {
		filter := And(
			Eq("value1"),
			Or(
				Like("%test%"),
				Like("%sample%"),
			),
		)
		assert.Equal(t, "(column = $1 AND (column LIKE $1 OR column LIKE $1))", filter.String("column", 1))
		assert.Equal(t, []any{"value1", "%test%", "%sample%"}, filter.Value())
	})
}

func TestComplexNestedFilters(t *testing.T) {
	filter := And(
		Or(
			Eq("value1"),
			And(
				Like("%partial%"),
				NotLike("%exclude%"),
			),
		),
		Or(
			Gt(100),
			And(
				Gte(50),
				Lte(75),
			),
			Lt(25),
		),
	)

	expectedSQL := "((column = $1 OR (column LIKE $1 AND column NOT LIKE $1)) AND (column > $1 OR (column >= $1 AND column <= $1) OR column < $1))"
	assert.Equal(t, expectedSQL, filter.String("column", 1))

	expectedValues := []any{"value1", "%partial%", "%exclude%", 100, 50, 75, 25}
	assert.Equal(t, expectedValues, filter.Value())
}

func TestFieldFilter(t *testing.T) {
	type UserFields string
	const (
		UserName  UserFields = "name"
		UserEmail UserFields = "email"
	)

	nameFilter := FieldFilter[UserFields]{
		Column: UserName,
		Filter: Eq("John"),
	}

	assert.Equal(t, "name", string(nameFilter.Column))
	assert.Equal(t, []any{"John"}, nameFilter.Filter.Value())
}

func TestSortBy(t *testing.T) {
	type UserFields string
	const (
		UserName      UserFields = "name"
		UserCreatedAt UserFields = "created_at"
		InvalidField  UserFields = "invalid_field"
	)

	fieldMapping := map[UserFields]string{
		UserName:      "users.name",
		UserCreatedAt: "users.created_at",
		// InvalidField intentionally not mapped
	}

	t.Run("ascending sort", func(t *testing.T) {
		sort := SortBy[UserFields]{
			Fields: []SortByField[UserFields]{
				{Field: UserName, Ascending: true},
				{Field: UserCreatedAt, Ascending: true},
			},
		}

		assert.Equal(t, UserName, sort.Fields[0].Field)
		assert.Equal(t, UserCreatedAt, sort.Fields[1].Field)
		assert.True(t, sort.Fields[0].Ascending)
		assert.True(t, sort.Fields[1].Ascending)

		sql := sort.ToSQL(fieldMapping)
		assert.Equal(t, "ORDER BY users.name ASC, users.created_at ASC", sql)
	})

	t.Run("descending sort", func(t *testing.T) {
		sort := SortBy[UserFields]{
			Fields: []SortByField[UserFields]{
				{Field: UserCreatedAt, Ascending: false},
			},
		}

		assert.Equal(t, UserCreatedAt, sort.Fields[0].Field)
		assert.False(t, sort.Fields[0].Ascending)

		sql := sort.ToSQL(fieldMapping)
		assert.Equal(t, "ORDER BY users.created_at DESC", sql)
	})

	t.Run("empty sort fields", func(t *testing.T) {
		sort := SortBy[UserFields]{
			Fields: []SortByField[UserFields]{},
		}

		sql := sort.ToSQL(fieldMapping)
		assert.Equal(t, "", sql)
	})

	t.Run("invalid field returns empty string", func(t *testing.T) {
		sort := SortBy[UserFields]{
			Fields: []SortByField[UserFields]{
				{Field: InvalidField, Ascending: true},
			},
		}

		sql := sort.ToSQL(fieldMapping)
		assert.Equal(t, "", sql, "Invalid field should return empty string")
	})

	t.Run("mixed valid and invalid fields", func(t *testing.T) {
		sort := SortBy[UserFields]{
			Fields: []SortByField[UserFields]{
				{Field: InvalidField, Ascending: true},
				{Field: UserName, Ascending: false},
				{Field: InvalidField, Ascending: false},
				{Field: UserCreatedAt, Ascending: true},
			},
		}

		sql := sort.ToSQL(fieldMapping)
		assert.Equal(t, "ORDER BY users.name DESC, users.created_at ASC", sql, "Should skip invalid fields and only include valid ones")
	})

	t.Run("sort with nulls last", func(t *testing.T) {
		sort := SortBy[UserFields]{
			Fields: []SortByField[UserFields]{
				{Field: UserName, Ascending: true, NullsLast: true},
			},
		}

		sql := sort.ToSQL(fieldMapping)
		assert.Equal(t, "ORDER BY users.name ASC NULLS LAST", sql)
	})

	t.Run("multiple fields with nulls last", func(t *testing.T) {
		sort := SortBy[UserFields]{
			Fields: []SortByField[UserFields]{
				{Field: UserName, Ascending: true, NullsLast: true},
				{Field: UserCreatedAt, Ascending: false, NullsLast: false},
			},
		}

		sql := sort.ToSQL(fieldMapping)
		assert.Equal(t, "ORDER BY users.name ASC NULLS LAST, users.created_at DESC", sql)
	})

	t.Run("sql injection protection - invalid field names ignored", func(t *testing.T) {
		type MaliciousFields string
		const (
			SQLInjection MaliciousFields = "name; DROP TABLE users;"
		)

		maliciousMapping := map[MaliciousFields]string{
			// Intentionally not mapping the malicious field
		}

		sort := SortBy[MaliciousFields]{
			Fields: []SortByField[MaliciousFields]{
				{Field: SQLInjection, Ascending: true},
			},
		}

		sql := sort.ToSQL(maliciousMapping)
		assert.Equal(t, "", sql, "Unmapped malicious field should return empty string")
	})
}
