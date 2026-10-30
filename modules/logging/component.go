// Package logging provides this package.
package logging

import (
	"embed"

	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/logging/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

//go:embed presentation/locales/*.json
var localeFiles embed.FS

func NewComponent() composition.Component {
	return &component{}
}

type component struct{}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "logging"}
}

func (c *component) LocaleFS() []*embed.FS {
	return []*embed.FS{&localeFiles}
}

func (c *component) Build(builder *composition.Builder) error {
	composition.ProvideFunc(builder, func() (*history.Registry, error) {
		registry := history.NewRegistry()
		if err := registry.Register(history.LegacyActionLogDefinition()); err != nil {
			return nil, err
		}
		return registry, nil
	})
	composition.ProvideFunc(builder, persistence.NewBusinessEventRepository)
	composition.ProvideFunc(builder, history.NewWriter)
	return nil
}
