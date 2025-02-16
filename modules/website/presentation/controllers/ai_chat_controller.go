package controllers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/templates/pages/aichat"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

func NewAIChatController(app application.Application) application.Controller {
	return &AIChatController{
		basePath: "/website/ai-chat",
		app:      app,
	}
}

type AIChatController struct {
	basePath string
	app      application.Application
}

func (c *AIChatController) Key() string {
	return "AiChatController"
}

func (c *AIChatController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.WithPageContext(),
		middleware.Tabs(),
		middleware.NavItems(),
	)
	router.HandleFunc("", c.configureAIChat).Methods(http.MethodGet)

	bareRouter := r.PathPrefix(c.basePath).Subrouter()
	bareRouter.HandleFunc("/payload", c.aiChat).Methods(http.MethodGet)
	bareRouter.HandleFunc("/test-wc", c.aiChatWC).Methods(http.MethodGet)
}

func (c *AIChatController) configureAIChat(w http.ResponseWriter, r *http.Request) {
	templ.Handler(aichat.Configure(aichat.Props{
		Title:       "AI Chatbot",
		Description: "Наш AI-бот готов помочь вам круглосуточно",
	})).ServeHTTP(w, r)
}

func (c *AIChatController) aiChat(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Query().Get("title")
	description := r.URL.Query().Get("description")
	templ.Handler(aichat.Chat(aichat.Props{
		Title:       title,
		Description: description,
	})).ServeHTTP(w, r)
}

func (c *AIChatController) aiChatWC(w http.ResponseWriter, r *http.Request) {
	templ.Handler(aichat.WebComponent()).ServeHTTP(w, r)
}
