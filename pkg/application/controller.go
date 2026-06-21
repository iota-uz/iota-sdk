package application

import (
	"net/http"
	"path"
	"strings"
)

func Descriptor(id string, order int, routes ...RouteSpec) ControllerDescriptor {
	return ControllerDescriptor{
		ID:     id,
		Order:  order,
		Routes: routes,
	}
}

func Route(method, routePath string) RouteSpec {
	return RouteSpec{
		Method: strings.ToUpper(strings.TrimSpace(method)),
		Path:   NormalizeRoutePath(routePath),
	}
}

func Get(routePath string) RouteSpec {
	return Route(http.MethodGet, routePath)
}

func Post(routePath string) RouteSpec {
	return Route(http.MethodPost, routePath)
}

func Put(routePath string) RouteSpec {
	return Route(http.MethodPut, routePath)
}

func Delete(routePath string) RouteSpec {
	return Route(http.MethodDelete, routePath)
}

func Prefix(routePath string) RouteSpec {
	return RouteSpec{
		Path:   NormalizeRoutePath(routePath),
		Prefix: true,
	}
}

func WithHost(host string, route RouteSpec) RouteSpec {
	route.Host = strings.TrimSpace(host)
	return route
}

func AllowCollision(route RouteSpec) RouteSpec {
	route.AllowCollision = true
	return route
}

func BaseRoute(method, basePath, suffix string) RouteSpec {
	return Route(method, JoinRoutePath(basePath, suffix))
}

func BasePrefix(basePath, suffix string) RouteSpec {
	return Prefix(JoinRoutePath(basePath, suffix))
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
