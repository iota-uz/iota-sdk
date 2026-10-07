// Package redisfanout implements Redis Pub/Sub realtime transport.
package redisfanout

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/realtime"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

const Topic = "iota:realtime:v1"

type Backend struct {
	client        *redis.Client
	logger        *logrus.Logger
	mu            sync.Mutex
	subscriptions []*redis.PubSub
	once          sync.Once
	closeErr      error
	closed        bool
	cancels       []context.CancelFunc
	reconnects    atomic.Uint64
	oversize      atomic.Uint64
	failed        atomic.Uint64
}

var _ realtime.Backend = (*Backend)(nil)

func New(ctx context.Context, url string, logger *logrus.Logger) (*Backend, error) {
	if logger == nil {
		logger = logrus.New()
	}
	if !strings.Contains(url, "://") {
		url = "redis://" + url
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = 5 * time.Second
	opts.WriteTimeout = 5 * time.Second
	client := redis.NewClient(opts)
	if err = client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &Backend{client: client, logger: logger}, nil
}
func (b *Backend) Publish(ctx context.Context, e realtime.Envelope) error {
	if err := e.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if len(data) > realtime.MaxPayloadBytes {
		return realtime.ErrOversize
	}
	return b.client.Publish(ctx, Topic, data).Err()
}
func (b *Backend) Subscribe(ctx context.Context, h realtime.Handler) error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return errors.New("realtime backend closed")
	}
	ctx, cancel := context.WithCancel(ctx)
	b.cancels = append(b.cancels, cancel)
	b.mu.Unlock()
	sub := b.client.Subscribe(ctx, Topic)
	if _, err := sub.Receive(ctx); err != nil {
		_ = sub.Close()
		return err
	}
	b.mu.Lock()
	b.subscriptions = append(b.subscriptions, sub)
	b.mu.Unlock()
	go func() {
		defer func() { _ = sub.Close() }()
		for {
			msg, err := sub.ReceiveMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				b.reconnects.Add(1)
				b.logger.WithError(err).Warn("realtime subscriber reconnect")
				select {
				case <-ctx.Done():
					return
				case <-time.After(100 * time.Millisecond):
				}
				continue
			}
			if len(msg.Payload) > realtime.MaxPayloadBytes {
				b.oversize.Add(1)
				b.logger.Warn("realtime oversize envelope rejected")
				continue
			}
			var e realtime.Envelope
			if err := json.Unmarshal([]byte(msg.Payload), &e); err != nil {
				b.logger.WithError(err).Warn("realtime invalid envelope")
				continue
			}
			if err := e.Validate(); err != nil {
				b.logger.WithError(err).Warn("realtime invalid envelope")
				continue
			}
			if err := h(ctx, e); err != nil {
				b.failed.Add(1)
				b.logger.WithError(err).Warn("realtime delivery failed")
			}
		}
	}()
	return nil
}
func (b *Backend) Close() error {
	b.once.Do(func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.closed = true
		for _, cancel := range b.cancels {
			cancel()
		}
		for _, sub := range b.subscriptions {
			_ = sub.Close()
		}
		b.closeErr = b.client.Close()
	})
	return b.closeErr
}

func (b *Backend) Counters() realtime.Counters {
	return realtime.Counters{Reconnect: b.reconnects.Load(), Oversize: b.oversize.Load(), DeliveryFailure: b.failed.Load()}
}
