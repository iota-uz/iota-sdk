package controllers

import (
	"net/http"

	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/home"
	"github.com/iota-agency/iota-erp/internal/presentation/types"
	"github.com/iota-agency/iota-erp/pkg/composables"
)

type HomeController struct {
	app *services.Application
}

func NewHomeController(app *services.Application) *HomeController {
	return &HomeController{
		app: app,
	}
}

func (c *HomeController) Home(w http.ResponseWriter, r *http.Request) {
	pathname := r.URL.Path
	localizer, found := composables.UseLocalizer(r.Context())
	if !found {
		http.Error(w, "localizer not found", http.StatusInternalServerError)
		return
	}
	pageCtx := &types.PageContext{
		Localizer: localizer,
		Pathname:  pathname,
		Title:     "Dashboard",
	}
	if err := home.Index(pageCtx).Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
