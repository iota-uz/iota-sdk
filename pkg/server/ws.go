package server

import (
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/ws"
)

const (
	ChannelChat string = "chat"
)

var hub = sync.OnceValue(func() *ws.Hub {
	return ws.NewHub()
})

func WsHub() *ws.Hub {
	return hub()
}
