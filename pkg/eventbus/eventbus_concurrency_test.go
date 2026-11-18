package eventbus

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type piiEvent struct {
	PINFL    string
	Passport string
	Phone    string
}

func newBufferedPublisher(t *testing.T, level logrus.Level) (EventBus, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	log := logrus.New()
	log.SetOutput(&buf)
	log.SetLevel(level)
	return NewEventPublisher(log), &buf
}

// TestPublisher_ConcurrentSubscribePublish must pass under `go test -race`.
func TestPublisher_ConcurrentSubscribePublish(t *testing.T) {
	t.Parallel()

	publisher, _ := newBufferedPublisher(t, logrus.PanicLevel)
	const workers = 8
	const iterations = 200

	var wg sync.WaitGroup
	for range workers {
		wg.Add(3)
		go func() {
			defer wg.Done()
			for range iterations {
				unsubscribe := publisher.Subscribe(func(*args) {})
				unsubscribe()
			}
		}()
		go func() {
			defer wg.Done()
			for range iterations {
				publisher.Publish(&args{data: "x"})
				_ = PublishCtx(context.Background(), publisher, &args{data: "y"})
			}
		}()
		go func() {
			defer wg.Done()
			for range iterations {
				_ = publisher.SubscribersCount()
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range iterations {
			publisher.Clear()
		}
	}()
	wg.Wait()
}

func TestPublisher_HandlerCanSubscribeAndUnsubscribe(t *testing.T) {
	t.Parallel()

	publisher, _ := newBufferedPublisher(t, logrus.PanicLevel)
	var unsubscribe func()
	unsubscribe = publisher.Subscribe(func(*args) {
		publisher.Subscribe(func(*args) {})
		unsubscribe()
	})

	publisher.Publish(&args{})
	assert.Equal(t, 1, publisher.SubscribersCount())
}

func TestPublisher_LogsNoPayload(t *testing.T) {
	t.Parallel()

	event := &piiEvent{PINFL: "31234567890123", Passport: "AB1234567", Phone: "+998901234567"}
	secrets := []string{event.PINFL, event.Passport, event.Phone}

	t.Run("no subscribers", func(t *testing.T) {
		publisher, buf := newBufferedPublisher(t, logrus.DebugLevel)
		publisher.Publish(event)
		out := buf.String()
		assert.Contains(t, out, "no matching subscribers")
		assert.Contains(t, out, "*eventbus.piiEvent")
		for _, s := range secrets {
			assert.NotContains(t, out, s)
		}
	})

	t.Run("handler error", func(t *testing.T) {
		publisher, buf := newBufferedPublisher(t, logrus.DebugLevel)
		publisher.Subscribe(func(*piiEvent) error { return errors.New("boom") })
		err := PublishCtx(context.Background(), publisher, event)
		require.Error(t, err)
		out := buf.String() + err.Error()
		for _, s := range secrets {
			assert.NotContains(t, out, s)
		}
	})
}

func TestPublishCtx(t *testing.T) {
	t.Parallel()

	t.Run("returns handler errors and panics, runs all handlers", func(t *testing.T) {
		publisher, _ := newBufferedPublisher(t, logrus.PanicLevel)
		sentinel := errors.New("handler failed")
		var calls []string
		publisher.Subscribe(func(*args) error { calls = append(calls, "err"); return sentinel })
		publisher.Subscribe(func(*args) { calls = append(calls, "panic"); panic("kaboom") })
		publisher.Subscribe(func(*args) error { calls = append(calls, "ok"); return nil })

		err := PublishCtx(context.Background(), publisher, &args{})
		require.Error(t, err)
		require.ErrorIs(t, err, sentinel)
		assert.Equal(t, []string{"err", "panic", "ok"}, calls)

		var herr *HandlerError
		require.ErrorAs(t, err, &herr)
		assert.Equal(t, "*eventbus.args", herr.Event)
		assert.Contains(t, err.Error(), "kaboom")
	})

	t.Run("nil when all handlers succeed or none match", func(t *testing.T) {
		publisher, _ := newBufferedPublisher(t, logrus.PanicLevel)
		publisher.Subscribe(func(*args) error { return nil })
		require.NoError(t, PublishCtx(context.Background(), publisher, &args{}))
		require.NoError(t, PublishCtx(context.Background(), publisher, "unmatched"))
	})

	t.Run("cancelled context skips handlers", func(t *testing.T) {
		publisher, _ := newBufferedPublisher(t, logrus.PanicLevel)
		called := false
		publisher.Subscribe(func(*args) { called = true })
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := PublishCtx(ctx, publisher, &args{})
		require.ErrorIs(t, err, context.Canceled)
		assert.False(t, called)
	})

	t.Run("cancellation between handlers stops delivery", func(t *testing.T) {
		publisher, _ := newBufferedPublisher(t, logrus.PanicLevel)
		ctx, cancel := context.WithCancel(context.Background())
		second := false
		publisher.Subscribe(func(*args) { cancel() })
		publisher.Subscribe(func(*args) { second = true })
		err := PublishCtx(ctx, publisher, &args{})
		require.ErrorIs(t, err, context.Canceled)
		assert.False(t, second)
	})

	t.Run("nil argument reaches pointer handler", func(t *testing.T) {
		publisher, _ := newBufferedPublisher(t, logrus.PanicLevel)
		got := &args{}
		publisher.Subscribe(func(e *args) { got = e })
		require.NoError(t, PublishCtx(context.Background(), publisher, nil))
		assert.Nil(t, got)
	})

	t.Run("falls back to Publish for foreign buses", func(t *testing.T) {
		bus := &legacyBus{}
		require.NoError(t, PublishCtx(context.Background(), bus, "e"))
		assert.Equal(t, 1, bus.published)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.ErrorIs(t, PublishCtx(ctx, bus, "e"), context.Canceled)
		assert.Equal(t, 1, bus.published)
	})
}

type legacyBus struct{ published int }

func (b *legacyBus) Publish(...interface{})       { b.published++ }
func (b *legacyBus) Subscribe(interface{}) func() { return func() {} }
func (b *legacyBus) Clear()                       {}
func (b *legacyBus) SubscribersCount() int        { return 0 }

func TestHandlerError_MessageHasNoValues(t *testing.T) {
	t.Parallel()
	err := &HandlerError{Handler: "func(*x.E)", Event: "*x.E", Err: errors.New("e")}
	assert.True(t, strings.HasPrefix(err.Error(), "eventbus: handler func(*x.E) failed"))
}
