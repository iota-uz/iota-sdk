package controllers

import (
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
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
	r.Handle("/ws", c.app.Websocket())
}
