package crud_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type ReportOption = func(r *report)

func WithID(id int) ReportOption {
	return func(r *report) {
		r.id = id
	}
}

func WithTitle(title string) ReportOption {
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

func WithTitleI18n(titleI18n map[string]string) ReportOption {
	return func(r *report) {
		r.titleI18n = titleI18n
	}
}

func WithSummaryI18n(summaryI18n map[string]string) ReportOption {
	return func(r *report) {
		r.summaryI18n = summaryI18n
	}
}

func NewReport(
	title string,
	opts ...ReportOption,
) Report {
	r := &report{
		id:          0,
		title:       title,
		author:      "",
		summary:     "",
		titleI18n:   make(map[string]string),
		summaryI18n: make(map[string]string),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

type Report interface {
	ID() int
	Title() string
	Author() string
	Summary() string
	TitleI18n() map[string]string
	SummaryI18n() map[string]string

	SetID(int) Report
	SetTitle(string) Report
	SetAuthor(string) Report
	SetSummary(string) Report
	SetTitleI18n(map[string]string) Report
	SetSummaryI18n(map[string]string) Report
}

type report struct {
	id          int
	title       string
	author      string
	summary     string
	titleI18n   map[string]string
	summaryI18n map[string]string
}

func (r *report) ID() int {
	return r.id
}

func (r *report) Title() string {
	return r.title
}

func (r *report) Author() string {
	return r.author
}

func (r *report) Summary() string {
	return r.summary
}

func (r *report) TitleI18n() map[string]string {
	return r.titleI18n
}

func (r *report) SummaryI18n() map[string]string {
	return r.summaryI18n
}

func (r *report) SetID(id int) Report {
	result := *r
	result.id = id
	return &result
}

func (r *report) SetTitle(title string) Report {
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

func (r *report) SetTitleI18n(titleI18n map[string]string) Report {
	result := *r
	result.titleI18n = make(map[string]string)
	for k, v := range titleI18n {
		result.titleI18n[k] = v
	}
	return &result
}

func (r *report) SetSummaryI18n(summaryI18n map[string]string) Report {
	result := *r
	result.summaryI18n = make(map[string]string)
	for k, v := range summaryI18n {
		result.summaryI18n[k] = v
	}
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
			title   string
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
				str, err := v.AsString()
				if err != nil {
					return nil, fmt.Errorf("invalid title field: %w", err)
				}
				title = str

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

			case "title_i18n":
				jsonData, err := v.AsJson()
				if err != nil {
					return nil, fmt.Errorf("invalid title_i18n field: %w", err)
				}
				if jsonData != nil {
					titleI18n, ok := jsonData.(map[string]interface{})
					if !ok {
						return nil, fmt.Errorf("title_i18n must be an object")
					}
					// Convert map[string]interface{} to map[string]string
					titleI18nStr := make(map[string]string)
					for k, v := range titleI18n {
						if str, ok := v.(string); ok {
							titleI18nStr[k] = str
						} else {
							return nil, fmt.Errorf("title_i18n values must be strings")
						}
					}
					options = append(options, WithTitleI18n(titleI18nStr))
				}

			case "summary_i18n":
				jsonData, err := v.AsJson()
				if err != nil {
					return nil, fmt.Errorf("invalid summary_i18n field: %w", err)
				}
				if jsonData != nil {
					summaryI18n, ok := jsonData.(map[string]interface{})
					if !ok {
						return nil, fmt.Errorf("summary_i18n must be an object")
					}
					// Convert map[string]interface{} to map[string]string
					summaryI18nStr := make(map[string]string)
					for k, v := range summaryI18n {
						if str, ok := v.(string); ok {
							summaryI18nStr[k] = str
						} else {
							return nil, fmt.Errorf("summary_i18n values must be strings")
						}
					}
					options = append(options, WithSummaryI18n(summaryI18nStr))
				}
			}
		}

		result[i] = NewReport(title, options...)
	}

	return result, nil
}

func (m *reportMapper) ToFieldValuesList(_ context.Context, entities ...Report) ([][]crud.FieldValue, error) {
	result := make([][]crud.FieldValue, len(entities))

	for i, entity := range entities {
		// Convert multilang maps to JSON strings
		titleI18nJSON := "{}"
		summaryI18nJSON := "{}"

		if len(entity.TitleI18n()) > 0 {
			if jsonBytes, err := json.Marshal(entity.TitleI18n()); err == nil {
				titleI18nJSON = string(jsonBytes)
			}
		}

		if len(entity.SummaryI18n()) > 0 {
			if jsonBytes, err := json.Marshal(entity.SummaryI18n()); err == nil {
				summaryI18nJSON = string(jsonBytes)
			}
		}

		fvs, err := m.fields.FieldValues(map[string]any{
			"id":           entity.ID(),
			"title":        entity.Title(),
			"author":       entity.Author(),
			"summary":      entity.Summary(),
			"title_i18n":   titleI18nJSON,
			"summary_i18n": summaryI18nJSON,
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
		crud.NewStringField("title", crud.WithSearchable()),
		crud.NewStringField("author", crud.WithSearchable()),
		crud.NewStringField("summary", crud.WithSearchable()),
		crud.NewJsonField("title_i18n", crud.WithMultiLang()),
		crud.NewJsonField("summary_i18n", crud.WithMultiLang()),
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
						title TEXT NOT NULL,
						author TEXT NOT NULL,
						summary TEXT NOT NULL,
						title_i18n JSONB NOT NULL DEFAULT '{}',
						summary_i18n JSONB NOT NULL DEFAULT '{}'
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
