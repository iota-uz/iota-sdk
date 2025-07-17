package crud_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/crud"

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
		report := createValidReport("Quarterly Results", WithAuthor("John"), WithSummary("Q1"))
		fields, err := schema.Mapper().ToFieldValues(ctx, report)
		require.NoError(t, err)

		created, err := rep.Create(ctx, fields)
		require.NoError(t, err)
		assert.NotZero(t, created.ID())
		assert.Equal(t, "Quarterly Results", created.Title())
		// Verify multilingual fields are preserved
		assert.NotEmpty(t, created.TitleI18n())
		assert.NotEmpty(t, created.SummaryI18n())
	})

	t.Run("Get", func(t *testing.T) {
		report := createValidReport("Get Test", WithAuthor("Alice"), WithSummary("Sample"))
		fields, err := schema.Mapper().ToFieldValues(ctx, report)
		require.NoError(t, err)

		created, err := rep.Create(ctx, fields)
		require.NoError(t, err)

		value := schema.Fields().KeyField().Value(created.ID())
		got, err := rep.Get(ctx, value)
		require.NoError(t, err)
		assert.Equal(t, "Alice", got.Author())
		assert.NotEmpty(t, got.TitleI18n())
		assert.NotEmpty(t, got.SummaryI18n())
	})

	t.Run("GetAll", func(t *testing.T) {
		fieldsA, err := schema.Mapper().ToFieldValues(ctx, createValidReport("All A", WithAuthor("A")))
		require.NoError(t, err)
		_, err = rep.Create(ctx, fieldsA)
		require.NoError(t, err)

		fieldsB, err := schema.Mapper().ToFieldValues(ctx, createValidReport("All B", WithAuthor("B")))
		require.NoError(t, err)
		_, err = rep.Create(ctx, fieldsB)
		require.NoError(t, err)

		all, err := rep.GetAll(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(all), 2)

		// Verify multilingual data is preserved in GetAll
		for _, report := range all {
			if len(report.TitleI18n()) > 0 {
				assert.NotEmpty(t, report.TitleI18n())
				assert.NotEmpty(t, report.SummaryI18n())
			}
		}
	})

	t.Run("Exists", func(t *testing.T) {
		report := createValidReport("Get Test", WithAuthor("Alice"), WithSummary("Sample"))
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
		fields, err := schema.Mapper().ToFieldValues(ctx, createValidReport("Filter Me", WithAuthor("FilterTest")))
		require.NoError(t, err)
		_, err = rep.Create(ctx, fields)
		require.NoError(t, err)

		list, err := rep.List(ctx, &crud.FindParams{
			Query:   "Filter Me",
			Filters: []crud.Filter{{Column: "author", Filter: repo.Eq("FilterTest")}},
			Limit:   1,
		})
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, "FilterTest", list[0].Author())
		assert.NotEmpty(t, list[0].TitleI18n())
		assert.NotEmpty(t, list[0].SummaryI18n())
	})

	t.Run("List with order", func(t *testing.T) {
		fields, err := schema.Mapper().ToFieldValues(ctx, createValidReport("Order Me", WithAuthor("OrderTest")))
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
		assert.NotEmpty(t, list[0].TitleI18n())
		assert.NotEmpty(t, list[0].SummaryI18n())
	})

	t.Run("Count with filter", func(t *testing.T) {
		fields, err := schema.Mapper().ToFieldValues(ctx, createValidReport("Count Me", WithAuthor("Counter")))
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
		report := createValidReport("ToUpdate", WithAuthor("Updater"), WithSummary("Initial"))
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
		// Verify multilingual fields are preserved during update
		assert.NotEmpty(t, result.TitleI18n())
		assert.NotEmpty(t, result.SummaryI18n())
	})

	t.Run("Delete", func(t *testing.T) {
		report := createValidReport("ToDelete", WithAuthor("Deleter"))
		fields, err := schema.Mapper().ToFieldValues(ctx, report)
		require.NoError(t, err)
		created, err := rep.Create(ctx, fields)
		require.NoError(t, err)

		key := schema.Fields().KeyField().Value(created.ID())
		deleted, err := rep.Delete(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, created.ID(), deleted.ID())
		// Verify that deleted entity still contains multilingual data
		assert.NotEmpty(t, deleted.TitleI18n())
		assert.NotEmpty(t, deleted.SummaryI18n())

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
		fields, err := schema.Mapper().ToFieldValues(ctx, createValidReport("Out of bounds"))
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
