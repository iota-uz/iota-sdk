package core_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/gorilla/mux"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/logging"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComponentSkipAdminNavItems(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		opts         *core.ModuleOptions
		wantAdminNav bool
	}{
		{
			name:         "zero value preserves admin nav items",
			opts:         &core.ModuleOptions{},
			wantAdminNav: true,
		},
		{
			name:         "SkipAdminNavItems suppresses nav items but keeps quick links",
			opts:         &core.ModuleOptions{SkipAdminNavItems: true},
			wantAdminNav: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			navItems, quickLinkKeys := compileCoreNav(t, tt.opts)

			assert.Equal(t, tt.wantAdminNav, hasNavItem(navItems, "core.administration"))
			assert.Equal(t, tt.wantAdminNav, hasNavItem(navItems, "core.dashboard"))
			assert.Contains(t, quickLinkKeys, "NavigationLinks.Users")
			assert.Contains(t, quickLinkKeys, "Users.List.New")
		})
	}
}

func compileCoreNav(t *testing.T, opts *core.ModuleOptions) ([]types.NavigationItem, map[string]struct{}) {
	t.Helper()

	container, app := compileCore(t, opts)

	docs, err := spotlight.CollectDocuments(context.Background(), app.QuickLinks(), spotlight.ProviderScope{
		TenantID: uuid.New(),
		Language: "en",
	})
	require.NoError(t, err)

	keys := make(map[string]struct{}, len(docs))
	for _, doc := range docs {
		keys[doc.Metadata["tr_key"]] = struct{}{}
	}
	return container.NavItems(), keys
}

func compileCore(t *testing.T, opts *core.ModuleOptions) (*composition.Container, application.Application) {
	t.Helper()

	if opts == nil {
		opts = &core.ModuleOptions{}
	}
	opts.PermissionSchema = defaults.PermissionSchema()

	logger := logrus.New()
	poolConfig, err := pgxpool.ParseConfig("postgres://iota:iota@127.0.0.1:1/iota?sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	src, err := config.Build(static.New(nil))
	require.NoError(t, err)

	app, err := application.New(&application.ApplicationOptions{
		Pool:               pool,
		Bundle:             application.LoadBundle(),
		EventBus:           eventbus.NewEventPublisher(logger),
		Logger:             logger,
		SupportedLanguages: application.DefaultSupportedLanguages(),
	})
	require.NoError(t, err)

	engine := composition.NewEngine()
	require.NoError(t, engine.Register(logging.NewComponent(), core.NewComponent(opts)))
	container, err := engine.Compile(
		composition.NewBuildContext(app, src, composition.WithLogger(logger)),
		composition.CapabilityAPI,
	)
	require.NoError(t, err)

	return container, app
}

func TestComponentSkipDashboardController(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		opts          *core.ModuleOptions
		wantDashboard bool
		wantAdmin     bool
	}{
		{
			name:          "zero value keeps the dashboard",
			opts:          &core.ModuleOptions{},
			wantDashboard: true,
			wantAdmin:     true,
		},
		{
			name:          "SkipDashboardController drops the dashboard only",
			opts:          &core.ModuleOptions{SkipDashboardController: true},
			wantDashboard: false,
			wantAdmin:     true,
		},
		{
			name:          "SkipAdminControllers drops the dashboard too",
			opts:          &core.ModuleOptions{SkipAdminControllers: true},
			wantDashboard: false,
			wantAdmin:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			container, _ := compileCore(t, tt.opts)
			ids := controllerIDs(container)

			assert.Equal(t, tt.wantDashboard, slices.Contains(ids, "core.dashboard"))
			assert.Equal(t, tt.wantDashboard, hasNavItem(container.NavItems(), "core.dashboard"))
			assert.Equal(t, tt.wantAdmin, slices.Contains(ids, "core.roles"))
			assert.Contains(t, ids, "core.login")

			if !tt.wantDashboard {
				// Building the router must not touch the Lens Solid assets: that is
				// the panic this option exists to avoid in a consumer without them.
				router := mux.NewRouter()
				require.NotPanics(t, func() {
					for _, ctrl := range container.Controllers() {
						ctrl.Register(router)
					}
				})
				// The dashboard is the only core controller that answers "/", so
				// skipping it must leave the site root unrouted.
				assert.False(t, router.Match(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil), &mux.RouteMatch{}))
			}
		})
	}
}

func controllerIDs(container *composition.Container) []string {
	controllers := container.Controllers()
	ids := make([]string, 0, len(controllers))
	for _, ctrl := range controllers {
		ids = append(ids, ctrl.Descriptor().ID)
	}
	return ids
}

func hasNavItem(items []types.NavigationItem, key string) bool {
	for _, item := range items {
		if item.Key == key {
			return true
		}
		if hasNavItem(item.Children, key) {
			return true
		}
	}
	return false
}
