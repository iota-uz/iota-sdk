// Package yamlfile provides a config.Provider that loads a YAML file.
// A missing file is a silent no-op. Malformed YAML returns an error from Load.
package yamlfile

import (
	"errors"
	"os"

	koanfyaml "github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"

	"github.com/iota-uz/iota-sdk/pkg/config"
)

// Ensure *yamlProvider implements config.Provider at compile time.
var _ config.Provider = (*yamlProvider)(nil)

// New returns a Provider that loads path as YAML.
// If path does not exist, Load is a no-op and returns nil, nil.
// If path exists but is malformed YAML, Load returns a descriptive error.
func New(path string) config.Provider {
	return &yamlProvider{path: path}
}

type yamlProvider struct {
	path string
}

// Name returns "yamlfile:<path>".
func (p *yamlProvider) Name() string {
	return "yamlfile:" + p.path
}

// Load parses the YAML file and returns its contents as a map[string]any.
// Uses a temporary koanf instance internally to leverage the YAML parser.
func (p *yamlProvider) Load() (map[string]any, error) {
	if _, err := os.Stat(p.path); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	k := koanf.New(".")
	if err := k.Load(file.Provider(p.path), koanfyaml.Parser()); err != nil {
		return nil, err
	}
	return k.Raw(), nil
}
