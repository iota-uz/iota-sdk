package helpcenter

import (
	"embed"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/helpcenter/internal/fakesource"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/helpdocs"
	"github.com/stretchr/testify/require"
)

// False green: asserting the pattern string instead of rendered HTML, or a
// pattern that lets any help-* class through.
func TestArticleRendererKeepsOnlyHelpCenterHooks(t *testing.T) {
	html, err := newArticleRenderer().Render([]byte("<a class=\"help-task-card\" href=\"/help/doc/a.md\"><div class=\"help-task-title\">A</div></a>\n\n<div class=\"help-shell\">x</div>\n\n<div class=\"help-task-card evil\">y</div>"))

	require.NoError(t, err)
	out := string(html)
	require.Contains(t, out, `<a class="help-task-card" href="/help/doc/a.md"`)
	require.Contains(t, out, `<div class="help-task-title">A</div>`)
	require.Contains(t, out, `<div>x</div>`)
	require.Contains(t, out, `<div>y</div>`)
}

type stubComponent struct {
	name     string
	requires []string
	build    func(*composition.Builder) error
}

func (c stubComponent) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: c.name, Requires: c.requires}
}

func (c stubComponent) LocaleFS() []*embed.FS { return nil }

func (c stubComponent) Build(builder *composition.Builder) error {
	if c.build == nil {
		return nil
	}
	return c.build(builder)
}

func docsComponent(sources ...helpdocs.Source) composition.Component {
	return stubComponent{name: "docs", requires: []string{"core"}, build: func(builder *composition.Builder) error {
		composition.ContributeHelpSources(builder, func(*composition.Container) ([]helpdocs.Source, error) {
			return sources, nil
		})
		return nil
	}}
}

func compileHelpCenter(sources ...helpdocs.Source) (*composition.Container, error) {
	engine := composition.NewEngine()
	if err := engine.Register(
		stubComponent{name: "core"},
		NewComponent(EmbeddedContentConfig()),
		docsComponent(sources...),
	); err != nil {
		return nil, err
	}
	return engine.Compile(composition.BuildContext{})
}

func mounted(mount string) *fakesource.Source {
	return &fakesource.Source{MountPath: mount, Names: map[string]string{"": mount}}
}

// False green: asserting that the component builds with sources, which holds
// when the controller factory never reads them; a mount on the embedded
// content must fail the build, and only a factory that received the sources
// can notice.
func TestComponentBuildFailsOnMountCollision(t *testing.T) {
	tests := []struct {
		name    string
		sources []helpdocs.Source
		message string
	}{
		{"mount on a file category", []helpdocs.Source{mounted("modules")}, `help source mount "modules" collides with`},
		{"overlapping sources", []helpdocs.Source{mounted("extensions"), mounted("extensions/reference")}, `"extensions" and "extensions/reference" overlap`},
		{"invalid mount", []helpdocs.Source{mounted("/extensions")}, "clean slash-separated path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileHelpCenter(tt.sources...)

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.message)
		})
	}
}

func TestComponentBuildsWithSourcesAndWithout(t *testing.T) {
	for name, sources := range map[string][]helpdocs.Source{
		"no sources":     nil,
		"valid sources":  {mounted("extensions/reference"), mounted("extensions/guides")},
		"nested in file": {mounted("modules/extra")},
	} {
		t.Run(name, func(t *testing.T) {
			container, err := compileHelpCenter(sources...)

			require.NoError(t, err)
			require.Len(t, container.Controllers(), 1)
		})
	}
}
