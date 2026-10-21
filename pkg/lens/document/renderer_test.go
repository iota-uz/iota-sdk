package document

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	lensbuild "github.com/iota-uz/iota-sdk/pkg/lens/build"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/require"
)

// False green: asserting only the builder field would pass while Build drops
// the renderer, or while Validate accepts a renderer the client cannot key.
func TestBuild_HostRendererTravelsWithItsFrameAndIsBounded(t *testing.T) {
	t.Parallel()
	rows, err := frame.New("rows",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Online"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{0.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(rows)
	require.NoError(t, err)
	legacy := panel.Table("legacy", "Legacy chart", "rows").IDField("label").
		Columns(panel.TableColumn{Field: "label", Label: "Label"}, panel.TableColumn{Field: "value", Label: "Value"}).
		Renderer("vega-lite", json.RawMessage(`{"mark":"bar"}`)).Terminal().Build()
	spec := lensbuild.Dashboard("host", "Host", lensbuild.Row(legacy)).Datasets(lensbuild.StaticDataset("rows", frames)).Build()
	result, err := runtime.New(runtime.Options{}).Execute(context.Background(), spec, runtime.Request{Locale: "en", DataScope: "t"}, runtime.DashboardScope())
	require.NoError(t, err)
	doc, err := Build(spec, result, BuildOptions{SnapshotID: "host", GeneratedAt: time.Unix(1, 0).UTC(), Locale: "en"})
	require.NoError(t, err)
	require.NoError(t, doc.Validate())
	require.Equal(t, &PanelRenderer{Name: "vega-lite", Options: json.RawMessage(`{"mark":"bar"}`)}, doc.Panels[0].Renderer)
	require.NotEmpty(t, doc.Frames[doc.Panels[0].Frame].Rows, "the host renderer reads the same frame export does")

	for name, renderer := range map[string]*PanelRenderer{
		"upper case":   {Name: "Vega"},
		"empty":        {Name: ""},
		"invalid JSON": {Name: "vega", Options: json.RawMessage(`{`)},
		"oversized":    {Name: "vega", Options: json.RawMessage(`"` + strings.Repeat("x", maxRendererOptionsBytes) + `"`)},
	} {
		doc.Panels[0].Renderer = renderer
		require.ErrorContains(t, doc.Validate(), "renderer", name)
	}
}
