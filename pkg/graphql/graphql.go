package graphql

import (
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/gorilla/websocket"
	"net/http"
	"time"
)

func NewDefaultGraphServer(schema graphql.ExecutableSchema) *handler.Server {
	srv := handler.New(schema)
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
