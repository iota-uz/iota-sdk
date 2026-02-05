package testharness

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCacheKey_StableAndPermissionOrderIndependent(t *testing.T) {
	t.Parallel()

	cfg := Config{
		ServerURL:           DefaultServerURL,
		GraphQLEndpointPath: DefaultGraphQLEndpointPath,
		StreamEndpointPath:  DefaultStreamEndpointPath,
		CookieName:          DefaultCookieName,
		SessionToken:        "token",
		JudgeModel:          "gpt-5-nano-2025-08-07",
		OpenAIAPIKey:        "k",
		CacheEnabled:        true,
		CacheDir:            "/tmp",
		IotaSDKRevision:     "iota",
		HostRevision:        "host",
	}

	cache := NewCache(cfg)

	suiteA := TestSuite{
		Tests: []TestCase{
			{
				ID:              "t1",
				UserPermissions: []string{"b", "a"},
				Turns: []Turn{
					{Prompt: "p"},
				},
			},
		},
	}
	suiteB := TestSuite{
		Tests: []TestCase{
			{
				ID:              "t1",
				UserPermissions: []string{"a", "b"},
				Turns: []Turn{
					{Prompt: "p"},
				},
			},
		},
	}

	key1, err := cache.Key(suiteA, cfg)
	require.NoError(t, err)
	key2, err := cache.Key(suiteA, cfg)
	require.NoError(t, err)
	require.Equal(t, key1, key2)

	key3, err := cache.Key(suiteB, cfg)
	require.NoError(t, err)
	require.Equal(t, key1, key3)
}
