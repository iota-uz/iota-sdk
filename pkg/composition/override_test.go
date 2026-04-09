package composition

import (
	"context"
	"testing"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/stretchr/testify/require"
)

// overrideRepo is a minimal concrete type used to exercise
// ProvideDefault/RemoveProvider semantics.
type overrideRepo struct{ label string }

func TestProvideDefault_OverriddenByConcreteInSameBuilder(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "combo"},
		build: func(builder *Builder) error {
			// Default registered first.
			ProvideDefault[*overrideRepo](builder, func() *overrideRepo {
				return &overrideRepo{label: "default"}
			})
			// Plain Provide for the same key must silently win.
			Provide[*overrideRepo](builder, func() *overrideRepo {
				return &overrideRepo{label: "concrete"}
			})
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	resolved, err := Resolve[*overrideRepo](container)
	require.NoError(t, err)
	require.Equal(t, "concrete", resolved.label)
}

func TestProvideDefault_DownstreamConcreteWinsAcrossComponents(t *testing.T) {
	defaultCalls := 0
	concreteCalls := 0

	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "upstream"},
			build: func(builder *Builder) error {
				ProvideDefault[*overrideRepo](builder, func() *overrideRepo {
					defaultCalls++
					return &overrideRepo{label: "default"}
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "downstream", Requires: []string{"upstream"}},
			build: func(builder *Builder) error {
				Provide[*overrideRepo](builder, func() *overrideRepo {
					concreteCalls++
					return &overrideRepo{label: "concrete"}
				})
				return nil
			},
		},
	)
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	resolved, err := Resolve[*overrideRepo](container)
	require.NoError(t, err)
	require.Equal(t, "concrete", resolved.label)

	// The default factory must never run — the whole point of the override
	// mechanism is that resolved values come from the winning provider only.
	require.Equal(t, 0, defaultCalls, "default factory must not run after override")
	require.Equal(t, 1, concreteCalls, "concrete factory runs exactly once")
}

func TestProvideDefault_OnlyDefaultResolves_WhenNoOverride(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "solo"},
		build: func(builder *Builder) error {
			ProvideDefault[*overrideRepo](builder, func() *overrideRepo {
				return &overrideRepo{label: "default"}
			})
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	resolved, err := Resolve[*overrideRepo](container)
	require.NoError(t, err)
	require.Equal(t, "default", resolved.label)
}

func TestProvideDefault_TwoDefaultsCollide_Error(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "alpha"},
			build: func(builder *Builder) error {
				ProvideDefault[*overrideRepo](builder, func() *overrideRepo {
					return &overrideRepo{label: "alpha"}
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "beta", Requires: []string{"alpha"}},
			build: func(builder *Builder) error {
				ProvideDefault[*overrideRepo](builder, func() *overrideRepo {
					return &overrideRepo{label: "beta"}
				})
				return nil
			},
		},
	)
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate default provider")
	require.Contains(t, err.Error(), `"alpha"`)
	require.Contains(t, err.Error(), `"beta"`)
}

func TestRemoveProvider_ReplacesUpstreamNonDefault(t *testing.T) {
	upstreamCalls := 0
	downstreamCalls := 0

	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "upstream"},
			build: func(builder *Builder) error {
				// Upstream does NOT mark this overridable — downstream
				// still needs the escape hatch.
				Provide[*overrideRepo](builder, func() *overrideRepo {
					upstreamCalls++
					return &overrideRepo{label: "upstream"}
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "downstream", Requires: []string{"upstream"}},
			build: func(builder *Builder) error {
				RemoveProvider[*overrideRepo](builder)
				Provide[*overrideRepo](builder, func() *overrideRepo {
					downstreamCalls++
					return &overrideRepo{label: "downstream"}
				})
				return nil
			},
		},
	)
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	resolved, err := Resolve[*overrideRepo](container)
	require.NoError(t, err)
	require.Equal(t, "downstream", resolved.label)
	require.Equal(t, 0, upstreamCalls, "removed provider must not run")
	require.Equal(t, 1, downstreamCalls)
}

func TestRemoveProvider_NoopForMissingKey(t *testing.T) {
	// RemoveProvider for a key that was never provided should be a no-op,
	// not an error — downstream should be able to write defensive removals
	// without probing the container first.
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "x"},
		build: func(builder *Builder) error {
			RemoveProvider[*overrideRepo](builder)
			return nil
		},
	})
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{})
	require.NoError(t, err)
}

func TestRemoveProvider_WithoutReplacement_ResolvesToNotProvided(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "upstream"},
			build: func(builder *Builder) error {
				Provide[*overrideRepo](builder, func() *overrideRepo {
					return &overrideRepo{label: "upstream"}
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "downstream", Requires: []string{"upstream"}},
			build: func(builder *Builder) error {
				// Remove without providing a replacement — the engine
				// should surface NOT_PROVIDED when something resolves T.
				RemoveProvider[*overrideRepo](builder)
				return nil
			},
		},
	)
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	_, err = Resolve[*overrideRepo](container)
	require.Error(t, err)
	require.True(t, IsNotProvided(err))
}

// ----- RemoveController -----

// overrideCtrl is a minimal application.Controller used to exercise the
// controller-removal filter in materialize.
type overrideCtrl struct{ key string }

func (c *overrideCtrl) Key() string            { return c.key }
func (c *overrideCtrl) Register(_ *mux.Router) {}

func TestRemoveController_FiltersByKey(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "upstream"},
			build: func(builder *Builder) error {
				ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
					return []application.Controller{
						&overrideCtrl{key: "/keep"},
						&overrideCtrl{key: "/drop"},
					}, nil
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "downstream", Requires: []string{"upstream"}},
			build: func(builder *Builder) error {
				RemoveController(builder, "/drop")
				return nil
			},
		},
	)
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	ctrls := container.Controllers()
	require.Len(t, ctrls, 1)
	require.Equal(t, "/keep", ctrls[0].Key())
}

func TestRemoveController_NoopForMissingKey(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "x"},
		build: func(builder *Builder) error {
			RemoveController(builder, "/never-registered")
			return nil
		},
	})
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{})
	require.NoError(t, err)
}

// ----- RemoveHook -----

func TestRemoveHook_FiltersByName(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "upstream"},
			build: func(builder *Builder) error {
				// noopStop exists so the hook Start closures can return
				// an explicit no-op StopFn instead of tripping the nilnil
				// linter with a double-nil return.
				noopStop := func(context.Context) error { return nil }
				ContributeHooks(builder, func(*Container) ([]Hook, error) {
					return []Hook{
						{
							Name: "keep",
							Start: func(context.Context) (StopFn, error) {
								return noopStop, nil
							},
						},
						{
							Name: "drop",
							Start: func(context.Context) (StopFn, error) {
								t.Fatalf("removed hook Start must not run")
								return noopStop, nil
							},
						},
					}, nil
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "downstream", Requires: []string{"upstream"}},
			build: func(builder *Builder) error {
				RemoveHook(builder, "drop")
				return nil
			},
		},
	)
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	require.Len(t, container.Hooks(), 1)
	require.Equal(t, "keep", container.Hooks()[0].Name)

	// Start/Stop exercise the filtered hook list — if the "drop" hook
	// survived, its Start would t.Fatalf above.
	require.NoError(t, Start(context.Background(), container))
	require.NoError(t, Stop(context.Background(), container))
}

// ----- ProvideDefaultAs -----

// defaultImpl satisfies greetingPort (defined in fixtures_test.go) so we can
// exercise the interface-bridging variant of ProvideDefault.
type defaultImpl struct{ value string }

func (d *defaultImpl) Greet() string { return d.value }

func TestProvideDefaultAs_BridgesAndIsOverridable(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "upstream"},
			build: func(builder *Builder) error {
				ProvideDefaultAs[greetingPort, *defaultImpl](builder, func() *defaultImpl {
					return &defaultImpl{value: "default"}
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "downstream", Requires: []string{"upstream"}},
			build: func(builder *Builder) error {
				// Replace just the concrete; the interface bridge must
				// continue to resolve via the new concrete value.
				Provide[*defaultImpl](builder, func() *defaultImpl {
					return &defaultImpl{value: "override"}
				})
				return nil
			},
		},
	)
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	concrete, err := Resolve[*defaultImpl](container)
	require.NoError(t, err)
	require.Equal(t, "override", concrete.value)

	// The interface key still resolves, and (because of the bridge) it
	// points at the same overridden concrete instance.
	port, err := Resolve[greetingPort](container)
	require.NoError(t, err)
	require.Equal(t, "override", port.Greet())
	require.Same(t, concrete, port.(*defaultImpl))
}

func TestProvideDefaultAs_InterfaceKeyCanAlsoBeRemoved(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "upstream"},
			build: func(builder *Builder) error {
				ProvideDefaultAs[greetingPort, *defaultImpl](builder, func() *defaultImpl {
					return &defaultImpl{value: "default"}
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "downstream", Requires: []string{"upstream"}},
			build: func(builder *Builder) error {
				// Remove the concrete via RemoveProvider, replace with a
				// completely different struct that also satisfies the
				// interface. The interface bridge from the upstream will
				// have been removed too (it was overridable), so we need
				// to provide our own for the interface key.
				RemoveProvider[*defaultImpl](builder)
				RemoveProvider[greetingPort](builder)
				Provide[greetingPort](builder, func() greetingPort {
					return &defaultImpl{value: "replacement"}
				})
				return nil
			},
		},
	)
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	port, err := Resolve[greetingPort](container)
	require.NoError(t, err)
	require.Equal(t, "replacement", port.Greet())
}

// ----- ProvideAs / ProvideDefaultAs fail-fast guards -----

type notAnInterface struct{}

type notImplementingGreeting struct{}

func TestProvideAs_PanicsWhenTargetNotInterface(t *testing.T) {
	require.PanicsWithValue(t,
		"composition: ProvideAs target must be an interface, got composition.notAnInterface (struct)",
		func() {
			_ = compileWithBuild(t, func(builder *Builder) error {
				// `notAnInterface` is a concrete struct, not an interface.
				ProvideAs[notAnInterface, *defaultImpl](builder, &defaultImpl{})
				return nil
			})
		},
		"ProvideAs must reject a non-interface target type",
	)
}

func TestProvideAs_PanicsWhenConcreteDoesNotImplement(t *testing.T) {
	require.Panics(t, func() {
		_ = compileWithBuild(t, func(builder *Builder) error {
			// *notImplementingGreeting does not have a Greet() method, so
			// it cannot satisfy greetingPort.
			ProvideAs[greetingPort, *notImplementingGreeting](builder, &notImplementingGreeting{})
			return nil
		})
	})
}

func TestProvideDefaultAs_PanicsWhenTargetNotInterface(t *testing.T) {
	require.Panics(t, func() {
		_ = compileWithBuild(t, func(builder *Builder) error {
			ProvideDefaultAs[notAnInterface, *defaultImpl](builder, func() *defaultImpl {
				return &defaultImpl{}
			})
			return nil
		})
	})
}
