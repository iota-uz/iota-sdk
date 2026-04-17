package pagination

// LegacyAliases returns the env-var → koanf-path alias map for pagination config.
func LegacyAliases() map[string]string {
	return map[string]string{
		"PAGE_SIZE":     "http.pagination.pagesize",
		"MAX_PAGE_SIZE": "http.pagination.maxpagesize",
	}
}
