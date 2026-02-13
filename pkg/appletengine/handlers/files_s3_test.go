package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewS3FilesStore_RequiresPool(t *testing.T) {
	t.Parallel()

	store, err := NewS3FilesStore(nil, S3FilesConfig{})
	require.Error(t, err)
	assert.Nil(t, store)
	assert.Contains(t, err.Error(), "postgres pool is required")
}
