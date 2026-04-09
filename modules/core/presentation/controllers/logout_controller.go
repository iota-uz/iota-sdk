// Package controllers provides this package.
package controllers

import (
	"net/http"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"

	"github.com/gorilla/mux"
)

type LogoutController struct {
}

func NewLogoutController() application.Controller {
	return &LogoutController{
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
