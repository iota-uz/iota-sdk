package ui

import (
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/error_pages"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/listext"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shellext"
)

// ControllerID is the descriptor ID of the runtime page host.
const ControllerID = "extruntime.ui"

// PageMountID is the id prefix of the mount element of a runtime page; it is
// the canonical client host mount, shared with every other client route.
const PageMountID = "iota-client-route"

// Controller is the single controller that serves every activated page, the
// immutable asset route and the browser RPC. It is registered once at
// startup; what it serves changes with the activated snapshot.
type Controller struct {
	*Runtime
}

var _ application.Controller = (*Controller)(nil)

// NewController registers nothing yet; add it to the application.
func NewController(rt *Runtime) *Controller { return &Controller{Runtime: rt} }

// Descriptor declares the static routes (assets, RPC). The dynamic page route
// is not declared: it matches only paths that an activated page or binding
// serves right now, and it is ordered before every other controller so a
// screen binding wins over the native route it replaces.
func (c *Controller) Descriptor() application.ControllerDescriptor {
	return application.Descriptor(ControllerID, math.MinInt32,
		application.Prefix(AssetsPrefix, application.Authenticated()),
		application.Post(RPCPath, application.Authenticated()),
	)
}

func (c *Controller) authenticated(r *http.Request) bool {
	u, err := composables.UseUser(r.Context())
	return err == nil && u != nil
}

// Register mounts the asset route, the RPC and the dynamic page route.
func (c *Controller) Register(r *mux.Router) {
	api := []mux.MiddlewareFunc{middleware.Authorize(), middleware.ProvideUser()}

	assets := r.PathPrefix(AssetsPrefix).Methods(http.MethodGet, http.MethodHead).Subrouter()
	assets.Use(api...)
	assets.PathPrefix("/").HandlerFunc(c.assetHandler)

	rpc := r.Path(RPCPath).Methods(http.MethodPost).Subrouter()
	rpc.Use(api...)
	rpc.Path("").HandlerFunc(c.rpcHandler)

	pages := r.MatcherFunc(c.matchPage).Methods(http.MethodGet).Subrouter()
	pages.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
		middleware.WithPageContext(),
		middleware.NavItems(),
	)
	pages.PathPrefix("/").HandlerFunc(c.servePage)
}

// matchPage reports whether an activated page or screen binding serves the
// request path right now. Nothing activated, nothing matched: the router goes
// on to the native routes as if this controller did not exist.
func (c *Controller) matchPage(r *http.Request, _ *mux.RouteMatch) bool {
	if strings.HasPrefix(r.URL.Path, AssetsPrefix) || r.URL.Path == RPCPath {
		return false
	}
	ix, err := c.Index(r.Context())
	if err != nil {
		c.cfg.Logger.WithError(err).Error("extruntime ui: read snapshot")
		return false
	}
	route, ok := ix.Resolve(r.URL.Path)
	if !ok || !paramsValid(route.Page.Params, route.Params) {
		return false
	}
	// A screen replaces a native screen, not the fragments the native page
	// loads from its own route: an HTMX partial request (not a boosted
	// navigation) still reaches the native handler.
	return route.ReplacedRoute == "" || !isPartialRequest(r)
}

// isPartialRequest reports an HTMX request that swaps a fragment in, as
// opposed to a full navigation or a boosted link.
func isPartialRequest(r *http.Request) bool {
	return htmx.IsHxRequest(r) && !htmx.IsBoosted(r) && !htmx.IsHistoryRestoreRequest(r)
}

// PageData is the initial value of a runtime page route: what the browser
// needs to load the page module and render it with its route parameters.
type PageData struct {
	Kind string  `json:"kind"`
	Page PageRef `json:"page"`
	// Params are the path parameters as strings; the client decodes them
	// with the route's schema.
	Params map[string]string `json:"params"`
	// ParamTypes are the declared types of the route's parameters, when the
	// page implements a typed route.
	ParamTypes *manifest.Type `json:"paramTypes,omitempty"`
	// ReplacedRoute is the route ID whose native screen this page replaces.
	ReplacedRoute string `json:"replacedRoute,omitempty"`
	// Slots are the slot contributions the viewer gets, by slot ID, for the
	// `<Slot>` components of the page.
	Slots map[string][]shellext.Module `json:"slots"`
}

// PageDataKind marks the initial value of a runtime page route.
const PageDataKind = "iota.runtime.page"

// PageRef identifies the module to load.
type PageRef struct {
	ID      string `json:"id"`
	Package string `json:"package"`
	Version string `json:"version"`
	// Module is the immutable asset URL of the page's client chunk.
	Module string `json:"module"`
	// Export is the export of Module holding the page; empty for default.
	Export string `json:"export,omitempty"`
	// Styles are the immutable URLs of the stylesheets of the page's version.
	// The page HTML links them; the browser host also ensures them before
	// the first render.
	Styles []string `json:"styles,omitempty"`
}

func (c *Controller) servePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ix, err := c.Index(ctx)
	if err != nil {
		c.cfg.Logger.WithError(err).Error("extruntime ui: read snapshot")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	route, ok := ix.Resolve(r.URL.Path)
	if !ok {
		// The router chose this handler from the previous snapshot and the
		// source refreshed in between (a binding was reverted, a package
		// deactivated): finish the request on the index that routed it.
		if prev := c.previous(); prev != nil {
			if route, ok = prev.Resolve(r.URL.Path); ok {
				ix = prev
			}
		}
	}
	if !ok || !paramsValid(route.Page.Params, route.Params) {
		http.NotFound(w, r)
		return
	}
	sub, err := listext.SubjectFromContext(ctx)
	if err != nil {
		http.Redirect(w, r, "/login?next="+r.URL.RequestURI(), http.StatusFound)
		return
	}
	// The permission is checked here, whatever the navigation shows.
	// A replacing page also needs what the native screen it replaces needs.
	if !sub.Has(route.Page.Permission) || !sub.Has(route.ReplacedPermission) {
		w.WriteHeader(http.StatusForbidden)
		if err := error_pages.Forbidden().Render(ctx, w); err != nil {
			_, _ = w.Write([]byte("<h1>403 Forbidden</h1>"))
		}
		return
	}
	page := route.Page
	lang := listext.LangFromContext(ctx)
	rc := clienthost.RouteContext{
		BootstrapVersion:  clienthost.BootstrapVersion,
		ProtocolVersion:   clienthost.ProtocolVersion,
		SDKReleaseVersion: c.cfg.Host.SDKReleaseVersion,
		SDKCommit:         c.cfg.Host.SDKCommit,
		Initial: mustJSON(PageData{
			Kind:          PageDataKind,
			Page:          PageRef{ID: page.ID, Package: page.Package, Version: page.Version, Module: page.AssetPath(), Export: page.Export, Styles: page.Styles},
			Params:        nonNil(route.Params),
			ParamTypes:    page.Params,
			ReplacedRoute: route.ReplacedRoute,
			Slots:         c.visibleSlots(ctx, ix, sub),
		}),
		Theme:       "light",
		Route:       clienthost.RouteIdentity{ID: page.ID, Path: r.URL.Path, FeatureID: page.ID},
		Locale:      clienthost.LocaleContext{Language: lang, Messages: map[string]string{}},
		Permissions: sub.Permissions,
		Services:    clienthost.ServiceEndpoints{RPC: RPCPath},
		User:        runtimeUser(ctx),
	}
	title := listext.LabelFor(page.Label, lang, page.ID)
	bootstrap := clienthost.Bootstrap(clienthost.Page{Title: title, MountID: PageMountID, Manifest: c.cfg.Host, Context: rc})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	templ.Handler(pageView(title, page.Styles, bootstrap), templ.WithStreaming()).ServeHTTP(w, r)
}

func nonNil(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

// paramsValid checks the path parameters against the route's declared types.
// A page without a typed route accepts any parameter value.
func paramsValid(t *manifest.Type, params map[string]string) bool {
	if t == nil || t.Kind != manifest.KindObject {
		return true
	}
	for _, p := range t.Properties {
		v, present := params[p.Name]
		if !present {
			if p.Type.Optional {
				continue
			}
			return false
		}
		if !paramValid(p.Type, v) {
			return false
		}
	}
	return true
}

var (
	decimalPattern = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)
	datePattern    = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
)

func paramValid(t manifest.Type, v string) bool {
	switch t.Kind {
	case manifest.KindUUID:
		_, err := uuid.Parse(v)
		return err == nil
	case manifest.KindInt:
		n, err := strconv.ParseInt(v, 10, 64)
		return err == nil && n >= -(1<<53-1) && n <= 1<<53-1
	case manifest.KindInt64:
		_, err := strconv.ParseInt(v, 10, 64)
		return err == nil
	case manifest.KindDecimal:
		return decimalPattern.MatchString(v)
	case manifest.KindDate:
		return datePattern.MatchString(v)
	case manifest.KindBool:
		return v == "true" || v == "false"
	case manifest.KindEnum:
		for _, e := range t.Values {
			if e == v {
				return true
			}
		}
		return false
	case manifest.KindString:
		return t.MaxLength == 0 || len([]rune(v)) <= t.MaxLength
	case manifest.KindDateTime, manifest.KindJSON, manifest.KindRef, manifest.KindObject, manifest.KindArray:
		// Not scalar types a URL parameter can carry.
		return false
	}
	return false
}
