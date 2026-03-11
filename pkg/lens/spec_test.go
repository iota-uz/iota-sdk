package lens_test

import (
	"testing"

	lensbuild "github.com/iota-uz/iota-sdk/pkg/lens/build"
	"github.com/stretchr/testify/require"
)

func TestStaticDatasetAllowsNilFrameSet(t *testing.T) {
	t.Parallel()

	spec := lensbuild.StaticDataset("empty", nil)
	require.NotNil(t, spec.Static, "expected nil static frameset to become an empty frameset")
}
