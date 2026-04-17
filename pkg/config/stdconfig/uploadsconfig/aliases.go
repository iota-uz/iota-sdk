package uploadsconfig

// LegacyAliases returns the env-var → koanf-path alias map for uploadsconfig.
func LegacyAliases() map[string]string {
	return map[string]string{
		"UPLOADS_PATH":      "uploads.path",
		"MAX_UPLOAD_SIZE":   "uploads.maxsize",
		"MAX_UPLOAD_MEMORY": "uploads.maxmemory",
	}
}
