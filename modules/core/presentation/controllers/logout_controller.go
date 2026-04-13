// Package controllers provides this package.
package controllers

import (
	"net/http"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/di"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/sirupsen/logrus"
)

type LogoutController struct {
}

func NewLogoutController() application.Controller {
	return &LogoutController{}
}

func (c *LogoutController) Key() string {
	return "/logout"
}

func (c *LogoutController) Register(r *mux.Router) {
	r.HandleFunc("/logout", di.H(c.Logout)).Methods(http.MethodGet)
}

func (c *LogoutController) Logout(
	w http.ResponseWriter,
	r *http.Request,
	sessionService *services.SessionService,
	logger *logrus.Entry,
) {
	conf := configuration.Use()

	if cookie, err := r.Cookie(conf.SidCookieKey); err == nil && cookie.Value != "" {
		if err := sessionService.Delete(r.Context(), cookie.Value); err != nil {
			logger.WithError(err).Warn("failed to delete session on logout")
		}
	}

	http.SetCookie(
		w, &http.Cookie{
			Name:     conf.SidCookieKey,
			Value:    "",
			Domain:   conf.Domain,
			Path:     "/",
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   conf.GoAppEnvironment == configuration.Production,
		},
	)

	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Clear-Site-Data", `"cache", "cookies", "storage"`)

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
