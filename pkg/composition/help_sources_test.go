package composition

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/helpdocs"
	"github.com/stretchr/testify/require"
)

type mountedSource struct{ mount string }

func (s mountedSource) Mount() string       { return s.mount }
func (s mountedSource) Title(string) string { return s.mount }
func (s mountedSource) Order() int          { return 0 }
func (s mountedSource) Tree(context.Context, string) ([]helpdocs.Node, error) {
	return nil, nil
}
func (s mountedSource) Get(context.Context, string, string) (helpdocs.Document, error) {
	return helpdocs.Document{}, helpdocs.ErrNotFound
}

// False green: asserting Container.HelpSources after Compile, which passes even
// when controller factories (the Help Center's) ran before the sources were
// collected and saw an empty list.
func TestControllerFactorySeesContributedHelpSources(t *testing.T) {
	engine := NewEngine()
	var seen []string
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "docs-owner"},
			build: func(builder *Builder) error {
				ContributeHelpSources(builder, func(*Container) ([]helpdocs.Source, error) {
					return []helpdocs.Source{mountedSource{mount: "extensions/guides"}}, nil
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "helpcenter", Requires: []string{"docs-owner"}},
			build: func(builder *Builder) error {
				ContributeControllers(builder, func(container *Container) ([]application.Controller, error) {
					for _, source := range container.HelpSources() {
						seen = append(seen, source.Mount())
					}
					return nil, nil
				})
				return nil
			},
		},
	)
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)
	require.Equal(t, []string{"extensions/guides"}, seen)
	require.Len(t, container.HelpSources(), 1)
}
