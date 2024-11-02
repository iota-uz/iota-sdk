package server

import (
	"github.com/NYTimes/gziphandler"
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/presentation/controllers"
)

type HttpServer struct {
	Controllers []shared.Controller
	Middlewares []mux.MiddlewareFunc
}

func (s *HttpServer) Start(socketAddress string) error {
	r := mux.NewRouter()
	r.Use(s.Middlewares...)
	for _, controller := range s.Controllers {
		controller.Register(r)
	}
	var notFoundHandler http.Handler = controllers.NotFound()
	var notAllowedHandler http.Handler = controllers.MethodNotAllowed()
	for i := len(s.Middlewares) - 1; i >= 0; i-- {
		notFoundHandler = s.Middlewares[i](notFoundHandler)
		notAllowedHandler = s.Middlewares[i](notAllowedHandler)
	}
	r.NotFoundHandler = notFoundHandler
	r.MethodNotAllowedHandler = notAllowedHandler
	return http.ListenAndServe(socketAddress, gziphandler.GzipHandler(r))
}
