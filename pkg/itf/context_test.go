package itf

import (
	"context"
	"embed"
	"errors"
	"reflect"
	"testing"

	"github.com/benbjohnson/hashfs"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/applets"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

type testController struct {
	key   string
	close func() error
}

func (c testController) Register(_ *mux.Router) {}

func (c testController) Key() string { return c.key }

func (c testController) Close() error {
	if c.close == nil {
		return nil
	}
	return c.close()
}

type plainController struct {
	key string
}

func (c plainController) Register(_ *mux.Router) {}

func (c plainController) Key() string { return c.key }

type fakeModule struct {
	name string
}

func (m fakeModule) Name() string { return m.name }

func (m fakeModule) Register(_ application.Application) error { return nil }

func TestCloseControllerResources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		build          func() ([]application.Controller, *int)
		expectedClosed int
	}{
		{
			name: "closes only closable controllers",
			build: func() ([]application.Controller, *int) {
				closed := 0
				return []application.Controller{
					testController{
						key: "closable",
						close: func() error {
							closed++
							return nil
						},
					},
					plainController{key: "plain"},
				}, &closed
			},
			expectedClosed: 1,
		},
		{
			name: "logs close errors",
			build: func() ([]application.Controller, *int) {
				closed := 0
				return []application.Controller{
					testController{
						key: "failing-closer",
						close: func() error {
							closed++
							return errors.New("close failed")
						},
					},
				}, &closed
			},
			expectedClosed: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			controllers, closed := tt.build()
			closeControllerResources(t, controllers)
			require.Equal(t, tt.expectedClosed, *closed)
		})
	}
}

func TestNormalizeHarnessConfigDefaults(t *testing.T) {
	t.Parallel()

	cfg := normalizeHarnessConfig(t, HarnessConfig{})

	require.Equal(t, "itf_harness_"+t.Name(), cfg.Name)
	require.Equal(t, ProvisioningSharedPerPackage, cfg.Database.Provisioning)
	require.Equal(t, CleanupDropOnExit, cfg.Database.Cleanup)
	require.Equal(t, MigrationApplyOnce, cfg.Migration.Policy)
	require.Equal(t, IsolationRollback, cfg.Isolation.Mode)
	require.Equal(t, SeedNone, cfg.Seed.Policy)
	require.Equal(t, []string{"en"}, cfg.Context.Locales)
}

func TestRunMigrationPolicyApplyOnceCallsApplicationMigrations(t *testing.T) {
	t.Parallel()

	migrations := &countingMigrationManager{}
	app := &testApp{
		migrations: migrations,
	}
	cfg := MigrationConfig{Policy: MigrationApplyOnce}

	err := runMigrationPolicy(context.Background(), nil, app, cfg)
	require.NoError(t, err)
	require.True(t, migrations.RunCalled())
}

func TestSharedHarnessManagerReferenceCounting(t *testing.T) {
	t.Parallel()

	manager := &harnessManager{
		entries: map[string]*managedHarnessState{},
	}

	const key = "itf-shared-manager-test"
	manager.entries[key] = &managedHarnessState{
		state: &harnessState{},
		refs:  2,
	}

	err := manager.close(key, CleanupKeep)
	require.NoError(t, err)
	require.Len(t, manager.entries, 1)
	require.Equal(t, 1, manager.entries[key].refs)

	err = manager.close(key, CleanupKeep)
	require.NoError(t, err)
	require.Empty(t, manager.entries)
}

func TestBuildHarnessKeyIncludesIsolationAndSeedPolicy(t *testing.T) {
	t.Parallel()

	module := fakeModule{name: "billing"}
	tenantID := uuid.MustParse("f2c31f45-dc7c-4e2e-bfc4-e1e2f4ff2f7d")
	cfg := normalizeHarnessConfig(t, HarnessConfig{
		Name:    "backend",
		Modules: []application.Module{module},
		Database: DatabaseConfig{
			Provisioning: ProvisioningPerTestDatabase,
			Cleanup:      CleanupKeep,
			Pool:         PoolConfig{MaxConns: 4},
		},
		Migration: MigrationConfig{
			Policy: MigrationSkip,
		},
		Isolation: IsolationConfig{
			Mode: IsolationCommitted,
		},
		Seed: SeedConfig{
			Policy: SeedOncePerHarness,
		},
		Context: ContextConfig{
			TenantID: &tenantID,
			Locales:  []string{"ru", "en"},
		},
	})

	key := buildHarnessKey(cfg)
	require.Contains(t, key, "name=backend")
	require.Contains(t, key, "prov=per_test_database")
	require.Contains(t, key, "migrate=skip")
	require.Contains(t, key, "iso=committed")
	require.Contains(t, key, "seed=once_per_harness")
	require.Contains(t, key, "pool=4/0/0s/0s")
	require.Contains(t, key, "tenant="+tenantID.String())
	require.Contains(t, key, "mods=["+reflect.TypeOf(module).String()+"]")
	require.Contains(t, key, "cleanup=keep")
	require.Contains(t, key, "locales=[ru en]")
}

func TestRunMigrationPolicyRejectsUnsupportedPolicy(t *testing.T) {
	t.Parallel()

	migrations := &countingMigrationManager{}
	app := &testApp{
		migrations: migrations,
	}
	cfg := MigrationConfig{
		Policy: MigrationPolicy("unsupported"),
	}

	err := runMigrationPolicy(context.Background(), nil, app, cfg)
	require.Error(t, err)

	var serr *serrors.Error
	require.ErrorAs(t, err, &serr)
	require.Equal(t, serrors.Invalid, serr.Kind)
	require.False(t, migrations.RunCalled())
}

type countingMigrationManager struct {
	runCalled bool
	runErr    error
}

func (m *countingMigrationManager) Rollback() error { return nil }

func (m *countingMigrationManager) Run() error {
	m.runCalled = true
	return m.runErr
}

func (m *countingMigrationManager) Status(context.Context) ([]application.MigrationStatus, error) {
	return nil, nil
}

func (m *countingMigrationManager) RunCalled() bool { return m.runCalled }

type testApp struct {
	migrations application.MigrationManager
}

func (a *testApp) DB() *pgxpool.Pool                                         { return nil }
func (a *testApp) EventPublisher() eventbus.EventBus                         { return nil }
func (a *testApp) Controllers() []application.Controller                     { return nil }
func (a *testApp) Middleware() []mux.MiddlewareFunc                          { return nil }
func (a *testApp) Assets() []*embed.FS                                       { return nil }
func (a *testApp) HashFsAssets() []*hashfs.FS                                { return nil }
func (a *testApp) Websocket() application.Huber                              { return nil }
func (a *testApp) Spotlight() spotlight.Service                              { return nil }
func (a *testApp) QuickLinks() *spotlight.QuickLinks                         { return nil }
func (a *testApp) NavItems(*i18n.Localizer) []types.NavigationItem           { return nil }
func (a *testApp) RegisterNavItems(items ...types.NavigationItem)            {}
func (a *testApp) RegisterControllers(controllers ...application.Controller) {}
func (a *testApp) RegisterHashFsAssets(fs ...*hashfs.FS)                     {}
func (a *testApp) RegisterAssets(fs ...*embed.FS)                            {}
func (a *testApp) RegisterLocaleFiles(fs ...*embed.FS)                       {}
func (a *testApp) RegisterGraphSchema(schema application.GraphSchema)        {}
func (a *testApp) GraphSchemas() []application.GraphSchema                   { return nil }
func (a *testApp) RegisterServices(services ...interface{})                  {}
func (a *testApp) RegisterMiddleware(middleware ...mux.MiddlewareFunc)       {}
func (a *testApp) Service(service interface{}) interface{}                   { return nil }
func (a *testApp) Services() map[reflect.Type]interface{}                    { return nil }
func (a *testApp) Bundle() *i18n.Bundle                                      { return nil }
func (a *testApp) GetSupportedLanguages() []string                           { return nil }
func (a *testApp) RegisterApplet(applet application.Applet) error            { return nil }
func (a *testApp) AppletRegistry() application.AppletRegistry                { return nil }
func (a *testApp) CreateAppletControllers(
	host applets.HostServices,
	sessionConfig applets.SessionConfig,
	logger *logrus.Logger,
	metrics applets.MetricsRecorder,
	opts ...applets.BuilderOption,
) ([]application.Controller, error) {
	return nil, nil
}

func (a *testApp) Migrations() application.MigrationManager { return a.migrations }
