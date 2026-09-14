package clienthost

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/sdkidentity"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type Manifest struct {
	Package           string            `json:"package"`
	PackageVersion    string            `json:"packageVersion"`
	SDKCommit         string            `json:"sdkCommit"`
	SDKReleaseVersion string            `json:"sdkReleaseVersion"`
	ProtocolVersion   string            `json:"protocolVersion"`
	Entry             string            `json:"entry"`
	Styles            []string          `json:"styles,omitempty"`
	Integrity         map[string]string `json:"integrity,omitempty"`
}

func (m Manifest) Validate() error {
	const op = serrors.Op("clienthost.Manifest.Validate")

	if strings.TrimSpace(m.Package) == "" || strings.TrimSpace(m.SDKCommit) == "" || strings.TrimSpace(m.SDKReleaseVersion) == "" {
		return serrors.E(op, fmt.Errorf("package, sdk release version, and sdk commit are required"))
	}
	if err := (sdkidentity.Identity{
		ReleaseVersion:  m.SDKReleaseVersion,
		SourceCommit:    m.SDKCommit,
		ProtocolVersion: m.ProtocolVersion,
	}).Validate(); err != nil {
		return serrors.E(op, err)
	}
	if strings.TrimSpace(m.Entry) == "" {
		return fmt.Errorf("clienthost manifest: entry is required")
	}
	if protocolMajor(m.ProtocolVersion) != protocolMajor(ProtocolVersion) {
		return fmt.Errorf("clienthost manifest: protocol %q is incompatible with %q", m.ProtocolVersion, ProtocolVersion)
	}
	return nil
}

func ParseManifest(data []byte) (Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("clienthost manifest: decode: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

type RouteContext struct {
	BootstrapVersion  string           `json:"bootstrapVersion"`
	ProtocolVersion   string           `json:"protocolVersion"`
	SDKReleaseVersion string           `json:"sdkReleaseVersion"`
	SDKCommit         string           `json:"sdkCommit"`
	Initial           json.RawMessage  `json:"initial"`
	Theme             string           `json:"theme"`
	CSRF              string           `json:"csrf,omitempty"`
	Route             RouteIdentity    `json:"route"`
	Session           SessionState     `json:"session"`
	Locale            LocaleContext    `json:"locale"`
	User              any              `json:"user,omitempty"`
	Tenant            any              `json:"tenant,omitempty"`
	Permissions       []string         `json:"permissions"`
	Services          ServiceEndpoints `json:"services"`
}

type RouteIdentity struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	FeatureID string `json:"featureId"`
}

type SessionState struct {
	CSRF            string `json:"csrf,omitempty"`
	ExpiresAt       string `json:"expiresAt,omitempty"`
	RefreshEndpoint string `json:"refreshEndpoint,omitempty"`
	ReauthURL       string `json:"reauthUrl,omitempty"`
}

type LocaleContext struct {
	Language string            `json:"language"`
	Messages map[string]string `json:"messages"`
}

type ServiceEndpoints struct {
	RPC       string `json:"rpc,omitempty"`
	Telemetry string `json:"telemetry,omitempty"`
}

type SessionContext struct {
	Theme       string
	CSRF        string
	ExpiresAt   string
	RefreshURL  string
	ReauthURL   string
	Locale      string
	Messages    map[string]string
	User        any
	Tenant      any
	Permissions []string
}

type RoutePayload struct {
	Title   string
	Initial any
	Screen  RenderedFeature
}

// Feature identifies one statically discovered client module. Implementations
// are descriptors only: they do not mount UI, perform requests, or build
// assets when a Go package initializes.
type Feature interface {
	FeatureID() string
	SourcePath() string
}

// RenderedFeature is the request-specific props snapshot for a Feature.
type RenderedFeature interface {
	FeatureID() string
	Props() any
}

type Route struct {
	Spec    application.RouteSpec
	Feature Feature
	Build   func(context.Context, *http.Request) (RoutePayload, error)
}

type Page struct {
	Title    string
	MountID  string
	Manifest Manifest
	Context  RouteContext
}

// Shell renders the canonical bootstrap fragment inside a product-owned page
// layout. The standard host owns assets/context; the product shell owns nav.
type Shell func(context.Context, Page, templ.Component) templ.Component

type SessionProvider func(*http.Request) (SessionContext, error)

type Controller struct {
	id       string
	manifest Manifest
	routes   []Route
	shell    Shell
	session  SessionProvider
	order    int
	services ServiceEndpoints
}

type Option func(*Controller)

func WithShell(shell Shell) Option { return func(controller *Controller) { controller.shell = shell } }
func WithSession(provider SessionProvider) Option {
	return func(controller *Controller) { controller.session = provider }
}
func WithOrder(order int) Option { return func(controller *Controller) { controller.order = order } }
func WithServices(services ServiceEndpoints) Option {
	return func(controller *Controller) { controller.services = services }
}

func NewController(id string, manifest Manifest, routes []Route, options ...Option) (*Controller, error) {
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	controller := &Controller{id: strings.TrimSpace(id), manifest: manifest, routes: append([]Route(nil), routes...), shell: StandaloneShell}
	if controller.id == "" {
		return nil, fmt.Errorf("clienthost controller: id is required")
	}
	routeIDs := make(map[string]struct{}, len(controller.routes))
	for index := range controller.routes {
		route := &controller.routes[index]
		if route.Spec.Renderer != application.RouteRendererClient && route.Spec.Renderer != application.RouteRendererReact {
			return nil, fmt.Errorf("clienthost controller %s: route %s must declare client renderer", controller.id, route.Spec.Path)
		}
		if route.Spec.Renderer == application.RouteRendererClient {
			if route.Feature != nil {
				featureID := strings.TrimSpace(route.Feature.FeatureID())
				if featureID == "" || strings.TrimSpace(route.Feature.SourcePath()) == "" {
					return nil, fmt.Errorf("clienthost controller %s: route %s has an invalid feature descriptor", controller.id, route.Spec.Path)
				}
				if route.Spec.FeatureID != "" && route.Spec.FeatureID != featureID {
					return nil, fmt.Errorf("clienthost controller %s: route %s feature identity %q conflicts with imported feature %q", controller.id, route.Spec.Path, route.Spec.FeatureID, featureID)
				}
				route.Spec.FeatureID = featureID
			}
			if route.Spec.RouteID == "" || route.Spec.FeatureID == "" {
				return nil, fmt.Errorf("clienthost controller %s: route %s requires route and feature identity", controller.id, route.Spec.Path)
			}
			if !route.Spec.AccessExplicit {
				return nil, fmt.Errorf("clienthost controller %s: route %s requires explicit access", controller.id, route.Spec.Path)
			}
			if _, exists := routeIDs[route.Spec.RouteID]; exists {
				return nil, fmt.Errorf("clienthost controller %s: duplicate route id %s", controller.id, route.Spec.RouteID)
			}
			routeIDs[route.Spec.RouteID] = struct{}{}
		}
		if route.Build == nil {
			return nil, fmt.Errorf("clienthost controller %s: route %s build is required", controller.id, route.Spec.Path)
		}
	}
	for _, option := range options {
		if option != nil {
			option(controller)
		}
	}
	return controller, nil
}

func (c *Controller) Descriptor() application.ControllerDescriptor {
	specs := make([]application.RouteSpec, len(c.routes))
	for index := range c.routes {
		specs[index] = c.routes[index].Spec
	}
	return application.Descriptor(c.id, c.order, specs...)
}

func (c *Controller) Register(router *mux.Router) {
	for index := range c.routes {
		route := c.routes[index]
		router.HandleFunc(route.Spec.Path, c.handler(route)).Methods(route.Spec.Method)
	}
}

func (c *Controller) handler(route Route) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		payload, err := route.Build(request.Context(), request)
		if err != nil {
			http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		initialValue := payload.Initial
		if route.Feature != nil {
			if payload.Screen == nil {
				http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			if payload.Screen.FeatureID() != route.Feature.FeatureID() {
				http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			initialValue = payload.Screen.Props()
		} else if payload.Screen != nil {
			http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if err := validateSafeIntegers(initialValue); err != nil {
			http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		initial, err := json.Marshal(initialValue)
		if err != nil {
			http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		session := SessionContext{Theme: "light", Locale: "en", Messages: map[string]string{}}
		if c.session != nil {
			session, err = c.session(request)
			if err != nil {
				http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}
		if strings.TrimSpace(session.Locale) == "" {
			session.Locale = "en"
		}
		if session.Theme != "dark" {
			session.Theme = "light"
		}
		page := Page{
			Title: payload.Title, MountID: "iota-client-route", Manifest: c.manifest,
			Context: RouteContext{
				BootstrapVersion:  BootstrapVersion,
				ProtocolVersion:   ProtocolVersion,
				SDKReleaseVersion: c.manifest.SDKReleaseVersion,
				SDKCommit:         c.manifest.SDKCommit,
				Initial:           initial,
				Theme:             session.Theme,
				CSRF:              session.CSRF,
				Route:             RouteIdentity{ID: route.Spec.RouteID, Path: route.Spec.Path, FeatureID: route.Spec.FeatureID},
				Session:           SessionState{CSRF: session.CSRF, ExpiresAt: session.ExpiresAt, RefreshEndpoint: session.RefreshURL, ReauthURL: session.ReauthURL},
				Locale:            LocaleContext{Language: session.Locale, Messages: cloneMessages(session.Messages)},
				User:              session.User, Tenant: session.Tenant,
				Permissions: append([]string(nil), session.Permissions...),
				Services:    c.services,
			},
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := c.shell(request.Context(), page, Bootstrap(page)).Render(request.Context(), writer); err != nil {
			http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}
}

func cloneMessages(messages map[string]string) map[string]string {
	cloned := make(map[string]string, len(messages))
	for key, value := range messages {
		cloned[key] = value
	}
	return cloned
}

// Bootstrap is the canonical mount/context/manifest fragment used by every
// product shell. JSON is escaped by encoding/json before entering the script.
func Bootstrap(page Page) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, writer io.Writer) error {
		contextJSON, err := json.Marshal(page.Context)
		if err != nil {
			return err
		}
		for _, style := range page.Manifest.Styles {
			if _, err := fmt.Fprintf(writer, `<link rel="stylesheet" href="%s">`, template.HTMLEscapeString(style)); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(writer, `<main id="%s" data-client-route-background><div id="%s-mount"></div></main><div id="%s-portals"></div>`, template.HTMLEscapeString(page.MountID), template.HTMLEscapeString(page.MountID), template.HTMLEscapeString(page.MountID)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(writer, `<script id="iota-client-context" type="application/json">%s</script>`, contextJSON); err != nil {
			return err
		}
		integrity := page.Manifest.Integrity[page.Manifest.Entry]
		integrityAttr := ""
		if integrity != "" {
			integrityAttr = ` integrity="` + template.HTMLEscapeString(integrity) + `" crossorigin="anonymous"`
		}
		_, err = fmt.Fprintf(writer, `<script type="module" src="%s" data-sdk-commit="%s" data-package="%s"%s></script>`, template.HTMLEscapeString(page.Manifest.Entry), template.HTMLEscapeString(page.Manifest.SDKCommit), template.HTMLEscapeString(page.Manifest.Package), integrityAttr)
		return err
	})
}

func StandaloneShell(_ context.Context, page Page, content templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, writer io.Writer) error {
		if _, err := fmt.Fprintf(writer, `<!doctype html><html data-theme="%s"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title></head><body>`, template.HTMLEscapeString(page.Context.Theme), template.HTMLEscapeString(page.Title)); err != nil {
			return err
		}
		if err := content.Render(ctx, writer); err != nil {
			return err
		}
		_, err := io.WriteString(writer, `</body></html>`)
		return err
	})
}

func protocolMajor(version string) string {
	return strings.SplitN(strings.TrimSpace(version), ".", 2)[0]
}
