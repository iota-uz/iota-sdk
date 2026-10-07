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
	tenants  *TenantService
	pool     *pgxpool.Pool
	logger   *logrus.Logger
}

func NewNotificationDispatchWorker(dispatch *NotificationDispatchService, tenants *TenantService, pool *pgxpool.Pool, logger *logrus.Logger) *NotificationDispatchWorker {
	return &NotificationDispatchWorker{dispatch: dispatch, tenants: tenants, pool: pool, logger: logger}
}
func (w *NotificationDispatchWorker) Start(parent context.Context) (composition.StopFn, error) {
	ctx, cancel := context.WithCancel(context.WithoutCancel(parent))
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			w.tick(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return func(stopCtx context.Context) error {
		cancel()
		select {
		case <-done:
			return nil
		case <-stopCtx.Done():
			return stopCtx.Err()
		}
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
