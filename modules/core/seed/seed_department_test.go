package seed

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOrgSeedFuncs_ValidSignatures ensures the department/user-position seed
// funcs construct without panicking. application.Seed validates the injected
// signature at construction time and panics on an invalid one, so a successful
// build proves the dependency-injected parameter shapes (department/user repos,
// org query) are well-formed.
func TestOrgSeedFuncs_ValidSignatures(t *testing.T) {
	require.NotNil(t, DepartmentsSeedFunc())
	require.NotNil(t, UserPositionsSeedFunc())
}
