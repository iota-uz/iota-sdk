package build

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/stretchr/testify/require"
)

func TestDashboardBuilderBuildsSpec(t *testing.T) {
	t.Parallel()

	staticSet, err := frame.FromRows("sales", frame.Row{"label": "A", "value": 1.0})
	require.NoError(t, err)

	spec := Dashboard("sales", "Sales",
		Row(panel.Bar("sales-by-day", "Sales by Day", "sales").Build()),
	).
		Description("Revenue overview").
		Variables(DateRangeVariable("range", "Range", 24*time.Hour)).
		Datasets(StaticDataset("sales", staticSet)).
		Build()

	require.Equal(t, "sales", spec.ID)
	require.Equal(t, "Revenue overview", spec.Description)
	require.Len(t, spec.Rows, 1)
	require.Len(t, spec.Variables, 1)
	require.Len(t, spec.Datasets, 1)
}

func TestStaticDatasetAllowsNilFrameSet(t *testing.T) {
	t.Parallel()

	spec := StaticDataset("empty", nil)
	require.Equal(t, lens.DatasetKindStatic, spec.Kind)
	require.NotNil(t, spec.Static)
}
