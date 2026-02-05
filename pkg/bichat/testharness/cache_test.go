package testharness

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJudgeCacheKey_Stable(t *testing.T) {
	t.Parallel()

	cfg := Config{
		ServerURL:           DefaultServerURL,
		GraphQLEndpointPath: DefaultGraphQLEndpointPath,
		StreamEndpointPath:  DefaultStreamEndpointPath,
		CookieName:          DefaultCookieName,
		JudgeModel:          DefaultJudgeModel,
		OpenAIAPIKey:        "k",
		CacheEnabled:        true,
		CacheDir:            t.TempDir(),
	}
	cache := NewCache(cfg)

	prompt := "Evaluate the following turn.\n\nUser prompt:\nhi"
	k1 := cache.JudgeKey("gpt-5-mini", prompt)
	k2 := cache.JudgeKey("gpt-5-mini", prompt)
	require.Equal(t, k1, k2)

	k3 := cache.JudgeKey("gpt-5-nano", prompt)
	require.NotEqual(t, k1, k3)
}

func TestJudgeCache_SaveLoadVerdict(t *testing.T) {
	t.Parallel()

	cfg := Config{
		ServerURL:           DefaultServerURL,
		GraphQLEndpointPath: DefaultGraphQLEndpointPath,
		StreamEndpointPath:  DefaultStreamEndpointPath,
		CookieName:          DefaultCookieName,
		JudgeModel:          DefaultJudgeModel,
		OpenAIAPIKey:        "k",
		CacheEnabled:        true,
		CacheDir:            t.TempDir(),
	}
	cache := NewCache(cfg)

	key := cache.JudgeKey("gpt-5-mini", "p")
	v := JudgeVerdict{Passed: true, Reason: "ok", EfficiencyScore: 5, EfficiencyNotes: "direct"}
	require.NoError(t, cache.SaveJudgeVerdict(key, v))

	loaded, ok, err := cache.LoadJudgeVerdict(key)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, v, *loaded)

	// sanity path check (judge subdir)
	p, err := cache.judgeFilePath(key)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(cfg.CacheDir, "judge", key+".json"), p)
}
