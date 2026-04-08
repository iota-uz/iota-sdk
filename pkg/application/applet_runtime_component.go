package application

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

type appletRuntimeComponent struct {
	manager         *appletengineruntime.Manager
	pool            *pgxpool.Pool
	logger          *logrus.Logger
	hasPostgresJobs bool
	startedJobs     atomic.Bool
}

func newAppletRuntimeComponent(
	manager *appletengineruntime.Manager,
	pool *pgxpool.Pool,
	logger *logrus.Logger,
	hasPostgresJobs bool,
) RuntimeComponent {
	return &appletRuntimeComponent{
		manager:         manager,
		pool:            pool,
		logger:          logger,
		hasPostgresJobs: hasPostgresJobs,
	}
}

func (c *appletRuntimeComponent) Name() string {
	return "applet-runtime"
}

func (c *appletRuntimeComponent) Start(ctx context.Context) error {
	const op serrors.Op = "application.appletRuntimeComponent.Start"

	if c.manager == nil || !c.hasPostgresJobs || c.startedJobs.Load() {
		return nil
	}
	if c.pool == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	runner, err := appletenginejobs.NewRunner(c.pool, c.manager, c.logger, 2*time.Second)
	if err != nil {
		return serrors.E(op, err)
	}
	if !c.startedJobs.CompareAndSwap(false, true) {
		return nil
	}
	jobCtx, jobCancel := context.WithCancel(context.Background())
	c.manager.SetJobCancel(jobCancel)
	go runner.Start(jobCtx)
	return nil
}

func (c *appletRuntimeComponent) Stop(ctx context.Context) error {
	if c.manager == nil {
		return nil
	}
	if err := c.manager.Shutdown(ctx); err != nil {
		return err
	}
	c.manager.SetJobCancel(nil)
	c.startedJobs.Store(false)
	return nil
}
