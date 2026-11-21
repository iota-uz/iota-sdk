// Package integrations is the integrations page of the automations module: a Solid screen at
// /settings/integrations where a tenant administrator connects accounts to the connectors the
// application registered, checks them, sees their status and where they are used.
//
// The screen calls the RPC methods declared on its solid.Import below; they are served by the
// controller's JSON-RPC endpoint (solid.RPCHandler) with the tenant and the user of the session.
package integrations

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/error_pages"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/originguard"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/listext"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ControllerID is the descriptor ID of the page.
const ControllerID = "automations.integrations"

// RouteID is the client route identity of the page.
const RouteID = "automations.integrations"

// DefaultPath is where the page is mounted.
const DefaultPath = "/settings/integrations"

// RPC method names of the screen.
const (
	MethodList        = "automations.connectors.list"
	MethodUsage       = "automations.connections.usage"
	MethodCreate      = "automations.connections.create"
	MethodOAuthStart  = "automations.connections.oauth_start"
	MethodUpdate      = "automations.connections.update"
	MethodTest        = "automations.connections.test"
	MethodSetDisabled = "automations.connections.set_disabled"
	MethodDelete      = "automations.connections.delete"
	MethodPlatform    = "automations.platform.set_enabled"
)

// screen declares the Solid module. The call keeps string literals so the iota-solid generator can
// discover it.
var screen = solid.Import[Props]("./web/Integrations.tsx", solid.WithRPC(
	solid.Query[Empty, ListResponse]("automations.connectors.list", solid.Cacheable(1)),
	solid.Query[IDRequest, UsageResponse]("automations.connections.usage"),
	solid.Action[CreateRequest, SaveResponse]("automations.connections.create", solid.Invalidates("automations.connectors.list")),
	solid.Action[OAuthStartRequest, OAuthStartResponse]("automations.connections.oauth_start"),
	solid.Action[UpdateRequest, SaveResponse]("automations.connections.update", solid.Invalidates("automations.connectors.list")),
	solid.Action[IDRequest, TestResponse]("automations.connections.test", solid.Invalidates("automations.connectors.list")),
	solid.Action[SetDisabledRequest, ConnectionResponse]("automations.connections.set_disabled", solid.Invalidates("automations.connectors.list")),
	solid.Action[IDRequest, DeleteResponse]("automations.connections.delete", solid.Invalidates("automations.connectors.list", "automations.connections.usage")),
	solid.Action[PlatformRequest, OKResponse]("automations.platform.set_enabled", solid.Invalidates("automations.connectors.list")),
))

// Config configures the controller.
type Config struct {
	Service *services.IntegrationsService
	// Host is the application's client host bundle manifest. Without it the page is not mounted (the
	// RPC and the OAuth2 callback still are).
	Host *clienthost.Manifest
	// Path is the page path (default /settings/integrations).
	Path string
	// AutomationsPath is where a use links to (default /automations).
	AutomationsPath string
	// Configured reports whether an encryption key is configured.
	Configured bool
	// Origin is the public origin of the application ("https://erp.example.com") the OAuth2 provider
	// redirects back to; empty: the request's scheme and host.
	Origin string
	// Messages are the locale keys the screen reads.
	MessageKeys []string
	// TrustedOrigins are origins besides the request's own the RPC accepts calls from.
	TrustedOrigins []string
	Logger         *slog.Logger
}

// Controller serves the page, its RPC and the OAuth2 callback.
type Controller struct {
	cfg    Config
	client *clienthost.Controller
	rpc    *solid.RPCHandler
	log    *slog.Logger
}

var _ application.Controller = (*Controller)(nil)

// NewController builds the controller.
func NewController(cfg Config) (*Controller, error) {
	const op = serrors.Op("automations.integrations.NewController")
	if cfg.Service == nil {
		return nil, serrors.New(serrors.Invalid, "integrations: Service is required").WithOp(op)
	}
	if cfg.Path == "" {
		cfg.Path = DefaultPath
	}
	if cfg.AutomationsPath == "" {
		cfg.AutomationsPath = "/automations"
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
		c.client, err = clienthost.NewController(ControllerID+".page", *cfg.Host, []clienthost.Route{{
			Spec:    application.Get(cfg.Path, application.ClientRoute(RouteID), application.RequireAll(permissions.ConnectionsManage)),
			Feature: screen,
			Build:   c.build,
		}}, clienthost.WithShell(shell), clienthost.WithSession(c.session), clienthost.WithServices(clienthost.ServiceEndpoints{RPC: c.rpcPath()}))
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
	}
	return c, nil
}

func (c *Controller) rpcPath() string      { return c.cfg.Path + "/rpc" }
func (c *Controller) callbackPath() string { return c.cfg.Path + "/oauth/callback" }

// PageMounted reports whether the page is served (a client host manifest is configured).
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
	d.Routes = append(d.Routes,
		// Each method checks its own permission: Automations.Author lists connections too.
		application.Post(c.rpcPath(), application.Authenticated()),
		application.Get(c.callbackPath(), application.RequireAll(permissions.ConnectionsManage)),
	)
	return d
}

// Register implements application.Controller.
func (c *Controller) Register(r *mux.Router) {
	if c.client != nil {
		page := r.NewRoute().Subrouter()
		page.Use(
			middleware.Authorize(),
			middleware.RedirectNotAuthenticated(),
			middleware.ProvideUser(),
			middleware.ProvideDynamicLogo(),
			middleware.WithPageContext(),
			middleware.NavItems(),
			requireManage,
		)
		c.client.Register(page)
	}
	rpc := r.Path(c.rpcPath()).Methods(http.MethodPost).Subrouter()
	rpc.Use(middleware.Authorize(), middleware.ProvideUser())
	rpc.Path("").Handler(withRequest(c.rpc))

	cb := r.Path(c.callbackPath()).Methods(http.MethodGet).Subrouter()
	cb.Use(middleware.Authorize(), middleware.RedirectNotAuthenticated(), middleware.ProvideUser(), requireManage)
	cb.Path("").HandlerFunc(c.oauthCallback)
}

// requireManage refuses users without Connections.Manage whatever the navigation shows.
func requireManage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sub, err := listext.SubjectFromContext(r.Context())
		if err != nil {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
			return
		}
		if !sub.Has(permissions.ConnectionsManage.Name()) {
			w.WriteHeader(http.StatusForbidden)
			if err := error_pages.Forbidden().Render(r.Context(), w); err != nil {
				_, _ = w.Write([]byte("<h1>403 Forbidden</h1>"))
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (c *Controller) build(ctx context.Context, _ *http.Request) (clienthost.RoutePayload, error) {
	return clienthost.RoutePayload{
		Title:  intl.TWithDefault(ctx, "Automations.Integrations.Title", "Integrations"),
		Screen: screen.Render(Props{BasePath: c.cfg.Path, AutomationsPath: c.cfg.AutomationsPath, Configured: c.cfg.Configured}),
	}, nil
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

// redirectURL is the absolute callback URL the provider returns to.
func (c *Controller) redirectURL(r *http.Request) string {
	origin := strings.TrimRight(c.cfg.Origin, "/")
	if origin == "" {
		scheme := "http"
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		origin = scheme + "://" + r.Host
	}
	return origin + c.callbackPath()
}

// oauthCallback finishes an OAuth2 authorization and sends the browser back to the page:
// ?connected=<id> or ?oauth_error=<code>, with the connector's kind so the page reopens its form.
func (c *Controller) oauthCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sub, err := listext.SubjectFromContext(ctx)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	a := actor(sub)
	q := r.URL.Query()
	back := func(params url.Values) {
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, c.cfg.Path+"?"+params.Encode(), http.StatusFound)
	}
	if providerErr := q.Get("error"); providerErr != "" {
		params := url.Values{"oauth_error": {"denied"}}
		if kind, err := c.cfg.Service.OAuthAbandon(ctx, a, q.Get("state")); err == nil {
			params.Set("kind", string(kind))
		}
		back(params)
		return
	}
	conn, err := c.cfg.Service.OAuthComplete(ctx, a, q.Get("state"), q.Get("code"))
	switch {
	case err == nil:
		back(url.Values{"connected": {conn.ID.String()}, "kind": {string(conn.Kind)}})
	case errors.Is(err, services.ErrStateInvalid):
		back(url.Values{"oauth_error": {"state"}})
	case errors.Is(err, services.ErrOAuthUnavailable):
		back(url.Values{"oauth_error": {"unavailable"}})
	default:
		c.log.ErrorContext(ctx, "automations: OAuth2 callback failed", "error", err)
		back(url.Values{"oauth_error": {"exchange"}})
	}
}

func actor(sub spi.Subject) services.Actor {
	return services.Actor{TenantID: sub.TenantID, UserID: sub.UserID}
}

// caller is the session subject of an RPC call that holds one of perms.
func caller(ctx context.Context, perms ...string) (spi.Subject, error) {
	sub, err := listext.SubjectFromContext(ctx)
	if err != nil {
		return spi.Subject{}, solid.NewRPCError(solid.CodeUnauthenticated, "sign in required", nil)
	}
	for _, p := range perms {
		if sub.Has(p) {
			return sub, nil
		}
	}
	return spi.Subject{}, solid.NewRPCError(solid.CodeForbidden, "permission "+strings.Join(perms, " or ")+" is required", map[string]string{"code": solid.CodeForbidden})
}

func notFound() error {
	return solid.NewRPCError(solid.CodeNotFound, "connection not found", map[string]string{"code": solid.CodeNotFound})
}

func parseID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return uuid.Nil, notFound()
	}
	return id, nil
}

func kindOf(s string) catalog.ConnectorKind { return catalog.ConnectorKind(strings.TrimSpace(s)) }

type requestKey struct{}

// withRequest keeps the request in the context of an RPC method (the OAuth2 redirect URL is built
// from its host).
func withRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestKey{}, r)))
	})
}

func requestFromContext(ctx context.Context) *http.Request {
	r, _ := ctx.Value(requestKey{}).(*http.Request)
	return r
}
