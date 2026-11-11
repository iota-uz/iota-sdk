package e2e_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/internal/assets"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/notifications/notificationstest"
	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	crmPerms "github.com/iota-uz/iota-sdk/modules/crm/permissions"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/headers"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/build/buildtest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/ui"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/sdkidentity"
)

// Permissions that package fixtures declare; the test application grants
// them to users by name, as an administrator would.
const (
	permNotesRead    = "e2e.notes.read"
	permNotesWrite   = "e2e.notes.write"
	permViewIntern   = "e2e.clientextra.viewInternal"
	permClientManage = "e2e.clientextra.manage"
	permLedgerRead   = "e2e.ledger.read"
	permLedgerWrite  = "e2e.ledger.write"
)

// actor is a signed-in user of the test application: a real user row with
// real permissions and a real session token.
type actor struct {
	id    uint
	email string
	token string
}

// env is one running application: the SDK's real modules, the extension
// runtime component on top of a real Node pool and a real Node build worker,
// fake providers for SMS, email and push, and an HTTP server serving the
// application's actual controllers behind the actual session middleware.
type env struct {
	t      *testing.T
	scope  *itf.Scope
	ctx    context.Context
	pool   *pgxpool.Pool
	tenant uuid.UUID

	cfg  runtime.Config
	rt   *runtime.Runtime
	deps runtime.Deps

	sms, email, push *notificationstest.Adapter
	users            *services.UserService
	auth             *services.AuthService
	srv              *httptest.Server
	admin            *actor
	stop             composition.StopFn // set by restart
}

type envOption func(*envConfig)

type envConfig struct {
	events    events.Config
	schedules events.SchedulerConfig
	poolSize  int
	uiRoutes  ui.NativeRoutes
	resolvers map[string]notifications.ContactResolver
}

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	cfg := envConfig{
		poolSize: 2,
		events:   events.Config{PollInterval: 100 * time.Millisecond, BackoffBase: 100 * time.Millisecond, BlockedRecheck: 500 * time.Millisecond},
		uiRoutes: ui.NativeRoutes{"crm.clients": "/crm/clients"},
	}
	for _, o := range opts {
		o(&cfg)
	}
	worker := hostWorker(t)
	builder, err := build.NewNodeBuilder(buildtest.Config(t))
	require.NoError(t, err)

	e := &env{t: t, sms: notificationstest.NewSMS(), email: notificationstest.NewEmail(), push: notificationstest.NewPush()}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	e.cfg = runtime.Config{
		WorkerScript: worker, CacheDir: t.TempDir(), PoolSize: cfg.poolSize,
		Builder: builder, Kits: builder,
		Git:    manage.GitConfig{AllowedSchemes: []string{"file"}},
		Events: cfg.events, Schedules: cfg.schedules,
		UI: &runtime.UIConfig{
			Host: clienthost.Manifest{
				Package: "@iota-uz/sdk", PackageVersion: "0.5.6", SDKCommit: "0123456789abcdef0123456789abcdef01234567",
				SDKReleaseVersion: sdkidentity.ReleaseVersion, ProtocolVersion: clienthost.ProtocolVersion, Entry: "/assets/runtime-host.js",
			},
			NativeRoutes: cfg.uiRoutes, SourceTTL: 10 * time.Millisecond,
		},
		// A webhook has no session: the application resolves its tenant (by host name in production).
		Endpoints: runtime.EndpointConfig{PublicTenant: func(*http.Request) (uuid.UUID, bool) { return e.tenant, true }},
		Logger:    logger,
	}
	comp, err := runtime.NewComponent(e.cfg)
	require.NoError(t, err)

	components := modules.Components()
	for i, c := range components {
		if c.Descriptor().Name == "core" {
			components[i] = core.NewComponent(&core.ModuleOptions{
				DisableNotificationWorker: true, PermissionSchema: defaults.PermissionSchema(),
				NotificationChannels:         []notifications.ChannelAdapter{e.email, e.sms, e.push},
				NotificationContactResolvers: cfg.resolvers,
			})
		}
	}
	components = append(components, comp)

	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:       "extruntime_e2e",
		Components: components,
		Database:   itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase, Pool: itf.PoolConfig{MaxConns: 24}},
		Isolation:  itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	e.scope = h.Scope(t)
	e.pool, e.tenant = e.scope.Pool, e.scope.Tenant.ID
	ctx := composables.WithParams(e.scope.Ctx, itf.DefaultParams())
	ctx = composables.WithSession(ctx, itf.MockSession())
	e.ctx = itf.TestExecution(ctx)

	rt := resolve[*runtime.Runtime](t, e.scope.Container)
	e.rt = rt
	e.users = resolve[*services.UserService](t, e.scope.Container)
	e.auth = resolve[*services.AuthService](t, e.scope.Container)
	e.deps = runtime.Deps{
		DB:        e.pool,
		Dispatch:  resolve[*services.NotificationDispatchService](t, e.scope.Container),
		Notices:   resolve[*notifications.Catalog](t, e.scope.Container),
		Renderers: resolve[*notifications.ContentRenderers](t, e.scope.Container),
		Contacts:  resolve[*notifications.ContactResolvers](t, e.scope.Container),
		Policies:  resolve[*notifications.DeliveryPolicies](t, e.scope.Container),
		Users:     resolve[user.Repository](t, e.scope.Container),
		// The Go defaults of native contracts resolve their services here, as
		// the component wires them.
		NativeDeps: e.scope.Container,
	}

	// The harness runs the container's start hooks, so the component's own
	// hook has started the runtime exactly as the application server does.
	t.Cleanup(func() {
		if e.stop != nil {
			_ = e.stop(context.Background())
		}
	})

	e.admin = e.newActor("admin", append(extensionPerms(), "User.Create", "User.Read", "Client.Read", "Client.Create", "Client.Update",
		permNotesRead, permNotesWrite, permViewIntern, permClientManage, permLedgerRead, permLedgerWrite)...)
	e.serve()
	return e
}

func resolve[T any](tb testing.TB, c *composition.Container) T {
	tb.Helper()
	v, err := composition.Resolve[T](c)
	require.NoError(tb, err)
	return v
}

func extensionPerms() []string {
	return []string{
		corePerms.ExtensionsRead.Name(), corePerms.ExtensionsAuthor.Name(), corePerms.ExtensionsReview.Name(),
		corePerms.ExtensionsPublish.Name(), corePerms.ExtensionsOperate.Name(),
	}
}

// restart replaces the running runtime with a new one on the same database,
// as a process restart does: new Node pool, new engine, new dispatcher.
func (e *env) restart() {
	e.t.Helper()
	ctx := context.Background()
	if e.stop != nil {
		require.NoError(e.t, e.stop(ctx))
		e.stop = nil
	}
	rt, err := runtime.New(e.rt.Config, e.deps)
	require.NoError(e.t, err)
	require.NoError(e.t, rt.Start(ctx))
	e.rt = rt
	e.stop = rt.Close
}

// permission resolves a permission by name, creating it as an administrator
// would for a permission a package declared.
func (e *env) permission(name string) permission.Permission {
	e.t.Helper()
	for _, p := range append(append([]permission.Permission{}, corePerms.Permissions...), crmPerms.Permissions...) {
		if p.Name() == name {
			return p
		}
	}
	return permission.New(permission.WithName(name), permission.WithResource("extension"), permission.WithAction(permission.ActionRead), permission.WithModifier(permission.ModifierAll))
}

// newActor creates a user holding exactly the named permissions and a
// signed-in session for it.
func (e *env) newActor(name string, perms ...string) *actor {
	e.t.Helper()
	ps := make([]permission.Permission, 0, len(perms))
	for _, p := range perms {
		ps = append(ps, e.permission(p))
	}
	permRepo := persistence.NewPermissionRepository()
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())
	addr := fmt.Sprintf("%s-%s@example.com", name, uuid.NewString()[:8])
	email, err := internet.NewEmail(addr)
	require.NoError(e.t, err)
	var created user.User
	require.NoError(e.t, composables.InTx(e.ctx, func(ctx context.Context) error {
		for _, p := range ps {
			if err := permRepo.Save(ctx, p); err != nil {
				return err
			}
		}
		var err error
		created, err = userRepo.Create(ctx, user.New(name, "Tester", email, user.UILanguageEN,
			user.WithTenantID(e.tenant), user.WithPermissions(ps)))
		return err
	}))
	sess, err := e.auth.CreateSession(e.ctx, created)
	require.NoError(e.t, err)
	return &actor{id: created.ID(), email: addr, token: sess.Token()}
}

// userContext is a context acting as the actor, for calls to core services
// that take a Go context (user creation).
func (e *env) userContext(a *actor) context.Context {
	e.t.Helper()
	u, err := e.users.GetByID(e.ctx, a.id)
	require.NoError(e.t, err)
	return composables.WithUser(e.ctx, u)
}

// serve mounts the application's controllers behind the production middleware
// stack (request params, localizer, container, pool) and the real session
// middleware that every authenticated controller applies.
func (e *env) serve() {
	e.t.Helper()
	app, container := e.scope.App, e.scope.Container
	headersCfg, err := composition.Resolve[*headers.Config](container)
	require.NoError(e.t, err)
	reqLog := logrus.New()
	reqLog.SetLevel(logrus.WarnLevel)
	container.AppendMiddleware(
		middleware.WithLogger(reqLog, middleware.DefaultLoggerOptions(), headersCfg),
		middleware.Provide(constants.AppKey, app),
		middleware.Provide(constants.ContainerKey, container),
		middleware.Provide(constants.HeadKey, layouts.DefaultHead()),
		middleware.Provide(constants.LogoKey, assets.DefaultLogo()),
		middleware.Provide(constants.PoolKey, e.pool),
		middleware.Provide(constants.TenantIDKey, e.tenant),
		middleware.ProvideLocalizer(app.Bundle(), app.GetSupportedLanguages()),
		middleware.RequestParams(headersCfg),
	)
	r := mux.NewRouter()
	mws := app.Middleware()
	r.Use(mws...)
	for _, c := range app.Controllers() {
		e.mountController(r, c)
	}
	notFound := http.NotFoundHandler()
	for i := len(mws) - 1; i >= 0; i-- {
		notFound = mws[i](notFound)
	}
	r.NotFoundHandler = notFound
	e.srv = httptest.NewServer(r)
	e.t.Cleanup(e.srv.Close)
}

// mountController registers a controller; one whose registration needs build
// products this environment does not have (the Lens Solid dist of the
// dashboard) is skipped, the rest of the application stays real.
func (e *env) mountController(r *mux.Router, c application.Controller) {
	defer func() {
		if rec := recover(); rec != nil {
			e.t.Logf("controller %s not mounted: %v", c.Descriptor().ID, rec)
		}
	}()
	c.Register(r)
}

// ---- assertions on the database (reads only) ----

func (e *env) scalarInt(sql string, args ...any) int {
	e.t.Helper()
	var n int
	require.NoError(e.t, e.pool.QueryRow(context.Background(), sql, args...).Scan(&n))
	return n
}

func (e *env) scalarString(sql string, args ...any) string {
	e.t.Helper()
	var s *string
	require.NoError(e.t, e.pool.QueryRow(context.Background(), sql, args...).Scan(&s))
	if s == nil {
		return ""
	}
	return *s
}

func (e *env) eventually(what string, timeout time.Duration, cond func() bool) {
	e.t.Helper()
	require.Eventually(e.t, cond, timeout, 50*time.Millisecond, what)
}

// pumpNotifications runs the notification worker's passes once the work is
// there: expansion of dispatches into deliveries, then provider sends. The
// application's own worker is disabled in tests so the fakes see every send
// on the test's schedule.
func (e *env) pumpNotifications() {
	e.t.Helper()
	dispatch := resolve[*services.NotificationDispatchService](e.t, e.scope.Container)
	channels := resolve[*services.NotificationChannelService](e.t, e.scope.Container)
	for range 10 {
		worked, err := dispatch.Process(e.ctx)
		require.NoError(e.t, err)
		if !worked {
			break
		}
	}
	_, err := channels.Process(e.ctx)
	require.NoError(e.t, err)
}

// untilDelivered pumps the notification worker until cond holds.
func (e *env) untilDelivered(what string, timeout time.Duration, cond func() bool) {
	e.t.Helper()
	e.eventually(what, timeout, func() bool {
		e.pumpNotifications()
		return cond()
	})
}
