package core_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
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
	require.NoError(t, engine.Register(core.NewComponent(opts)))
	container, err := engine.Compile(
		composition.NewBuildContext(app, src, composition.WithLogger(logger)),
		composition.CapabilityAPI,
	)
	require.NoError(t, err)

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
