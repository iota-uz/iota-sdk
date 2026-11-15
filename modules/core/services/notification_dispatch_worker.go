package services

import (
	"context"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

type NotificationDispatchWorker struct {
	dispatch *NotificationDispatchService
	channels *NotificationChannelService
	tenants  *TenantService
	pool     *pgxpool.Pool
	logger   *logrus.Logger
}

func NewNotificationDispatchWorker(dispatch *NotificationDispatchService, channels *NotificationChannelService, tenants *TenantService, pool *pgxpool.Pool, logger *logrus.Logger) *NotificationDispatchWorker {
	return &NotificationDispatchWorker{dispatch: dispatch, channels: channels, tenants: tenants, pool: pool, logger: logger}
}
func (w *NotificationDispatchWorker) Start(parent context.Context) (composition.StopFn, error) {
	ctx, cancel := context.WithCancel(context.WithoutCancel(parent))
	loop := func(tick func(context.Context)) <-chan struct{} {
		done := make(chan struct{})
		go func() {
			defer close(done)
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				tick(ctx)
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
		return done
	}
	// Slow providers must not delay in-app dispatch, so channels get their own loop.
	dispatchDone := loop(w.tick)
	channelsDone := loop(w.tickChannels)
	return func(stopCtx context.Context) error {
		cancel()
		for _, done := range []<-chan struct{}{dispatchDone, channelsDone} {
			select {
			case <-done:
			case <-stopCtx.Done():
				return stopCtx.Err()
			}
		}
		return nil
	}, nil
}
func (w *NotificationDispatchWorker) tick(ctx context.Context) {
	base := composables.WithPool(ctx, w.pool)
	tenants, err := w.tenants.List(base)
	if err != nil {
		if ctx.Err() == nil {
			w.logger.WithError(err).Error("failed to enumerate notification dispatch tenants")
		}
		return
	}
	for _, tenant := range tenants {
		if !tenant.IsActive() {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		batchCtx, cancel := context.WithTimeout(composables.WithTenantID(base, tenant.ID()), 30*time.Second)
		_, err := w.dispatch.Process(batchCtx)
		cancel()
		if err != nil && ctx.Err() == nil {
			w.logger.WithError(err).WithField("tenant_id", tenant.ID()).Error("notification delivery batch failed; durable retry scheduled")
		}
	}
}

func (w *NotificationDispatchWorker) tickChannels(ctx context.Context) {
	if w.channels == nil {
		return
	}
	base := composables.WithPool(ctx, w.pool)
	tenants, err := w.tenants.List(base)
	if err != nil {
		if ctx.Err() == nil {
			w.logger.WithError(err).Error("failed to enumerate notification channel tenants")
		}
		return
	}
	for _, tenant := range tenants {
		if !tenant.IsActive() {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		channelCtx, cancel := context.WithTimeout(composables.WithTenantID(base, tenant.ID()), 2*time.Minute)
		_, err := w.channels.Process(channelCtx)
		cancel()
		if err != nil && ctx.Err() == nil {
			w.logger.WithError(err).WithField("tenant_id", tenant.ID()).Error("notification channel delivery failed; retry scheduled")
		}
	}
}
