package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/telemetryconfig"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/health"
	"github.com/iota-uz/iota-sdk/pkg/logging"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

// IotaSource is the "new world" entrypoint — all factories derive from a
// config.Source only, without ever touching *configuration.Configuration.
// Use this when the application has fully migrated off the legacy type.
//
// It calls WithSource(src) internally so the Source is attached to the
// Runtime and available to components via the composition BuildContext.
//
// Parallel to IotaConfig; both options coexist during the transition period.
func IotaSource(src config.Source) Option {
	return IotaSourceWithServiceName(src, "")
}

// IotaSourceWithServiceName is like IotaSource but lets the caller override
// the OpenTelemetry service name. When serviceName is empty the value from the
// telemetry source ("telemetry.otel.servicename") is used.
func IotaSourceWithServiceName(src config.Source, serviceName string) Option {
	return func(o *options) {
		// Attach Source to Runtime so components can use ProvideConfig[T].
		o.source = src

		// Ensure a shared CapabilityRegistry exists so telemetry sub-features
		// surface on /system/info alongside module-level gates. Callers that
		// attach their own via bootstrap.WithCapabilityRegistry will have set
		// o.capabilityRegistry already — respect that and don't overwrite.
		if o.capabilityRegistry == nil {
			o.capabilityRegistry = health.NewCapabilityRegistry()
		}
		capReg := o.capabilityRegistry

		reg := config.NewRegistry(src)

		// Eagerly load telemetry config; errors surface immediately at bootstrap
		// before any side-effects via poisoned factories.
		telCfg, telErr := config.Register[telemetryconfig.Config](reg)

		effectiveServiceName := serviceName
		if telErr == nil && effectiveServiceName == "" {
			effectiveServiceName = telCfg.OTEL.ServiceName
		}

		o.loggerFactory = func(_ context.Context, _ any) (*logrus.Logger, func() error, error) {
			if telErr != nil {
				return nil, nil, fmt.Errorf("bootstrap: load telemetryconfig from source: %w", telErr)
			}

			lokiPath := telCfg.Loki.LogPath
			if lokiPath == "" {
				lokiPath = "./logs/app.log"
			}

			_, logger, err := logging.FileLogger(telCfg.LogrusLogLevel(), lokiPath)
			if err != nil {
				return nil, nil, fmt.Errorf("bootstrap: create file logger: %w", err)
			}

			if telCfg.Loki.IsConfigured() {
				appName := telCfg.Loki.AppName
				if appName == "" {
					appName = "sdk"
				}
				if err := logging.AddLokiHook(logger, telCfg.Loki.URL, appName); err != nil {
					return nil, nil, fmt.Errorf("bootstrap: add Loki hook: %w", err)
				}
				registerLokiCapability(capReg, health.StatusHealthy, "")
			} else {
				registerLokiCapability(capReg, health.StatusDisabled, telCfg.Loki.DisabledReason())
			}

			if !telCfg.OTEL.IsConfigured() {
				registerOTELCapability(capReg, health.StatusDisabled, "TELEMETRY_OTEL_TEMPOURL and TELEMETRY_OTEL_SERVICENAME required")
				return logger, func() error { return nil }, nil
			}

			cleanup := logging.SetupTracing(
				context.Background(),
				effectiveServiceName,
				telCfg.OTEL.TempoURL,
			)
			logger.Info("OpenTelemetry tracing enabled, exporting to Tempo at " + telCfg.OTEL.TempoURL)
			registerOTELCapability(capReg, health.StatusHealthy, "")
			return logger, func() error {
				cleanup()
				return nil
			}, nil
		}

		o.poolFactory = func(ctx context.Context, _ any, _ *logrus.Logger) (*pgxpool.Pool, func() error, error) {
			poolCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()

			dbCfg, err := config.Register[dbconfig.Config](reg)
			if err != nil {
				return nil, nil, fmt.Errorf("bootstrap: load dbconfig from source: %w", err)
			}

			poolCfg, err := dbCfg.PoolConfig()
			if err != nil {
				return nil, nil, fmt.Errorf("bootstrap: build pgxpool config: %w", err)
			}

			pool, err := pgxpool.NewWithConfig(poolCtx, poolCfg)
			if err != nil {
				return nil, nil, err
			}
			return pool, func() error {
				pool.Close()
				return nil
			}, nil
		}

		o.bundleFactory = func(context.Context, any) (*i18n.Bundle, error) {
			return application.LoadBundle(), nil
		}

		o.appFactory = func(_ context.Context, rt *Runtime) (application.Application, error) {
			// Read the allowed origin for WebSocket / CORS CheckOrigin from the source.
			var allowedOrigin string
			if _, hasOrigin := src.Get("http.origin"); hasOrigin {
				type originOnly struct {
					Origin string `koanf:"origin"`
				}
				var oo originOnly
				if err := src.Unmarshal("http", &oo); err == nil {
					allowedOrigin = strings.TrimSpace(oo.Origin)
				}
			}

			return application.New(&application.ApplicationOptions{
				Pool:               rt.Pool,
				Bundle:             rt.Bundle,
				EventBus:           eventbus.NewEventPublisher(rt.Logger),
				Logger:             rt.Logger,
				SupportedLanguages: application.DefaultSupportedLanguages(),
				Huber: application.NewHub(&application.HuberOptions{
					Pool:           rt.Pool,
					Logger:         rt.Logger,
					Bundle:         rt.Bundle,
					UserRepository: persistence.NewUserRepository(persistence.NewUploadRepository()),
					CheckOrigin: func(r *http.Request) bool {
						requestOrigin := strings.TrimSpace(r.Header.Get("Origin"))
						if requestOrigin == "" {
							return true
						}
						if allowedOrigin == "" {
							return true
						}
						return requestOrigin == allowedOrigin
					},
				}),
			})
		}
	}
}

// registerLokiCapability emits a one-shot Loki hook capability probe so
// /system/info reflects whether log shipping to Loki is active.
func registerLokiCapability(capReg health.CapabilityRegistry, status health.Status, message string) {
	if capReg == nil {
		return
	}
	capReg.Register(health.CapabilityProbeFunc(func(context.Context) health.Capability {
		return health.Capability{
			Key:     "telemetry.loki",
			Name:    "telemetry.loki",
			Enabled: status == health.StatusHealthy,
			Status:  status,
			Message: message,
		}
	}))
}

// registerOTELCapability emits an OTEL tracing capability probe so the same
// /system/info panel covers Tempo alongside Loki.
func registerOTELCapability(capReg health.CapabilityRegistry, status health.Status, message string) {
	if capReg == nil {
		return
	}
	capReg.Register(health.CapabilityProbeFunc(func(context.Context) health.Capability {
		return health.Capability{
			Key:     "telemetry.otel",
			Name:    "telemetry.otel",
			Enabled: status == health.StatusHealthy,
			Status:  status,
			Message: message,
		}
	}))
}
