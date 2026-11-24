package mcpops

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRevisionArgAcceptsEveryFormOfAPositiveInteger(t *testing.T) {
	for raw, want := range map[string]int{`3`: 3, `"3"`: 3, `1.0`: 1, `1e0`: 1, `2E1`: 20} {
		var r revisionArg
		require.NoError(t, json.Unmarshal([]byte(raw), &r), raw)
		assert.True(t, r.set, raw)
		assert.Equal(t, want, r.n, raw)
	}
	for _, raw := range []string{`0`, `-1`, `1.5`, `"1.0"`, `"x"`, `1e300`, `true`} {
		var r revisionArg
		require.Error(t, json.Unmarshal([]byte(raw), &r), raw)
	}
	var r revisionArg
	require.NoError(t, json.Unmarshal([]byte(`null`), &r))
	assert.False(t, r.set)
}
