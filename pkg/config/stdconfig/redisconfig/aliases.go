package redisconfig

// LegacyAliases returns the env-var → koanf-path alias map for redisconfig.
func LegacyAliases() map[string]string {
	return map[string]string{
		"REDIS_URL": "redis.url",
	}
}
