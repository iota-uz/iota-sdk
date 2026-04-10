// Package controllers provides this package.
package controllers

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

type LogoutController struct {
	sessionService *services.SessionService
}

func NewLogoutController(sessionService *services.SessionService) application.Controller {
	return &LogoutController{
		sessionService: sessionService,
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

	// Delete the session from the database so the token cannot be reused.
	if token, err := r.Cookie(conf.SidCookieKey); err == nil && token.Value != "" {
		_ = c.sessionService.Delete(r.Context(), token.Value)
	}

	http.SetCookie(
		w, &http.Cookie{
			Name:     conf.SidCookieKey,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		},
	)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
