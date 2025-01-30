package controllers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/modules/crm/presentation/templates/pages/chats"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type ChatController struct {
	app      application.Application
	basePath string
}

func NewChatController(app application.Application) application.Controller {
	return &ChatController{
		app:      app,
		basePath: "/crm/chats",
	}
}

func (c *ChatController) Key() string {
	return c.basePath
}

func (c *ChatController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}
	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)
	getRouter.HandleFunc("", c.List).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())
}

func (c *ChatController) List(w http.ResponseWriter, r *http.Request) {
	props := &chats.IndexPageProps{}
	templ.Handler(chats.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
}
