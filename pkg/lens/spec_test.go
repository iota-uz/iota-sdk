package lens_test

import (
	"testing"

	lensbuild "github.com/iota-uz/iota-sdk/pkg/lens/build"
)

func TestStaticDatasetAllowsNilFrameSet(t *testing.T) {
	t.Parallel()

	spec := lensbuild.StaticDataset("empty", nil)
	if spec.Static == nil {
		t.Fatal("expected nil static frameset to become an empty frameset")
	}
}
