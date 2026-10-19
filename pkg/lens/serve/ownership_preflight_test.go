package serve

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
)

func TestOwnedSpecChecksIgnoredMutableFieldsBeforeCloning(t *testing.T) {
	// A generic error assertion would also pass after cloning the oversized slice
	// and reaching the private-field sentinel. Require the preflight limit error.
	type sentinel struct{ private string }
	payload := struct {
		Ignored  []string `json:"-"`
		Sentinel sentinel `json:"-"`
	}{Ignored: make([]string, 1024), Sentinel: sentinel{private: "clone must not reach this"}}
	require.NotEmpty(t, payload.Sentinel.private)
	source := lens.DashboardSpec{ID: "preflight", Variables: []lens.VariableSpec{{Name: "payload", Default: payload}}}
	_, err := resource.EstimateJSON(source, 8<<10)
	require.NoError(t, err, "serialization-only bounds deliberately omit the retained slice")
	_, _, err = ownReportSpec(source, 8<<10)
	require.ErrorIs(t, err, resource.ErrLimit)
}
