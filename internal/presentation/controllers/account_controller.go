package controllers

import (
	"github.com/a-h/templ"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/account"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/iota-agency/iota-erp/pkg/middleware"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/app/services"
)

type AccountController struct {
	app      *services.Application
	basePath string
}

func NewAccountController(app *services.Application) Controller {
	return &AccountController{
		app:      app,
		basePath: "/account",
	}
}

func (c *AccountController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(middleware.RequireAuthorization())
	router.HandleFunc("", c.Get).Methods(http.MethodGet)
}

func (c *AccountController) Get(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(
		r,
		composables.NewPageData("Account.Meta.Index.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &account.ProfilePageProps{
		PageContext: pageCtx,
		PostPath:    c.basePath,
	}
	templ.Handler(account.Index(props)).ServeHTTP(w, r)
}
