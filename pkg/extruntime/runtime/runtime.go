// Package runtime is the composition root of the extension runtime: it wires
// the Node pool, the invocation host, the data store, the active catalog, the
// management service and the notification adapter, accepts the workflow,
// events and UI slices through interfaces, and ships the iota-sdk component
// that enables all of it.
package runtime

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/artifact"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/notify"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
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
	// events, pages, slots) published in the catalog and the kit.
	NativeResources []NativeResource
	// PermissionUniverse lists the permissions a trigger owner's rights are
	// computed over. Default: the core permissions.
	PermissionUniverse []permission.Permission

	// Optional slices, plugged through interfaces.
	Outbox     spi.Outbox
	Starter    spi.Starter
	Workflow   host.WorkflowHandler
	Secrets    host.SecretResolver
	WriteGuard spi.WriteGuard
	// Extensions attach after the runtime is built (UI route host, event
	// dispatcher, workflow scheduler, MCP): each receives the Runtime and
	// returns a stop function.
	Extensions []Extension

	Logger *slog.Logger
}

// NativeResource is one native contract registered by Go code.
type NativeResource = nativeResource

// Deps are the pieces of the host application the runtime builds on.
type Deps struct {
	DB *pgxpool.Pool
	// Notification wiring; when Dispatch is nil the host has no notifier and
	// tx.notify fails explicitly.
	Dispatch  notify.Dispatcher
	Notices   *notifications.Catalog
	Renderers *notifications.ContentRenderers
	Contacts  *notifications.ContactResolvers
	// Users loads trigger owners for contact resolvers.
	Users user.Repository
}

// Extension is a slice that needs the finished runtime (workflow scheduler,
// event dispatcher, UI host, MCP tools).
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
	// Notify is nil without notification dependencies.
	Notify   *notify.Adapter
	Notifier *notify.Notifier
	Subjects notify.SubjectResolver

	db  *pgxpool.Pool
	log *slog.Logger

	mu    sync.Mutex
	stops []func(context.Context) error
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

	var err error
	if rt.Artifacts, err = artifact.NewStore(deps.DB, cfg.CacheDir); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rt.Natives = catalog.NewNativeSet(cfg.Natives)
	for _, r := range cfg.NativeResources {
		if err := rt.Natives.Register(r.Resource()); err != nil {
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
	rt.Data = data.NewStore(rt.Catalog, cfg.Natives, cfg.WriteGuard)
	rt.Bindings = host.NewPgBindings(deps.DB, rt.Manage)

	size := cfg.PoolSize
	if size <= 0 {
		size = 2
	}
	if rt.Pool, err = pool.New(pool.Config{Size: size, NodePath: cfg.NodeBinary, WorkerScript: cfg.WorkerScript}); err != nil {
		return nil, serrors.Wrap(op, err)
	}

	hostCfg := host.Config{
		Pool: rt.Pool, DB: deps.DB, Bindings: rt.Bindings, Catalog: rt.Catalog, Data: rt.Data,
		Outbox: cfg.Outbox, Starter: cfg.Starter, Workflow: cfg.Workflow, Secrets: cfg.Secrets, Logger: cfg.Logger,
	}
	if deps.Dispatch != nil {
		rt.Notifier = notify.NewNotifier(deps.Dispatch, rt.Catalog, rt.Manage, nil)
		hostCfg.Notifier = rt.Notifier
	}
	if rt.Host, err = host.New(hostCfg); err != nil {
		_ = rt.Pool.Close(context.Background())
		return nil, serrors.Wrap(op, err)
	}
	if deps.Users != nil {
		rt.Subjects = UserSubjects{Users: deps.Users, Universe: cfg.PermissionUniverse}
	}
	if rt.Notifier != nil && deps.Notices != nil && deps.Renderers != nil {
		rt.Notify, err = notify.NewAdapter(notify.Config{
			Catalog: rt.Catalog, Notices: deps.Notices, Renderers: deps.Renderers, Contacts: deps.Contacts,
			Runner: rt.Host, Artifacts: rt.Manage, Manifests: rt.Manage, Owners: rt.Manage, Subjects: rt.Subjects,
			Published: notify.PgPublished{DB: deps.DB}, Logger: cfg.Logger,
		})
		if err != nil {
			_ = rt.Pool.Close(context.Background())
			return nil, serrors.Wrap(op, err)
		}
		rt.Notifier.SetEnsurer(rt.Notify)
	}
	return rt, nil
}

// Start runs the background parts: the catalog's NOTIFY listener, the
// notification adapter that follows activations, and the extensions. It
// returns after they are started; Close stops them.
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

// Close stops the extensions in reverse order, then the worker pool.
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
// grants) for owner-run work.
type UserSubjects struct {
	Users    user.Repository
	Universe []permission.Permission
}

var _ notify.SubjectResolver = UserSubjects{}

// ResolveSubject implements notify.SubjectResolver.
func (s UserSubjects) ResolveSubject(ctx context.Context, tenantID uuid.UUID, userID int64) (spi.Subject, error) {
	const op = serrors.Op("extruntime.runtime.UserSubjects.ResolveSubject")
	if userID <= 0 {
		return spi.Subject{}, serrors.New(serrors.Invalid, "invalid user").WithOp(op)
	}
	ctx = composables.WithTenantID(ctx, tenantID)
	u, err := s.Users.GetByID(ctx, uint(userID))
	if err != nil {
		return spi.Subject{}, serrors.Wrap(op, err)
	}
	var names []string
	for _, p := range user.EffectivePermissions(u) {
		names = append(names, p.Name())
	}
	return spi.Subject{TenantID: tenantID, UserID: userID, Kind: spi.SubjectOwner, Permissions: names, Locale: string(u.UILanguage())}, nil
}
