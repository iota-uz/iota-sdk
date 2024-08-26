package controllers

import (
	"net/http"

	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/login"
	"github.com/iota-agency/iota-erp/pkg/composables"
)

func NewLoginController(app *services.Application) *LoginController {
	return &LoginController{
		app: app,
	}
}

type LoginController struct {
	app *services.Application
}

func (c *LoginController) Get(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{
		Title: "Login.Meta.Title",
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := login.Index(pageCtx).Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (c *LoginController) Post(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	email := r.FormValue("email")
	password := r.FormValue("password")
	if email == "" || password == "" {
		http.Error(w, "email or password is empty", http.StatusBadRequest)
		return
	}
	_, session, err := c.app.AuthService.Authenticate(r.Context(), email, password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	conf := configuration.Use()
	cookie := &http.Cookie{
		Name:     conf.SidCookieKey,
		Value:    session.Token,
		Expires:  session.ExpiresAt,
		HttpOnly: false,
		SameSite: http.SameSiteNoneMode,
		Secure:   conf.GoAppEnvironment == "production",
		Domain:   conf.FrontendDomain,
	}
	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/", http.StatusFound)
}
