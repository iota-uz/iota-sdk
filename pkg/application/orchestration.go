package application

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
)

func Wire(app Application, modules ...Module) error {
	for _, module := range modules {
		if err := module.RegisterWiring(app); err != nil {
			return fmt.Errorf("%s wiring: %w", module.Name(), err)
		}
	}
	return nil
}

func RegisterTransports(app Application, modules ...Module) error {
	for _, module := range modules {
		if err := module.RegisterTransports(app); err != nil {
			return fmt.Errorf("%s transports: %w", module.Name(), err)
		}
	}
	return nil
}

// ApplyProfile wires modules first, adds transports when the profile exposes APIs,
// and starts registered runtime components only for long-running profiles.
func ApplyProfile(ctx context.Context, app Application, profile CompositionProfile, modules ...Module) error {
	normalizedProfile, err := normalizeCompositionProfile(profile)
	if err != nil {
		return err
	}
	if err := Wire(app, modules...); err != nil {
		return err
	}
	if normalizedProfile.IncludesTransports() {
		if err := RegisterTransports(app, modules...); err != nil {
			return err
		}
	}
	if normalizedProfile.StartsRuntime() {
		if err := app.StartRuntime(ctx, normalizedProfile); err != nil {
			return err
		}
	}
	return nil
}

type runtimeComponent struct {
	name  string
	start func(ctx context.Context) error
	stop  func(ctx context.Context) error
}

func (c *runtimeComponent) Name() string {
	return c.name
}

func (c *runtimeComponent) Start(ctx context.Context) error {
	if c.start == nil {
		return nil
	}
	return c.start(ctx)
}

func (c *runtimeComponent) Stop(ctx context.Context) error {
	if c.stop == nil {
		return nil
	}
	return c.stop(ctx)
}

func newSpotlightRuntimeComponent(cfg *configuration.Configuration, service spotlight.Service) RuntimeComponent {
	return &runtimeComponent{
		name: "spotlight",
		start: func(ctx context.Context) error {
			if cfg.MeiliURL != "" {
				if err := service.Readiness(ctx); err != nil {
					return fmt.Errorf("spotlight preflight check: %w", err)
				}
			}
			if err := service.Start(ctx); err != nil {
				return fmt.Errorf("start spotlight service: %w", err)
			}
			return nil
		},
		stop: func(ctx context.Context) error {
			return service.Stop(ctx)
		},
	}
}
