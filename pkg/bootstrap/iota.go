package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/applets"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/logging"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
)

func IotaConfig(conf *configuration.Configuration) Option {
	return IotaConfigWithServiceName(conf, conf.OpenTelemetry.ServiceName)
}

func IotaConfigWithServiceName(conf *configuration.Configuration, serviceName string) Option {
	return func(o *options) {
		o.config = conf
		o.loggerFactory = func(_ context.Context, _ any) (*logrus.Logger, func() error, error) {
			logger := conf.Logger()
			if !conf.OpenTelemetry.IsConfigured() {
				return logger, func() error {
					conf.Unload()
					return nil
				}, nil
			}

			cleanup := logging.SetupTracing(
				context.Background(),
				serviceName,
				conf.OpenTelemetry.TempoURL,
			)
			logger.Info("OpenTelemetry tracing enabled, exporting to Tempo at " + conf.OpenTelemetry.TempoURL)
			return logger, func() error {
				cleanup()
				conf.Unload()
				return nil
			}, nil
		}
		o.poolFactory = func(ctx context.Context, _ any, _ *logrus.Logger) (*pgxpool.Pool, func() error, error) {
			poolCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			pool, err := pgxpool.New(poolCtx, conf.Database.Opts)
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
						return true
					},
				}),
			})
		}
	}
}

type sdkAppletUserAdapter struct {
	user user.User
}

func (a *sdkAppletUserAdapter) ID() uint {
	return a.user.ID()
}

func (a *sdkAppletUserAdapter) DisplayName() string {
	return strings.TrimSpace(a.user.FirstName() + " " + a.user.LastName())
}

func (a *sdkAppletUserAdapter) HasPermission(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	for _, permissionName := range composables.EffectivePermissionNames(a.user) {
		if strings.ToLower(permissionName) == name {
			return true
		}
	}
	return false
}

func (a *sdkAppletUserAdapter) PermissionNames() []string {
	return composables.EffectivePermissionNames(a.user)
}

type sdkHostServices struct {
	pool *pgxpool.Pool
}

func NewSDKHostServices(pool *pgxpool.Pool) applets.HostServices {
	return &sdkHostServices{pool: pool}
}

func (h *sdkHostServices) ExtractUser(ctx context.Context) (applets.AppletUser, error) {
	currentUser, err := composables.UseUser(ctx)
	if err != nil || currentUser == nil {
		return nil, err
	}
	return &sdkAppletUserAdapter{user: currentUser}, nil
}

func (h *sdkHostServices) ExtractTenantID(ctx context.Context) (uuid.UUID, error) {
	return composables.UseTenantID(ctx)
}

func (h *sdkHostServices) ExtractPool(context.Context) (*pgxpool.Pool, error) {
	if h.pool == nil {
		return nil, fmt.Errorf("pool is not configured")
	}
	return h.pool, nil
}

func (h *sdkHostServices) ExtractPageLocale(ctx context.Context) language.Tag {
	pageContext, ok := ctx.Value(constants.PageContext).(types.PageContext)
	if !ok {
		return language.English
	}
	return pageContext.GetLocale()
}
