package controllers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/templates/pages/aichat"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

func NewAIChatController(app application.Application) application.Controller {
	return &AIChatController{}
}

type AIChatController struct {
}

func (c *AIChatController) Key() string {
	return "AiChatController"
}

func (c *AIChatController) Register(r *mux.Router) {
	r.HandleFunc("/ai-chat", c.aiChat).Methods(http.MethodGet)
	r.HandleFunc("/ai-chat/test-wc", c.aiChatWC).Methods(http.MethodGet)
}

func (c *AIChatController) aiChat(w http.ResponseWriter, r *http.Request) {
	templ.Handler(aichat.Chat()).ServeHTTP(w, r)
}

func (c *AIChatController) aiChatWC(w http.ResponseWriter, r *http.Request) {
	templ.Handler(aichat.WebComponent()).ServeHTTP(w, r)
}
