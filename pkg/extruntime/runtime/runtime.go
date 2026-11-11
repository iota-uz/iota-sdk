// Package runtime is the composition root of the extension runtime: it wires
// the Node pool, the invocation host, the data store, the active catalog, the
// management service, the durable workflow engine, the event dispatcher and
// scheduler, the notification adapter and the UI slice, and ships the
// iota-sdk component that enables all of it.
package runtime

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/artifact"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/notify"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Config is the runtime configuration. It is what an application sets when
// it enables the runtime component.
type Config struct {
	// NodeBinary is the node executable. Default "node".
	NodeBinary string
	// WorkerScript is the path of the host worker (dist/host/worker.mjs of
	// the SDK distribution). Required.
	WorkerScript string
	// PoolSize is the number of worker processes. Default 2.
	PoolSize int
	// CacheDir is where artifacts are materialized for the workers and the
	// UI asset route. Required.
	CacheDir string
	// Git bounds repository imports.
	Git manage.GitConfig

	// Builder is the build worker adapter (Check/Build/Test). Required.
	Builder build.Builder
	// Kits renders the developer kit. Without it, builds cannot run.
	Kits build.KitGenerator

	// Natives are the native entity descriptors. Default data.DefaultNatives.
	Natives *data.NativeRegistry
	// NativeResources are additional native contracts (commands, operations,
	// pages, slots) published in the catalog and the kit. Native events
	// declared through events.RegisterNative are included automatically.
	NativeResources []manifest.Resource
	// NativeLists are the native lists packages may extend with columns.
	// Default: the CRM clients list.
	NativeLists []string
	// PermissionUniverse lists the permissions a trigger owner's rights are
	// computed over. Default: the core permissions.
	PermissionUniverse []permission.Permission

	// Secrets serves secrets.get for secrets a package declared.
	Secrets host.SecretResolver
	// Workflow and Events tune the durable engine, the event dispatcher and
	// the scheduler; zero values select their defaults.
	Workflow  WorkflowConfig
	Events    events.Config
	Schedules events.SchedulerConfig
	// DisableBackground keeps the engine, dispatcher and scheduler from being
	// served by Start (their RunOnce entry points stay available).
	DisableBackground bool

	// UI enables the runtime UI slice (page host, navigation, shell, native
	// list columns) on top of the management storage. Nil disables it.
	UI *UIConfig
	// Endpoints configures the package HTTP endpoints served under
	// /_ext/endpoints (mounted with the API capability).
	Endpoints EndpointConfig

	// Extensions attach after the runtime is built (MCP tools, extra
	// consumers); each receives the Runtime and returns a stop function.
	Extensions []Extension

	Logger *slog.Logger

	// bind publishes the built Runtime to lazily wired consumers (the UI
	// component is constructed before the container builds the Runtime).
	bind *holder
}

// WorkflowConfig tunes the workflow engine.
type WorkflowConfig struct {
	Concurrency int
}

// Deps are the pieces of the host application the runtime builds on.
type Deps struct {
	DB *pgxpool.Pool
	// Notification wiring; when Dispatch is nil the host has no notifier and
	// tx.notify fails explicitly.
	Dispatch  notify.Dispatcher
	Notices   *notifications.Catalog
	Renderers *notifications.ContentRenderers
	Contacts  *notifications.ContactResolvers
	// Users loads the current rights of owners and initiators.
	Users user.Repository
}

// Extension is a slice that needs the finished runtime.
type Extension interface {
	Name() string
	// Attach starts the slice. The returned stop function runs on Close.
	Attach(ctx context.Context, rt *Runtime) (stop func(context.Context) error, err error)
}

// Runtime holds the wired parts. Fields are read-only after New.
type Runtime struct {
	Config    Config
	Pool      *pool.Pool
	Host      *host.Host
	Data      *data.Store
	Bindings  *host.PgBindings
	Catalog   *catalog.Catalog
	Natives   *catalog.NativeSet
	Artifacts *artifact.Store
	Manage    *manage.Service
	// Workflows is the durable engine (host.WorkflowHandler and spi.Starter).
	Workflows  *workflow.Engine
	Outbox     *events.Outbox
	Dispatcher *events.Dispatcher
	Scheduler  *events.Scheduler
	// Notify is nil without notification dependencies.
	Notify   *notify.Adapter
	Notifier *notify.Notifier
	// Subjects loads current rights; nil without a user repository.
	Subjects *UserSubjects

	db  *pgxpool.Pool
	log *slog.Logger

	mu    sync.Mutex
	stops []func(context.Context) error
}

// guardRef lets the data store reference the host (its write-rule guard)
// before the host exists: the host needs the store, the store needs the host.
type guardRef struct{ h *host.Host }

func (g *guardRef) CheckWrite(ctx context.Context, tx pgx.Tx, sc spi.Scope, c spi.WriteChange) error {
	if g.h == nil {
		return nil
	}
	return g.h.CheckWrite(ctx, tx, sc, c)
}

// New wires the runtime. It starts the Node pool but no background loops;
// call Start.
func New(cfg Config, deps Deps) (*Runtime, error) {
	const op = serrors.Op("extruntime.runtime.New")
	switch {
	case deps.DB == nil:
		return nil, serrors.New(serrors.Invalid, "extruntime: database is required").WithOp(op)
	case cfg.WorkerScript == "", cfg.CacheDir == "":
		return nil, serrors.New(serrors.Invalid, "extruntime: WorkerScript and CacheDir are required").WithOp(op)
	case cfg.Builder == nil:
		return nil, serrors.New(serrors.Invalid, "extruntime: Builder is required").WithOp(op)
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Natives == nil {
		cfg.Natives = data.DefaultNatives()
	}
	if len(cfg.PermissionUniverse) == 0 {
		cfg.PermissionUniverse = corePerms.Permissions
	}
	rt := &Runtime{Config: cfg, db: deps.DB, log: cfg.Logger}
	if deps.Users != nil {
		rt.Subjects = &UserSubjects{Users: deps.Users, Universe: cfg.PermissionUniverse, Pool: deps.DB}
	}

	var err error
	if rt.Artifacts, err = artifact.NewStore(deps.DB, cfg.CacheDir); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rt.Natives = catalog.NewNativeSet(cfg.Natives)
	native := append([]manifest.Resource{}, events.Default.Resources()...)
	native = append(native, cfg.NativeResources...)
	if cfg.UI != nil {
		for id, path := range cfg.UI.NativeRoutes {
			native = append(native, manifest.Resource{ID: id, Kind: manifest.ResRoute, Path: path})
		}
	}
	for _, r := range native {
		if err := rt.Natives.Register(r); err != nil {
			return nil, serrors.Wrap(op, err)
		}
	}
	lists := cfg.NativeLists
	if len(lists) == 0 {
		lists = []string{data.NativeClientList}
	}
	for _, l := range lists {
		if err := rt.Natives.RegisterList(l); err != nil {
			return nil, serrors.Wrap(op, err)
		}
	}
	rt.Catalog = catalog.New(deps.DB, rt.Natives)
	if rt.Manage, err = manage.New(manage.Config{
		DB: deps.DB, Builder: cfg.Builder, Kits: cfg.Kits, Artifacts: rt.Artifacts, Catalog: rt.Catalog,
		Natives: cfg.Natives, Git: cfg.Git, Logger: cfg.Logger,
	}); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	guard := &guardRef{}
	rt.Data = data.NewStore(rt.Catalog, cfg.Natives, guard)
	rt.Bindings = host.NewPgBindings(deps.DB, rt.Manage)
	rt.Outbox = events.NewOutbox(rt.Catalog)

	size := cfg.PoolSize
	if size <= 0 {
		size = 2
	}
	if rt.Pool, err = pool.New(pool.Config{Size: size, NodePath: cfg.NodeBinary, WorkerScript: cfg.WorkerScript}); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	fail := func(err error) (*Runtime, error) {
		_ = rt.Pool.Close(context.Background())
		return nil, serrors.Wrap(op, err)
	}

	var subjects host.SubjectLoader
	if rt.Subjects != nil {
		subjects = rt.Subjects
	}
	hostCfg := host.Config{
		Pool: rt.Pool, DB: deps.DB, Bindings: rt.Bindings, Catalog: rt.Catalog, Data: rt.Data, Artifacts: rt.Manage,
		Outbox: rt.Outbox, Secrets: cfg.Secrets, Logger: cfg.Logger,
	}
	if subjects != nil {
		rt.Workflows, err = workflow.New(workflow.Config{
			DB: deps.DB, Catalog: rt.Catalog, Bindings: rt.Bindings, Artifacts: rt.Manage, Impl: rt.Manage.Implementations(),
			Subjects: subjects, Concurrency: cfg.Workflow.Concurrency, TaskAdminPermission: manage.PermOperate, Logger: cfg.Logger,
		})
		if err != nil {
			return fail(err)
		}
		hostCfg.Workflow, hostCfg.Starter, hostCfg.Signals = rt.Workflows, rt.Workflows, rt.Workflows
		rt.Manage.SetWorkflows(rt.Workflows)
	}
	if deps.Dispatch != nil {
		rt.Notifier = notify.NewNotifier(deps.Dispatch, rt.Catalog, rt.Manage, nil)
		hostCfg.Notifier = rt.Notifier
	}
	if rt.Host, err = host.New(hostCfg); err != nil {
		return fail(err)
	}
	guard.h = rt.Host
	if rt.Workflows != nil {
		rt.Workflows.Attach(rt.Host)
	}
	if subjects != nil {
		dcfg := cfg.Events
		dcfg.DB, dcfg.Runner, dcfg.Subscriptions, dcfg.Subjects = deps.DB, rt.Host, rt.Manage.SubscriptionSource(), subjects
		if dcfg.Logger == nil {
			dcfg.Logger = cfg.Logger
		}
		if rt.Dispatcher, err = events.NewDispatcher(dcfg); err != nil {
			return fail(err)
		}
		scfg := cfg.Schedules
		scfg.DB, scfg.Runner, scfg.Schedules, scfg.Subjects = deps.DB, rt.Host, rt.Manage.ScheduleSource(), subjects
		if scfg.Logger == nil {
			scfg.Logger = cfg.Logger
		}
		if rt.Scheduler, err = events.NewScheduler(scfg); err != nil {
			return fail(err)
		}
	}
	if rt.Notifier != nil && deps.Notices != nil && deps.Renderers != nil {
		var owners notify.SubjectResolver
		if rt.Subjects != nil {
			owners = rt.Subjects
		}
		rt.Notify, err = notify.NewAdapter(notify.Config{
			Catalog: rt.Catalog, Notices: deps.Notices, Renderers: deps.Renderers, Contacts: deps.Contacts,
			Runner: rt.Host, Artifacts: rt.Manage, Manifests: rt.Manage, Owners: rt.Manage, Subjects: owners,
			Published: notify.PgPublished{DB: deps.DB}, Logger: cfg.Logger,
		})
		if err != nil {
			return fail(err)
		}
		rt.Notifier.SetEnsurer(rt.Notify)
	}
	if cfg.bind != nil {
		cfg.bind.p.Store(rt)
	}
	return rt, nil
}

// Start runs the background parts: the catalog's NOTIFY listener, the
// notification adapter that follows activations, the workflow engine, the
// event dispatcher, the scheduler and the extensions. It returns after they
// are started; Close stops them.
func (rt *Runtime) Start(ctx context.Context) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	bg, cancel := context.WithCancel(context.WithoutCancel(ctx))
	var wg sync.WaitGroup
	run := func(f func(context.Context)) {
		wg.Add(1)
		go func() { defer wg.Done(); f(bg) }()
	}
	run(func(c context.Context) { _ = rt.Catalog.Listen(c) })
	if rt.Notify != nil {
		run(rt.Notify.Run)
	}
	rt.stops = append(rt.stops, func(context.Context) error { cancel(); wg.Wait(); return nil })
	if !rt.Config.DisableBackground {
		if rt.Workflows != nil {
			rt.Workflows.Serve(bg)
		}
		if rt.Dispatcher != nil {
			rt.Dispatcher.Start(bg)
			rt.stops = append(rt.stops, func(context.Context) error { rt.Dispatcher.Stop(); return nil })
		}
		if rt.Scheduler != nil {
			rt.Scheduler.Start(bg)
			rt.stops = append(rt.stops, func(context.Context) error { rt.Scheduler.Stop(); return nil })
		}
	}
	for _, ext := range rt.Config.Extensions {
		stop, err := ext.Attach(bg, rt)
		if err != nil {
			return serrors.Wrap("extruntime.runtime.Start", errors.Join(errors.New("extension "+ext.Name()), err))
		}
		if stop != nil {
			rt.stops = append(rt.stops, stop)
		}
	}
	return nil
}

// Close stops the background parts in reverse order, then the worker pool.
func (rt *Runtime) Close(ctx context.Context) error {
	rt.mu.Lock()
	stops := rt.stops
	rt.stops = nil
	rt.mu.Unlock()
	var errs []error
	for i := len(stops) - 1; i >= 0; i-- {
		errs = append(errs, stops[i](ctx))
	}
	errs = append(errs, rt.Pool.Close(ctx))
	return errors.Join(errs...)
}

// SubjectFromContext builds the acting subject of a session request: the
// user's id and effective permission names, tenant and locale from the
// context.
func SubjectFromContext(ctx context.Context) (spi.Subject, error) {
	u, err := composables.UseUser(ctx)
	if err != nil {
		return spi.Subject{}, serrors.New(serrors.Unauthenticated, "authentication is required").WithCause(err)
	}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return spi.Subject{}, serrors.New(serrors.Unauthenticated, "authentication is required").WithCause(err)
	}
	return spi.Subject{
		TenantID: tenant, UserID: int64(u.ID()), Kind: spi.SubjectInitiator,
		Permissions: permissionNames(user.EffectivePermissions(u)), Locale: string(u.UILanguage()), Credential: "session",
	}, nil
}

func permissionNames(ps []permission.Permission) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Name())
	}
	return out
}

// UserSubjects loads the current rights of a user (direct, role and group
// grants). It serves triggers (events, schedules, workflow runs) as a
// host.SubjectLoader and contact resolvers as a notify.SubjectResolver.
type UserSubjects struct {
	Users    user.Repository
	Universe []permission.Permission
	// Pool is put into the context of repository reads: triggers run in
	// background goroutines whose context carries no database.
	Pool *pgxpool.Pool
}

var (
	_ notify.SubjectResolver = (*UserSubjects)(nil)
	_ host.SubjectLoader     = (*UserSubjects)(nil)
)

func (s *UserSubjects) load(ctx context.Context, tenantID uuid.UUID, userID int64, kind spi.SubjectKind) (spi.Subject, error) {
	if userID <= 0 {
		return spi.Subject{}, &host.SubjectBlockedError{TenantID: tenantID, UserID: userID, Reason: "no user"}
	}
	ctx = composables.WithTenantID(ctx, tenantID)
	if s.Pool != nil && ctx.Value(constants.TxKey) == nil {
		ctx = composables.WithPool(ctx, s.Pool)
	}
	u, err := s.Users.GetByID(ctx, uint(userID))
	if err != nil {
		if serrors.CodeOf(err) == serrors.NotFound {
			return spi.Subject{}, &host.SubjectBlockedError{TenantID: tenantID, UserID: userID, Reason: "user not found"}
		}
		return spi.Subject{}, err
	}
	if u.IsBlocked() {
		return spi.Subject{}, &host.SubjectBlockedError{TenantID: tenantID, UserID: userID, Reason: "user is blocked"}
	}
	return spi.Subject{
		TenantID: tenantID, UserID: userID, Kind: kind,
		Permissions: permissionNames(user.EffectivePermissions(u)), Locale: string(u.UILanguage()),
	}, nil
}

// LoadSubject implements host.SubjectLoader.
func (s *UserSubjects) LoadSubject(ctx context.Context, tenantID uuid.UUID, userID int64, kind spi.SubjectKind) (spi.Subject, error) {
	return s.load(ctx, tenantID, userID, kind)
}

// ResolveSubject implements notify.SubjectResolver.
func (s *UserSubjects) ResolveSubject(ctx context.Context, tenantID uuid.UUID, userID int64) (spi.Subject, error) {
	return s.load(ctx, tenantID, userID, spi.SubjectOwner)
}
