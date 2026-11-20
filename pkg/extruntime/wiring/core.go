package wiring

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/extruntimeconfig"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
)

// ContributeConsumers registers Go consumers of the outbox, like the
// composition.Contribute* helpers: the factory runs once when the
// application is composed and may resolve any provider of the container.
// Each consumer chooses its events, watermark and run-as subject (see
// events.Consumer and events.ResolveSubject).
func ContributeConsumers(b *composition.Builder, factory func(*composition.Container) ([]events.Consumer, error)) {
	if factory == nil {
		return
	}
	ContributeConsumerSources(b, func(c *composition.Container) ([]events.ConsumerSource, error) {
		list, err := factory(c)
		if err != nil || len(list) == 0 {
			return nil, err
		}
		return []events.ConsumerSource{events.StaticConsumers(list...)}, nil
	})
}

// ContributeConsumerSources registers sources of Go consumers that change at
// run time: a source is read on every dispatcher pass.
func ContributeConsumerSources(b *composition.Builder, factory func(*composition.Container) ([]events.ConsumerSource, error)) {
	composition.Contribute(b, factory)
}

// contributedConsumers reads the contributed sources on every pass: they are
// collected after the dispatcher is constructed.
func contributedConsumers(container *composition.Container) events.ConsumerSource {
	return events.ConsumerSourceFunc(func(ctx context.Context) ([]events.Consumer, error) {
		srcs, err := composition.Contributions[events.ConsumerSource](container)
		if err != nil {
			return nil, err
		}
		return events.ComposeConsumers(srcs...).Consumers(ctx)
	})
}

// buildCore provides the always-on part: the native set, the subject loader
// and the outbox dispatcher.
func (c *component) buildCore(b *composition.Builder, section extruntimeconfig.Config, natives *native.Set) {
	composition.Provide[*native.Set](b, natives)
	// Applications with their own user model replace the loader.
	composition.ProvideDefault[host.SubjectLoader](b, func(container *composition.Container) (host.SubjectLoader, error) {
		db, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		users, err := composition.Resolve[user.Repository](container)
		if err != nil {
			return nil, err
		}
		return &runtime.UserSubjects{Users: users, Universe: corePerms.Permissions, Pool: db}, nil
	})
	composition.Provide[*events.Dispatcher](b, func(container *composition.Container) (*events.Dispatcher, error) {
		db, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		subjects, err := composition.Resolve[host.SubjectLoader](container)
		if err != nil {
			return nil, err
		}
		cfg := c.opts.Events
		cfg.DB, cfg.Subjects, cfg.Consumers = db, subjects, contributedConsumers(container)
		if cfg.Retention <= 0 {
			cfg.Retention = section.Events.Retention
		}
		if cfg.Logger == nil {
			cfg.Logger = c.opts.Logger
		}
		return events.NewDispatcher(cfg)
	})
}

// startCore serves the dispatcher with the API or worker capability unless
// the section disables background work. It is contributed after the runtime,
// so the dispatcher starts after the Node pool is up and stops before it.
func startCore(b *composition.Builder, section extruntimeconfig.Config) {
	bc := b.Context()
	if section.DisableBackground || (!bc.HasCapability(composition.CapabilityAPI) && !bc.HasCapability(composition.CapabilityWorker)) {
		return
	}
	composition.ContributeHooks(b, func(container *composition.Container) ([]composition.Hook, error) {
		d, err := composition.Resolve[*events.Dispatcher](container)
		if err != nil {
			return nil, err
		}
		return []composition.Hook{{Name: "extruntime.events", Start: func(ctx context.Context) (composition.StopFn, error) {
			d.Start(composition.LifecycleContext(ctx))
			return func(context.Context) error { d.Stop(); return nil }, nil
		}}}, nil
	})
}
