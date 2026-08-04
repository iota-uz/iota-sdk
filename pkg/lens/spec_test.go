package lens_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	lensbuild "github.com/iota-uz/iota-sdk/pkg/lens/build"
	"github.com/stretchr/testify/require"
)

func TestStaticDatasetAllowsNilFrameSet(t *testing.T) {
	t.Parallel()

	spec := lensbuild.StaticDataset("empty", nil)
	require.NotNil(t, spec.Static, "expected nil static frameset to become an empty frameset")
}

func TestDefaultPeriodRequestKeysAreCanonical(t *testing.T) {
	t.Parallel()
	spec := lens.VariableSpec{Name: "ActualRange", Kind: lens.VariableDateRange}
	start, end := lens.DateRangeRequestKeys(spec)
	require.Equal(t, "actual-range-start", start)
	require.Equal(t, "actual-range-end", end)
	mode, compareStart, compareEnd := lens.CompareRequestKeys(lens.VariableSpec{Name: "ComparePeriod"})
	require.Equal(t, "compare-period", mode)
	require.Equal(t, "compare-period-start", compareStart)
	require.Equal(t, "compare-period-end", compareEnd)
}
