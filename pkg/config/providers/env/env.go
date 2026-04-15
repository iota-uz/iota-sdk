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
	"os"
	"strings"

	"github.com/joho/godotenv"
	koanfenv "github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/v2"

	"github.com/iota-uz/iota-sdk/pkg/config"
)

// New returns a Provider that:
//  1. Loads each of the given .env files in order (missing files are silently ignored;
//     malformed files return an error from Load).
//  2. Overlays the live process environment (os.Environ) on top.
//
// Key transform: single underscore → dot, lowercased. See package doc.
func New(files ...string) config.Provider {
	return &envProvider{files: files}
}

type envProvider struct {
	files []string
}

func (p *envProvider) Load(k *koanf.Koanf) error {
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
		TransformFunc: transformKey,
		EnvironFunc:   func() []string { return environ },
	})

	return k.Load(provider, nil)
}

// transformKey applies the locked single-underscore-to-dot transform.
// Double (or more) underscores are treated as literal underscores in the key
// (they collapse to a single underscore — a concern for W0.2 stdconfig naming).
//
// Transform steps:
//  1. Strip leading/trailing underscores.
//  2. Lowercase the whole string.
//  3. Replace each remaining "_" with ".".
func transformKey(k, v string) (string, any) {
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
