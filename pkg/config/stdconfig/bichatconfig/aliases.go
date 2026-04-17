package bichatconfig

// LegacyAliases returns the env-var → koanf-path alias map for bichatconfig.
// Includes legacy OPENAI_* and BICHAT_KNOWLEDGE_* env var names.
func LegacyAliases() map[string]string {
	return map[string]string{
		"OPENAI_API_KEY":              "bichat.openai.apikey",
		"OPENAI_KEY":                  "bichat.openai.apikey", // alternate legacy name
		"OPENAI_MODEL":                "bichat.openai.model",
		"OPENAI_BASE_URL":             "bichat.openai.baseurl",
		"OPENAI_API_RESOLVE_IP":       "bichat.openai.resolveip",
		"LANGFUSE_PUBLIC_KEY":         "bichat.langfuse.publickey",
		"LANGFUSE_SECRET_KEY":         "bichat.langfuse.secretkey",
		"LANGFUSE_BASE_URL":           "bichat.langfuse.baseurl",
		"LANGFUSE_HOST":               "bichat.langfuse.host",
		"BICHAT_KNOWLEDGE_DIR":        "bichat.knowledge.dir",
		"BICHAT_KB_INDEX_PATH":        "bichat.knowledge.kbindexpath",
		"BICHAT_SCHEMA_METADATA_DIR":  "bichat.knowledge.schemametadata",
		"BICHAT_KNOWLEDGE_AUTO_LOAD":  "bichat.knowledge.autoload",
		"IOTA_APPLET_VITE_URL_BICHAT": "bichat.applet.viteurl",
		"IOTA_APPLET_ENTRY_BICHAT":    "bichat.applet.entry",
		"IOTA_APPLET_CLIENT_BICHAT":   "bichat.applet.client",
	}
}
