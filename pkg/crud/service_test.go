package crud_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportService_CRUD(t *testing.T) {
	t.Helper()

	fixture := setupTest(t)
	ctx := fixture.ctx

	service := crud.DefaultService(
		fixture.schema,
		crud.DefaultRepository[Report](fixture.schema),
		fixture.publisher,
	)

	t.Run("Save (Create)", func(t *testing.T) {
		report := NewReport(CreateMultiLangTitle("Service Create"), WithAuthor("SAuthor"), WithSummary("SSummary"))
		created, err := service.Save(ctx, report)
		require.NoError(t, err)
		assert.NotZero(t, created.ID())
		assert.Equal(t, "SAuthor", created.Author())
	})

	t.Run("Save (Update)", func(t *testing.T) {
		original := NewReport(CreateMultiLangTitle("To Update"), WithAuthor("Orig"), WithSummary("Old"))
		created, err := service.Save(ctx, original)
		require.NoError(t, err)

		updated := created.SetSummary("New")
		saved, err := service.Save(ctx, updated)
		require.NoError(t, err)
		assert.Equal(t, "New", saved.Summary())
	})

	t.Run("Get", func(t *testing.T) {
		report := NewReport(CreateMultiLangTitle("Service Get"), WithAuthor("GetAuthor"))
		created, err := service.Save(ctx, report)
		require.NoError(t, err)

		key := fixture.schema.Fields().KeyField().Value(created.ID())
		got, err := service.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, "GetAuthor", got.Author())
	})

	t.Run("GetAll", func(t *testing.T) {
		_, err := service.Save(ctx, NewReport(CreateMultiLangTitle("All 1"), WithAuthor("A")))
		require.NoError(t, err)
		_, err = service.Save(ctx, NewReport(CreateMultiLangTitle("All 2"), WithAuthor("B")))
		require.NoError(t, err)

		all, err := service.GetAll(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(all), 2)
	})

	t.Run("Exists", func(t *testing.T) {
		created, err := service.Save(ctx, NewReport(CreateMultiLangTitle("Exists"), WithAuthor("Ex")))
		require.NoError(t, err)

		key := fixture.schema.Fields().KeyField().Value(created.ID())
		ok, err := service.Exists(ctx, key)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("List with filter", func(t *testing.T) {
		_, err := service.Save(ctx, NewReport(CreateMultiLangTitle("Filter Test"), WithAuthor("SvcFilter")))
		require.NoError(t, err)

		list, err := service.List(ctx, &crud.FindParams{
			Filters: []crud.Filter{{Column: "author", Filter: repo.Eq("SvcFilter")}},
			Limit:   1,
		})
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, "SvcFilter", list[0].Author())
	})

	t.Run("Count with filter", func(t *testing.T) {
		_, err := service.Save(ctx, NewReport(CreateMultiLangTitle("Countable"), WithAuthor("SvcCounter")))
		require.NoError(t, err)

		count, err := service.Count(ctx, &crud.FindParams{
			Filters: []crud.Filter{{Column: "author", Filter: repo.Eq("SvcCounter")}},
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("Delete", func(t *testing.T) {
		created, err := service.Save(ctx, NewReport(CreateMultiLangTitle("To Be Deleted"), WithAuthor("Del")))
		require.NoError(t, err)

		key := fixture.schema.Fields().KeyField().Value(created.ID())
		deleted, err := service.Delete(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, created.ID(), deleted.ID())

		_, err = service.Get(ctx, key)
		require.Error(t, err)
	})
}
