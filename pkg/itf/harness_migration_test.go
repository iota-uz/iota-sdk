package itf

import (
	"context"
	"embed"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/extruntimeconfig"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/wiring"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
)

// Regression for #1412: the automations module contributes its native
// contracts to the always-on extension core. When a package executor is
// attached to the core's *workflow.Engine the way runtime.New attaches it,
// the "extruntime.workflow" start hook validates those contracts against the
// package catalog, which reads extruntime.activations. The hook must see the
// migrated schema, so the harness applies the migrations of the modules it
// composes between compile and start, as production does.
func TestHarnessAppliesMigrationsBeforeStartHooks(t *testing.T) {
	h := OpenHarness(t, HarnessConfig{
		Name: "itf_migrations_before_start",
		Components: append(modules.Components(),
			packageExecutorComponent{},
			wiring.NewComponent(wiring.Options{Natives: modules.Natives(), Section: &extruntimeconfig.Config{}}),
		),
	})
	t.Cleanup(func() { require.NoError(t, h.Close()) })
	s := h.Scope(t)
	var exists bool
	require.NoError(t, s.Pool.QueryRow(context.Background(),
		"SELECT to_regclass('extruntime.activations') IS NOT NULL AND to_regclass('automation.triggers') IS NOT NULL",
	).Scan(&exists))
	require.True(t, exists)
}

// packageExecutorComponent attaches a package executor to the core's
// *workflow.Engine at compile, as runtime.New does when the extension
// runtime runs (pkg/extruntime/runtime/runtime.go).
type packageExecutorComponent struct{}

func (packageExecutorComponent) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "itf.package-executor"}
}

func (packageExecutorComponent) LocaleFS() []*embed.FS { return nil }

func (packageExecutorComponent) Build(b *composition.Builder) error {
	composition.ContributeHooks(b, func(container *composition.Container) ([]composition.Hook, error) {
		eng, err := composition.Resolve[*workflow.Engine](container)
		if err != nil {
			return nil, err
		}
		db, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		natives, err := composition.Resolve[*native.Set](container)
		if err != nil {
			return nil, err
		}
		if err := eng.AttachPackages(context.Background(), workflow.Packages{
			Catalog:   catalog.New(db, catalog.Natives(natives)),
			Bindings:  noBinding{},
			Artifacts: noArtifacts{},
			Impl:      noImpls{},
			Runner:    noRunner{},
		}); err != nil {
			return nil, err
		}
		return nil, nil
	})
	return nil
}

type noBinding struct{}

func (noBinding) Resolve(context.Context, string) (host.Binding, error) {
	return host.Binding{}, host.ErrNoBinding
}

type noArtifacts struct{}

func (noArtifacts) ResolveArtifact(context.Context, string, string) (pool.Artifact, []string, error) {
	return pool.Artifact{}, nil, errors.New("no package artifacts")
}

type noImpls struct{}

func (noImpls) Active(context.Context, string) (workflow.Implementation, error) {
	return workflow.Implementation{}, errors.New("no package implementations")
}

func (noImpls) At(context.Context, string, string, string) (workflow.Implementation, error) {
	return workflow.Implementation{}, errors.New("no package implementations")
}

type noRunner struct{}

func (noRunner) Run(context.Context, host.RunRequest) (host.Result, error) {
	return host.Result{}, errors.New("no package runner")
}
