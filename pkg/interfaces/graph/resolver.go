package graph

import (
	"github.com/iota-agency/iota-sdk/pkg/application"
	"net/http"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/gorilla/websocket"
)

//go:generate go run github.com/99designs/gqlgen generate

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	app *application.Application
}

func NewDefaultServer(app *application.Application) *handler.Server {
	srv := handler.New(NewExecutableSchema(
		Config{ //nolint:exhaustruct
			Resolvers: &Resolver{
				app: app,
			},
		},
	))
	srv.AddTransport(transport.Websocket{ //nolint:exhaustruct
		KeepAlivePingInterval: 10 * time.Second,
		Upgrader: websocket.Upgrader{ //nolint:exhaustruct
			// TODO: Add origin check
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		},
	})
	srv.AddTransport(transport.Options{})       //nolint:exhaustruct
	srv.AddTransport(transport.GET{})           //nolint:exhaustruct
	srv.AddTransport(transport.POST{})          //nolint:exhaustruct
	srv.AddTransport(transport.MultipartForm{}) //nolint:exhaustruct

	// TODO: make LRU work
	// srv.SetQueryCache(lru.New(1000))

	srv.Use(extension.Introspection{})
	// srv.Use(extension.AutomaticPersistedQuery{
	//	Cache: lru.New(100),
	// })
	return srv
}
