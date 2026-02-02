package controllers

import (
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/assets"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/templates/pages"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type BiChatWebController struct {
	app      application.Application
	assets   *presentation.ViteAssets
	assetsFS fs.FS
	config   interface{} // Module config for feature flags (can be nil)
}

func NewBiChatWebController(app application.Application) (*BiChatWebController, error) {
	viteAssets, err := presentation.LoadViteAssets("/bi-chat/assets")
	if err != nil {
		return nil, serrors.E(serrors.Op("NewBiChatWebController"), err)
	}

	// Create sub-filesystem rooted at "dist/" for serving static files
	distFS, err := fs.Sub(assets.DistFS, "dist")
	if err != nil {
		return nil, serrors.E(serrors.Op("NewBiChatWebController"), err)
	}

	return &BiChatWebController{
		app:      app,
		assets:   viteAssets,
		assetsFS: distFS,
	}, nil
}

func (c *BiChatWebController) Register(r *mux.Router) {
	// Create middleware chain for HTML pages
	pageMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(c.app),
		middleware.ProvideLocalizer(c.app),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}

	// Asset serving (no auth required for static files)
	assetHandler := http.StripPrefix("/bi-chat/assets", http.FileServer(http.FS(c.assetsFS)))
	r.PathPrefix("/bi-chat/assets/").Handler(assetHandler).Methods(http.MethodGet)

	// Main page route
	mainRoute := r.Methods(http.MethodGet).Subrouter()
	for _, mw := range pageMiddleware {
		mainRoute.Use(mw)
	}
	mainRoute.HandleFunc("/bi-chat", c.indexHandler)

	// SPA routes (for client-side routing like /bi-chat/session/{id})
	spaRoute := r.Methods(http.MethodGet).PathPrefix("/bi-chat/session").Subrouter()
	for _, mw := range pageMiddleware {
		spaRoute.Use(mw)
	}
	spaRoute.PathPrefix("/").HandlerFunc(c.indexHandler)
}

func (c *BiChatWebController) Key() string {
	return "bichat_web"
}

func (c *BiChatWebController) indexHandler(w http.ResponseWriter, r *http.Request) {
	pageCtx := composables.UsePageCtx(r.Context())
	user, err := composables.UseUser(r.Context())
	if err != nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	// Build user name
	userName := user.FirstName()
	if lastName := user.LastName(); lastName != "" {
		if userName != "" {
			userName += " " + lastName
		} else {
			userName = lastName
		}
	}

	// Build IOTA context for React app
	iotaContext := map[string]interface{}{
		"user": map[string]interface{}{
			"id":    user.ID(),
			"name":  userName,
			"email": user.Email().Value(),
		},
		"tenant": map[string]interface{}{
			"id":   user.TenantID().String(),
			"name": "", // TODO: Get tenant name if needed
		},
		"config": map[string]interface{}{
			"locale":          pageCtx.GetLocale().String(),
			"graphQLEndpoint": "/bi-chat/graphql",
			"streamEndpoint":  "/bi-chat/stream",
		},
		"session": nil, // No session pre-selected
		"extensions": map[string]interface{}{
			"features": map[string]bool{
				"vision":          false, // TODO: Get from config
				"webSearch":       false,
				"codeInterpreter": false,
				"multiAgent":      false,
			},
		},
	}

	contextJSON, err := json.Marshal(iotaContext)
	if err != nil {
		http.Error(w, "Failed to build context", http.StatusInternalServerError)
		return
	}

	templ.Handler(pages.Index(pageCtx, c.assets, string(contextJSON))).ServeHTTP(w, r)
}
