package server

import (
	"net/http"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/ws"
)

const (
	ChannelChat string = "chat"
)

var hub = sync.OnceValue(func() *ws.Hub {
	return ws.NewHub(&ws.HubOptions{
		Logger: configuration.Use().Logger(),
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	})
})

func WsHub() *ws.Hub {
	return hub()
}
