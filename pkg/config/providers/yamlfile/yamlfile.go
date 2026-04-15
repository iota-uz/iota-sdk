// Package yamlfile provides a config.Provider that loads a YAML file.
// A missing file is a silent no-op. Malformed YAML returns an error from Load.
package yamlfile

import (
	"errors"
	"os"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"

	"github.com/iota-uz/iota-sdk/pkg/config"
)

// New returns a Provider that loads path as YAML.
// If path does not exist, Load is a no-op and returns nil.
// If path exists but is malformed YAML, Load returns a descriptive error.
func New(path string) config.Provider {
	return &yamlProvider{path: path}
}

type yamlProvider struct {
	path string
}

func (p *yamlProvider) Load(k *koanf.Koanf) error {
	if _, err := os.Stat(p.path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return k.Load(file.Provider(p.path), yaml.Parser())
}
