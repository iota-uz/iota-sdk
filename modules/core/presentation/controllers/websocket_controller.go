package controllers

import (
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

func NewWebSocketController(app application.Application) application.Controller {
	return &WebSocketController{
		app: app,
	}
}

type WebSocketController struct {
	app application.Application
}

func (c *WebSocketController) Key() string {
	return "/ws"
}

func (c *WebSocketController) Register(r *mux.Router) {
	router := r.PathPrefix("/ws").Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.ProvideUser(),
	)

	router.Handle("", c.app.Websocket())
}
