package application

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/realtime"
	"github.com/iota-uz/iota-sdk/pkg/realtime/redisfanout"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestRedisHubTargetsAllTabsAndDeduplicates(t *testing.T) {
	redis := miniredis.RunT(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tenant := uuid.New()
	makeHub := func() Huber {
		b, err := redisfanout.New(ctx, "redis://"+redis.Addr(), logrus.New())
		require.NoError(t, err)
		h := NewHub(&HuberOptions{Backend: b})
		require.NoError(t, h.Start(ctx))
		t.Cleanup(func() { require.NoError(t, h.Close()) })
		return h
	}
	a, b := makeHub(), makeHub()
	connect := func(h Huber, tenantID uuid.UUID, id uint) *websocket.Conn {
		email, err := internet.NewEmail("test@example.com")
		require.NoError(t, err)
		usr := user.New("Test", "User", email, user.UILanguage("en"), user.WithID(id), user.WithTenantID(tenantID))
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r.WithContext(composables.WithUser(r.Context(), usr)))
		}))
		t.Cleanup(srv.Close)
		conn, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
		if response != nil {
			require.NoError(t, response.Body.Close())
		}
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	}
	tabs := []*websocket.Conn{connect(a, tenant, 7), connect(a, tenant, 7), connect(b, tenant, 7)}
	others := []*websocket.Conn{connect(a, tenant, 8), connect(b, uuid.New(), 7)}
	e := realtime.Envelope{ID: uuid.New(), TenantID: tenant, Channel: realtime.UserChannel(tenant, 7), PublishedAt: time.Now(), Payload: []byte("hello")}
	require.NoError(t, b.Publish(ctx, e))
	require.NoError(t, b.Publish(ctx, e))
	for _, conn := range tabs {
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
		_, data, err := conn.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, "hello", string(data))
	}
	require.Eventually(t, func() bool { return a.Counters().Duplicate == 1 && b.Counters().Duplicate == 1 }, time.Second, time.Millisecond*5)
	for _, conn := range append(tabs, others...) {
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(50*time.Millisecond)))
		_, _, err := conn.ReadMessage()
		require.Error(t, err)
	}
}

type alternateFanout struct{ handler realtime.Handler }

func (b *alternateFanout) Publish(ctx context.Context, e realtime.Envelope) error {
	return b.handler(ctx, e)
}
func (b *alternateFanout) Subscribe(_ context.Context, h realtime.Handler) error {
	b.handler = h
	return nil
}
func (b *alternateFanout) Close() error { return nil }

var _ realtime.Backend = (*alternateFanout)(nil)

func TestHubAcceptsAlternateBackend(t *testing.T) {
	b := &alternateFanout{}
	h := NewHub(&HuberOptions{Backend: b})
	ctx := context.Background()
	require.NoError(t, h.Start(ctx))
	defer func() { require.NoError(t, h.Close()) }()
	tenant := uuid.New()
	require.NoError(t, h.Publish(ctx, realtime.Envelope{ID: uuid.New(), TenantID: tenant, Channel: realtime.UserChannel(tenant, 1), PublishedAt: time.Now()}))
	require.EqualValues(t, 1, h.Counters().Received)
}
