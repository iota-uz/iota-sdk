package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDBQueryOptions(t *testing.T) {
	t.Parallel()

	options, err := parseDBQueryOptions(map[string]any{
		"order": "asc",
		"take":  float64(5),
		"index": map[string]any{
			"name":  "by_user",
			"field": "user.id",
			"op":    "eq",
			"value": "u-1",
		},
		"filters": []any{
			map[string]any{
				"field": "status",
				"op":    "eq",
				"value": "open",
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, options.Index)
	assert.Equal(t, "asc", options.Order)
	assert.Equal(t, 5, options.Limit)
	assert.Equal(t, "by_user", options.Index.Name)
	assert.Equal(t, "user.id", options.Index.Field)
	require.Len(t, options.Filter, 1)
	assert.Equal(t, "status", options.Filter[0].Field)
}

func TestParseDBQueryOptions_UnsupportedOp(t *testing.T) {
	t.Parallel()

	_, err := parseDBQueryOptions(map[string]any{
		"filters": []any{
			map[string]any{
				"field": "status",
				"op":    "neq",
				"value": "open",
			},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported filter op")
}
