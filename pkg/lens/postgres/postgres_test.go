package postgres

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRejectsEmptyConnectionString(t *testing.T) {
	t.Parallel()

	ds, err := New(Config{})
	require.Nil(t, ds)
	require.Error(t, err)
	require.ErrorContains(t, err, "connection string is required")
}

func TestApplyMaxRowsWrapsQuery(t *testing.T) {
	t.Parallel()

	require.Equal(t, "SELECT 1", applyMaxRows("SELECT 1", 0))
	require.Equal(t, "SELECT * FROM (SELECT 1) AS lens_query LIMIT 25", applyMaxRows("SELECT 1", 25))
}
