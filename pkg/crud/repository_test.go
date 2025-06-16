package crud_test

import (
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportRepository_AllMethods(t *testing.T) {
	t.Helper()

	fixture := setupTest(t)
	ctx := fixture.ctx
	schema := fixture.schema
	rep := crud.DefaultRepository[Report](schema)

	t.Run("Create", func(t *testing.T) {
		report := NewReport("Quarterly Results", WithAuthor("John"), WithSummary("Q1"))
		fields, err := schema.Mapper().ToFieldValues(ctx, report)
		require.NoError(t, err)

		created, err := rep.Create(ctx, fields)
		require.NoError(t, err)
		assert.NotZero(t, created.ID())
		assert.Equal(t, "Quarterly Results", created.Title())
	})

	t.Run("Get", func(t *testing.T) {
		report := NewReport("Get Test", WithAuthor("Alice"), WithSummary("Sample"))
		fields, err := schema.Mapper().ToFieldValues(ctx, report)
		require.NoError(t, err)

		created, err := rep.Create(ctx, fields)
		require.NoError(t, err)

		value := schema.Fields().KeyField().Value(created.ID())
		got, err := rep.Get(ctx, value)
		require.NoError(t, err)
		assert.Equal(t, "Alice", got.Author())
	})

	t.Run("GetAll", func(t *testing.T) {
		fieldsA, err := schema.Mapper().ToFieldValues(ctx, NewReport("All A", WithAuthor("A")))
		require.NoError(t, err)
		_, err = rep.Create(ctx, fieldsA)
		require.NoError(t, err)

		fieldsB, err := schema.Mapper().ToFieldValues(ctx, NewReport("All B", WithAuthor("B")))
		require.NoError(t, err)
		_, err = rep.Create(ctx, fieldsB)
		require.NoError(t, err)

		all, err := rep.GetAll(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(all), 2)
	})

	t.Run("Exists", func(t *testing.T) {
		report := NewReport("Get Test", WithAuthor("Alice"), WithSummary("Sample"))
		fields, err := schema.Mapper().ToFieldValues(ctx, report)
		require.NoError(t, err)

		created, err := rep.Create(ctx, fields)
		require.NoError(t, err)

		value := schema.Fields().KeyField().Value(created.ID())
		exists, err := rep.Exists(ctx, value)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("List with filter", func(t *testing.T) {
		fields, err := schema.Mapper().ToFieldValues(ctx, NewReport("Filter Me", WithAuthor("FilterTest")))
		require.NoError(t, err)
		_, err = rep.Create(ctx, fields)
		require.NoError(t, err)

		list, err := rep.List(ctx, &crud.FindParams{
			Search:  "Filter Me",
			Filters: []crud.Filter{{Column: "author", Filter: repo.Eq("FilterTest")}},
			Limit:   1,
		})
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, "FilterTest", list[0].Author())
	})

	t.Run("List with order", func(t *testing.T) {
		fields, err := schema.Mapper().ToFieldValues(ctx, NewReport("Order Me", WithAuthor("OrderTest")))
		require.NoError(t, err)
		_, err = rep.Create(ctx, fields)
		require.NoError(t, err)

		list, err := rep.List(ctx, &crud.FindParams{
			Filters: []crud.Filter{{Column: "author", Filter: repo.Eq("OrderTest")}},
			SortBy: crud.SortBy{
				Fields: []repo.SortByField[string]{
					{Field: "author", Ascending: true, NullsLast: true},
				},
			},
			Limit: 1,
		})
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, "OrderTest", list[0].Author())
	})

	t.Run("Count with filter", func(t *testing.T) {
		fields, err := schema.Mapper().ToFieldValues(ctx, NewReport("Count Me", WithAuthor("Counter")))
		require.NoError(t, err)
		_, err = rep.Create(ctx, fields)
		require.NoError(t, err)

		count, err := rep.Count(ctx, &crud.FindParams{
			Filters: []crud.Filter{{Column: "author", Filter: repo.Eq("Counter")}},
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("Update", func(t *testing.T) {
		report := NewReport("ToUpdate", WithAuthor("Updater"), WithSummary("Initial"))
		fields, err := schema.Mapper().ToFieldValues(ctx, report)
		require.NoError(t, err)
		created, err := rep.Create(ctx, fields)
		require.NoError(t, err)

		updated := created.SetSummary("Updated!")
		updateFields, err := schema.Mapper().ToFieldValues(ctx, updated)
		require.NoError(t, err)

		result, err := rep.Update(ctx, updateFields)
		require.NoError(t, err)
		assert.Equal(t, "Updated!", result.Summary())
	})

	t.Run("Delete", func(t *testing.T) {
		report := NewReport("ToDelete", WithAuthor("Deleter"))
		fields, err := schema.Mapper().ToFieldValues(ctx, report)
		require.NoError(t, err)
		created, err := rep.Create(ctx, fields)
		require.NoError(t, err)

		key := schema.Fields().KeyField().Value(created.ID())
		deleted, err := rep.Delete(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, created.ID(), deleted.ID())

		_, err = rep.Get(ctx, key)
		require.Error(t, err)
	})

	t.Run("Invalid Filter Column", func(t *testing.T) {
		_, err := rep.Count(ctx, &crud.FindParams{
			Filters: []crud.Filter{{Column: "not_exist", Filter: repo.Eq("x")}},
		})
		require.Error(t, err)

		_, err = rep.List(ctx, &crud.FindParams{
			Filters: []crud.Filter{{Column: "not_exist", Filter: repo.Eq("x")}},
		})
		require.Error(t, err)
	})

	t.Run("Offset beyond result", func(t *testing.T) {
		fields, err := schema.Mapper().ToFieldValues(ctx, NewReport("Out of bounds"))
		require.NoError(t, err)
		_, err = rep.Create(ctx, fields)
		require.NoError(t, err)

		list, err := rep.List(ctx, &crud.FindParams{
			Limit:  10,
			Offset: 1000,
		})
		require.NoError(t, err)
		assert.Empty(t, list)
	})
}
