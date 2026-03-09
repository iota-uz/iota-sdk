// Package controllers provides this package.
package controllers

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
)

// LoginPageRenderer renders a custom login page component for the provided view model.
type LoginPageRenderer func(ctx context.Context, vm LoginPageViewModel) templ.Component

// LoginMethodProvider extends login with additional methods (e.g. external identity providers).
type LoginMethodProvider interface {
	ID() string
	RegisterRoutes(r *mux.Router, c *LoginController)
	BuildMethod(ctx context.Context, r *http.Request) (*LoginMethod, error)
}

// LoginMethod describes a single login method shown on the login page.
type LoginMethod struct {
	ID         string
	Label      string
	Href       string
	Variant    string
	Icon       templ.Component
	Attributes templ.Attributes
}

// LoginPageViewModel contains data required to render a login page.
type LoginPageViewModel struct {
	ErrorsMap    map[string]string
	ErrorMessage string
	Email        string
	Methods      []LoginMethod
	Logo         templ.Component
}
