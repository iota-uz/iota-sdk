// Package wiring turns the extension runtime on in an iota-sdk application
// from configuration. It is the composition root the SDK's own server
// binaries (cmd/server, cmd/worker) and applications use:
//
//	components = append(components, wiring.NewComponent(wiring.Options{
//	    Natives: append(modules.Natives(), app.Natives),
//	}))
//
// The component reads the extruntimeconfig section (EXTRUNTIME_* environment
// variables) from the config source of the build context. While the section
// is disabled, which is the default, it contributes nothing and the
// application behaves as if it were not there. When it is enabled the
// component composes runtime.NewComponent: the native set from the
// contributors, the Node workers of the installed @iota-uz/sdk package, the
// management API, the package endpoints, the UI slice and the configurator
// the section asks for, and the MCP tools of an application that has an MCP
// registry.
package wiring

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/extruntimeconfig"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/configurator"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/mcptools"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Options are the parts of the composition that configuration cannot express:
// Go values of the application.
type Options struct {
	// Natives are the native registrations of the application: the SDK
	// modules' (modules.Natives()) followed by its own. They are composed into
	// the one native.Set the runtime serves.
	Natives []native.Contributor
	// Section overrides the configuration read from the build context's
	// source. Leave nil to configure through EXTRUNTIME_* variables.
	Section *extruntimeconfig.Config
	// PublicTenant overrides the resolution of the tenant of a request
	// without a session (a webhook) that the section configures.
	PublicTenant func(r *http.Request) (uuid.UUID, bool)
	// Secrets serves secrets.get for the secrets packages declare.
	Secrets host.SecretResolver
	// MCPRegistry receives the extension tools (package, lifecycle, workflow,
	// catalog and invoke tools) of the application's MCP gateway. Nil
	// publishes none; the SDK servers host no gateway of their own.
	MCPRegistry *mcp.Registry
	// Extensions attach after the runtime is built, next to the MCP tools.
	Extensions []runtime.Extension
	// Tune adjusts the runtime configuration the section produced, for
	// settings that are Go values: event and schedule polling, Config.Types
	// of the configurator, a custom build adapter.
	Tune func(cfg *runtime.Config) error
	// Logger is the runtime's structured logger. Default slog.Default().
	Logger *slog.Logger
}

// NewComponent returns the composition component that enables the runtime
// when its configuration says so.
func NewComponent(opts Options) composition.Component {
	return &component{opts: opts}
}

type component struct{ opts Options }

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "extruntime.wiring"}
}

func (c *component) LocaleFS() []*embed.FS { return nil }

func (c *component) Build(b *composition.Builder) error {
	const op = serrors.Op("extruntime.wiring.Build")
	section, configured, err := c.section(b)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	logger := b.Context().Logger()
	if !configured || !section.Enabled {
		if logger != nil {
			logger.Info("extension runtime is disabled (EXTRUNTIME_ENABLED is not set)")
		}
		return nil
	}
	cfg, dist, err := RuntimeConfig(section, c.opts)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if logger != nil {
		logger.Infof("extension runtime is enabled: pool=%d, @iota-uz/sdk %s at %s, ui=%t, configurator=%t",
			cfg.PoolSize, dist.Identity.ReleaseVersion, dist.Dir, cfg.UI != nil, cfg.Configurator != nil)
	}
	inner, err := runtime.NewComponent(cfg)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	return inner.Build(b)
}

// section returns the configuration: the explicit one, else the section of
// the build context's source. configured is false without either.
func (c *component) section(b *composition.Builder) (extruntimeconfig.Config, bool, error) {
	if c.opts.Section != nil {
		explicit := *c.opts.Section
		return explicit, true, explicit.Validate()
	}
	bc := b.Context()
	if bc.Source() == nil {
		return extruntimeconfig.Config{}, false, nil
	}
	loaded, err := config.Register[extruntimeconfig.Config](bc.Registry())
	if err != nil {
		return extruntimeconfig.Config{}, false, err
	}
	return *loaded, true, nil
}

// RuntimeConfig translates the section into the runtime's configuration and
// verifies the installed @iota-uz/sdk distribution (the version and the
// presence of both workers), so a deployment that cannot run the workers
// fails when the application is composed, not at the first invocation. It is
// exported for applications that construct runtime.NewComponent themselves.
func RuntimeConfig(section extruntimeconfig.Config, opts Options) (runtime.Config, runtime.Dist, error) {
	const op = serrors.Op("extruntime.wiring.RuntimeConfig")
	if err := section.Validate(); err != nil {
		return runtime.Config{}, runtime.Dist{}, serrors.Wrap(op, err)
	}
	natives, err := native.Compose(opts.Natives...)
	if err != nil {
		return runtime.Config{}, runtime.Dist{}, serrors.Wrap(op, err)
	}
	distCfg := runtime.DistConfig{Dir: section.DistDir, AppRoot: section.AppRoot}
	dist, err := runtime.ResolveDist(distCfg)
	if err != nil {
		return runtime.Config{}, runtime.Dist{}, serrors.Wrap(op, err)
	}
	cacheDir := section.CacheDir
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "iota-extruntime")
	}
	cfg := runtime.Config{
		NodeBinary:        section.NodeBinary,
		Dist:              distCfg,
		PoolSize:          section.PoolSize,
		CacheDir:          cacheDir,
		Natives:           natives,
		Secrets:           opts.Secrets,
		Workflow:          runtime.WorkflowConfig{Concurrency: section.WorkflowConcurrency},
		DisableBackground: section.DisableBackground,
		Git: manage.GitConfig{
			AllowedSchemes: section.GitSchemes(), AllowedHosts: section.GitHosts(), AllowPrivateNetworks: section.Git.AllowPrivateNetworks,
			Binary: section.Git.Binary, Timeout: section.Git.Timeout,
		},
		Endpoints: runtime.EndpointConfig{
			PublicTenant: opts.PublicTenant,
			TenantByHost: section.Endpoints.PublicTenantByHost,
			MaxBodyBytes: section.Endpoints.MaxBodyBytes,
			Timeout:      section.Endpoints.Timeout,
		},
		Extensions: append([]runtime.Extension(nil), opts.Extensions...),
		Logger:     opts.Logger,
	}
	if cfg.Endpoints.PublicTenant == nil {
		if id := section.PublicTenantID(); id != uuid.Nil {
			cfg.Endpoints.PublicTenant = func(*http.Request) (uuid.UUID, bool) { return id, true }
		}
	}
	if opts.MCPRegistry != nil {
		cfg.Extensions = append(cfg.Extensions, mcptools.Extension{Registry: opts.MCPRegistry})
	}
	var hostManifest clienthost.Manifest
	if path := strings.TrimSpace(section.UI.HostManifest); path != "" {
		hostManifest, err = LoadHostManifest(path)
		if err != nil {
			return runtime.Config{}, runtime.Dist{}, serrors.Wrap(op, err)
		}
		cfg.UI = &runtime.UIConfig{Host: hostManifest, SourceTTL: section.UI.SourceTTL}
	}
	if section.Configurator.Enabled {
		cfg.Configurator = &configurator.Config{
			Host:  hostManifest,
			Path:  strings.TrimSpace(section.Configurator.Path),
			Types: typesFS(section),
		}
	}
	if opts.Tune != nil {
		if err := opts.Tune(&cfg); err != nil {
			return runtime.Config{}, runtime.Dist{}, serrors.Wrap(op, err)
		}
	}
	return cfg, dist, nil
}

// LoadHostManifest reads and validates the client-host manifest the
// application's frontend build emits: the release identity must be the Go
// SDK's, so a stale bundle fails at startup rather than in the browser.
func LoadHostManifest(path string) (clienthost.Manifest, error) {
	const op = serrors.Op("extruntime.wiring.LoadHostManifest")
	raw, err := os.ReadFile(path)
	if err != nil {
		return clienthost.Manifest{}, serrors.Wrap(op, fmt.Errorf("read client host manifest (EXTRUNTIME_UI_HOSTMANIFEST): %w", err))
	}
	m, err := clienthost.ParseManifest(raw)
	if err != nil {
		return clienthost.Manifest{}, serrors.Wrap(op, fmt.Errorf("client host manifest %s: %w", path, err))
	}
	return m, nil
}

// typesFS is the node_modules tree Monaco's type information is served from.
func typesFS(section extruntimeconfig.Config) fs.FS {
	dir := strings.TrimSpace(section.Configurator.TypesDir)
	if dir == "" {
		root := section.AppRoot
		if root == "" {
			root = "."
		}
		dir = filepath.Join(root, "node_modules")
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return nil
	}
	return os.DirFS(dir)
}
