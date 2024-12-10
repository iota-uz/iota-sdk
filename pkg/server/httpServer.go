package server

import (
	"github.com/NYTimes/gziphandler"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"net/http"

	"github.com/gorilla/mux"
)

type HttpServer struct {
	Controllers             []application.Controller
	Middlewares             []mux.MiddlewareFunc
	NotFoundHandler         http.Handler
	MethodNotAllowedHandler http.Handler
}

func (s *HttpServer) Start(socketAddress string) error {
	r := mux.NewRouter()
	r.Use(s.Middlewares...)
	for _, controller := range s.Controllers {
		controller.Register(r)
	}

	var notFoundHandler = s.NotFoundHandler
	var notAllowedHandler = s.MethodNotAllowedHandler
	for i := len(s.Middlewares) - 1; i >= 0; i-- {
		notFoundHandler = s.Middlewares[i](notFoundHandler)
		notAllowedHandler = s.Middlewares[i](notAllowedHandler)
	}
	r.NotFoundHandler = notFoundHandler
	r.MethodNotAllowedHandler = notAllowedHandler
	return http.ListenAndServe(socketAddress, gziphandler.GzipHandler(r))
}
