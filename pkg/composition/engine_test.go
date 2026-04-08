package composition

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type chatService struct{}

func TestEngineCompileTopoSort(t *testing.T) {
	engine := NewEngine()
	var buildOrder []string

	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "core"},
			build: func(builder *Builder) error {
				buildOrder = append(buildOrder, "core")
				Provide[string](builder, func() string { return "core" })
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "crm", Requires: []string{"core"}},
			build: func(builder *Builder) error {
				buildOrder = append(buildOrder, "crm")
				Provide[int](builder, func() int { return 7 })
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "ui", Requires: []string{"crm"}},
			build: func(builder *Builder) error {
				buildOrder = append(buildOrder, "ui")
				return nil
			},
		},
	)
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{})
	require.NoError(t, err)
	require.Equal(t, []string{"core", "crm", "ui"}, buildOrder)
}

func TestEngineCompileDetectsCycles(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(
		testComponent{descriptor: Descriptor{Name: "a", Requires: []string{"b"}}},
		testComponent{descriptor: Descriptor{Name: "b", Requires: []string{"a"}}},
	)
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "component cycle")
	require.Contains(t, err.Error(), "a -> b -> a")
}

func TestEngineCompileReportsMissingDependencyPath(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "crm"},
		build: func(builder *Builder) error {
			missingProvider := UseNamed[string]("twilioProvider")
			Provide[*chatService](builder, func(container *Container) (*chatService, error) {
				if _, err := missingProvider.Resolve(container); err != nil {
					return nil, err
				}
				return &chatService{}, nil
			})
			return nil
		},
	})
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "crm -> chatService -> twilioProvider")
	require.Contains(t, strings.ToUpper(err.Error()), "NOT PROVIDED")
}

func TestCapabilityFilteringGatesProvidersAndHooks(t *testing.T) {
	engine := NewEngine()
	var apiBuilds, workerBuilds int
	var started, stopped []string

	err := engine.Register(
		testComponent{
			descriptor: Descriptor{Name: "api", Capabilities: []Capability{CapabilityAPI}},
			build: func(builder *Builder) error {
				Provide[string](builder, func() string {
					apiBuilds++
					return "api"
				})
				ContributeHooks(builder, func(*Container) ([]Hook, error) {
					return []Hook{{
						Name: "api",
						Start: func(context.Context, *Container) error {
							started = append(started, "api")
							return nil
						},
						Stop: func(context.Context, *Container) error {
							stopped = append(stopped, "api")
							return nil
						},
					}}, nil
				})
				return nil
			},
		},
		testComponent{
			descriptor: Descriptor{Name: "worker", Capabilities: []Capability{CapabilityWorker}},
			build: func(builder *Builder) error {
				Provide[int](builder, func() int {
					workerBuilds++
					return 1
				})
				ContributeHooks(builder, func(*Container) ([]Hook, error) {
					return []Hook{{
						Name: "worker",
						Start: func(context.Context, *Container) error {
							started = append(started, "worker")
							return nil
						},
						Stop: func(context.Context, *Container) error {
							stopped = append(stopped, "worker")
							return nil
						},
					}}, nil
				})
				return nil
			},
		},
	)
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{}, CapabilityAPI)
	require.NoError(t, err)
	require.Equal(t, 1, apiBuilds)
	require.Equal(t, 0, workerBuilds)

	err = engine.Start(context.Background(), container)
	require.NoError(t, err)
	require.Equal(t, []string{"api"}, started)

	err = engine.Stop(context.Background(), container)
	require.NoError(t, err)
	require.Equal(t, []string{"api"}, stopped)
}
