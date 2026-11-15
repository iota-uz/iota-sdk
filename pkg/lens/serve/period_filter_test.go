package serve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	lensbuild "github.com/iota-uz/iota-sdk/pkg/lens/build"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

type rangeSource struct {
	mu     sync.Mutex
	params []map[string]any
}

func (s *rangeSource) Run(_ context.Context, req datasource.QueryRequest) (*frame.FrameSet, error) {
	s.mu.Lock()
	s.params = append(s.params, req.Params)
	s.mu.Unlock()
	primary, err := frame.New(req.Source,
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"row-1"}},
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"total"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{1.0}},
	)
	if err != nil {
		return nil, err
	}
	return frame.NewFrameSet(primary)
}

func (s *rangeSource) Capabilities() datasource.CapabilitySet {
	return datasource.CapabilitySet{datasource.CapabilityParameterizedQueries: true}
}

func (s *rangeSource) last() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.params[len(s.params)-1]
}

func periodHandlers(t *testing.T, cfg func(*Config)) (*Handlers, *rangeSource) {
	t.Helper()
	source := &rangeSource{}
	spec := lensbuild.Dashboard("period", "Period",
		lensbuild.Row(panel.Table("rows", "Rows", "rows").IDField("id").Columns(panel.TableColumn{Field: "label", Label: "Label"}).Terminal().Build()),
	).Variables(lensbuild.DateRangeVariable("period", "Period", 30*24*time.Hour)).
		Datasets(lens.DatasetSpec{
			Name: "rows", Kind: lens.DatasetKindQuery, Source: "primary",
			Query: &lens.QuerySpec{
				Text: "SELECT 1", Kind: datasource.QueryKindRaw,
				Params: map[string]lens.ParamValue{
					"since": lens.RangeParam("period", lens.RangeStart),
					"until": lens.RangeParam("period", lens.RangeUntil),
				},
			},
		}).Build()
	config := Config{
		Spec: spec, Engine: lensruntime.New(lensruntime.Options{}), Snapshots: document.NewMemoryStore(time.Minute, 8),
		BasePath: "/dash",
		Request: func(r *http.Request) lensruntime.Request {
			return lensruntime.Request{
				Locale: "en", DataScope: "tenant:test", Request: r.URL.Query(),
				DataSources:          map[string]datasource.DataSource{"primary": source},
				DataSourceIdentities: map[string]string{"primary": "test:v1"},
			}
		},
	}
	if cfg != nil {
		cfg(&config)
	}
	handlers, err := New(config)
	require.NoError(t, err)
	return handlers, source
}

func TestDocument_DateRangeVariableBecomesPeriodFilterAndBindsQueryParams(t *testing.T) {
	t.Parallel()
	handlers, source := periodHandlers(t, nil)

	doc := requestDocument(t, handlers, "/dash/document?period-start=2026-04-01&period-end=2026-06-30")

	require.Len(t, doc.Filters, 1)
	period := doc.Filters[0]
	require.Equal(t, "period", period.ID)
	require.Equal(t, document.FilterKindPeriod, period.Kind)
	require.Equal(t, "Period", period.Label)
	require.Equal(t, "period-start", period.Period.StartParam)
	require.Equal(t, "period-end", period.Period.EndParam)
	require.Equal(t, document.PeriodValue{Start: "2026-04-01", End: "2026-06-30"}, period.Period.Value)
	require.True(t, period.Period.AllowEmpty, "the variable allows all time")
	require.NoError(t, doc.Validate())

	require.Equal(t, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), source.last()["since"])
	require.Equal(t, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), source.last()["until"])
}

func TestDocument_PeriodWidgetAllTimeReachesQueryAsNull(t *testing.T) {
	t.Parallel()
	handlers, source := periodHandlers(t, nil)

	doc := requestDocument(t, handlers, "/dash/document?period-start=&period-end=")

	require.Equal(t, document.PeriodValue{}, doc.Filters[0].Period.Value)
	require.NoError(t, doc.Validate())
	require.Nil(t, source.last()["since"])
	require.Nil(t, source.last()["until"])
}

func TestDocument_WithoutSelectionShowsTheDefaultRange(t *testing.T) {
	t.Parallel()
	handlers, _ := periodHandlers(t, nil)

	doc := requestDocument(t, handlers, "/dash/document")

	require.NoError(t, doc.Validate())
	require.NotEmpty(t, doc.Filters[0].Period.Value.Start)
	require.NotEmpty(t, doc.Filters[0].Period.Value.End)
}

func TestDocument_DocumentOptionsShapeTheChromePerRequest(t *testing.T) {
	t.Parallel()
	var seen lensruntime.Request
	handlers, _ := periodHandlers(t, func(cfg *Config) {
		cfg.DocumentOptions = func(r *http.Request, req lensruntime.Request) document.BuildOptions {
			seen = req
			return document.BuildOptions{
				Drawer: &document.DrawerHeader{
					Eyebrow: "Expenses", Title: r.URL.Query().Get("title"), Caption: "01.04 — 30.06", Size: document.DrawerSizeWide,
				},
			}
		}
	})

	doc := requestDocument(t, handlers, "/dash/document?title=Salary&period-start=2026-04-01&period-end=2026-06-30")

	require.Equal(t, "tenant:test", seen.DataScope)
	require.Equal(t, "2026-04-01", seen.Request.Get("period-start"))
	require.NotNil(t, doc.Drawer)
	require.Equal(t, "Salary", doc.Drawer.Title)
	require.Equal(t, document.DrawerSizeWide, doc.Drawer.Size)
	require.Len(t, doc.Filters, 1, "options that leave Filters unset keep the derived period filter")
}

func TestDocument_DocumentOptionsFiltersReplaceTheDerivedOnes(t *testing.T) {
	t.Parallel()
	handlers, _ := periodHandlers(t, func(cfg *Config) {
		cfg.DocumentOptions = func(*http.Request, lensruntime.Request) document.BuildOptions {
			return document.BuildOptions{Filters: []document.Filter{}}
		}
	})

	doc := requestDocument(t, handlers, "/dash/document")

	require.Empty(t, doc.Filters)
	recorder := httptest.NewRecorder()
	handlers.Document(recorder, httptest.NewRequest(http.MethodGet, "/dash/document", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestDocument_DocumentOptionsCarryTheHeader(t *testing.T) {
	t.Parallel()
	handlers, _ := periodHandlers(t, func(cfg *Config) {
		cfg.DocumentOptions = func(*http.Request, lensruntime.Request) document.BuildOptions {
			return document.BuildOptions{Header: &document.DocumentHeader{Title: "Custom", Subtitle: "Quarter"}}
		}
	})

	doc := requestDocument(t, handlers, "/dash/document")

	require.NotNil(t, doc.Header)
	require.Equal(t, "Custom", doc.Header.Title)
	require.Equal(t, "Quarter", doc.Header.Subtitle)
}
