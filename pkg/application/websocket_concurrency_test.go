package application

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/ws"
)

func TestHuberOnConnectOnDisconnectConcurrent(t *testing.T) {
	h := &huber{
		connectionsMeta: make(map[*ws.Connection]*MetaInfo),
	}

	const (
		workers    = 48
		iterations = 500
	)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			<-start

			for j := 0; j < iterations; j++ {
				conn := &ws.Connection{}
				if err := h.onConnect(req, nil, conn); err != nil {
					t.Errorf("onConnect returned error: %v", err)
					return
				}

				meta, ok := h.getConnectionMeta(conn)
				if !ok || meta == nil {
					t.Errorf("expected connection meta to exist")
					return
				}

				h.onDisconnect(conn)
			}
		}()
	}

	close(start)
	wg.Wait()

	h.connectionsMetaMu.RLock()
	defer h.connectionsMetaMu.RUnlock()
	if got := len(h.connectionsMeta); got != 0 {
		t.Fatalf("expected all connection meta entries to be removed, got %d", got)
	}
}
