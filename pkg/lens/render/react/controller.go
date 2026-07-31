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
	basePath         string
	handler          http.Handler
	versionedHandler http.Handler
}

var _ application.Controller = (*StaticController)(nil)

func NewStaticController() *StaticController {
	return NewStaticControllerAt(DefaultAssetBasePath)
}

func NewStaticControllerAt(basePath string) *StaticController {
	basePath = normalizeAssetBasePath(basePath)
	return &StaticController{
		basePath:         basePath,
		handler:          http.StripPrefix(basePath+"/", http.FileServer(http.FS(DistFS()))),
		versionedHandler: http.StripPrefix(basePath+"/"+productionAssets.Revision+"/", http.FileServer(http.FS(DistFS()))),
	}
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
	versionedPrefix := productionAssets.Revision + "/"
	if strings.HasPrefix(relativePath, "assets/") || strings.HasPrefix(relativePath, versionedPrefix+"assets/") {
		w.Header().Set("Cache-Control", immutableCacheControl)
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if strings.HasPrefix(relativePath, versionedPrefix) {
		c.versionedHandler.ServeHTTP(w, r)
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
