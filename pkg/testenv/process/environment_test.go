package process

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Characterisation of Config.Unset. Falsely green if the filter were applied
// to the explicit Environment/Configure values as well: the adapter appends
// those after inheritedEnvironment, so only inherited keys are asserted here.
func TestInheritedEnvironmentDropsUnsetPrefixes(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://shared:6379")
	t.Setenv("REDIS_HOST", "shared")
	t.Setenv("RATELIMIT_REDISURL", "redis://shared:6379")
	t.Setenv("RATELIMIT_STORAGE", "redis")
	t.Setenv("TESTENV_KEEP", "1")

	keys := map[string]bool{}
	for _, entry := range inheritedEnvironment([]string{"REDIS_", "RATELIMIT_REDIS", ""}) {
		key, _, _ := strings.Cut(entry, "=")
		keys[key] = true
	}
	require.False(t, keys["REDIS_URL"])
	require.False(t, keys["REDIS_HOST"])
	require.False(t, keys["RATELIMIT_REDISURL"])
	require.True(t, keys["RATELIMIT_STORAGE"], "prefix must not drop unrelated keys")
	require.True(t, keys["TESTENV_KEEP"])
	require.Len(t, inheritedEnvironment(nil), len(inheritedEnvironment([]string{"NO_SUCH_PREFIX_"})))
}
