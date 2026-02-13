package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPostgresJobsStore_RequiresPool(t *testing.T) {
	t.Parallel()

	store, err := NewPostgresJobsStore(nil)
	require.Error(t, err)
	assert.Nil(t, store)
	assert.Contains(t, err.Error(), "postgres pool is required")
}
