package crud_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createValidReport creates a Report with valid multilingual fields for testing
func createValidReport(title string, opts ...ReportOption) Report {
	// Add default multilingual fields that satisfy validation requirements
	defaultTitleI18n := map[string]string{
		"ru": "Русский: " + title,
		"uz": "O'zbek: " + title,
		"en": "English: " + title,
	}
	defaultSummaryI18n := map[string]string{
		"ru": "Краткое описание для " + title,
		"uz": title + " uchun qisqacha tavsif",
		"en": "Brief description for " + title,
	}

	// Combine default options with user-provided options
	allOpts := append([]ReportOption{
		WithTitleI18n(defaultTitleI18n),
		WithSummaryI18n(defaultSummaryI18n),
	}, opts...)

	return NewReport(title, allOpts...)
}

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
		report := createValidReport("Service Create", WithAuthor("SAuthor"), WithSummary("SSummary"))
		created, err := service.Save(ctx, report)
		require.NoError(t, err)
		assert.NotZero(t, created.ID())
		assert.Equal(t, "SAuthor", created.Author())
		// Verify multilingual fields are preserved
		assert.NotEmpty(t, created.TitleI18n())
		assert.NotEmpty(t, created.SummaryI18n())
	})

	t.Run("Save (Update)", func(t *testing.T) {
		original := createValidReport("To Update", WithAuthor("Orig"), WithSummary("Old"))
		created, err := service.Save(ctx, original)
		require.NoError(t, err)

		updated := created.SetSummary("New")
		saved, err := service.Save(ctx, updated)
		require.NoError(t, err)
		assert.Equal(t, "New", saved.Summary())
		// Verify multilingual fields are preserved during update
		assert.NotEmpty(t, saved.TitleI18n())
		assert.NotEmpty(t, saved.SummaryI18n())
	})

	t.Run("Get", func(t *testing.T) {
		report := createValidReport("Service Get", WithAuthor("GetAuthor"))
		created, err := service.Save(ctx, report)
		require.NoError(t, err)

		key := fixture.schema.Fields().KeyField().Value(created.ID())
		got, err := service.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, "GetAuthor", got.Author())
		assert.NotEmpty(t, got.TitleI18n())
		assert.NotEmpty(t, got.SummaryI18n())
	})

	t.Run("GetAll", func(t *testing.T) {
		_, err := service.Save(ctx, createValidReport("All 1", WithAuthor("A")))
		require.NoError(t, err)
		_, err = service.Save(ctx, createValidReport("All 2", WithAuthor("B")))
		require.NoError(t, err)

		all, err := service.GetAll(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(all), 2)

		// Verify that multilingual data is preserved in GetAll
		for _, report := range all {
			if len(report.TitleI18n()) > 0 {
				assert.NotEmpty(t, report.TitleI18n())
				assert.NotEmpty(t, report.SummaryI18n())
			}
		}
	})

	t.Run("Exists", func(t *testing.T) {
		created, err := service.Save(ctx, createValidReport("Exists", WithAuthor("Ex")))
		require.NoError(t, err)

		key := fixture.schema.Fields().KeyField().Value(created.ID())
		ok, err := service.Exists(ctx, key)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("List with filter", func(t *testing.T) {
		_, err := service.Save(ctx, createValidReport("Filter Test", WithAuthor("SvcFilter")))
		require.NoError(t, err)

		list, err := service.List(ctx, &crud.FindParams{
			Query:   "Filter",
			Filters: []crud.Filter{{Column: "author", Filter: repo.Eq("SvcFilter")}},
			Limit:   1,
		})
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, "SvcFilter", list[0].Author())
		assert.NotEmpty(t, list[0].TitleI18n())
		assert.NotEmpty(t, list[0].SummaryI18n())
	})

	t.Run("Count with filter", func(t *testing.T) {
		_, err := service.Save(ctx, createValidReport("Countable", WithAuthor("SvcCounter")))
		require.NoError(t, err)

		count, err := service.Count(ctx, &crud.FindParams{
			Filters: []crud.Filter{{Column: "author", Filter: repo.Eq("SvcCounter")}},
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("Delete", func(t *testing.T) {
		created, err := service.Save(ctx, createValidReport("To Be Deleted", WithAuthor("Del")))
		require.NoError(t, err)

		key := fixture.schema.Fields().KeyField().Value(created.ID())
		deleted, err := service.Delete(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, created.ID(), deleted.ID())
		// Verify that deleted entity still contains multilingual data
		assert.NotEmpty(t, deleted.TitleI18n())
		assert.NotEmpty(t, deleted.SummaryI18n())

		_, err = service.Get(ctx, key)
		require.Error(t, err)
	})
}
