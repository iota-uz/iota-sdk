//go:build !dev

package agentsignin

import (
	"github.com/stretchr/testify/require"
	"testing"
)

// Falsely green if enabled production configuration is not exercised.
func TestProductionControllerUnavailable(t *testing.T) {
	c, err := NewController(Options{})
	require.NoError(t, err)
	require.Nil(t, c)
	_, err = NewController(Options{Enabled: true})
	require.Error(t, err)
}
