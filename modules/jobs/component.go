// Package jobs provides this package.
package jobs

import (
	"context"
	"embed"

	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/jobs/domain/aggregates/job"
	"github.com/iota-uz/iota-sdk/modules/jobs/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/jobs/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/jobs/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

//go:embed presentation/locales/*.json
var localeFiles embed.FS

// ModuleOptions tunes the jobs module.
type ModuleOptions struct {
	// DisableWorker stops the embedded background worker from starting.
	// Deployments running a dedicated worker process set this on API instances.
	DisableWorker bool
	// Worker configures the embedded worker pool. Zero values fall back to
	// services defaults.
	Worker services.WorkerOptions
}

func NewComponent(opts ...ModuleOptions) composition.Component {
	options := ModuleOptions{}
	if len(opts) > 0 {
		options = opts[0]
	}
	return &component{options: options}
}

type component struct {
	options ModuleOptions
}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{
		Name:     "jobs",
		Requires: []string{"core"},
	}
}

func (c *component) LocaleFS() []*embed.FS {
	return []*embed.FS{&localeFiles}
}

func (c *component) Build(builder *composition.Builder) error {
	composition.Provide[*services.Registry](builder, services.NewRegistry)
	composition.ProvideFunc(builder, persistence.NewJobRepository)
	composition.ProvideFunc(builder, services.NewJobService)

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllersFunc(builder, func(
			jobService *services.JobService,
		) ([]application.Controller, error) {
			return []application.Controller{controllers.NewJobController()}, nil
		})
	}

	if !c.options.DisableWorker {
		composition.ContributeHooks(builder, func(container *composition.Container) ([]composition.Hook, error) {
			repo, err := composition.Resolve[job.Repository](container)
			if err != nil {
				return nil, err
			}
			registry, err := composition.Resolve[*services.Registry](container)
			if err != nil {
				return nil, err
			}
			uploads, err := composition.Resolve[*coreservices.UploadService](container)
			if err != nil {
				return nil, err
			}
			uploadsCfg, err := composition.Resolve[*uploadsconfig.Config](container)
			if err != nil {
				return nil, err
			}
			httpCfg, err := composition.Resolve[*httpconfig.Config](container)
			if err != nil {
				return nil, err
			}
			appCfg, err := composition.Resolve[*appconfig.Config](container)
			if err != nil {
				return nil, err
			}
			pool, err := composition.Resolve[*pgxpool.Pool](container)
			if err != nil {
				return nil, err
			}
			logger, err := composition.Resolve[*logrus.Logger](container)
			if err != nil {
				return nil, err
			}
			worker := services.NewWorker(pool, repo, registry, uploads, uploadsCfg, httpCfg, appCfg, logger, c.options.Worker)
			return []composition.Hook{{
				Name: "jobs-worker",
				Start: func(ctx context.Context) (composition.StopFn, error) {
					workerCtx, cancel := context.WithCancel(context.Background())
					done := make(chan struct{})
					go func() {
						defer close(done)
						if startErr := worker.Start(workerCtx); startErr != nil {
							logger.WithError(startErr).Warn("jobs worker stopped with error")
						}
					}()
					return func(stopCtx context.Context) error {
						cancel()
						select {
						case <-done:
						case <-stopCtx.Done():
							return stopCtx.Err()
						}
						return nil
					}, nil
				},
			}}, nil
		})
	}
	return nil
}
