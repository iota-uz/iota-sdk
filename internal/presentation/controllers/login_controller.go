package controllers

import (
	"github.com/gorilla/mux"
	"net/http"

	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/login"
	"github.com/iota-agency/iota-erp/pkg/composables"
)

func NewLoginController(app *services.Application) Controller {
	return &LoginController{
		app: app,
	}
}

type LoginController struct {
	app *services.Application
}

func (c *LoginController) Register(r *mux.Router) {
	r.HandleFunc("/oauth/google/callback", c.app.AuthService.OauthGoogleCallback)
	r.HandleFunc("/login", c.Get).Methods(http.MethodGet)
	r.HandleFunc("/login", c.Post).Methods(http.MethodPost)
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
	cookie, err := c.app.AuthService.CookieAuthenticate(r.Context(), email, password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/", http.StatusFound)
}
