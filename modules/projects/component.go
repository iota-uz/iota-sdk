// Package projects provides this package.
package projects

import (
	"embed"

	"github.com/iota-uz/iota-sdk/modules/projects/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/projects/services"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

//go:embed presentation/locales/*.json
var localeFiles embed.FS

func NewComponent() composition.Component {
	return &component{}
}

type component struct{}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{
		Name:     "projects",
		Requires: []string{"core"},
	}
}

func (c *component) LocaleFS() []*embed.FS {
	return []*embed.FS{&localeFiles}
}

func (c *component) Build(builder *composition.Builder) error {
	composition.AddNavNodes(builder, ProjectsNavNode)

	composition.ProvideFunc(builder, persistence.NewProjectRepository)
	composition.ProvideFunc(builder, persistence.NewProjectStageRepository)
	composition.ProvideFunc(builder, services.NewProjectService)
	composition.ProvideFunc(builder, services.NewProjectStageService)

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.AddControllers(builder,
			controllers.NewProjectController(),
			controllers.NewProjectStageController(),
		)
	}
	return nil
}
