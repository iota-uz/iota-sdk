package persistence_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/stretchr/testify/require"
)

// Falsely green if the private directory starts with mode 0700.
func TestNewFSStorage_RestrictsExistingPrivateDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	privatePath := filepath.Join(root, uploadsconfig.PrivateDirectory)
	require.NoError(t, os.MkdirAll(privatePath, 0755))
	require.NoError(t, os.Chmod(privatePath, 0755))

	_, err := persistence.NewFSStorage(&uploadsconfig.Config{Path: root})
	require.NoError(t, err)

	rootInfo, err := os.Stat(root)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0755), rootInfo.Mode().Perm())
	info, err := os.Stat(privatePath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0700), info.Mode().Perm())
}
