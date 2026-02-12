package controllers

import (
	"net/http"

	"github.com/gorilla/mux"
	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
)

type RPCController struct {
	dispatcher *appletenginerpc.Dispatcher
}

func NewRPCController(dispatcher *appletenginerpc.Dispatcher) *RPCController {
	return &RPCController{dispatcher: dispatcher}
}

func (c *RPCController) Register(r *mux.Router) {
	r.HandleFunc("/rpc", func(w http.ResponseWriter, req *http.Request) {
		c.dispatcher.HandlePublicHTTP(w, req)
	}).Methods(http.MethodPost)
}

func (c *RPCController) Key() string {
	return "applet_rpc"
}
