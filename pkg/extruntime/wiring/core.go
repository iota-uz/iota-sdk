package wiring

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// coreConfig is what configures the core: the section and, with the runtime
// enabled, the runtime configuration after Options.Tune.
type coreConfig struct {
	retention         time.Duration
	events            events.Config
	schedules         events.SchedulerConfig
	disableBackground bool
}

// mergeSchedules fills the zero settings of primary from fallback.
func mergeSchedules(primary, fallback events.SchedulerConfig) events.SchedulerConfig {
	pick := func(p *time.Duration, f time.Duration) {
		if *p <= 0 {
			*p = f
		}
	}
	pick(&primary.PollInterval, fallback.PollInterval)
	pick(&primary.Lookback, fallback.Lookback)
	pick(&primary.BackoffBase, fallback.BackoffBase)
	pick(&primary.BlockedRecheck, fallback.BlockedRecheck)
	if primary.MaxAttempts <= 0 {
		primary.MaxAttempts = fallback.MaxAttempts
	}
	if primary.Now == nil {
		primary.Now = fallback.Now // the clock a Tune configured
	}
	if primary.Tenants == nil {
		primary.Tenants = fallback.Tenants
	}
	if primary.Logger == nil {
		primary.Logger = fallback.Logger
	}
	return primary
}

// mergeEvents fills the zero settings of primary from fallback.
func mergeEvents(primary, fallback events.Config) events.Config {
	pick := func(p *time.Duration, f time.Duration) {
		if *p <= 0 {
			*p = f
		}
	}
	pick(&primary.PollInterval, fallback.PollInterval)
	pick(&primary.BackoffBase, fallback.BackoffBase)
	pick(&primary.BackoffMax, fallback.BackoffMax)
	pick(&primary.BlockedRecheck, fallback.BlockedRecheck)
	pick(&primary.Retention, fallback.Retention)
	pick(&primary.PruneInterval, fallback.PruneInterval)
	if primary.BatchSize <= 0 {
		primary.BatchSize = fallback.BatchSize
	}
	if primary.Concurrency <= 0 {
		primary.Concurrency = fallback.Concurrency
	}
	if primary.MaxAttempts <= 0 {
		primary.MaxAttempts = fallback.MaxAttempts
	}
	if primary.Logger == nil {
		primary.Logger = fallback.Logger
	}
	return primary
}

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

// ContributeSchedules registers a source of Go schedules (events.ScheduleDef
// with a Handler), like the composition.Contribute* helpers: the factory runs
// once when the application is composed and may resolve any provider of the
// container. The source is read on every scheduler pass, so schedules that
// appear or disappear at run time (those of a tenant's automation graphs, for
// example) take effect without a restart. The scheduler needs no Node
// runtime: with the runtime enabled the package schedules join the same
// scheduler. A nil source is ignored.
func ContributeSchedules(b *composition.Builder, factory func(*composition.Container) (events.ScheduleSource, error)) {
	if factory == nil {
		return
	}
	composition.Contribute(b, func(c *composition.Container) ([]events.ScheduleSource, error) {
		src, err := factory(c)
		if err != nil || src == nil {
			return nil, err
		}
		return []events.ScheduleSource{src}, nil
	})
}

// contributedSchedules reads the contributed sources on every pass: they are
// collected after the scheduler is constructed. It includes the package
// schedules when the runtime is enabled (the runtime contributes them).
func contributedSchedules(container *composition.Container) events.ScheduleSource {
	return events.ScheduleSourceFunc(func(ctx context.Context) ([]events.ScheduleDef, error) {
		srcs, err := composition.Contributions[events.ScheduleSource](container)
		if err != nil {
			return nil, err
		}
		return events.ComposeSchedules(srcs...).ActiveSchedules(ctx)
	})
}

// lazyRunner runs package schedules through the events.Runner the runtime
// provides. It is resolved on first use: the runtime depends on the core, so
// the scheduler cannot resolve it while the core is being built. Without a
// runtime the slot is blocked (events.ErrRunnerUnavailable).
type lazyRunner struct{ container *composition.Container }

func (l lazyRunner) Run(ctx context.Context, req host.RunRequest) (host.Result, error) {
	r, err := composition.Resolve[events.Runner](l.container)
	if composition.IsNotProvided(err) {
		return host.Result{}, fmt.Errorf("%w: package schedule %s needs the extension runtime, which is not enabled", events.ErrRunnerUnavailable, req.Export.Resource)
	}
	if err != nil {
		// Not the handler's failure: the slot is retried without counting.
		return host.Result{}, &host.InfraError{Err: err}
	}
	return r.Run(ctx, req)
}

// lazySubjects loads the owner of a package schedule with the runtime's own
// loader (runtime.UserSubjects over Config.PermissionUniverse), exactly as
// the runtime's scheduler did, and falls back to the core's loader when there
// is no runtime or it has none. It is resolved on use for the reason
// lazyRunner is.
type lazySubjects struct {
	core    host.SubjectLoader
	runtime func() (*runtime.Runtime, bool)
}

func (l lazySubjects) loader() host.SubjectLoader {
	if rt, ok := l.runtime(); ok && rt != nil && rt.Subjects != nil {
		return rt.Subjects
	}
	return l.core
}

func (l lazySubjects) LoadSubject(ctx context.Context, tenantID uuid.UUID, userID int64, kind spi.SubjectKind) (spi.Subject, error) {
	return l.loader().LoadSubject(ctx, tenantID, userID, kind)
}

// buildCore provides the always-on part: the native set, the subject loader,
// the outbox dispatcher and the scheduler.
func (c *component) buildCore(b *composition.Builder, natives *native.Set, core coreConfig) {
	composition.Contribute(b, func(*composition.Container) ([]events.DispatcherOwner, error) {
		return []events.DispatcherOwner{{Name: "wiring"}}, nil
	})
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
		cfg := core.events
		cfg.DB, cfg.Subjects, cfg.Consumers = db, subjects, contributedConsumers(container)
		if cfg.Retention <= 0 {
			cfg.Retention = core.retention
		}
		if cfg.Logger == nil {
			cfg.Logger = c.opts.Logger
		}
		return events.NewDispatcher(cfg)
	})
	composition.Provide[*events.Scheduler](b, func(container *composition.Container) (*events.Scheduler, error) {
		db, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		subjects, err := composition.Resolve[host.SubjectLoader](container)
		if err != nil {
			return nil, err
		}
		cfg := core.schedules
		cfg.DB, cfg.Schedules, cfg.Runner = db, contributedSchedules(container), lazyRunner{container: container}
		// Go schedules resolve no subject; package schedules run as their
		// owner with the runtime's loader, not the core's.
		cfg.Subjects = lazySubjects{core: subjects, runtime: func() (*runtime.Runtime, bool) {
			rt, err := composition.Resolve[*runtime.Runtime](container)
			return rt, err == nil
		}}
		if cfg.Logger == nil {
			cfg.Logger = c.opts.Logger
		}
		return events.NewScheduler(cfg)
	})
}

// startCore serves the dispatcher and the scheduler with the API or worker
// capability unless the section disables background work. It is contributed
// after the runtime, so they start after the Node pool is up and stop before
// it.
func startCore(b *composition.Builder, core coreConfig) {
	bc := b.Context()
	if core.disableBackground || (!bc.HasCapability(composition.CapabilityAPI) && !bc.HasCapability(composition.CapabilityWorker)) {
		return
	}
	composition.ContributeHooks(b, func(container *composition.Container) ([]composition.Hook, error) {
		d, err := composition.Resolve[*events.Dispatcher](container)
		if err != nil {
			return nil, err
		}
		sched, err := composition.Resolve[*events.Scheduler](container)
		if err != nil {
			return nil, err
		}
		return []composition.Hook{
			{Name: "extruntime.events", Start: func(ctx context.Context) (composition.StopFn, error) {
				d.Start(composition.LifecycleContext(ctx))
				return func(context.Context) error { d.Stop(); return nil }, nil
			}},
			{Name: "extruntime.schedules", Start: func(ctx context.Context) (composition.StopFn, error) {
				sched.Start(composition.LifecycleContext(ctx))
				return func(context.Context) error { sched.Stop(); return nil }, nil
			}},
		}, nil
	})
}
