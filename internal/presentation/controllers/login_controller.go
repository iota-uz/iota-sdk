package controllers

import (
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/login"
	"github.com/iota-agency/iota-erp/internal/presentation/types"
	"net/http"
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
	if err := login.Index(pageCtx).Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
