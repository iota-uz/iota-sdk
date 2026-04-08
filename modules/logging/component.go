// Package logging provides this package.
package logging

import (
	"embed"

	"github.com/iota-uz/iota-sdk/pkg/composition"
)

//go:embed presentation/locales/*.json
var localeFiles embed.FS

//go:embed infrastructure/persistence/schema/logging-schema.sql
var migrationFiles embed.FS

func NewComponent() composition.Component {
	return &component{}
}

type component struct{}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "logging"}
}

func (c *component) Build(builder *composition.Builder) error {
	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&localeFiles}, nil
	})
	return nil
}
