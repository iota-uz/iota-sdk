package extruntimeconfig_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/config"
	envprov "github.com/iota-uz/iota-sdk/pkg/config/providers/env"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/extruntimeconfig"
)

func register(t *testing.T, values map[string]any) (*extruntimeconfig.Config, error) {
	t.Helper()
	src, err := config.Build(static.New(values))
	require.NoError(t, err)
	return config.Register[extruntimeconfig.Config](config.NewRegistry(src))
}

func TestDefaults_DisabledAndInert(t *testing.T) {
	t.Parallel()
	cfg, err := register(t, nil)
	require.NoError(t, err)
	assert.False(t, cfg.Enabled)
	assert.Equal(t, "node", cfg.NodeBinary)
	assert.Equal(t, 2, cfg.PoolSize)
	assert.Equal(t, 2*time.Second, cfg.UI.SourceTTL)
	assert.False(t, cfg.Configurator.Enabled)
	assert.Empty(t, cfg.UI.HostManifest)
}

func TestRegister_FromEnvShapedKeys(t *testing.T) {
	t.Parallel()
	tenant := uuid.NewString()
	cfg, err := register(t, map[string]any{
		"extruntime.enabled":                      true,
		"extruntime.poolsize":                     6,
		"extruntime.approot":                      "/srv/app",
		"extruntime.cachedir":                     "/var/cache/app",
		"extruntime.ui.hostmanifest":              "/srv/app/host.json",
		"extruntime.configurator.enabled":         true,
		"extruntime.configurator.path":            "/admin/extensions",
		"extruntime.endpoints.publictenant":       tenant,
		"extruntime.endpoints.timeout":            "10s",
		"extruntime.git.allowedschemes":           "https",
		"extruntime.workflowconcurrency":          3,
		"extruntime.endpoints.maxbodybytes":       2048,
		"extruntime.nodebinary":                   "/usr/local/bin/node",
		"extruntime.distdir":                      "/opt/sdk",
		"extruntime.configurator.typesdir":        "/srv/app/node_modules",
		"extruntime.endpoints.publictenantbyhost": false,
	})
	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 6, cfg.PoolSize)
	assert.Equal(t, "/srv/app", cfg.AppRoot)
	assert.Equal(t, "/var/cache/app", cfg.CacheDir)
	assert.Equal(t, "/srv/app/host.json", cfg.UI.HostManifest)
	assert.True(t, cfg.Configurator.Enabled)
	assert.Equal(t, 10*time.Second, cfg.Endpoints.Timeout)
	assert.Equal(t, []string{"https"}, cfg.GitSchemes())
	assert.Equal(t, tenant, cfg.PublicTenantID().String())
}

func TestValidate(t *testing.T) {
	t.Parallel()
	tenant := uuid.NewString()
	tests := []struct {
		name    string
		values  map[string]any
		wantErr string
	}{
		{name: "disabled skips validation", values: map[string]any{"extruntime.poolsize": 0, "extruntime.configurator.enabled": true}},
		{name: "enabled defaults are valid", values: map[string]any{"extruntime.enabled": true}},
		{name: "pool size below range", values: map[string]any{"extruntime.enabled": true, "extruntime.poolsize": 0}, wantErr: "poolsize"},
		{name: "pool size above range", values: map[string]any{"extruntime.enabled": true, "extruntime.poolsize": 65}, wantErr: "poolsize"},
		{name: "configurator needs the host manifest", values: map[string]any{"extruntime.enabled": true, "extruntime.configurator.enabled": true}, wantErr: "hostmanifest"},
		{name: "configurator with the host manifest", values: map[string]any{"extruntime.enabled": true, "extruntime.configurator.enabled": true, "extruntime.ui.hostmanifest": "host.json"}},
		{name: "relative configurator path", values: map[string]any{"extruntime.enabled": true, "extruntime.configurator.path": "extensions"}, wantErr: "configurator.path"},
		{name: "public tenant is a uuid", values: map[string]any{"extruntime.enabled": true, "extruntime.endpoints.publictenant": "acme"}, wantErr: "publictenant"},
		{name: "nil uuid is not a tenant", values: map[string]any{"extruntime.enabled": true, "extruntime.endpoints.publictenant": uuid.Nil.String()}, wantErr: "publictenant"},
		{name: "fixed tenant and host resolution are exclusive", values: map[string]any{"extruntime.enabled": true, "extruntime.endpoints.publictenant": tenant, "extruntime.endpoints.publictenantbyhost": true}, wantErr: "exclusive"},
		{name: "host resolution alone", values: map[string]any{"extruntime.enabled": true, "extruntime.endpoints.publictenantbyhost": true}},
		{name: "unknown git scheme", values: map[string]any{"extruntime.enabled": true, "extruntime.git.allowedschemes": "https, ftp"}, wantErr: "ftp"},
		{name: "negative body bound", values: map[string]any{"extruntime.enabled": true, "extruntime.endpoints.maxbodybytes": -1}, wantErr: "maxbodybytes"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := register(t, tc.values)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestPublicTenantID_UnsetIsNil(t *testing.T) {
	t.Parallel()
	cfg, err := register(t, map[string]any{"extruntime.enabled": true})
	require.NoError(t, err)
	assert.Equal(t, uuid.Nil, cfg.PublicTenantID())
}

// TestRegister_FromEnvironment pins the variable names operators set: one
// underscore per dot of the koanf path, lists comma-separated.
func TestRegister_FromEnvironment(t *testing.T) {
	tenant := uuid.NewString()
	t.Setenv("EXTRUNTIME_ENABLED", "true")
	t.Setenv("EXTRUNTIME_POOLSIZE", "3")
	t.Setenv("EXTRUNTIME_CACHEDIR", "/var/cache/app")
	t.Setenv("EXTRUNTIME_UI_HOSTMANIFEST", "/srv/host.json")
	t.Setenv("EXTRUNTIME_UI_SOURCETTL", "5s")
	t.Setenv("EXTRUNTIME_CONFIGURATOR_ENABLED", "true")
	t.Setenv("EXTRUNTIME_ENDPOINTS_PUBLICTENANT", tenant)
	t.Setenv("EXTRUNTIME_ENDPOINTS_TIMEOUT", "45s")
	t.Setenv("EXTRUNTIME_GIT_ALLOWEDSCHEMES", "https,ssh")
	t.Setenv("EXTRUNTIME_TRUSTEDORIGINS", "https://admin.example.com, http://localhost:3200")
	t.Setenv("EXTRUNTIME_GIT_ALLOWEDHOSTS", "Git.Example.com, *.corp.example.com")
	t.Setenv("EXTRUNTIME_GIT_ALLOWPRIVATENETWORKS", "true")

	src, err := config.Build(envprov.New())
	require.NoError(t, err)
	cfg, err := config.Register[extruntimeconfig.Config](config.NewRegistry(src))
	require.NoError(t, err)

	assert.True(t, cfg.Enabled)
	assert.Equal(t, 3, cfg.PoolSize)
	assert.Equal(t, "/var/cache/app", cfg.CacheDir)
	assert.Equal(t, "/srv/host.json", cfg.UI.HostManifest)
	assert.Equal(t, 5*time.Second, cfg.UI.SourceTTL)
	assert.True(t, cfg.Configurator.Enabled)
	assert.Equal(t, tenant, cfg.Endpoints.PublicTenant)
	assert.Equal(t, 45*time.Second, cfg.Endpoints.Timeout)
	assert.Equal(t, []string{"https", "ssh"}, cfg.GitSchemes())
	assert.Equal(t, []string{"git.example.com", "*.corp.example.com"}, cfg.GitHosts())
	assert.True(t, cfg.Git.AllowPrivateNetworks)
	assert.Equal(t, []string{"https://admin.example.com", "http://localhost:3200"}, cfg.TrustedOriginList())
	require.NoError(t, cfg.Validate())

	cfg.TrustedOrigins = "admin.example.com"
	require.ErrorContains(t, cfg.Validate(), "trustedorigins")
	cfg.TrustedOrigins = "https://admin.example.com/path"
	require.ErrorContains(t, cfg.Validate(), "trustedorigins")
	cfg.TrustedOrigins = ""

	cfg.Git.AllowedHosts = "https://git.example.com"
	require.ErrorContains(t, cfg.Validate(), "git.allowedhosts")
}
