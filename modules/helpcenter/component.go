package helpcenter

import (
	"embed"

	"github.com/iota-uz/iota-sdk/modules/helpcenter/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/helpcenter/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/markdown"
)

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

func NewComponent(config ContentConfig) composition.Component {
	return &component{config: config.Normalized()}
}

type component struct {
	config ContentConfig
}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "helpcenter", Requires: []string{"core"}}
}

func (c *component) LocaleFS() []*embed.FS {
	return []*embed.FS{&LocaleFiles}
}

func (c *component) Build(builder *composition.Builder) error {
	composition.AddNavNodes(builder, HelpCenterNavNode)
	composition.Provide[services.ContentConfig](builder, c.config)
	composition.ProvideFunc(builder, markdown.NewRenderer)
	composition.ProvideFunc(builder, services.NewContentService)

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllers(builder, func(container *composition.Container) ([]application.Controller, error) {
			contentService, err := composition.Resolve[*services.ContentService](container)
			if err != nil {
				return nil, err
			}
			renderer, err := composition.Resolve[markdown.Renderer](container)
			if err != nil {
				return nil, err
			}
			searcher, _ := composition.Resolve[kb.KBSearcher](container)

			return []application.Controller{
				controllers.NewHelpCenterController(controllers.HelpCenterControllerConfig{
					BasePath:       "/help",
					ContentService: contentService,
					Renderer:       renderer,
					Searcher:       searcher,
				}),
			}, nil
		})
	}

	return nil
}
