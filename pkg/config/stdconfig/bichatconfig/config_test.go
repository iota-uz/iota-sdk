package bichatconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	staticprov "github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/bichatconfig"
)

func TestIsConfigured(t *testing.T) {
	t.Parallel()

	t.Run("OpenAIConfig.IsConfigured false when empty", func(t *testing.T) {
		t.Parallel()
		c := bichatconfig.OpenAIConfig{}
		if c.IsConfigured() {
			t.Error("expected IsConfigured() = false when APIKey is empty")
		}
	})

	t.Run("OpenAIConfig.IsConfigured true when key set", func(t *testing.T) {
		t.Parallel()
		c := bichatconfig.OpenAIConfig{APIKey: "sk-test"}
		if !c.IsConfigured() {
			t.Error("expected IsConfigured() = true when APIKey is set")
		}
	})

	t.Run("LangfuseConfig.IsConfigured false when empty", func(t *testing.T) {
		t.Parallel()
		c := bichatconfig.LangfuseConfig{}
		if c.IsConfigured() {
			t.Error("expected IsConfigured() = false when PublicKey is empty")
		}
	})

	t.Run("LangfuseConfig.IsConfigured true when key set", func(t *testing.T) {
		t.Parallel()
		c := bichatconfig.LangfuseConfig{PublicKey: "pk-test"}
		if !c.IsConfigured() {
			t.Error("expected IsConfigured() = true when PublicKey is set")
		}
	})
}

func TestRoundTripFromSource(t *testing.T) {
	t.Parallel()

	src, err := config.Build(
		staticprov.New(map[string]any{
			"bichat.openai.apikey":            "sk-source",
			"bichat.openai.model":             "gpt-5",
			"bichat.langfuse.publickey":       "pk-lf",
			"bichat.langfuse.secretkey":       "sk-lf",
			"bichat.langfuse.baseurl":         "https://cloud.langfuse.com",
			"bichat.knowledge.dir":            "/data/kb",
			"bichat.knowledge.kbindexpath":    "/data/kb.bleve",
			"bichat.knowledge.schemametadata": "/data/tables",
			"bichat.knowledge.autoload":       true,
			"bichat.applet.viteurl":           "http://localhost:5174",
		}),
	)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}

	reg := config.NewRegistry(src)
	ptr, err := config.Register[bichatconfig.Config](reg)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if ptr.OpenAI.APIKey != "sk-source" {
		t.Errorf("OpenAI.APIKey: got %q, want %q", ptr.OpenAI.APIKey, "sk-source")
	}
	if ptr.OpenAI.Model != "gpt-5" {
		t.Errorf("OpenAI.Model: got %q, want %q", ptr.OpenAI.Model, "gpt-5")
	}
	if ptr.Langfuse.PublicKey != "pk-lf" {
		t.Errorf("Langfuse.PublicKey: got %q, want %q", ptr.Langfuse.PublicKey, "pk-lf")
	}
	if ptr.Knowledge.Dir != "/data/kb" {
		t.Errorf("Knowledge.Dir: got %q, want %q", ptr.Knowledge.Dir, "/data/kb")
	}
	if ptr.Knowledge.AutoLoad != true {
		t.Error("Knowledge.AutoLoad: expected true")
	}
	if ptr.Applet.ViteURL != "http://localhost:5174" {
		t.Errorf("Applet.ViteURL: got %q, want %q", ptr.Applet.ViteURL, "http://localhost:5174")
	}
}
