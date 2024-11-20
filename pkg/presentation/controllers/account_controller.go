package controllers

import (
	"github.com/a-h/templ"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/presentation/templates/pages/account"
	"github.com/iota-agency/iota-sdk/pkg/shared"
	"github.com/iota-agency/iota-sdk/pkg/shared/middleware"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"net/http"

	"github.com/gorilla/mux"
)

type AccountController struct {
	app      *application.Application
	basePath string
}

func NewAccountController(app *application.Application) shared.Controller {
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
		types.NewPageData("Account.Meta.Index.Title", ""),
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
