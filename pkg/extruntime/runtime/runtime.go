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
	"path/filepath"
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
	"github.com/iota-uz/iota-sdk/pkg/extruntime/configurator"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/notify"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/originguard"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Config is the runtime configuration. It is what an application sets when
// it enables the runtime component.
type Config struct {
	// NodeBinary is the node executable. Default "node".
	NodeBinary string
	// WorkerScript is the path of the host worker. Default: the worker of
	// the installed @iota-uz/sdk distribution (Dist).
	WorkerScript string
	// Dist locates the installed @iota-uz/sdk distribution the workers are
	// taken from when WorkerScript or Builder is not set; its version must
	// be the Go SDK's (ResolveDist).
	Dist DistConfig
	// PoolSize is the number of worker processes. Default 2.
	PoolSize int
	// CacheDir is where artifacts are materialized for the workers and the
	// UI asset route. Required.
	CacheDir string
	// Git bounds repository imports.
	Git manage.GitConfig

	// Builder is the build worker adapter (Check/Build/Test). Default: a
	// build.NodeBuilder over the build worker of the distribution (Dist),
	// working under CacheDir/build.
	Builder build.Builder
	// Kits renders the developer kit. Default: the default Builder (a
	// NodeBuilder renders kits); without it, builds cannot run.
	Kits build.KitGenerator

	// Natives is every native (Go-owned) resource of the application:
	// entities, lists, events, Go-implemented operations/commands/queries,
	// routes and declarations such as slots. It is composed once from the
	// registrations of the application's modules (native.Compose(
	// append(modules.Natives(), app.Natives)...)) and feeds the catalog, the
	// developer kit, the data store, the invocation host and the UI slice.
	// Required.
	Natives *native.Set
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

	// Configurator mounts the SDK's extension configurator page (Monaco
	// editor, versions, workflows) next to the management API. Its Host
	// defaults to UI.Host; Service is the runtime's. Nil leaves the page out.
	Configurator *configurator.Config

	// TrustedOrigins are the origins (scheme://host[:port]) that may make
	// state-changing requests to the cookie-authenticated surfaces (/_ext/rpc,
	// /_ext/manage, non-public endpoints) from another origin than the
	// application's, for example a UI served from a separate host. Default:
	// none; a request from another site is refused.
	TrustedOrigins []string

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
	// Policies composes delivery policies; the notify adapter installs the
	// policies of active packages on it.
	Policies *notifications.DeliveryPolicies
	// Users loads the current rights of owners and initiators.
	Users user.Repository
	// NativeDeps resolves the dependencies of the Go defaults of native
	// contracts (the composition container). Without it a native contract
	// whose default needs a dependency fails when invoked.
	NativeDeps native.Deps
}

// Extension is a slice that needs the finished runtime.
type Extension interface {
	Name() string
	// Attach starts the slice. The returned stop function runs on Close.
	Attach(ctx context.Context, rt *Runtime) (stop func(context.Context) error, err error)
}

// Runtime holds the wired parts. Fields are read-only after New.
type Runtime struct {
	Config   Config
	Pool     *pool.Pool
	Host     *host.Host
	Data     *data.Store
	Bindings *host.PgBindings
	Catalog  *catalog.Catalog
	// Natives is Config.Natives.
	Natives *native.Set
	// NativeImpls are the Go defaults of the native contracts, bound to the
	// application's dependencies.
	NativeImpls *native.Implementations
	Artifacts   *artifact.Store
	Manage      *manage.Service
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
	// Reference is the live API reference over the catalog (Help Center,
	// configurator, MCP and the OpenAPI document render it).
	Reference *reference.Builder

	db      *pgxpool.Pool
	log     *slog.Logger
	origins *originguard.Guard

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

// resolveWorkers fills WorkerScript, Builder and Kits from the installed
// @iota-uz/sdk distribution when they are not set.
func (cfg *Config) resolveWorkers() error {
	if cfg.WorkerScript != "" && cfg.Builder != nil {
		return nil
	}
	dist, err := ResolveDist(cfg.Dist)
	if err != nil {
		return err
	}
	if cfg.WorkerScript == "" {
		cfg.WorkerScript = dist.HostWorker
	}
	if cfg.Builder == nil {
		b, err := build.NewNodeBuilder(build.Config{Node: cfg.NodeBinary, WorkerScript: dist.BuildWorker, WorkDir: filepath.Join(cfg.CacheDir, "build")})
		if err != nil {
			return err
		}
		cfg.Builder = b
		if cfg.Kits == nil {
			cfg.Kits = b
		}
	}
	return nil
}

// New wires the runtime. It starts the Node pool but no background loops;
// call Start.
func New(cfg Config, deps Deps) (*Runtime, error) {
	const op = serrors.Op("extruntime.runtime.New")
	switch {
	case deps.DB == nil:
		return nil, serrors.New(serrors.Invalid, "extruntime: database is required").WithOp(op)
	case cfg.CacheDir == "":
		return nil, serrors.New(serrors.Invalid, "extruntime: CacheDir is required").WithOp(op)
	case cfg.Natives == nil:
		return nil, serrors.New(serrors.Invalid, "extruntime: Natives is required (compose the module registrations with native.Compose(modules.Natives()...))").WithOp(op)
	}
	if err := cfg.resolveWorkers(); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if len(cfg.PermissionUniverse) == 0 {
		cfg.PermissionUniverse = corePerms.Permissions
	}
	origins, err := originguard.New(cfg.TrustedOrigins...)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rt := &Runtime{Config: cfg, db: deps.DB, log: cfg.Logger, origins: origins}
	if deps.Users != nil {
		rt.Subjects = &UserSubjects{Users: deps.Users, Universe: cfg.PermissionUniverse, Pool: deps.DB}
	}

	if rt.Artifacts, err = artifact.NewStore(deps.DB, cfg.CacheDir); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rt.Natives = cfg.Natives
	rt.NativeImpls = native.Bind(cfg.Natives, deps.NativeDeps)
	rt.Catalog = catalog.New(deps.DB, catalog.Natives(cfg.Natives))
	rt.Reference = reference.New(rt.Catalog)
	entities := cfg.Natives.EntityRegistry()
	mcfg := manage.Config{
		DB: deps.DB, Builder: cfg.Builder, Kits: cfg.Kits, Artifacts: rt.Artifacts, Catalog: rt.Catalog,
		Natives: entities, Git: cfg.Git, Logger: cfg.Logger,
	}
	if rt.Subjects != nil {
		mcfg.Subjects = rt.Subjects
	}
	if rt.Manage, err = manage.New(mcfg); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	guard := &guardRef{}
	rt.Data = data.NewStore(rt.Catalog, entities, guard)
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
		Outbox: rt.Outbox, Secrets: cfg.Secrets, Natives: rt.NativeImpls, Logger: cfg.Logger,
		NativeContext: HistoryExecution(deps.Users),
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
			Catalog: rt.Catalog, Notices: deps.Notices, Renderers: deps.Renderers, Contacts: deps.Contacts, Policies: deps.Policies,
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
