package application

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/applet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockApplet is a test implementation of the Applet interface
type mockApplet struct {
	name     string
	basePath string
}

func (m *mockApplet) Name() string {
	return m.name
}

func (m *mockApplet) BasePath() string {
	return m.basePath
}

func (m *mockApplet) Config() applet.Config {
	return applet.Config{
		WindowGlobal: "__TEST_CONTEXT__",
	}
}

func TestAppletRegistry_Register(t *testing.T) {
	t.Parallel()

	t.Run("successfully registers applet", func(t *testing.T) {
		t.Parallel()
		registry := applet.NewRegistry()
		applet := &mockApplet{name: "test-applet", basePath: "/test"}

		err := registry.Register(applet)
		require.NoError(t, err)

		assert.True(t, registry.Has("test-applet"))
		assert.Equal(t, applet, registry.Get("test-applet"))
	})

	t.Run("returns error for duplicate applet name", func(t *testing.T) {
		t.Parallel()
		registry := applet.NewRegistry()
		applet1 := &mockApplet{name: "duplicate", basePath: "/path1"}
		applet2 := &mockApplet{name: "duplicate", basePath: "/path2"}

		err := registry.Register(applet1)
		require.NoError(t, err)

		err = registry.Register(applet2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("returns error for empty applet name", func(t *testing.T) {
		t.Parallel()
		registry := applet.NewRegistry()
		applet := &mockApplet{name: "", basePath: "/test"}

		err := registry.Register(applet)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})
}

func TestAppletRegistry_Get(t *testing.T) {
	t.Parallel()

	t.Run("returns applet by name", func(t *testing.T) {
		t.Parallel()
		registry := applet.NewRegistry()
		applet := &mockApplet{name: "test", basePath: "/test"}

		_ = registry.Register(applet)

		result := registry.Get("test")
		assert.Equal(t, applet, result)
	})

	t.Run("returns nil for non-existent applet", func(t *testing.T) {
		t.Parallel()
		registry := applet.NewRegistry()

		result := registry.Get("non-existent")
		assert.Nil(t, result)
	})
}

func TestAppletRegistry_Has(t *testing.T) {
	t.Parallel()

	t.Run("returns true for registered applet", func(t *testing.T) {
		t.Parallel()
		registry := applet.NewRegistry()
		applet := &mockApplet{name: "test", basePath: "/test"}

		_ = registry.Register(applet)

		assert.True(t, registry.Has("test"))
	})

	t.Run("returns false for non-existent applet", func(t *testing.T) {
		t.Parallel()
		registry := applet.NewRegistry()

		assert.False(t, registry.Has("non-existent"))
	})
}

func TestAppletRegistry_All(t *testing.T) {
	t.Parallel()

	t.Run("returns all registered applets", func(t *testing.T) {
		t.Parallel()
		registry := applet.NewRegistry()
		applet1 := &mockApplet{name: "applet1", basePath: "/path1"}
		applet2 := &mockApplet{name: "applet2", basePath: "/path2"}

		_ = registry.Register(applet1)
		_ = registry.Register(applet2)

		all := registry.All()
		assert.Len(t, all, 2)
		assert.Contains(t, all, applet1)
		assert.Contains(t, all, applet2)
	})

	t.Run("returns empty slice for no applets", func(t *testing.T) {
		t.Parallel()
		registry := applet.NewRegistry()

		all := registry.All()
		assert.Len(t, all, 0)
	})
}

func TestApplication_RegisterApplet(t *testing.T) {
	t.Parallel()

	t.Run("successfully registers applet via application", func(t *testing.T) {
		t.Parallel()
		app := New(&ApplicationOptions{
			Bundle:             LoadBundle(),
			SupportedLanguages: []string{"en"},
		})
		applet := &mockApplet{name: "test", basePath: "/test"}

		err := app.RegisterApplet(applet)
		require.NoError(t, err)

		registry := app.AppletRegistry()
		assert.True(t, registry.Has("test"))
		assert.Equal(t, applet, registry.Get("test"))
	})

	t.Run("returns error for duplicate applet", func(t *testing.T) {
		t.Parallel()
		app := New(&ApplicationOptions{
			Bundle:             LoadBundle(),
			SupportedLanguages: []string{"en"},
		})
		applet1 := &mockApplet{name: "dup", basePath: "/path1"}
		applet2 := &mockApplet{name: "dup", basePath: "/path2"}

		err := app.RegisterApplet(applet1)
		require.NoError(t, err)

		err = app.RegisterApplet(applet2)
		assert.Error(t, err)
	})
}

func TestApplication_AppletRegistry(t *testing.T) {
	t.Parallel()

	t.Run("returns applet registry", func(t *testing.T) {
		t.Parallel()
		app := New(&ApplicationOptions{
			Bundle:             LoadBundle(),
			SupportedLanguages: []string{"en"},
		})

		registry := app.AppletRegistry()
		require.NotNil(t, registry)

		// Registry should be empty initially
		all := registry.All()
		assert.Len(t, all, 0)
	})

	t.Run("registry is shared across calls", func(t *testing.T) {
		t.Parallel()
		app := New(&ApplicationOptions{
			Bundle:             LoadBundle(),
			SupportedLanguages: []string{"en"},
		})
		applet := &mockApplet{name: "test", basePath: "/test"}

		err := app.RegisterApplet(applet)
		require.NoError(t, err)

		registry1 := app.AppletRegistry()
		registry2 := app.AppletRegistry()

		// Both should reference the same registry
		assert.True(t, registry1.Has("test"))
		assert.True(t, registry2.Has("test"))
		assert.Equal(t, applet, registry1.Get("test"))
		assert.Equal(t, applet, registry2.Get("test"))
	})
}
