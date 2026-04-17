// Package controllers provides this package.
package controllers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/cookies"
	"github.com/iota-uz/iota-sdk/pkg/di"
)

type LogoutController struct {
	cfg        *httpconfig.Config
	cookiesCfg *cookies.Config
	appCfg     *appconfig.Config
}

func NewLogoutController(cfg *httpconfig.Config, cookiesCfg *cookies.Config, appCfg *appconfig.Config) application.Controller {
	return &LogoutController{cfg: cfg, cookiesCfg: cookiesCfg, appCfg: appCfg}
}

func (c *LogoutController) Key() string {
	return "/logout"
}

func (c *LogoutController) Register(r *mux.Router) {
	r.HandleFunc("/logout", di.H(c.Logout)).Methods(http.MethodPost)
	r.HandleFunc("/logout", c.MethodNotAllowed).Methods(http.MethodGet)
}

func (c *LogoutController) MethodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Allow", http.MethodPost)
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

func (c *LogoutController) Logout(
	w http.ResponseWriter,
	r *http.Request,
	sessionService *services.SessionService,
	logger *logrus.Entry,
) {
	if cookie, err := r.Cookie(c.cookiesCfg.SID); err == nil && cookie.Value != "" {
		if err := sessionService.Delete(r.Context(), cookie.Value); err != nil && !errors.Is(err, persistence.ErrSessionNotFound) {
			logger.WithError(err).Warn("failed to delete session on logout")
		}
	}

	http.SetCookie(
		w, &http.Cookie{
			Name:     c.cookiesCfg.SID,
			Value:    "",
			Domain:   c.cfg.Domain,
			Path:     "/",
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   c.appCfg.IsProduction(),
		},
	)

	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Clear-Site-Data", `"cache", "cookies", "storage"`)

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
