package ratelimitconfig

// LegacyAliases returns the env-var → koanf-path alias map for ratelimitconfig.
func LegacyAliases() map[string]string {
	return map[string]string{
		"RATE_LIMIT_ENABLED":    "ratelimit.enabled",
		"RATE_LIMIT_GLOBAL_RPS": "ratelimit.globalrps",
		"RATE_LIMIT_STORAGE":    "ratelimit.storage",
		"RATE_LIMIT_REDIS_URL":  "ratelimit.redisurl",
	}
}
