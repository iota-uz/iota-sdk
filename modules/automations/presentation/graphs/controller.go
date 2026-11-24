// Package graphs is the automations section: the list of a tenant's graphs at /automations and the
// graph editor at /automations/{id}, a Solid screen that mounts @iota-uz/sdk/automation-editor (loaded
// as a lazy chunk on the editor page only).
//
// The screen calls the RPC methods declared on its solid.Import below; they are served by the
// controller's JSON-RPC endpoint (solid.RPCHandler) as a thin layer over engine.Service,
// engine.Authoring and engine.Operations. The tenant, the user and the origin of an actor come from the
// session (an interactive one is human), never from the request.
package graphs

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	icons "github.com/iota-uz/icons/phosphor"

	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/error_pages"
	"github.com/iota-uz/iota-sdk/pkg/agentsession"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/originguard"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/listext"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ControllerID is the descriptor ID of the section.
const ControllerID = "automations.graphs"

// Route identities of the screen.
const (
	RouteList   = "automations.list"
	RouteNew    = "automations.new"
	RouteEditor = "automations.editor"
	RouteRun    = "automations.run"
)

// DefaultPath is where the section is mounted.
const DefaultPath = "/automations"

// RPC method names of the screen.
const (
	MethodList           = "automations.list"
	MethodGet            = "automations.get"
	MethodCatalog        = "automations.catalog"
	MethodCreate         = "automations.create"
	MethodSave           = "automations.save"
	MethodSaveLayout     = "automations.save_layout"
	MethodValidate       = "automations.validate"
	MethodEnableReview   = "automations.enable_review"
	MethodEnable         = "automations.enable"
	MethodDisable        = "automations.disable"
	MethodDelete         = "automations.delete"
	MethodLive           = "automations.live"
	MethodRunManual      = "automations.run_manual"
	MethodDryRun         = "automations.dry_run"
	MethodTestNode       = "automations.test_node"
	MethodSearchSubjects = "automations.search_subjects"
	MethodRuns           = "automations.runs"
	MethodRun            = "automations.run"
	MethodRunCancel      = "automations.run_cancel"
	MethodRunResolve     = "automations.run_resolve"
	MethodRunRetry       = "automations.run_retry"
	MethodRunRestart     = "automations.run_restart"
)

// screen declares the Solid module. The call keeps string literals so the iota-solid generator can
// discover it.
var screen = solid.Import[Props]("./web/Automations.tsx", solid.WithRPC(
	solid.Query[ListRequest, ListResponse]("automations.list"),
	solid.Query[GetRequest, GetResponse]("automations.get"),
	solid.Query[CatalogRequest, CatalogResponse]("automations.catalog"),
	solid.Action[CreateRequest, SaveResult]("automations.create", solid.Invalidates("automations.list")),
	solid.Action[SaveRequest, SaveResult]("automations.save", solid.Invalidates("automations.list", "automations.get")),
	solid.Action[SaveLayoutRequest, SaveLayoutResponse]("automations.save_layout"),
	solid.Query[ValidateRequest, ValidateResponse]("automations.validate"),
	solid.Query[ReviewRequest, ReviewResponse]("automations.enable_review"),
	solid.Action[EnableRequest, EnableResponse]("automations.enable", solid.Invalidates("automations.list", "automations.get")),
	solid.Action[IDRequest, DisableResponse]("automations.disable", solid.Invalidates("automations.list", "automations.get")),
	solid.Action[IDRequest, OKResponse]("automations.delete", solid.Invalidates("automations.list")),
	solid.Query[IDRequest, LiveView]("automations.live"),
	solid.Action[RunManualRequest, RunManualResponse]("automations.run_manual", solid.Invalidates("automations.list", "automations.runs")),
	solid.Query[DryRunRequest, DryRunResponse]("automations.dry_run"),
	solid.Action[TestNodeRequest, TestNodeResponse]("automations.test_node"),
	solid.Query[SearchSubjectsRequest, []SubjectOption]("automations.search_subjects"),
	solid.Query[RunsRequest, RunsResponse]("automations.runs"),
	solid.Query[RunRequest, RunData]("automations.run"),
	solid.Action[RunRequest, RunCancelResponse]("automations.run_cancel", solid.Invalidates("automations.runs", "automations.run", "automations.list")),
	solid.Action[RunResolveRequest, OKResponse]("automations.run_resolve", solid.Invalidates("automations.runs", "automations.run")),
	solid.Action[RunRetryRequest, OKResponse]("automations.run_retry", solid.Invalidates("automations.runs", "automations.run")),
	solid.Action[RunRestartRequest, RunRef]("automations.run_restart", solid.Invalidates("automations.runs", "automations.list")),
))

// Engine is what the controller calls: the engine's services (engine.Runtime provides all three).
type Engine struct {
	Service    engine.Service
	Authoring  engine.Authoring
	Operations engine.Operations
}

// UserNames are the display names of a tenant's users.
type UserNames interface {
	Names(ctx context.Context, tenantID uuid.UUID, ids []int64) (map[int64]string, error)
}

// Config configures the controller.
type Config struct {
	Engine Engine
	// Catalog is the catalog registry the editor's catalog is read from (AsAuthor, WithLocale).
	Catalog catalog.Registry
	// Behaviors are the graph behaviors of the catalog (default graph.DefaultBehaviors).
	Behaviors *graph.Behaviors
	Users     UserNames
	// Origin is who acts in a request; default: human for an interactive session, agent for an agent
	// session (agentsession.Audience), a request with an Authorization header or no session.
	Origin func(r *http.Request) engine.Origin
	// Host is the client host bundle manifest the screen is rendered with. Without it the pages are not
	// mounted (the RPC still is).
	Host *clienthost.Manifest
	// Assets serves the files of the module's own bundle under AssetsPath (nil: the Host bundle is the
	// application's).
	Assets http.Handler
	// Path is the list path (default /automations).
	Path string
	// IntegrationsPath is the integrations page (default /settings/integrations).
	IntegrationsPath string
	// MessageKeys are the locale keys the screen reads.
	MessageKeys    []string
	TrustedOrigins []string
	Logger         *slog.Logger
}

// Controller serves the section's pages and their RPC.
type Controller struct {
	cfg    Config
	client *clienthost.Controller
	rpc    *solid.RPCHandler
	log    *slog.Logger
}

var _ application.Controller = (*Controller)(nil)

// NewController builds the controller.
func NewController(cfg Config) (*Controller, error) {
	const op = serrors.Op("automations.graphs.NewController")
	if cfg.Engine.Service == nil || cfg.Engine.Authoring == nil || cfg.Engine.Operations == nil || cfg.Catalog == nil {
		return nil, serrors.New(serrors.Invalid, "graphs: the engine's Service, Authoring, Operations and the Catalog are required").WithOp(op)
	}
	if cfg.Path == "" {
		cfg.Path = DefaultPath
	}
	if cfg.IntegrationsPath == "" {
		cfg.IntegrationsPath = "/settings/integrations"
	}
	if cfg.Behaviors == nil {
		cfg.Behaviors = graph.DefaultBehaviors()
	}
	if cfg.Origin == nil {
		cfg.Origin = SessionOrigin
	}
	if cfg.Users == nil {
		cfg.Users = noNames{}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	c := &Controller{cfg: cfg, log: cfg.Logger}
	guard, err := originguard.New(cfg.TrustedOrigins...)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	c.rpc, err = solid.NewRPCHandler(c.bindings(), solid.WithRPCGuard(guard.Check), solid.WithRPCLogger(cfg.Logger))
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	if cfg.Host != nil {
		id := `{id:[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}}`
		read := application.RequireAll(permissions.AutomationsRead)
		c.client, err = clienthost.NewController(ControllerID+".page", *cfg.Host, []clienthost.Route{
			{Spec: application.Get(cfg.Path, application.ClientRoute(RouteList), read), Feature: screen, Build: c.build(ViewList)},
			{Spec: application.Get(cfg.Path+"/new", application.ClientRoute(RouteNew), application.RequireAll(permissions.AutomationsRead, permissions.AutomationsAuthor)), Feature: screen, Build: c.build(ViewNew)},
			{Spec: application.Get(cfg.Path+"/"+id, application.ClientRoute(RouteEditor), read), Feature: screen, Build: c.build(ViewEditor)},
			{Spec: application.Get(cfg.Path+"/"+id+"/runs/{run:[0-9a-fA-F-]{36}}", application.ClientRoute(RouteRun), read), Feature: screen, Build: c.build(ViewEditor)},
		}, clienthost.WithShell(shell), clienthost.WithSession(c.session), clienthost.WithServices(clienthost.ServiceEndpoints{RPC: c.rpcPath()}))
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
	}
	return c, nil
}

func (c *Controller) rpcPath() string { return c.cfg.Path + "/rpc" }

// PageMounted reports whether the pages are served (a client host manifest is configured).
func (c *Controller) PageMounted() bool { return c.client != nil }

// Descriptor implements application.Controller.
func (c *Controller) Descriptor() application.ControllerDescriptor {
	var d application.ControllerDescriptor
	if c.client != nil {
		d = c.client.Descriptor()
	} else {
		d = application.Descriptor(ControllerID, 0)
	}
	d.ID = ControllerID
	// Each method checks its own permission.
	d.Routes = append(d.Routes, application.Post(c.rpcPath(), application.Authenticated()))
	if c.cfg.Assets != nil {
		d.Routes = append(d.Routes, application.Prefix(AssetsPath, application.Public()))
	}
	if c.client != nil {
		d = d.WithNav(NavNode(c.cfg.Path))
	}
	return d
}

// NavNode is the section's entry in the navigation, shown to users with Automations.Read.
func NavNode(path string) application.NavNode {
	return application.NavNode{
		ID: "automations", TitleKey: "NavigationLinks.Automations", Path: path, Order: 90,
		Icon:       icons.FlowArrow(icons.Props{Size: "24"}),
		Keywords:   []string{"automation", "workflow", "trigger"},
		Visibility: &application.AuthPolicy{Permissions: []permission.Permission{permissions.AutomationsRead}, Logic: application.PermissionLogicAll},
	}
}

// Register implements application.Controller.
func (c *Controller) Register(r *mux.Router) {
	if c.cfg.Assets != nil {
		r.PathPrefix(AssetsPath+"/").Methods(http.MethodGet, http.MethodHead).Handler(c.cfg.Assets)
	}
	rpc := r.Path(c.rpcPath()).Methods(http.MethodPost).Subrouter()
	rpc.Use(middleware.Authorize(), middleware.ProvideUser())
	rpc.Path("").Handler(c.withOrigin(c.rpc))
	if c.client != nil {
		page := r.NewRoute().Subrouter()
		page.Use(
			middleware.Authorize(),
			middleware.RedirectNotAuthenticated(),
			middleware.ProvideUser(),
			middleware.ProvideDynamicLogo(),
			middleware.WithPageContext(),
			middleware.NavItems(),
			c.requirePage,
		)
		c.client.Register(page)
	}
}

// requirePage refuses users without Automations.Read (and the new page without Automations.Author)
// whatever the navigation shows.
func (c *Controller) requirePage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sub, err := listext.SubjectFromContext(r.Context())
		if err != nil {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
			return
		}
		need := []string{permissions.AutomationsRead.Name()}
		if r.URL.Path == c.cfg.Path+"/new" {
			need = append(need, permissions.AutomationsAuthor.Name())
		}
		for _, p := range need {
			if !sub.Has(p) {
				w.WriteHeader(http.StatusForbidden)
				if err := error_pages.Forbidden().Render(r.Context(), w); err != nil {
					_, _ = w.Write([]byte("<h1>403 Forbidden</h1>"))
				}
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (c *Controller) build(view View) func(context.Context, *http.Request) (clienthost.RoutePayload, error) {
	return func(ctx context.Context, r *http.Request) (clienthost.RoutePayload, error) {
		props := Props{BasePath: c.cfg.Path, IntegrationsPath: c.cfg.IntegrationsPath, View: view}
		vars := mux.Vars(r)
		props.ID, props.Run = strings.ToLower(vars["id"]), strings.ToLower(vars["run"])
		agent := c.cfg.Origin(r) != engine.OriginHuman
		if sub, err := listext.SubjectFromContext(ctx); err == nil {
			props.Can = canOf(sub, agent)
		}
		props.Agent = agent
		title := intl.TWithDefault(ctx, "Automations.Title", "Automations")
		return clienthost.RoutePayload{Title: title, Screen: screen.Render(props)}, nil
	}
}

func (c *Controller) session(r *http.Request) (clienthost.SessionContext, error) {
	ctx := r.Context()
	messages := make(map[string]string, len(c.cfg.MessageKeys))
	for _, k := range c.cfg.MessageKeys {
		messages[k] = intl.TWithDefault(ctx, k, k)
	}
	sc := clienthost.SessionContext{Theme: "light", Locale: listext.LangFromContext(ctx), Messages: messages}
	if sub, err := listext.SubjectFromContext(ctx); err == nil {
		sc.Permissions = append([]string(nil), sub.Permissions...)
	}
	if u, err := composables.UseUser(ctx); err == nil {
		sc.User = map[string]any{"id": u.ID(), "name": strings.TrimSpace(u.FirstName() + " " + u.LastName())}
	}
	return sc, nil
}

func shell(_ context.Context, page clienthost.Page, content templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		layout := layouts.Authenticated(layouts.AuthenticatedProps{BaseProps: layouts.BaseProps{Title: page.Title}})
		return layout.Render(templ.WithChildren(ctx, content), w)
	})
}

// SessionOrigin is the default origin of a request: human for an interactive browser session; agent for
// an agent session, a request authenticated by an Authorization header, or one without a session.
func SessionOrigin(r *http.Request) engine.Origin {
	if strings.TrimSpace(r.Header.Get("Authorization")) != "" {
		return engine.OriginAgent
	}
	sess, err := composables.UseSession(r.Context())
	if err != nil || sess == nil || sess.Audience() == agentsession.Audience {
		return engine.OriginAgent
	}
	return engine.OriginHuman
}

type originKey struct{}

// withOrigin keeps the request's origin in the context of an RPC method.
func (c *Controller) withOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), originKey{}, c.cfg.Origin(r))))
	})
}

func originFrom(ctx context.Context) engine.Origin {
	if o, ok := ctx.Value(originKey{}).(engine.Origin); ok {
		return o
	}
	return engine.OriginAgent
}

type noNames struct{}

func (noNames) Names(context.Context, uuid.UUID, []int64) (map[int64]string, error) {
	return map[int64]string{}, nil
}
