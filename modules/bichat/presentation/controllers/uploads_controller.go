package controllers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	bichatperm "github.com/iota-uz/iota-sdk/modules/bichat/permissions"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

// UploadsController serves artifact/export files from a local directory.
//
// This is intentionally generic: it authorizes access at the endpoint level
// (via RequireAccessPermission) and relies on hard-to-guess filenames (UUIDs).
// If you need per-file authorization, serve downloads through a DB-backed controller.
type UploadsController struct {
	app     application.Application
	baseDir string
	opts    ControllerOptions
}

func NewUploadsController(
	app application.Application,
	baseDir string,
	opts ...ControllerOption,
) *UploadsController {
	return &UploadsController{
		app:     app,
		baseDir: baseDir,
		opts:    applyControllerOptions(opts...),
	}
}

func (c *UploadsController) Key() string {
	return "bichat.UploadsController"
}

func (c *UploadsController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(c.app),
		middleware.ProvideLocalizer(c.app),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}

	subRouter := r.PathPrefix(c.opts.BasePath).Subrouter()
	subRouter.Use(commonMiddleware...)
	subRouter.HandleFunc("/uploads/{name}", c.Serve).Methods(http.MethodGet)
}

func (c *UploadsController) Serve(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	name = strings.TrimSpace(name)

	// Basic traversal hardening.
	if name == "" ||
		strings.Contains(name, "/") ||
		strings.Contains(name, "\\") ||
		strings.Contains(name, "..") {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Enforce access permission (defaults to BiChat.Access when not set).
	require := c.opts.RequireAccessPermission
	if require == nil {
		require = bichatperm.BiChatAccess
	}
	if err := composables.CanUser(r.Context(), require); err != nil {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	fullPath := filepath.Join(c.baseDir, filepath.Base(name))
	if _, err := os.Stat(fullPath); err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, fullPath)
}
