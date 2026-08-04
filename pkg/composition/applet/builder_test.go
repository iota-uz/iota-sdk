package applet

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"testing/fstest"

	"github.com/google/uuid"
	"github.com/iota-uz/applets"
	appletsconfig "github.com/iota-uz/applets/config"
	appletenginehandlers "github.com/iota-uz/iota-sdk/pkg/appletengine/handlers"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

type testApplet struct {
	name     string
	basePath string
	method   string
}

func (a *testApplet) Name() string     { return a.name }
func (a *testApplet) BasePath() string { return a.basePath }
func (a *testApplet) Config() applets.Config {
	return applets.Config{
		WindowGlobal: "__DEMO_CONTEXT__",
		Shell:        applets.ShellConfig{Mode: applets.ShellModeStandalone, Title: "Demo"},
		Assets: applets.AssetConfig{
			FS: fstest.MapFS{
				"manifest.json":  {Data: []byte(`{"index.html":{"file":"assets/main.js","isEntry":true}}`)},
				"assets/main.js": {Data: []byte("console.log('demo')")},
			},
			BasePath:     "/assets",
			ManifestPath: "manifest.json",
			Entrypoint:   "index.html",
		},
		RPC: &applets.RPCConfig{
			Path: "/rpc",
			Methods: map[string]applets.RPCMethod{
				a.method: {
					Handler: func(_ context.Context, _ json.RawMessage) (any, error) {
						return map[string]any{"ok": true}, nil
					},
				},
			},
		},
	}
}

type testUser struct{}

func (u *testUser) ID() uint                    { return 1 }
func (u *testUser) DisplayName() string         { return "Demo User" }
func (u *testUser) HasPermission(_ string) bool { return true }
func (u *testUser) PermissionNames() []string   { return []string{"Demo.Access"} }

type fakeFilesStore struct{}

func (fakeFilesStore) Store(context.Context, string, string, []byte) (map[string]any, error) {
	return map[string]any{"id": "fake"}, nil
}

func (fakeFilesStore) Get(context.Context, string) (map[string]any, bool, error) {
	return nil, false, nil
}

func (fakeFilesStore) Delete(context.Context, string) (bool, error) { return false, nil }

func TestAppletEngineBuilder_UsesCustomBackendFactory(t *testing.T) {
	builder := NewAppletEngineBuilder()
	used := false
	err := builder.Backends().Files.RegisterFactory(BackendFactory[appletenginehandlers.FilesStore]{
		ID: "test-only",
		BuildFunc: func(*Container) (appletenginehandlers.FilesStore, error) {
			used = true
			return fakeFilesStore{}, nil
		},
	})
	require.NoError(t, err)

	_, err = builder.Build(BuildInput{
		Applets:       []applets.Applet{&testApplet{name: "demo", basePath: "/demo", method: "demo.ping"}},
		Host:          &builderTestHostServices{},
		SessionConfig: applets.DefaultSessionConfig,
		Logger:        logrus.New(),
		ProjectConfig: &appletsconfig.ProjectConfig{
			Applets: map[string]*appletsconfig.AppletConfig{
				"demo": {
					Engine: &appletsconfig.AppletEngineConfig{
						Runtime: appletsconfig.EngineRuntimeOff,
						Backends: appletsconfig.AppletEngineBackendsConfig{
							KV:      appletsconfig.KVBackendMemory,
							DB:      appletsconfig.DBBackendMemory,
							Jobs:    appletsconfig.JobsBackendMemory,
							Files:   "test-only",
							Secrets: appletsconfig.SecretsBackendEnv,
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)
	assert.True(t, used)
}

type builderTestHostServices struct{}

func (h *builderTestHostServices) ExtractUser(context.Context) (applets.AppletUser, error) {
	return &testUser{}, nil
}

func (h *builderTestHostServices) ExtractTenantID(context.Context) (uuid.UUID, error) {
	return uuid.MustParse("00000000-0000-0000-0000-000000000001"), nil
}

func (h *builderTestHostServices) ExtractPool(context.Context) (*pgxpool.Pool, error) {
	return nil, fmt.Errorf("no pool in tests")
}

func (h *builderTestHostServices) ExtractPageLocale(context.Context) language.Tag {
	return language.English
}

func TestDefaultBunBinResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		configs map[string]appletsconfig.AppletEngineConfig
		want    string
		wantErr string
	}{
		{
			name: "empty inherits",
			configs: map[string]appletsconfig.AppletEngineConfig{
				"a": {BunBin: ""},
				"b": {BunBin: "/usr/local/bin/bun"},
				"c": {BunBin: ""},
			},
			want: "/usr/local/bin/bun",
		},
		{
			name: "matching explicit values",
			configs: map[string]appletsconfig.AppletEngineConfig{
				"a": {BunBin: "/usr/local/bin/bun"},
				"b": {BunBin: "/usr/local/bin/bun"},
			},
			want: "/usr/local/bin/bun",
		},
		{
			name: "mismatch fails",
			configs: map[string]appletsconfig.AppletEngineConfig{
				"a": {BunBin: "/usr/local/bin/bun"},
				"b": {BunBin: "/opt/homebrew/bin/bun"},
			},
			wantErr: "runtime bun_bin mismatch across enabled applets",
		},
	}

	resolver := DefaultBunBinResolver{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolver.Resolve(tc.configs)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
