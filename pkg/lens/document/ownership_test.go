package document

import (
	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFrozenDocument_DeepOwnsInputsAndEachReader(t *testing.T) {
	t.Parallel()
	source := &DashboardDocument{I18n: map[string]string{"key": "base"}, Frames: map[FrameRef]Frame{"rows": {Rows: [][]any{{map[string][]string{"nested": {"base"}}}}}}, Header: &DocumentHeader{Title: "base"}}
	frozen, err := Freeze(source, 1<<20)
	require.NoError(t, err)
	source.Frames["rows"].Rows[0][0].(map[string][]string)["nested"][0] = "producer"
	first, err := frozen.Document()
	require.NoError(t, err)
	first.Header.Title = "reader"
	first.Frames["rows"].Rows[0][0].(map[string]any)["nested"].([]any)[0] = "reader"
	second, err := frozen.Document()
	require.NoError(t, err)
	require.Equal(t, "base", second.Header.Title)
	require.Equal(t, "base", second.Frames["rows"].Rows[0][0].(map[string]any)["nested"].([]any)[0])
}
func TestFrozenDocument_RejectsOversizeAndCyclesBeforeEncoding(t *testing.T) {
	t.Parallel()
	_, err := Freeze(&DashboardDocument{I18n: map[string]string{"large": string(make([]byte, 10000))}}, 1000)
	require.ErrorIs(t, err, resource.ErrLimit)
	cycle := map[string]any{}
	cycle["self"] = cycle
	_, err = Freeze(&DashboardDocument{Frames: map[FrameRef]Frame{"rows": {Rows: [][]any{{cycle}}}}}, 1<<20)
	require.Error(t, err)
}
