// Package helpcenter provides this package.
package helpcenter

import (
	"embed"
	"regexp"

	"github.com/iota-uz/iota-sdk/modules/helpcenter/presentation/assets"
	"github.com/iota-uz/iota-sdk/modules/helpcenter/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/helpcenter/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/markdown"
)

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

// ContentFiles embeds the markdown help content so it ships inside the binary
// and is served without an on-disk content directory (deploy-safe under
// GOWORK=off). See EmbeddedContentConfig.
//
//go:embed content
var ContentFiles embed.FS

// EmbeddedContentConfig returns a ContentConfig backed by the embedded markdown
// docs (ContentFiles), rooted at "content". Register with NewComponent to serve
// /help/doc/... without a disk content dir. Set HideNav on the returned config
// to suppress the sidebar nav node when only inline help links are wanted.
func EmbeddedContentConfig() ContentConfig {
	return ContentConfig{
		FS:            ContentFiles,
		Root:          "content",
		DefaultLocale: "en",
		Locales:       []string{"en"},
	}
}

// articleClassPattern lists the Help Center presentation hooks an article may
// use in raw HTML, such as the course start page's step path and cards.
var articleClassPattern = regexp.MustCompile(`^help-(home-path|task-(grid|card|meta|title|description|outcome|footer))$`)

func newArticleRenderer() markdown.Renderer {
	return markdown.NewRendererWithOptions(markdown.WithAllowedClasses(articleClassPattern, "a", "div", "span"))
}

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
	composition.AddHashFS(builder, assets.HashFS)
	if !c.config.HideNav {
		composition.AddNavNodes(builder, HelpCenterNavNode)
	}
	composition.Provide[services.ContentConfig](builder, c.config)
	composition.ProvideFunc(builder, newArticleRenderer)
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
