package application

import (
	"context"
	"net/http"
	"path"
	"strings"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

type PermissionLogic int

const (
	PermissionLogicAll PermissionLogic = iota
	PermissionLogicAny
)

type AuthPolicy struct {
	Public      bool
	Permissions []permission.Permission
	Logic       PermissionLogic
}

type ControllerDescriptor struct {
	ID       string
	Order    int
	Replaces []string
	Routes   []RouteSpec
	Nav      []NavNode
}

type RouteSpec struct {
	Method         string
	Path           string
	Prefix         bool
	Host           string
	AllowCollision bool
	Auth           AuthPolicy
}

type Surface string

const (
	SurfaceSidebar        Surface = "sidebar"
	SurfaceSpotlight      Surface = "spotlight"
	SurfaceSitemap        Surface = "sitemap"
	SurfaceCommandPalette Surface = "command_palette"
)

type SurfaceOptions struct {
	Hidden   bool
	TitleKey string
	Path     string
	Icon     templ.Component
	Order    int
	Keywords []string
}

type NavAction struct {
	ID       string
	TitleKey string
	Path     string
	Auth     *AuthPolicy
	Surfaces map[Surface]SurfaceOptions
}

type NavNode struct {
	ID         string
	Parent     string
	Workspace  string
	TitleKey   string
	Path       string
	Icon       templ.Component
	Order      int
	Before     string
	After      string
	Keywords   []string
	Surfaces   map[Surface]SurfaceOptions
	Actions    []NavAction
	IsBeta     bool
	Visibility *AuthPolicy
}

type NavScope struct {
	TenantID    uuid.UUID
	UserID      uint
	Roles       []string
	Permissions []string
	Workspace   string
}

type NavProvider interface {
	ProvideNav(ctx context.Context, scope NavScope) ([]NavNode, error)
}

func Descriptor(id string, order int, routes ...RouteSpec) ControllerDescriptor {
	return ControllerDescriptor{
		ID:     id,
		Order:  order,
		Routes: routes,
	}
}

func (d ControllerDescriptor) WithNav(nodes ...NavNode) ControllerDescriptor {
	d.Nav = append(d.Nav, nodes...)
	return d
}

func Route(method, routePath string, opts ...RouteOption) RouteSpec {
	route := RouteSpec{
		Method: strings.ToUpper(strings.TrimSpace(method)),
		Path:   NormalizeRoutePath(routePath),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&route)
		}
	}
	return route
}

type RouteOption func(*RouteSpec)

func WithAuth(auth AuthPolicy) RouteOption {
	return func(route *RouteSpec) {
		route.Auth = auth
	}
}

func Public() RouteOption {
	return WithAuth(AuthPolicy{Public: true})
}

func RequireAll(permissions ...permission.Permission) RouteOption {
	return WithAuth(AuthPolicy{Permissions: normalizePermissions(permissions), Logic: PermissionLogicAll})
}

func RequireAny(permissions ...permission.Permission) RouteOption {
	return WithAuth(AuthPolicy{Permissions: normalizePermissions(permissions), Logic: PermissionLogicAny})
}

func normalizePermissions(values []permission.Permission) []permission.Permission {
	out := make([]permission.Permission, 0, len(values))
	for _, perm := range values {
		if perm != nil {
			out = append(out, perm)
		}
	}
	return out
}

func Get(routePath string, opts ...RouteOption) RouteSpec {
	return Route(http.MethodGet, routePath, opts...)
}

func Post(routePath string, opts ...RouteOption) RouteSpec {
	return Route(http.MethodPost, routePath, opts...)
}

func Put(routePath string, opts ...RouteOption) RouteSpec {
	return Route(http.MethodPut, routePath, opts...)
}

func Delete(routePath string, opts ...RouteOption) RouteSpec {
	return Route(http.MethodDelete, routePath, opts...)
}

func Prefix(routePath string, opts ...RouteOption) RouteSpec {
	route := RouteSpec{
		Path:   NormalizeRoutePath(routePath),
		Prefix: true,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&route)
		}
	}
	return route
}

func WithHost(host string, route RouteSpec) RouteSpec {
	route.Host = strings.TrimSpace(host)
	return route
}

func AllowCollision(route RouteSpec) RouteSpec {
	route.AllowCollision = true
	return route
}

func BaseRoute(method, basePath, suffix string, opts ...RouteOption) RouteSpec {
	return Route(method, JoinRoutePath(basePath, suffix), opts...)
}

func BasePrefix(basePath, suffix string, opts ...RouteOption) RouteSpec {
	return Prefix(JoinRoutePath(basePath, suffix), opts...)
}

func NormalizeRoutePath(routePath string) string {
	routePath = strings.TrimSpace(routePath)
	if routePath == "" {
		return "/"
	}
	if !strings.HasPrefix(routePath, "/") {
		routePath = "/" + routePath
	}
	return path.Clean(routePath)
}

func JoinRoutePath(basePath, suffix string) string {
	base := NormalizeRoutePath(basePath)
	suffix = strings.TrimSpace(suffix)
	if suffix == "" || suffix == "/" {
		return base
	}
	return path.Join(base, strings.TrimPrefix(suffix, "/"))
}
