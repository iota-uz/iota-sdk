package datasource_test

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/stretchr/testify/require"
)

func TestNamedDataSourceRoutesFrozenRequestToRegisteredHandler(t *testing.T) {
	source := datasource.NewNamedDataSource()
	var received datasource.QueryRequest
	require.NoError(t, source.Register("analytics.profitability.kpis", func(_ context.Context, req datasource.QueryRequest) (*frame.FrameSet, error) {
		received = req
		result, err := frame.New("kpis", frame.Field{Name: "value", Type: frame.FieldTypeNumber})
		require.NoError(t, err)
		require.NoError(t, result.AppendRow(map[string]any{"value": 42.0}))
		return frame.NewFrameSet(result)
	}))

	request := datasource.QueryRequest{
		Text: "analytics.profitability.kpis", Kind: datasource.QueryKindNamed,
		Params: map[string]any{"period": "2026"}, Locale: "ru",
	}
	result, err := source.Run(t.Context(), request)
	require.NoError(t, err)
	require.Equal(t, request, received)
	require.NotNil(t, result.Primary())
}

func TestNamedDataSourceFailsClosed(t *testing.T) {
	source := datasource.NewNamedDataSource()
	require.Error(t, source.Register("", func(context.Context, datasource.QueryRequest) (*frame.FrameSet, error) { return nil, nil }))
	require.NoError(t, source.Register("known", func(context.Context, datasource.QueryRequest) (*frame.FrameSet, error) { return nil, nil }))
	require.Error(t, source.Register("known", func(context.Context, datasource.QueryRequest) (*frame.FrameSet, error) { return nil, nil }))

	_, err := source.Run(t.Context(), datasource.QueryRequest{Text: "missing", Kind: datasource.QueryKindNamed})
	require.ErrorContains(t, err, "not registered")
	_, err = source.Run(t.Context(), datasource.QueryRequest{Text: "known", Kind: datasource.QueryKindRaw})
	require.ErrorContains(t, err, "does not support")
}
