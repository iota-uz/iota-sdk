// Package controllers provides this package.
package controllers

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/di"
)

type LogoutController struct {
	cfg             *httpconfig.Config
	browserSessions *services.BrowserSessionService
}

func NewLogoutController(cfg *httpconfig.Config, browserSessions *services.BrowserSessionService) application.Controller {
	return &LogoutController{cfg: cfg, browserSessions: browserSessions}
}

func (c *LogoutController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("core.logout", 0, application.Route("", "/logout"))
}

func (c *LogoutController) Register(r *mux.Router) {
	r.HandleFunc("/logout", di.H(c.Logout)).Methods(http.MethodPost)
	r.HandleFunc("/logout/all", di.H(c.LogoutAll)).Methods(http.MethodPost)
	r.HandleFunc("/logout", c.MethodNotAllowed).Methods(http.MethodGet)
}

func (c *LogoutController) MethodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Allow", http.MethodPost)
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

func (c *LogoutController) Logout(
	w http.ResponseWriter,
	r *http.Request,
	logger *logrus.Entry,
) {
	sessions, err := c.browserSessions.RemoveCurrent(w, r)
	if err != nil {
		logger.WithError(err).Warn("failed to delete current session on logout")
		c.browserSessions.Clear(w)
	}

	setLogoutHeaders(w)
	redirectURL := "/login"
	if len(sessions) > 0 {
		redirectURL = "/"
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (c *LogoutController) LogoutAll(w http.ResponseWriter, r *http.Request, logger *logrus.Entry) {
	if err := c.browserSessions.RemoveAll(w, r); err != nil {
		logger.WithError(err).Warn("failed to delete all sessions on logout")
		c.browserSessions.Clear(w)
	}
	setLogoutHeaders(w)
	w.Header().Set("Clear-Site-Data", `"cache", "cookies", "storage"`)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func setLogoutHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}
