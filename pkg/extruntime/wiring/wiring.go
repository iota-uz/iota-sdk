// Package wiring composes the extension core of an iota-sdk application and
// turns the extension runtime on from configuration. It is the composition
// root the SDK's own server binaries (cmd/server, cmd/worker) and
// applications use:
//
//	components = append(components, wiring.NewComponent(wiring.Options{
//	    Natives: append(modules.Natives(), app.Natives),
//	}))
//
// The core is always on and needs no Node: the native.Set composed from the
// contributors, the transactional outbox and its one dispatcher (with
// pruning), the host.SubjectLoader consumers resolve run-as subjects with,
// and the one *workflow.Engine of the process, which runs the Go
// implementations of workflow contracts. Modules and applications register Go
// consumers of the outbox with ContributeConsumers and native workflows with
// ContributeNativeWorkflows. The dispatcher and the engine run in Start in a
// process with the API or the worker capability, unless DisableBackground is
// set; a process with neither capability drains no events or workflow runs.
//
// The component reads the extruntimeconfig section (EXTRUNTIME_* environment
// variables) from the config source of the build context. While the section
// is disabled, which is the default, the Node runtime is left out. When it is
// enabled the component also composes runtime.NewComponent, whose package
// subscriptions become consumers of the core dispatcher: the native set from the
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
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/extruntimeconfig"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/configurator"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
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
	// Events tunes the core's outbox dispatcher (polling, batching, retries,
	// pruning); DB, Consumers and Subjects are set by the core. A zero
	// Retention takes the section's EXTRUNTIME_EVENTS_RETENTION.
	Events events.Config
	// Schedules tunes the core's scheduler (polling, missed-slot window,
	// retries); DB, Runner, Schedules and Subjects are set by the core. With
	// the runtime enabled, Tune's Config.Schedules fills the zero settings.
	Schedules events.SchedulerConfig
	// Logger is the runtime's structured logger. Default slog.Default().
	Logger *slog.Logger
}

// NewComponent returns the composition component of the extension core,
// which also enables the runtime when its configuration says so.
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
	natives, err := native.Compose(c.opts.Natives...)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	logger := b.Context().Logger()
	core := coreConfig{
		retention: section.Events.Retention, events: c.opts.Events, schedules: c.opts.Schedules, disableBackground: section.DisableBackground,
		workflowConcurrency: section.WorkflowConcurrency,
	}
	if !configured || !section.Enabled {
		c.buildCore(b, natives, core)
		c.buildWorkflows(b, natives, core, false)
		if logger != nil {
			logger.Info("extension runtime is disabled (EXTRUNTIME_ENABLED is not set); the event core and native workflows run without it")
		}
		startCore(b, core)
		startWorkflows(b, core)
		return nil
	}
	cfg, dist, err := runtimeConfig(section, c.opts, natives)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	// Tune may have set the dispatcher settings and DisableBackground on the
	// runtime configuration: the core's dispatcher is the one that applies them.
	core.events = mergeEvents(core.events, cfg.Events)
	core.schedules = mergeSchedules(core.schedules, cfg.Schedules)
	core.disableBackground = cfg.DisableBackground
	core.workflowConcurrency = cfg.Workflow.Concurrency
	c.buildCore(b, natives, core)
	c.buildWorkflows(b, natives, core, true)
	if logger != nil {
		logger.Infof("extension runtime is enabled: pool=%d, @iota-uz/sdk %s at %s, ui=%t, configurator=%t",
			cfg.PoolSize, dist.Identity.ReleaseVersion, dist.Dir, cfg.UI != nil, cfg.Configurator != nil)
	}
	inner, err := runtime.NewComponent(cfg)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if err := inner.Build(b); err != nil {
		return err
	}
	startCore(b, core)
	startWorkflows(b, core)
	return nil
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

// DefaultCacheDir is the artifact cache when none is configured: a directory
// of the user's own cache (os.UserCacheDir), never a name in the shared
// temporary directory that another local user could create first. Without a
// user cache directory (no home) it is a per-user name below the temporary
// directory; the store refuses a directory another user owns.
func DefaultCacheDir() string {
	if base, err := os.UserCacheDir(); err == nil && base != "" {
		return filepath.Join(base, "iota-extruntime")
	}
	return filepath.Join(os.TempDir(), "iota-extruntime-"+strconv.Itoa(os.Geteuid()))
}

// RuntimeConfig translates the section into the runtime's configuration and
// verifies the installed @iota-uz/sdk distribution (the version and the
// presence of both workers), so a deployment that cannot run the workers
// fails when the application is composed, not at the first invocation. It is
// exported for applications that construct runtime.NewComponent themselves.
func RuntimeConfig(section extruntimeconfig.Config, opts Options) (runtime.Config, runtime.Dist, error) {
	natives, err := native.Compose(opts.Natives...)
	if err != nil {
		return runtime.Config{}, runtime.Dist{}, serrors.Wrap("extruntime.wiring.RuntimeConfig", err)
	}
	return runtimeConfig(section, opts, natives)
}

func runtimeConfig(section extruntimeconfig.Config, opts Options, natives *native.Set) (runtime.Config, runtime.Dist, error) {
	const op = serrors.Op("extruntime.wiring.RuntimeConfig")
	if err := section.Validate(); err != nil {
		return runtime.Config{}, runtime.Dist{}, serrors.Wrap(op, err)
	}
	distCfg := runtime.DistConfig{Dir: section.DistDir, AppRoot: section.AppRoot}
	dist, err := runtime.ResolveDist(distCfg)
	if err != nil {
		return runtime.Config{}, runtime.Dist{}, serrors.Wrap(op, err)
	}
	cacheDir := section.CacheDir
	if cacheDir == "" {
		cacheDir = DefaultCacheDir()
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
		TrustedOrigins:    section.TrustedOriginList(),
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
