package controllers

import (
	"github.com/iota-agency/iota-erp/internal/configuration"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/app/services"
)

type LogoutController struct {
	app *services.Application
}

func NewLogoutController(app *services.Application) Controller {
	return &LogoutController{
		app: app,
	}
}

func (c *LogoutController) Register(r *mux.Router) {
	r.HandleFunc("/logout", c.Logout).Methods(http.MethodGet)
}

func (c *LogoutController) Logout(w http.ResponseWriter, r *http.Request) {
	conf := configuration.Use()
	http.SetCookie(
		w, &http.Cookie{
			Name:   conf.SidCookieKey,
			Value:  "",
			MaxAge: -1,
		},
	)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
