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
