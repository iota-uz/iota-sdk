package controllers

import (
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"net/http"

	"github.com/gorilla/mux"
)

type LogoutController struct {
	app application.Application
}

func NewLogoutController(app application.Application) application.Controller {
	return &LogoutController{
		app: app,
	}
}

func (c *LogoutController) Key() string {
	return "/logout"
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
