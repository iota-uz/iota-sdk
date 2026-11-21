package composition

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type widget struct{ name string }

func TestContributionsAreCollectedPerTypeInRegistrationOrder(t *testing.T) {
	engine := NewEngine()
	var fromProvider error
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "owner"},
			build: func(builder *Builder) error {
				Provide[string](builder, func(c *Container) (string, error) {
					// A provider runs before contributions are collected.
					_, fromProvider = Contributions[widget](c)
					return "dep", nil
				})
				Contribute(builder, func(*Container) ([]widget, error) { return []widget{{name: "a"}}, nil })
				Contribute(builder, func(*Container) ([]int, error) { return []int{7}, nil })
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "app", Requires: []string{"owner"}},
			build: func(builder *Builder) error {
				Contribute(builder, func(c *Container) ([]widget, error) {
					dep, err := Resolve[string](c)
					if err != nil {
						return nil, err
					}
					return []widget{{name: "b-" + dep}, {name: "c"}}, nil
				})
				return nil
			},
		},
	)
	require.NoError(t, err)
	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	require.Error(t, fromProvider, "contributions are not readable while providers are instantiated")
	widgets, err := Contributions[widget](container)
	require.NoError(t, err)
	require.Equal(t, []widget{{name: "a"}, {name: "b-dep"}, {name: "c"}}, widgets)
	ints, err := Contributions[int](container)
	require.NoError(t, err)
	require.Equal(t, []int{7}, ints)
	none, err := Contributions[float64](container)
	require.NoError(t, err)
	require.Empty(t, none)
}

func TestContributionErrorFailsCompile(t *testing.T) {
	engine := NewEngine()
	require.NoError(t, engine.Register(testComponent{
		descriptor: Descriptor{Name: "broken"},
		build: func(builder *Builder) error {
			Contribute(builder, func(*Container) ([]widget, error) { return nil, errors.New("boom") })
			return nil
		},
	}))
	_, err := engine.Compile(BuildContext{})
	require.ErrorContains(t, err, "boom")
	require.ErrorContains(t, err, "broken")
}
