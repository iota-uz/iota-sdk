package core_test

import (
	"context"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/logging"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func compileSettings(t *testing.T, opts *core.ModuleOptions, loggingOpts ...logging.ComponentOption) (*composition.Container, application.Application) {
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
	require.NoError(t, engine.Register(logging.NewComponent(loggingOpts...), core.NewComponent(opts)))
	container, err := engine.Compile(
		composition.NewBuildContext(app, src, composition.WithLogger(logger)),
		composition.CapabilityAPI,
	)
	require.NoError(t, err)
	return container, app
}

func settingsPagesByKey(sections []types.SettingsSection) map[string]types.SettingsPage {
	out := map[string]types.SettingsPage{}
	for _, section := range sections {
		for _, page := range section.Pages {
			out[page.Key] = page
		}
	}
	return out
}

// TestComponentSettingsContributions is falsely green if contributions were
// unconditional; the SkipAdminControllers case must drop every core page.
func TestComponentSettingsContributions(t *testing.T) {
	t.Parallel()

	corePages := []string{
		core.SettingsPageGeneral,
		core.SettingsPageNotifications,
		core.SettingsPageUsers,
		core.SettingsPageRoles,
		core.SettingsPageGroups,
		core.SettingsPageDepartments,
		core.SettingsPagePositions,
	}

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()
		container, _ := compileSettings(t, nil)
		pages := settingsPagesByKey(container.SettingsSections())
		for _, key := range corePages {
			assert.Contains(t, pages, key)
		}
		assert.Equal(t, "/users", pages[core.SettingsPageUsers].Href)
		assert.Equal(t, "/admin/history", pages[logging.SettingsPageHistory].Href)
	})

	t.Run("custom paths follow controller options", func(t *testing.T) {
		t.Parallel()
		container, _ := compileSettings(t, &core.ModuleOptions{
			UserControllerOptions: []controllers.UserControllerOption{controllers.WithUserControllerBasePath("/people")},
		}, logging.WithHistoryBasePath("/audit"))
		pages := settingsPagesByKey(container.SettingsSections())
		assert.Equal(t, "/people", pages[core.SettingsPageUsers].Href)
		assert.Equal(t, "/audit", pages[logging.SettingsPageHistory].Href)
	})

	t.Run("SkipAdminControllers drops core pages", func(t *testing.T) {
		t.Parallel()
		container, _ := compileSettings(t, &core.ModuleOptions{SkipAdminControllers: true})
		pages := settingsPagesByKey(container.SettingsSections())
		for _, key := range corePages {
			assert.NotContains(t, pages, key)
		}
		assert.Contains(t, pages, logging.SettingsPageHistory)
	})
}

// TestComponentSettingsKeysAreTranslated is falsely green if a key were
// missing from en but present elsewhere; the locale parity check covers the
// remaining languages.
func TestComponentSettingsKeysAreTranslated(t *testing.T) {
	t.Parallel()
	container, app := compileSettings(t, nil)
	localizer := i18n.NewLocalizer(app.Bundle(), "en")

	sections := container.SettingsSections()
	require.NotEmpty(t, sections)
	for _, section := range sections {
		keys := []string{section.Name}
		for _, page := range section.Pages {
			keys = append(keys, page.Name, page.Description)
		}
		for _, key := range keys {
			require.NotEmpty(t, key, "section %q", section.Key)
			_, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: key})
			assert.NoError(t, err, "missing translation for %q", key)
		}
	}
}
