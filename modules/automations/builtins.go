package automations

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/access"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/actions"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	sandbox "github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// registerBuiltins registers the built-in node types (set, code, http, start_automation, run_automation)
// in the module's registry. What they reach is resolved from the container on first use: the secret
// resolver, the connection store and the engine (an engine.RunStarter the composition provides) are built
// after the registry, and the Code sandbox only when a process runs its first Code node.
func registerBuiltins(reg catalog.Registry, container *composition.Container) error {
	return builtin.Register(reg, builtin.Deps{
		Engines: func(ctx context.Context) (sandbox.Engine, error) {
			if composition.Has[sandbox.Engine](container) {
				return composition.Resolve[sandbox.Engine](container)
			}
			return sandbox.NewEngine(ctx)
		},
		Secrets:     &lazySecrets{container: container},
		Connections: &lazyConnections{container: container},
		Runs: func() (engine.RunStarter, error) {
			if !composition.Has[engine.RunStarter](container) {
				return nil, errors.New("the automation engine is not composed")
			}
			return composition.Resolve[engine.RunStarter](container)
		},
	})
}

// actionsHook registers the actions modules contributed (actions.Contribute) when the application starts;
// a key contributed twice fails the start.
func actionsHook(container *composition.Container) composition.Hook {
	return composition.Hook{
		Name: "automations.actions",
		Start: func(context.Context) (composition.StopFn, error) {
			const op = serrors.Op("automations.actions")
			list, err := actions.Contributed(container)
			if err != nil {
				return nil, serrors.Wrap(op, err)
			}
			if len(list) == 0 {
				return nil, nil
			}
			reg, err := composition.Resolve[catalog.Registry](container)
			if err != nil {
				return nil, serrors.Wrap(op, err)
			}
			if err := actions.Register(reg, list...); err != nil {
				return nil, serrors.Wrap(op, err)
			}
			return nil, nil
		},
	}
}

// lazySecrets reads secrets through the container's host.SecretResolver (the connections' secret fields
// and what the application adds behind them).
type lazySecrets struct {
	container *composition.Container
	once      sync.Once
	s         access.Secrets
}

func (l *lazySecrets) Secret(ctx context.Context, tenantID uuid.UUID, runAs spi.Subject, name string) (string, error) {
	l.once.Do(func() {
		var r host.SecretResolver
		if composition.Has[host.SecretResolver](l.container) {
			r, _ = composition.Resolve[host.SecretResolver](l.container)
		}
		l.s = access.FromResolver(r)
	})
	return l.s.Secret(ctx, tenantID, runAs, name)
}

// lazyConnections opens connections from the container's connection store and directory.
type lazyConnections struct {
	container *composition.Container
	once      sync.Once
	c         access.Connections
	err       error
}

func (l *lazyConnections) Open(ctx context.Context, tenantID uuid.UUID, name string) (access.Connection, error) {
	l.once.Do(func() {
		st, err := composition.Resolve[connectors.Store](l.container)
		if err != nil {
			l.err = err
			return
		}
		dir, err := composition.Resolve[connectors.Directory](l.container)
		if err != nil {
			l.err = err
			return
		}
		var oauth connectors.OAuth2
		if composition.Has[connectors.OAuth2](l.container) {
			oauth, _ = composition.Resolve[connectors.OAuth2](l.container)
		}
		l.c = access.StoreConnections(st, dir, oauth)
	})
	if l.err != nil {
		return access.Connection{}, &host.InfraError{Err: l.err}
	}
	return l.c.Open(ctx, tenantID, name)
}
