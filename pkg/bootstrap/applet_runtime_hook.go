package bootstrap

import (
	"context"
	"sync/atomic"
	"time"

	appletenginejobs "github.com/iota-uz/iota-sdk/pkg/appletengine/jobs"
	appletengineruntime "github.com/iota-uz/iota-sdk/pkg/appletengine/runtime"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

type appletRuntimeHook struct {
	manager         *appletengineruntime.Manager
	pool            *pgxpool.Pool
	logger          *logrus.Logger
	hasPostgresJobs bool
	startedJobs     atomic.Bool
}

func (h *appletRuntimeHook) Name() string {
	return "applet-runtime"
}

func (h *appletRuntimeHook) Start(ctx context.Context) error {
	const op serrors.Op = "bootstrap.appletRuntimeHook.Start"

	if h.manager == nil || !h.hasPostgresJobs || h.startedJobs.Load() {
		return nil
	}
	if h.pool == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	runner, err := appletenginejobs.NewRunner(h.pool, h.manager, h.logger, 2*time.Second)
	if err != nil {
		return serrors.E(op, err)
	}
	if !h.startedJobs.CompareAndSwap(false, true) {
		return nil
	}
	jobCtx, jobCancel := context.WithCancel(context.Background())
	h.manager.SetJobCancel(jobCancel)
	go runner.Start(jobCtx)
	return nil
}

func (h *appletRuntimeHook) Stop(ctx context.Context) error {
	if h.manager == nil {
		return nil
	}
	if err := h.manager.Shutdown(ctx); err != nil {
		return err
	}
	h.manager.SetJobCancel(nil)
	h.startedJobs.Store(false)
	return nil
}
