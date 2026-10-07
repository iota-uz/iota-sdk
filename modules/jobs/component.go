// Package jobs provides this package.
package jobs

import (
	"context"
	"embed"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/modules/jobs/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/jobs"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/redis/go-redis/v9"
)

//go:embed presentation/locales/*.json
var localeFiles embed.FS

// ModuleOptions wires the jobs primitive into an application.
type ModuleOptions struct {
	// RedisURL points at the Redis instance backing job state. When empty,
	// an in-process store is used: fine for single-instance deployments,
	// but job progress is not shared across replicas and is lost on
	// restart.
	RedisURL string
	// Retention is how long terminal jobs and their stored results live.
	// Zero falls back to jobs.DefaultRetention.
	Retention time.Duration
	// Runner configures execution. Zero values fall back to jobs defaults.
	Runner jobs.RunnerOptions
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
	opts := c.options
	if opts.Retention <= 0 {
		opts.Retention = jobs.DefaultRetention
	}
	if opts.Runner.StaleAfter <= 0 {
		opts.Runner.StaleAfter = jobs.DefaultStaleAfter
	}

	store, err := newStore(opts)
	if err != nil {
		return err
	}
	registry := jobs.NewRegistry()
	runner := jobs.NewRunner(store, registry, opts.Runner)

	composition.Provide[*jobs.Registry](builder, func() *jobs.Registry { return registry })
	composition.Provide[*jobs.Runner](builder, func() *jobs.Runner { return runner })

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllersFunc(builder, func(
			runner *jobs.Runner,
		) ([]application.Controller, error) {
			return []application.Controller{controllers.NewJobController(runner)}, nil
		})
	}

	composition.ContributeHooks(builder, func(container *composition.Container) ([]composition.Hook, error) {
		return []composition.Hook{{
			Name: "jobs-runner",
			Start: func(ctx context.Context) (composition.StopFn, error) {
				return func(context.Context) error {
					runner.Wait()
					if closer, ok := store.(interface{ Close() error }); ok {
						return closer.Close()
					}
					return nil
				}, nil
			},
		}}, nil
	})
	return nil
}

// newStore builds the job store: Redis when a URL is configured, in-process
// memory otherwise.
func newStore(opts ModuleOptions) (jobs.Store, error) {
	const op serrors.Op = "jobs.newStore"
	url := strings.TrimSpace(opts.RedisURL)
	if url == "" {
		return jobs.NewMemoryStore(opts.Retention), nil
	}
	client, err := newRedisClient(url)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	store, err := jobs.NewRedisStore(client, opts.Retention, jobs.WithStaleAfter(opts.Runner.StaleAfter))
	if err != nil {
		_ = client.Close()
		return nil, serrors.E(op, err)
	}
	return store, nil
}

func newRedisClient(redisURL string) (*redis.Client, error) {
	const op serrors.Op = "jobs.newRedisClient"
	var options *redis.Options
	var err error
	if strings.Contains(redisURL, "://") {
		options, err = redis.ParseURL(redisURL)
	} else {
		options = &redis.Options{Addr: redisURL}
	}
	if err != nil {
		return nil, serrors.E(op, "parse redis url", err)
	}
	client := redis.NewClient(options)
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if pingErr := client.Ping(pingCtx).Err(); pingErr != nil {
		_ = client.Close()
		return nil, serrors.E(op, "ping redis", pingErr)
	}
	return client, nil
}
