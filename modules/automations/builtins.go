package automations

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/access"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/actions"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	sandbox "github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/secrets"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
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
		// Code and HTTP nodes use a connection only for a run-as subject that may: Connections.Use.
		ConnectionUse: permissions.ConnectionsUse.Name(),
		Runs: func() (engine.RunStarter, error) {
			if !composition.Has[engine.RunStarter](container) {
				return nil, errors.New("the automation engine is not composed")
			}
			return composition.Resolve[engine.RunStarter](container)
		},
	})
}

// actionsHook registers the node types of the action commands (actions.Native) of the application's
// native set when the application starts. The set refused a key registered twice when it was composed.
func actionsHook(container *composition.Container) composition.Hook {
	return composition.Hook{
		Name: "automations.actions",
		Start: func(context.Context) (composition.StopFn, error) {
			const op = serrors.Op("automations.actions")
			if !composition.Has[*native.Set](container) {
				return nil, nil // no extension runtime wiring: no native commands, no actions
			}
			set, err := composition.Resolve[*native.Set](container)
			if err != nil {
				return nil, serrors.Wrap(op, err)
			}
			nodes, err := actions.NodesOf(set)
			if err != nil {
				return nil, serrors.Wrap(op, err)
			}
			if len(nodes) == 0 {
				return nil, nil
			}
			reg, err := composition.Resolve[catalog.Registry](container)
			if err != nil {
				return nil, serrors.Wrap(op, err)
			}
			for _, n := range nodes {
				if err := reg.RegisterNode(n); err != nil {
					return nil, serrors.Wrap(op, err)
				}
			}
			return nil, nil
		},
	}
}

// lazySecrets reads secrets through the tenant's Secret connector (secrets.Resolver), then the container's
// host.SecretResolver (the connections' secret fields and what the application adds behind them). Each is
// resolved on first use, and the application's only when a name reaches it: a tenant secret resolves even
// when the application's resolver cannot be built.
type lazySecrets struct {
	container *composition.Container
	mu        sync.Mutex
	s         access.Secrets
}

func (l *lazySecrets) Secret(ctx context.Context, tenantID uuid.UUID, runAs spi.Subject, name string) (string, error) {
	l.mu.Lock()
	if l.s == nil {
		var tenant secrets.Resolver
		if composition.Has[secrets.Resolver](l.container) {
			var err error
			if tenant, err = composition.Resolve[secrets.Resolver](l.container); err != nil {
				l.mu.Unlock()
				return "", &host.InfraError{Err: err}
			}
		}
		// The tenant's Secret connector first (for a run-as that may use secrets), then the application's.
		l.s = access.ChainSecrets(tenant, permissions.SecretsUse.Name(), &lazyAppSecrets{container: l.container})
	}
	s := l.s
	l.mu.Unlock()
	return s.Secret(ctx, tenantID, runAs, name)
}

// lazyAppSecrets is the application's chain (access.FromResolver over host.SecretResolver), built when a name
// first reaches it; a failure to build it is not kept.
type lazyAppSecrets struct {
	container *composition.Container
	mu        sync.Mutex
	s         access.Secrets
}

func (l *lazyAppSecrets) Secret(ctx context.Context, tenantID uuid.UUID, runAs spi.Subject, name string) (string, error) {
	l.mu.Lock()
	if l.s == nil {
		var r host.SecretResolver
		if composition.Has[host.SecretResolver](l.container) {
			var err error
			if r, err = composition.Resolve[host.SecretResolver](l.container); err != nil {
				l.mu.Unlock()
				return "", &host.InfraError{Err: err}
			}
		}
		l.s = access.FromResolver(r)
	}
	s := l.s
	l.mu.Unlock()
	return s.Secret(ctx, tenantID, runAs, name)
}

// lazyConnections opens connections from the container's connection store and directory.
type lazyConnections struct {
	container *composition.Container
	mu        sync.Mutex
	c         access.Connections
}

func (l *lazyConnections) Open(ctx context.Context, tenantID uuid.UUID, name string) (access.Connection, error) {
	c, err := l.resolve()
	if err != nil {
		return access.Connection{}, &host.InfraError{Err: err}
	}
	return c.Open(ctx, tenantID, name)
}

// resolve builds the connections once they resolve; a failure is not kept.
func (l *lazyConnections) resolve() (access.Connections, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.c != nil {
		return l.c, nil
	}
	st, err := composition.Resolve[connectors.Store](l.container)
	if err != nil {
		return nil, err
	}
	dir, err := composition.Resolve[connectors.Directory](l.container)
	if err != nil {
		return nil, err
	}
	var oauth connectors.OAuth2
	if composition.Has[connectors.OAuth2](l.container) {
		if oauth, err = composition.Resolve[connectors.OAuth2](l.container); err != nil {
			return nil, err
		}
	}
	l.c = access.StoreConnections(st, dir, oauth)
	return l.c, nil
}
