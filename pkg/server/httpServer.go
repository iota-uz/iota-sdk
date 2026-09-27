// Package server provides this package.
package server

import (
	"errors"
	"net/http"

	"github.com/NYTimes/gziphandler"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

func NewHTTPServer(
	app application.Application,
	notFoundHandler, methodNotAllowedHandler http.Handler,
) *HTTPServer {
	return &HTTPServer{
		Application:             app,
		NotFoundHandler:         notFoundHandler,
		MethodNotAllowedHandler: methodNotAllowedHandler,
	}
}

type HTTPServer struct {
	Application             application.Application
	NotFoundHandler         http.Handler
	MethodNotAllowedHandler http.Handler
	httpServer              *http.Server
}

func (s *HTTPServer) Start(socketAddress string) error {
	s.httpServer = &http.Server{
		Addr:    socketAddress,
		Handler: s.handler(),
	}
	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *HTTPServer) handler() http.Handler {
	r := mux.NewRouter()
	middlewares := s.Application.Middleware()
	r.Use(middlewares...)
	for _, controller := range s.Application.Controllers() {
		controller.Register(r)
	}

	notFoundHandler := s.NotFoundHandler
	notAllowedHandler := s.MethodNotAllowedHandler
	for i := len(middlewares) - 1; i >= 0; i-- {
		notFoundHandler = middlewares[i](notFoundHandler)
		notAllowedHandler = middlewares[i](notAllowedHandler)
	}
	r.NotFoundHandler = notFoundHandler
	r.MethodNotAllowedHandler = notAllowedHandler
	return gziphandler.GzipHandler(r)
}
