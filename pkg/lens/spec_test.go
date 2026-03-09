package lens

import "testing"

func TestStaticDatasetAllowsNilFrameSet(t *testing.T) {
	t.Parallel()

	spec := StaticDataset("empty", nil)
	if spec.Static == nil {
		t.Fatal("expected nil static frameset to become an empty frameset")
	}
}
