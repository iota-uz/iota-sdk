package controllers

import (
	"net/http"

	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/login"
	"github.com/iota-agency/iota-erp/internal/presentation/types"
	"github.com/iota-agency/iota-erp/pkg/composables"
)

func NewLoginController(app *services.Application) *LoginController {
	return &LoginController{
		app: app,
	}
}

type LoginController struct {
	app *services.Application
}

func (c *LoginController) Login(w http.ResponseWriter, r *http.Request) {
	pageCtx := &types.PageContext{}
	_, found := composables.UseLocalizer(r.Context())
	if !found {
		http.Error(w, "localizer not found", http.StatusInternalServerError)
		return
	}
	if err := login.Index(pageCtx).Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
