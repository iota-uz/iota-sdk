package wiring_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/extruntimeconfig"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/wiring"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/sdkidentity"
)

const commit = "0123456789abcdef0123456789abcdef01234567"

// fakeDist lays out the files ResolveDist verifies: the package, its release
// identity and both workers.
func fakeDist(t *testing.T, version string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "node_modules", "@iota-uz", "sdk")
	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	}
	pkg, err := json.Marshal(map[string]string{"name": "@iota-uz/sdk", "version": version})
	require.NoError(t, err)
	id, err := json.Marshal(sdkidentity.Identity{ReleaseVersion: sdkidentity.ReleaseVersion, SourceCommit: commit, ProtocolVersion: sdkidentity.ProtocolVersion})
	require.NoError(t, err)
	write("package.json", string(pkg))
	write("dist/sdk-identity.json", string(id))
	write("dist/runtime/host/worker.mjs", "")
	write("dist/runtime/build/worker.mjs", "")
	return dir
}

func hostManifest(t *testing.T, release string) string {
	t.Helper()
	raw, err := json.Marshal(clienthost.Manifest{
		Package: "@iota-uz/sdk", PackageVersion: release, SDKCommit: commit, SDKReleaseVersion: release,
		ProtocolVersion: clienthost.ProtocolVersion, Entry: "/assets/host.js",
	})
	require.NoError(t, err)
	p := filepath.Join(t.TempDir(), "host.json")
	require.NoError(t, os.WriteFile(p, raw, 0o600))
	return p
}

func contributor(s *native.Set) error { return nil }

func TestRuntimeConfig_TranslatesTheSection(t *testing.T) {
	t.Parallel()
	dist := fakeDist(t, sdkidentity.ReleaseVersion)
	tenant := uuid.New()
	section := extruntimeconfig.Config{
		Enabled: true, NodeBinary: "node", PoolSize: 5, DistDir: dist, CacheDir: "/var/cache/app", WorkflowConcurrency: 7, DisableBackground: true,
		UI:           extruntimeconfig.UIConfig{HostManifest: hostManifest(t, sdkidentity.ReleaseVersion)},
		Configurator: extruntimeconfig.ConfiguratorConfig{Enabled: true, Path: "/admin/ext"},
		Endpoints:    extruntimeconfig.EndpointsConfig{PublicTenant: tenant.String(), MaxBodyBytes: 4096},
		Git:          extruntimeconfig.GitConfig{AllowedSchemes: "https"},
	}
	registry := mcp.NewRegistry()

	cfg, resolved, err := wiring.RuntimeConfig(section, wiring.Options{Natives: []native.Contributor{contributor}, MCPRegistry: registry})
	require.NoError(t, err)

	require.Equal(t, dist, resolved.Dir)
	require.Equal(t, 5, cfg.PoolSize)
	require.Equal(t, "/var/cache/app", cfg.CacheDir)
	require.Equal(t, 7, cfg.Workflow.Concurrency)
	require.True(t, cfg.DisableBackground)
	require.Equal(t, int64(4096), cfg.Endpoints.MaxBodyBytes)
	require.Equal(t, []string{"https"}, cfg.Git.AllowedSchemes)
	require.NotNil(t, cfg.Natives)
	require.NotNil(t, cfg.UI)
	require.Equal(t, "/assets/host.js", cfg.UI.Host.Entry)
	require.NotNil(t, cfg.Configurator)
	require.Equal(t, "/admin/ext", cfg.Configurator.Path)
	require.Equal(t, cfg.UI.Host, cfg.Configurator.Host)
	got, ok := cfg.Endpoints.PublicTenant(httptest.NewRequest(http.MethodPost, "/", nil))
	require.True(t, ok)
	require.Equal(t, tenant, got, "a configured tenant answers every public request")
	require.Len(t, cfg.Extensions, 1)
	require.Equal(t, "extruntime-mcp", cfg.Extensions[0].Name())
}

func TestRuntimeConfig_Minimal(t *testing.T) {
	t.Parallel()
	section := extruntimeconfig.Config{Enabled: true, NodeBinary: "node", PoolSize: 2, DistDir: fakeDist(t, sdkidentity.ReleaseVersion)}

	cfg, _, err := wiring.RuntimeConfig(section, wiring.Options{})
	require.NoError(t, err)

	require.False(t, cfg.DisableBackground, "the background parts run by default")
	require.Nil(t, cfg.UI, "no host manifest, no UI slice and no browser RPC")
	require.Nil(t, cfg.Configurator)
	require.Nil(t, cfg.Endpoints.PublicTenant, "public endpoints fall back to the tenant of the request context")
	require.Empty(t, cfg.Extensions, "no MCP registry, no tools")
	require.NotEmpty(t, cfg.CacheDir, "the cache directory defaults to a temporary one")
}

func TestRuntimeConfig_HostResolutionAndOverrides(t *testing.T) {
	t.Parallel()
	override := uuid.New()
	section := extruntimeconfig.Config{
		Enabled: true, NodeBinary: "node", PoolSize: 2, DistDir: fakeDist(t, sdkidentity.ReleaseVersion),
		Endpoints: extruntimeconfig.EndpointsConfig{PublicTenantByHost: true},
	}

	cfg, _, err := wiring.RuntimeConfig(section, wiring.Options{})
	require.NoError(t, err)
	require.True(t, cfg.Endpoints.TenantByHost)

	cfg, _, err = wiring.RuntimeConfig(section, wiring.Options{PublicTenant: func(*http.Request) (uuid.UUID, bool) { return override, true }})
	require.NoError(t, err)
	got, _ := cfg.Endpoints.PublicTenant(httptest.NewRequest(http.MethodPost, "/", nil))
	require.Equal(t, override, got, "the application's resolver wins over the configured one")
}

func TestRuntimeConfig_Tune(t *testing.T) {
	t.Parallel()
	section := extruntimeconfig.Config{Enabled: true, NodeBinary: "node", PoolSize: 2, DistDir: fakeDist(t, sdkidentity.ReleaseVersion)}
	cfg, _, err := wiring.RuntimeConfig(section, wiring.Options{Tune: func(c *runtime.Config) error { c.PoolSize = 9; return nil }})
	require.NoError(t, err)
	require.Equal(t, 9, cfg.PoolSize)
}

func TestRuntimeConfig_Failures(t *testing.T) {
	t.Parallel()
	valid := func() extruntimeconfig.Config {
		return extruntimeconfig.Config{Enabled: true, NodeBinary: "node", PoolSize: 2, DistDir: fakeDist(t, sdkidentity.ReleaseVersion)}
	}
	tests := []struct {
		name    string
		mutate  func(*extruntimeconfig.Config)
		wantErr string
	}{
		{name: "dist is missing", mutate: func(c *extruntimeconfig.Config) { c.DistDir = filepath.Join(t.TempDir(), "nothing") }, wantErr: "install @iota-uz/sdk@"},
		{name: "dist is another release", mutate: func(c *extruntimeconfig.Config) { c.DistDir = fakeDist(t, "0.0.1") }, wantErr: "does not match the Go SDK release"},
		{name: "host manifest is unreadable", mutate: func(c *extruntimeconfig.Config) { c.UI.HostManifest = filepath.Join(t.TempDir(), "absent.json") }, wantErr: "EXTRUNTIME_UI_HOSTMANIFEST"},
		{name: "host manifest is another release", mutate: func(c *extruntimeconfig.Config) { c.UI.HostManifest = hostManifest(t, "0.0.1") }, wantErr: "incompatible"},
		{name: "section is invalid", mutate: func(c *extruntimeconfig.Config) { c.PoolSize = 0 }, wantErr: "poolsize"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			section := valid()
			tc.mutate(&section)
			_, _, err := wiring.RuntimeConfig(section, wiring.Options{})
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestRuntimeConfig_NativeSetErrorsSurface(t *testing.T) {
	t.Parallel()
	section := extruntimeconfig.Config{Enabled: true, NodeBinary: "node", PoolSize: 2, DistDir: fakeDist(t, sdkidentity.ReleaseVersion)}
	_, _, err := wiring.RuntimeConfig(section, wiring.Options{Natives: []native.Contributor{func(*native.Set) error { return os.ErrInvalid }}})
	require.ErrorIs(t, err, os.ErrInvalid)
}
