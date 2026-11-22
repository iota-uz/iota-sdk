package wiring

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/jackc/pgx/v5"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// nativeWorkflows is one contribution of ContributeNativeWorkflows.
type nativeWorkflows map[string]workflow.NativeContract

// ContributeNativeWorkflows registers Go implementations of workflow
// contracts (docs/extension-runtime.md §9.3), like the composition.Contribute*
// helpers: the factory runs once when the application is composed and may
// resolve any provider of the container. The core's *workflow.Engine runs
// them with or without the extension runtime. Two contributions of one
// contract ID, or a contract ID an active package declares as its workflow,
// fail the composition.
func ContributeNativeWorkflows(b *composition.Builder, factory func(*composition.Container) (map[string]workflow.NativeContract, error)) {
	if factory == nil {
		return
	}
	composition.Contribute(b, func(c *composition.Container) ([]nativeWorkflows, error) {
		m, err := factory(c)
		if err != nil || len(m) == 0 {
			return nil, err
		}
		return []nativeWorkflows{m}, nil
	})
}

// taskObservers is one contribution of ContributeTaskObservers.
type taskObservers []workflow.TaskObserver

// ContributeTaskObservers registers observers of the human tasks of the
// core's *workflow.Engine (workflow.Config.TaskObservers): the factory runs
// once when the application is composed. Contributions are called in the
// order they were made, each in the transaction of the task change; an error
// rolls the change back. The TaskAudience the engine uses is resolved from
// the container (the core's roles and groups unless the application
// provides its own).
func ContributeTaskObservers(b *composition.Builder, factory func(*composition.Container) ([]workflow.TaskObserver, error)) {
	if factory == nil {
		return
	}
	composition.Contribute(b, func(c *composition.Container) ([]taskObservers, error) {
		list, err := factory(c)
		if err != nil || len(list) == 0 {
			return nil, err
		}
		return []taskObservers{list}, nil
	})
}

func collectTaskObservers(c *composition.Container) ([]workflow.TaskObserver, error) {
	list, err := composition.Contributions[taskObservers](c)
	if err != nil {
		return nil, err
	}
	var out []workflow.TaskObserver
	for _, l := range list {
		out = append(out, l...)
	}
	return out, nil
}

// lazyTaskObservers calls the contributed observers, collected on first use
// (a failed collection is retried by the next change, which it rolls back).
type lazyTaskObservers struct {
	load func() ([]workflow.TaskObserver, error)

	mu     sync.Mutex
	loaded bool
	list   []workflow.TaskObserver
}

func (l *lazyTaskObservers) observers() ([]workflow.TaskObserver, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.loaded {
		list, err := l.load()
		if err != nil {
			return nil, err
		}
		l.list, l.loaded = list, true
	}
	return l.list, nil
}

func (l *lazyTaskObservers) TaskChanged(ctx context.Context, tx pgx.Tx, t workflow.Task, change workflow.TaskChange) error {
	list, err := l.observers()
	if err != nil {
		return err
	}
	for _, o := range list {
		if err := o.TaskChanged(ctx, tx, t, change); err != nil {
			return err
		}
	}
	return nil
}

// collectNativeWorkflows merges the contributions; a duplicate ID is an error.
func collectNativeWorkflows(c *composition.Container) (map[string]workflow.NativeContract, error) {
	list, err := composition.Contributions[nativeWorkflows](c)
	if err != nil {
		return nil, err
	}
	return mergeNativeWorkflows(list)
}

func mergeNativeWorkflows(list []nativeWorkflows) (map[string]workflow.NativeContract, error) {
	out := map[string]workflow.NativeContract{}
	for _, m := range list {
		for _, id := range slices.Sorted(maps.Keys(m)) {
			if _, dup := out[id]; dup {
				return nil, serrors.New(serrors.Invalid, fmt.Sprintf("extruntime: native workflow %s is contributed twice", id))
			}
			out[id] = m[id]
		}
	}
	return out, nil
}

// buildWorkflows provides the one *workflow.Engine of the process. Without
// the runtime (runtimeOn false) its commands run on a host without a Node
// pool: native contracts execute through their Go defaults and package
// exports fail as unavailable. With the runtime, runtime.New attaches the
// package executor and its own host to this same engine.
func (c *component) buildWorkflows(b *composition.Builder, natives *native.Set, core coreConfig, runtimeOn bool) {
	composition.Provide[*workflow.Engine](b, func(container *composition.Container) (*workflow.Engine, error) {
		db, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		subjects, err := composition.Resolve[host.SubjectLoader](container)
		if err != nil {
			return nil, err
		}
		audience, err := composition.Resolve[workflow.TaskAudience](container)
		if err != nil {
			return nil, err
		}
		// The contributions are collected after the providers are built: the
		// engine reads them on first use (Ready at start, or the first start
		// or run without background work).
		cfg := workflow.Config{
			DB: db, Subjects: subjects, TaskAdminPermission: manage.PermOperate, Logger: c.opts.Logger,
			// Contributions are read once the container is compiled: on the
			// first task change.
			TaskAudience: audience, TaskObservers: []workflow.TaskObserver{&lazyTaskObservers{load: func() ([]workflow.TaskObserver, error) { return collectTaskObservers(container) }}},
			Concurrency:  core.workflowConcurrency,
			NativeSource: func() (map[string]workflow.NativeContract, error) { return collectNativeWorkflows(container) },
		}
		if runtimeOn {
			return workflow.New(cfg)
		}
		cat := catalog.New(db, catalog.Natives(natives))
		cfg.Catalog = cat
		eng, err := workflow.New(cfg)
		if err != nil {
			return nil, err
		}
		hcfg := host.Config{
			DB: db, Bindings: noBindings{}, Catalog: cat, Data: data.NewStore(cat, natives.EntityRegistry(), nil),
			Outbox: events.NewOutbox(cat), Starter: eng, Signals: eng, Natives: native.Bind(natives, container), Logger: c.opts.Logger,
		}
		if users, uerr := composition.Resolve[user.Repository](container); uerr == nil {
			hcfg.NativeContext = runtime.HistoryExecution(users)
		}
		h, err := host.New(hcfg)
		if err != nil {
			return nil, err
		}
		eng.SetCommands(h)
		return eng, nil
	})
}

// noBindings is the binding table of a process without the runtime: no
// package implements anything, so native contracts run their Go defaults.
type noBindings struct{}

func (noBindings) Resolve(context.Context, string) (host.Binding, error) {
	return host.Binding{}, host.ErrNoBinding
}

// startWorkflows serves the engine with the API or worker capability unless
// background work is disabled. Contributed after the runtime, so it starts
// after the package executor is attached and stops before the Node pool.
func startWorkflows(b *composition.Builder, core coreConfig) {
	bc := b.Context()
	if core.disableBackground || (!bc.HasCapability(composition.CapabilityAPI) && !bc.HasCapability(composition.CapabilityWorker)) {
		return
	}
	composition.ContributeHooks(b, func(container *composition.Container) ([]composition.Hook, error) {
		eng, err := composition.Resolve[*workflow.Engine](container)
		if err != nil {
			return nil, err
		}
		return []composition.Hook{{Name: "extruntime.workflow", Start: func(ctx context.Context) (composition.StopFn, error) {
			if err := eng.Ready(ctx); err != nil {
				return nil, err
			}
			eng.Serve(composition.LifecycleContext(ctx))
			return func(context.Context) error { eng.Shutdown(); return nil }, nil
		}}}, nil
	})
}
