package crud_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/crud/models"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type ReportOption = func(r *report)

func CreateMultiLangTitle(en string) models.MultiLang {
	return models.NewMultiLang("", "", en)
}

func WithID(id int) ReportOption {
	return func(r *report) {
		r.id = id
	}
}

func WithTitle(title models.MultiLang) ReportOption {
	return func(r *report) {
		r.title = title
	}
}

func WithAuthor(author string) ReportOption {
	return func(r *report) {
		r.author = author
	}
}

func WithSummary(summary string) ReportOption {
	return func(r *report) {
		r.summary = summary
	}
}

func NewReport(
	title models.MultiLang,
	opts ...ReportOption,
) Report {
	r := &report{
		id:      0,
		title:   title,
		author:  "",
		summary: "",
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

type Report interface {
	ID() int
	Title() models.MultiLang
	Author() string
	Summary() string

	SetID(int) Report
	SetTitle(models.MultiLang) Report
	SetAuthor(string) Report
	SetSummary(string) Report
}

type report struct {
	id      int
	title   models.MultiLang
	author  string
	summary string
}

func (r *report) ID() int {
	return r.id
}

func (r *report) Title() models.MultiLang {
	return r.title
}

func (r *report) Author() string {
	return r.author
}

func (r *report) Summary() string {
	return r.summary
}

func (r *report) SetID(id int) Report {
	result := *r
	result.id = id
	return &result
}

func (r *report) SetTitle(title models.MultiLang) Report {
	result := *r
	result.title = title
	return &result
}

func (r *report) SetAuthor(author string) Report {
	result := *r
	result.author = author
	return &result
}

func (r *report) SetSummary(summary string) Report {
	result := *r
	result.summary = summary
	return &result
}

func NewReportMapper(fields crud.Fields) crud.Mapper[Report] {
	return &reportMapper{
		fields: fields,
	}
}

type reportMapper struct {
	fields crud.Fields
}

func (m *reportMapper) ToEntities(_ context.Context, values ...[]crud.FieldValue) ([]Report, error) {
	result := make([]Report, len(values))

	for i, fvs := range values {
		var (
			title   models.MultiLang
			options []ReportOption
		)

		for _, v := range fvs {
			switch v.Field().Name() {
			case "id":
				id, err := v.AsInt()
				if err != nil {
					return nil, fmt.Errorf("invalid id field: %w", err)
				}
				options = append(options, WithID(id))

			case "title":
				str, err := v.AsJSON()
				if err != nil {
					return nil, fmt.Errorf("invalid title field: %w", err)
				}
				if str != "" {
					ml, err := models.MultiLangFromString(str)
					if err != nil {
						return nil, fmt.Errorf("invalid title multilang field: %w", err)
					}
					title = ml
				}

			case "author":
				str, err := v.AsString()
				if err != nil {
					return nil, fmt.Errorf("invalid author field: %w", err)
				}
				options = append(options, WithAuthor(str))

			case "summary":
				str, err := v.AsString()
				if err != nil {
					return nil, fmt.Errorf("invalid summary field: %w", err)
				}
				options = append(options, WithSummary(str))
			}
		}

		result[i] = NewReport(title, options...)
	}

	return result, nil
}

func (m *reportMapper) ToFieldValuesList(_ context.Context, entities ...Report) ([][]crud.FieldValue, error) {
	result := make([][]crud.FieldValue, len(entities))

	for i, entity := range entities {
		fvs, err := m.fields.FieldValues(map[string]any{
			"id":      entity.ID(),
			"title":   entity.Title(),
			"author":  entity.Author(),
			"summary": entity.Summary(),
		})
		if err != nil {
			return nil, err
		}
		result[i] = fvs
	}

	return result, nil
}

func buildReportSchema() crud.Schema[Report] {
	fields := crud.NewFields([]crud.Field{
		crud.NewIntField("id", crud.WithKey()),
		crud.NewJSONField[models.MultiLang]("title", crud.JSONFieldConfig[models.MultiLang]{
			Validator: func(ml models.MultiLang) error {
				if ml.IsEmpty() {
					return models.ErrEmptyMultiLang
				}
				return nil
			},
		}),
		crud.NewStringField("author", crud.WithSearchable()),
		crud.NewStringField("summary", crud.WithSearchable()),
	})
	return crud.NewSchema(
		"reports",
		fields,
		NewReportMapper(fields),
	)
}

type testFixtures struct {
	ctx       context.Context
	pool      *pgxpool.Pool
	schema    crud.Schema[Report]
	publisher eventbus.EventBus
}

// setupTest creates all necessary dependencies for tests
func setupTest(t *testing.T) *testFixtures {
	t.Helper()

	// Use DatabaseManager for proper semaphore handling
	dm := itf.NewDatabaseManager(t)
	pool := dm.Pool()

	ctx := composables.WithPool(context.Background(), pool)

	conf := configuration.Use()
	publisher := eventbus.NewEventPublisher(conf.Logger())

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	})

	ctx = composables.WithTx(ctx, tx)

	// Create table
	createTableSQL := `
					CREATE TABLE IF NOT EXISTS reports (
						id INT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
						title JSONB NOT NULL,
						author TEXT NOT NULL,
						summary TEXT NOT NULL
					);`
	_, err = pool.Exec(ctx, createTableSQL)
	require.NoError(t, err)

	// Init schema and repo
	schema := buildReportSchema()

	return &testFixtures{
		ctx:       ctx,
		pool:      pool,
		schema:    schema,
		publisher: publisher,
	}
}
