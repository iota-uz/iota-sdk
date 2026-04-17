package dbconfig

// LegacyAliases returns the env-var → koanf-path alias map for dbconfig.
// Note: DB_NAME, DB_HOST, DB_PORT, DB_USER, DB_PASSWORD already transform
// correctly via the single-underscore-to-dot rule (db.name, db.host, etc.).
// Only pool sub-fields and MIGRATIONS_DIR require explicit aliases.
func LegacyAliases() map[string]string {
	return map[string]string{
		"DB_MAX_CONNS":                "db.pool.maxconns",
		"DB_MIN_CONNS":                "db.pool.minconns",
		"DB_MAX_CONN_LIFETIME":        "db.pool.maxconnlifetime",
		"DB_MAX_CONN_LIFETIME_JITTER": "db.pool.maxconnlifetimejitter",
		"DB_MAX_CONN_IDLE_TIME":       "db.pool.maxconnidletime",
		"DB_HEALTH_CHECK_PERIOD":      "db.pool.healthcheckperiod",
		"DB_CONNECT_TIMEOUT":          "db.pool.connecttimeout",
		"MIGRATIONS_DIR":              "db.migrationsdir",
	}
}
