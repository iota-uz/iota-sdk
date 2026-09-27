package react

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

const immutableCacheControl = "public, max-age=31536000, immutable"
const staticRouteName = "lens.react.assets"

type StaticController struct {
	basePath string
	handler  http.Handler
	// versionedHandler is nil when the bundle is read from disk: the revision
	// changes on every rebuild, so the prefix is resolved per request instead.
	versionedHandler http.Handler
}

var _ application.Controller = (*StaticController)(nil)

func NewStaticController() *StaticController {
	return NewStaticControllerAt(DefaultAssetBasePath)
}

func NewStaticControllerAt(basePath string) *StaticController {
	basePath = normalizeAssetBasePath(basePath)
	assets := Assets()
	controller := &StaticController{
		basePath: basePath,
		handler:  http.StripPrefix(basePath+"/", http.FileServer(http.FS(DistFS()))),
	}
	if !source.live {
		controller.versionedHandler = versionedFileHandler(basePath, assets.Revision)
	}
	return controller
}

func versionedFileHandler(basePath, revision string) http.Handler {
	return http.StripPrefix(basePath+"/"+revision+"/", http.FileServer(http.FS(DistFS())))
}

func (c *StaticController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor(
		"lens.react.assets",
		0,
		application.Prefix(c.basePath, application.Public()),
	)
}

func (c *StaticController) Register(router *mux.Router) {
	if router.GetRoute(staticRouteName) != nil {
		return
	}
	router.PathPrefix(c.basePath + "/").Name(staticRouteName).Handler(http.HandlerFunc(c.serveHTTP))
}

func (c *StaticController) serveHTTP(w http.ResponseWriter, r *http.Request) {
	relativePath := strings.TrimPrefix(r.URL.Path, c.basePath+"/")
	assets, err := source.assets()
	if err != nil {
		http.Error(w, compatibilityAssetsError(err).Error(), http.StatusServiceUnavailable)
		return
	}
	revision := assets.Revision
	versionedPrefix := revision + "/"
	if strings.HasPrefix(relativePath, "assets/") || strings.HasPrefix(relativePath, versionedPrefix+"assets/") {
		w.Header().Set("Cache-Control", immutableCacheControl)
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if revision != "" && strings.HasPrefix(relativePath, versionedPrefix) {
		versioned := c.versionedHandler
		if versioned == nil {
			versioned = versionedFileHandler(c.basePath, revision)
		}
		versioned.ServeHTTP(w, r)
		return
	}
	c.handler.ServeHTTP(w, r)
}

func normalizeAssetBasePath(basePath string) string {
	basePath = "/" + strings.Trim(strings.TrimSpace(basePath), "/")
	if basePath == "/" {
		return DefaultAssetBasePath
	}
	return basePath
}
