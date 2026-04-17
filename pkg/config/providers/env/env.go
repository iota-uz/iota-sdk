// Package env provides a config.Provider that loads .env files and overlays
// process environment variables.
//
// Key transform (locked — do not change without Wave approval):
// Single underscore → dot, lowercased. Leading/trailing underscores stripped.
//
//	BICHAT_OPENAI_API_KEY → bichat.openai.api_key
//	_LEADING → leading
//	TRAILING_ → trailing
package env

import (
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	koanfenv "github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/v2"

	"github.com/iota-uz/iota-sdk/pkg/config"
)

// Ensure *Provider implements config.Provider at compile time.
var _ config.Provider = (*Provider)(nil)

// Load implements config.Provider. It loads .env files and process environment
// variables, applies the key transform / aliases, and returns the result as a
// map[string]any. A temporary koanf instance is used internally; the result
// is extracted via k.Raw() and returned to the caller.
func (p *Provider) Load() (map[string]any, error) {
	k := koanf.New(".")
	if err := p.loadInto(k); err != nil {
		return nil, err
	}
	return k.Raw(), nil
}

// Provider is the env config provider. Use New to create and optionally chain
// WithAliases to register legacy env-var mappings.
type Provider struct {
	files    []string
	aliases  map[string]string
	warnOnce sync.Map // map[string]*sync.Once — fires at most once per alias key
}

// New returns a Provider that:
//  1. Loads each of the given .env files in order (missing files are silently ignored;
//     malformed files return an error from Load).
//  2. Overlays the live process environment (os.Environ) on top.
//
// Key transform: single underscore → dot, lowercased. See package doc.
//
// Wire legacy alias maps from stdconfig packages via WithAliases:
//
//	env.New(".env").WithAliases(stdconfig.AllLegacyAliases()...)
func New(files ...string) *Provider {
	return &Provider{
		files:   files,
		aliases: make(map[string]string),
	}
}

// WithAliases merges one or more env-var → koanf-path alias maps into the
// provider. Later maps override earlier ones on key collision. Returns p for
// chaining.
//
// Each alias hit produces a one-time slog.Warn log so operators know they are
// relying on deprecated env var names.
func (p *Provider) WithAliases(maps ...map[string]string) *Provider {
	for _, m := range maps {
		for k, v := range m {
			p.aliases[k] = v
		}
	}
	return p
}

// loadInto is the internal implementation that populates a koanf instance.
func (p *Provider) loadInto(k *koanf.Koanf) error {
	// Collect vars from .env files first (earlier files have lower precedence
	// than later ones, and all file vars have lower precedence than process env).
	fileVars := map[string]string{}
	for _, f := range p.files {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			// Silently skip missing files.
			continue
		}
		m, err := godotenv.Read(f)
		if err != nil {
			return err
		}
		for k, v := range m {
			fileVars[k] = v
		}
	}

	// Build a merged environ: file vars first, then process env overrides.
	// We supply this via EnvironFunc so the koanf env provider uses our merged set.
	processEnv := environToMap(os.Environ())
	merged := make(map[string]string, len(fileVars)+len(processEnv))
	for k, v := range fileVars {
		merged[k] = v
	}
	for k, v := range processEnv {
		merged[k] = v
	}

	environ := mapToEnviron(merged)

	provider := koanfenv.Provider(".", koanfenv.Opt{
		TransformFunc: p.transformKey,
		EnvironFunc:   func() []string { return environ },
	})

	return k.Load(provider, nil)
}

// transformKey applies the locked single-underscore-to-dot transform with a
// legacy-alias bypass for env vars whose natural transform doesn't match the
// stdconfig koanf paths (multi-word leaf names, bare top-level vars, or
// renamed prefixes).
//
// Transform steps:
//  1. If the key is a known legacy alias, return its mapped path verbatim and
//     emit a one-time slog.Warn.
//  2. Otherwise: strip leading/trailing underscores, lowercase, replace each
//     remaining "_" with ".".
func (p *Provider) transformKey(k, v string) (string, any) {
	if alias, ok := p.aliases[k]; ok {
		// Warn once per alias key so operators notice deprecated names.
		actual, _ := p.warnOnce.LoadOrStore(k, &sync.Once{})
		actual.(*sync.Once).Do(func() {
			slog.Warn("env: deprecated env var name — rename to canonical form",
				"legacy_key", k,
				"canonical_path", alias,
			)
		})
		return alias, v
	}
	k = strings.Trim(k, "_")
	k = strings.ToLower(k)
	k = strings.ReplaceAll(k, "_", ".")
	return k, v
}

func environToMap(environ []string) map[string]string {
	m := make(map[string]string, len(environ))
	for _, entry := range environ {
		key, val, _ := strings.Cut(entry, "=")
		m[key] = val
	}
	return m
}

func mapToEnviron(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}
