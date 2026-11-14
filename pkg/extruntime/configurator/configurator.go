// Package configurator is the SDK-authored extension configurator: an
// authenticated page at /extensions where authors edit package drafts in
// Monaco, check/build/test them, publish, review, activate and bind versions
// and operate workflow instances.
//
// The page is a standard Solid screen of the canonical client host
// (solid.Import + the generated bindings), not a runtime package and not a
// second host. Its data comes from the same management backend as MCP: the
// browser calls the /_ext/manage API (pkg/extruntime/runtime) with the user's
// session, so rights and audit are the service's. The only routes this
// package adds are the page and the two JSON endpoints that give Monaco its
// type information (developer kit snapshot and SDK typings).
package configurator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/listext"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/serrors/serrorhttp"

	icons "github.com/iota-uz/icons/phosphor"

	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/error_pages"
)

// ControllerID is the descriptor ID of the configurator.
const ControllerID = "extruntime.configurator"

// DefaultPath is where the page is mounted.
const DefaultPath = "/extensions"

// RouteID is the client route identity of the page.
const RouteID = "extruntime.configurator"

// ManageAPI is the management API prefix the screen calls. It is the same
// constant as runtime.APIPrefix; duplicated to keep this package free of the
// runtime composition root.
const ManageAPI = "/_ext/manage"

// Props are the initial props of the screen.
type Props struct {
	// BasePath is the page path; the screen keeps its selection in the query.
	BasePath string `json:"basePath"`
	// APIBase is the management API prefix.
	APIBase string `json:"apiBase"`
	// TypesBase serves Monaco's type information: <TypesBase>/kit and
	// <TypesBase>/sdk.
	TypesBase string `json:"typesBase"`
}

// screen declares the Solid module. The call must keep a string literal
// source so the iota-solid generator can discover it.
var screen = solid.Import[Props]("./web/Configurator.tsx")

// Config configures the controller.
type Config struct {
	// Host is the application's client host bundle manifest. Required; it is
	// the same host that serves every other Solid screen.
	Host clienthost.Manifest
	// Service is the management service. Required.
	Service *manage.Service
	// Types is a node_modules-shaped tree of .d.ts files and package.json
	// files (for example os.DirFS of the application's node_modules) that
	// gives Monaco the types of @iota-uz/sdk and solid-js. Nil serves only the
	// developer kit; diagnostics from the server-side check are unaffected.
	Types fs.FS
	// TypePackages are the packages of Types that are served. Default:
	// @iota-uz/sdk, solid-js and csstype.
	TypePackages []string
	// Path is the page path. Default /extensions.
	Path   string
	Logger *slog.Logger
}

// maxTypesBytes bounds the typings served to one editor session.
const maxTypesBytes = 16 << 20

// Controller serves the page and Monaco's type information.
type Controller struct {
	cfg    Config
	client *clienthost.Controller
	log    *slog.Logger

	// sdkOnce computes the typings of the SDK once per process: they come
	// from the installed node_modules, which cannot change under a running
	// process, and walking them on every editor session is expensive.
	sdkMu    sync.Mutex
	sdkBody  []byte
	sdkEtag  string
	sdkReady bool
}

var _ application.Controller = (*Controller)(nil)

// NewController builds the controller.
func NewController(cfg Config) (*Controller, error) {
	const op = serrors.Op("extruntime.configurator.NewController")
	if cfg.Service == nil {
		return nil, serrors.New(serrors.Invalid, "configurator: Service is required").WithOp(op)
	}
	if cfg.Path == "" {
		cfg.Path = DefaultPath
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if len(cfg.TypePackages) == 0 {
		cfg.TypePackages = []string{"@iota-uz/sdk", "solid-js", "csstype"}
	}
	c := &Controller{cfg: cfg, log: cfg.Logger}
	client, err := clienthost.NewController(ControllerID, cfg.Host, []clienthost.Route{{
		Spec:    application.Get(cfg.Path, application.ClientRoute(RouteID), application.RequireAll(corePerms.ExtensionsRead)),
		Feature: screen,
		Build:   c.build,
	}}, clienthost.WithShell(shell), clienthost.WithSession(session))
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	c.client = client
	return c, nil
}

// Descriptor implements application.Controller. The navigation entry sits
// under Administration and is visible to holders of Extensions.Read.
func (c *Controller) Descriptor() application.ControllerDescriptor {
	d := c.client.Descriptor()
	d.Routes = append(d.Routes,
		application.Get(c.cfg.Path+"/types/kit", application.RequireAll(corePerms.ExtensionsRead)),
		application.Get(c.cfg.Path+"/types/sdk", application.RequireAll(corePerms.ExtensionsRead)),
	)
	return d.WithNav(application.NavNode{
		ID:       "core.extensions",
		Parent:   "core.administration",
		TitleKey: "NavigationLinks.Extensions",
		Path:     c.cfg.Path,
		Icon:     icons.PuzzlePiece(icons.Props{Size: "20"}),
		Order:    65,
	})
}

// Register implements application.Controller.
func (c *Controller) Register(r *mux.Router) {
	sub := r.NewRoute().Subrouter()
	sub.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
		middleware.WithPageContext(),
		middleware.NavItems(),
		requireRead,
	)
	c.client.Register(sub)
	types := r.PathPrefix(c.cfg.Path + "/types").Subrouter()
	types.Use(middleware.Authorize(), middleware.ProvideUser())
	types.HandleFunc("/kit", c.kitTypes).Methods(http.MethodGet)
	types.HandleFunc("/sdk", c.sdkTypes).Methods(http.MethodGet)
}

// requireRead refuses users without Extensions.Read whatever the navigation
// shows; the management API checks every action again.
func requireRead(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sub, err := listext.SubjectFromContext(r.Context())
		if err != nil {
			http.Redirect(w, r, "/login?next="+r.URL.RequestURI(), http.StatusFound)
			return
		}
		if !sub.Has(manage.PermRead) {
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
		Title: intl.TWithDefault(ctx, "Extensions.Configurator.Title", "Extensions"),
		Screen: screen.Render(Props{
			BasePath: c.cfg.Path, APIBase: ManageAPI, TypesBase: c.cfg.Path + "/types",
		}),
	}, nil
}

// session supplies theme, locale, messages and permissions to the host
// context. It carries no CSRF token: the screen's state-changing calls go to
// /_ext/manage, which refuses a request a browser marks as cross-site
// (package originguard, the SDK's Fetch Metadata / Origin mechanism), so a
// token would only be a secret nobody checks.
func session(r *http.Request) (clienthost.SessionContext, error) {
	ctx := r.Context()
	sc := clienthost.SessionContext{Theme: "light", Locale: listext.LangFromContext(ctx), Messages: Messages(ctx)}
	if sub, err := listext.SubjectFromContext(ctx); err == nil {
		sc.Permissions = append([]string(nil), sub.Permissions...)
	}
	if u, err := composables.UseUser(ctx); err == nil {
		sc.User = map[string]any{"id": u.ID(), "name": strings.TrimSpace(u.FirstName() + " " + u.LastName())}
	}
	return sc, nil
}

// shell renders the page inside the authenticated application layout.
func shell(_ context.Context, page clienthost.Page, content templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		layout := layouts.Authenticated(layouts.AuthenticatedProps{BaseProps: layouts.BaseProps{Title: page.Title}})
		return layout.Render(templ.WithChildren(ctx, content), w)
	})
}

func (c *Controller) subject(w http.ResponseWriter, r *http.Request) (spi.Subject, bool) {
	sub, err := listext.SubjectFromContext(r.Context())
	if err != nil {
		http.Error(w, "sign in required", http.StatusUnauthorized)
		return spi.Subject{}, false
	}
	if !sub.Has(manage.PermRead) {
		http.Error(w, "permission "+manage.PermRead+" is required", http.StatusForbidden)
		return spi.Subject{}, false
	}
	return sub, true
}

func writeJSON(w http.ResponseWriter, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		slog.Default().Error("extruntime configurator: encode response", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-cache")
	if _, err := w.Write(append(body, '\n')); err != nil {
		slog.Default().Debug("extruntime configurator: write response", "error", err)
	}
}

// kitTypes serves the files of the current developer kit snapshot: the
// typings of core and other packages. The kit comes from the management
// service, so rights are its own (Extensions.Read).
func (c *Controller) kitTypes(w http.ResponseWriter, r *http.Request) {
	sub, ok := c.subject(w, r)
	if !ok {
		return
	}
	hash, _, err := c.cfg.Service.KitModules(r.Context(), sub)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}
	kit, err := c.cfg.Service.KitSnapshot(r.Context(), sub, hash)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}
	files := KitFiles(kit)
	// The kit hash identifies the content: the editor revalidates cheaply.
	if notModified(w, r, `"`+files.Hash+`"`) {
		return
	}
	writeJSON(w, files)
}

// notModified sets the ETag and answers 304 when the client already has it.
func notModified(w http.ResponseWriter, r *http.Request, etag string) bool {
	w.Header().Set("ETag", etag)
	for _, candidate := range strings.Split(r.Header.Get("If-None-Match"), ",") {
		if c := strings.TrimSpace(candidate); c == etag || c == "W/"+etag || c == "*" {
			w.Header().Set("Cache-Control", "private, no-cache")
			w.WriteHeader(http.StatusNotModified)
			return true
		}
	}
	return false
}

// TypeFiles is the JSON served to Monaco: virtual node_modules paths to file
// contents.
type TypeFiles struct {
	// Hash identifies the content (kit snapshot hash, or a digest of the SDK
	// typings) so the editor can skip reloading.
	Hash  string            `json:"hash"`
	Files map[string]string `json:"files"`
}

// KitFiles maps a kit to node_modules paths ("@iota-kit/core/index.d.ts").
// Modules without a package.json get one that points at their typings, so the
// TypeScript worker resolves `@iota-kit/<namespace>` like npm would.
func KitFiles(kit build.Kit) TypeFiles {
	out := TypeFiles{Hash: kit.Hash, Files: map[string]string{}}
	for module, tree := range kit.Modules {
		for p, content := range tree {
			if strings.HasSuffix(p, ".d.ts") || p == "package.json" || strings.HasSuffix(p, ".js") {
				out.Files[path.Join(module, p)] = string(content)
			}
		}
		if _, has := tree["package.json"]; !has {
			out.Files[path.Join(module, "package.json")] = fmt.Sprintf(`{"name":%q,"version":"0.0.0","main":"index.js","types":"index.d.ts"}`, module)
		}
	}
	return out
}

// sdkTypes serves the typings of @iota-uz/sdk (and the packages they import)
// from the configured node_modules tree. They are collected on the first
// request and kept for the life of the process (a failure is not kept); the
// digest of the typings is the ETag, so an editor that has them gets a 304.
func (c *Controller) sdkTypes(w http.ResponseWriter, r *http.Request) {
	if _, ok := c.subject(w, r); !ok {
		return
	}
	body, etag, err := c.sdkSnapshot()
	if err != nil {
		c.log.Error("extruntime configurator: collect SDK typings", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if notModified(w, r, etag) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-cache")
	if _, err := w.Write(body); err != nil {
		c.log.Debug("extruntime configurator: write response", "error", err)
	}
}

func (c *Controller) sdkSnapshot() ([]byte, string, error) {
	c.sdkMu.Lock()
	defer c.sdkMu.Unlock()
	if c.sdkReady {
		return c.sdkBody, c.sdkEtag, nil
	}
	files, err := collectTypes(c.cfg.Types, c.cfg.TypePackages)
	if err != nil {
		return nil, "", err
	}
	body, err := json.Marshal(files)
	if err != nil {
		return nil, "", err
	}
	c.sdkBody, c.sdkEtag, c.sdkReady = append(body, '\n'), `"`+files.Hash+`"`, true
	return c.sdkBody, c.sdkEtag, nil
}

// collectTypes reads .d.ts and package.json files of the allowed top-level
// packages. A nil tree yields no files.
func collectTypes(tree fs.FS, packages []string) (TypeFiles, error) {
	out := TypeFiles{Files: map[string]string{}}
	if tree == nil {
		return out, nil
	}
	var total int64
	for _, pkg := range packages {
		root := path.Clean(pkg)
		if _, err := fs.Stat(tree, root); err != nil {
			continue
		}
		err := fs.WalkDir(tree, root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "node_modules" && p != root {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(p, ".d.ts") && !strings.HasSuffix(p, ".d.mts") && d.Name() != "package.json" {
				return nil
			}
			b, err := fs.ReadFile(tree, p)
			if err != nil {
				return err
			}
			total += int64(len(b))
			if total > maxTypesBytes {
				return serrors.New(serrors.FailedPrecondition, "typings exceed "+fmt.Sprint(maxTypesBytes)+" bytes")
			}
			out.Files[p] = string(b)
			return nil
		})
		if err != nil {
			return TypeFiles{}, err
		}
	}
	paths := make([]string, 0, len(out.Files))
	for p := range out.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	h := newDigest()
	for _, p := range paths {
		h.add(p, out.Files[p])
	}
	out.Hash = h.sum()
	return out, nil
}
