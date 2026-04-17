// Package bichatconfig provides typed configuration for the BiChat module.
// It is a stdconfig package intended to be registered via config.Register[bichatconfig.Config].
package bichatconfig

// OpenAIConfig groups OpenAI API settings.
//
// Env prefix: "bichat.openai" (e.g. BICHAT_OPENAI_API_KEY → bichat.openai.apikey,
// BICHAT_OPENAI_MODEL → bichat.openai.model, BICHAT_OPENAI_BASE_URL → bichat.openai.baseurl,
// BICHAT_OPENAI_API_RESOLVE_IP → bichat.openai.resolveip).
//
// Note: legacy env var OPENAI_API_KEY maps to this field via FromLegacy / the
// env provider's single-underscore dot transform.
type OpenAIConfig struct {
	APIKey    string `koanf:"apikey"    secret:"true"`
	Model     string `koanf:"model"`
	BaseURL   string `koanf:"baseurl"`
	ResolveIP string `koanf:"resolveip"`
}

// IsConfigured reports whether an OpenAI API key is present.
func (c OpenAIConfig) IsConfigured() bool {
	return c.APIKey != ""
}

// LangfuseConfig groups Langfuse observability settings.
//
// Env prefix: "bichat.langfuse" (e.g. BICHAT_LANGFUSE_PUBLIC_KEY → bichat.langfuse.publickey,
// BICHAT_LANGFUSE_SECRET_KEY → bichat.langfuse.secretkey,
// BICHAT_LANGFUSE_BASE_URL → bichat.langfuse.baseurl,
// BICHAT_LANGFUSE_HOST → bichat.langfuse.host).
type LangfuseConfig struct {
	PublicKey string `koanf:"publickey" secret:"true"`
	SecretKey string `koanf:"secretkey" secret:"true"`
	BaseURL   string `koanf:"baseurl"`
	Host      string `koanf:"host"`
}

// IsConfigured reports whether a Langfuse public key is present.
func (c LangfuseConfig) IsConfigured() bool {
	return c.PublicKey != ""
}

// KnowledgeConfig groups knowledge-base and schema-metadata settings.
//
// Env prefix: "bichat.knowledge" (e.g. BICHAT_KNOWLEDGE_DIR → bichat.knowledge.dir,
// BICHAT_KNOWLEDGE_KB_INDEX_PATH → bichat.knowledge.kbindexpath,
// BICHAT_KNOWLEDGE_SCHEMA_METADATA → bichat.knowledge.schemametadata,
// BICHAT_KNOWLEDGE_AUTO_LOAD → bichat.knowledge.autoload).
type KnowledgeConfig struct {
	Dir            string `koanf:"dir"`
	KBIndexPath    string `koanf:"kbindexpath"`
	SchemaMetadata string `koanf:"schemametadata"`
	AutoLoad       bool   `koanf:"autoload"`
}

// AppletConfig groups dev-mode Vite applet settings.
//
// Env prefix: "bichat.applet" (e.g. BICHAT_APPLET_VITE_URL → bichat.applet.viteurl,
// BICHAT_APPLET_ENTRY → bichat.applet.entry,
// BICHAT_APPLET_CLIENT → bichat.applet.client).
type AppletConfig struct {
	ViteURL string `koanf:"viteurl"`
	Entry   string `koanf:"entry"`
	Client  string `koanf:"client"`
}

// Config holds all BiChat module configuration.
//
// Env prefix: "bichat". Sub-groups follow their respective prefixes above.
type Config struct {
	OpenAI    OpenAIConfig    `koanf:"openai"`
	Langfuse  LangfuseConfig  `koanf:"langfuse"`
	Knowledge KnowledgeConfig `koanf:"knowledge"`
	Applet    AppletConfig    `koanf:"applet"`
}

// ConfigPrefix returns the koanf prefix for bichatconfig ("bichat").
func (Config) ConfigPrefix() string { return "bichat" }

// SetDefaults fills zero-value Applet fields with documented defaults.
func (c *Config) SetDefaults() {
	if c.Applet.ViteURL == "" {
		c.Applet.ViteURL = "http://localhost:5173"
	}
	if c.Applet.Entry == "" {
		c.Applet.Entry = "/src/main.tsx"
	}
	if c.Applet.Client == "" {
		c.Applet.Client = "/@vite/client"
	}
}
