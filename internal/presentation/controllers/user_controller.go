package controllers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/users"
	"github.com/iota-agency/iota-erp/internal/presentation/types"
	"github.com/iota-agency/iota-erp/pkg/composables"
)

type UserController struct {
	app *services.Application
}

func NewUserController(app *services.Application) *UserController {
	return &UserController{
		app: app,
	}
}

func (c *UserController) Users(w http.ResponseWriter, r *http.Request) {
	pathname := r.URL.Path
	localizer, found := composables.UseLocalizer(r.Context())
	if !found {
		http.Error(w, "localizer not found", http.StatusInternalServerError)
		return
	}
	pageCtx := &types.PageContext{
		Localizer: localizer,
		Pathname:  pathname,
		Title:     "Users",
	}
	us, err := c.app.UserService.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Error retreving users", http.StatusInternalServerError)
		return
	}
	templ.Handler(users.Index(pageCtx, us), templ.WithStreaming()).ServeHTTP(w, r)
}
