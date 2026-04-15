package bichatconfig

import "github.com/iota-uz/iota-sdk/pkg/configuration"

// FromLegacy produces a Config from the monolithic *configuration.Configuration.
//
// Fields that exist in the legacy struct are mapped directly. Fields that were
// read only from env vars at call-site (Langfuse, Applet, OpenAI.Model/BaseURL/ResolveIP)
// have no legacy struct equivalent and are left as zero values here; the env
// provider path populates them for new consumers.
//
// SetDefaults is called so zero-value Applet fields receive documented defaults.
func FromLegacy(c *configuration.Configuration) Config {
	cfg := Config{
		OpenAI: OpenAIConfig{
			// Legacy env var is OPENAI_API_KEY; the monolith stored it as OpenAIKey.
			APIKey: c.OpenAIKey,
			// Model, BaseURL, ResolveIP: env-only, no legacy field — left zero.
		},
		Knowledge: KnowledgeConfig{
			Dir:            c.BiChatKnowledgeDir,
			KBIndexPath:    c.BiChatKBIndexPath,
			SchemaMetadata: c.BiChatSchemaMetadataDir,
			AutoLoad:       c.BiChatKnowledgeAutoLoad,
		},
		// Langfuse and Applet: env-only, no legacy struct fields — left zero.
	}
	cfg.SetDefaults()
	return cfg
}
