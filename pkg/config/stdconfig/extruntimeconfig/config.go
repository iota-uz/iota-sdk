// Package extruntimeconfig provides typed configuration for the extension
// runtime (pkg/extruntime). It is a stdconfig package intended to be
// registered via config.Register[extruntimeconfig.Config].
//
// The runtime is off unless Enabled is set: the SDK server then behaves
// exactly as it does without the runtime. The type is a pure value; the
// translation into runtime.Config lives in pkg/extruntime/wiring.
package extruntimeconfig

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// UIConfig groups the browser side of the runtime: the page host, the
// package pages and the browser RPC (/_ext/rpc).
//
// Env prefix: "extruntime.ui" (EXTRUNTIME_UI_HOSTMANIFEST, EXTRUNTIME_UI_SOURCETTL).
type UIConfig struct {
	// HostManifest is the path of the client-host manifest JSON (a
	// clienthost.Manifest) that the application's frontend build emits next to
	// its host bundle. Setting it mounts the UI slice: package pages,
	// navigation, /_ext/rpc and the /_ext/assets route. Empty leaves the UI
	// slice, and with it the browser RPC, out.
	HostManifest string `koanf:"hostmanifest"`
	// SourceTTL is how long the snapshot of activated UI manifests is cached.
	SourceTTL time.Duration `koanf:"sourcettl" default:"2s"`
}

// ConfiguratorConfig groups the extension configurator page.
//
// Env prefix: "extruntime.configurator" (EXTRUNTIME_CONFIGURATOR_ENABLED, ...).
type ConfiguratorConfig struct {
	// Enabled mounts the configurator page (Monaco editor, versions,
	// workflows). It is a Solid screen of the application's client host, so it
	// requires UI.HostManifest and a host bundle that includes the SDK's
	// solidFeatures.
	Enabled bool `koanf:"enabled"`
	// Path is where the page is mounted. Default "/extensions".
	Path string `koanf:"path"`
	// TypesDir is a node_modules-shaped directory that gives Monaco the types
	// of @iota-uz/sdk and solid-js. Default: <AppRoot>/node_modules when it
	// exists; without it the editor has the developer kit only.
	TypesDir string `koanf:"typesdir"`
}

// EndpointsConfig groups the package HTTP endpoints (/_ext/endpoints).
//
// Env prefix: "extruntime.endpoints" (EXTRUNTIME_ENDPOINTS_PUBLICTENANT, ...).
type EndpointsConfig struct {
	// PublicTenant is the tenant ID every request without a session (a
	// webhook) acts in: the single-tenant deployment.
	PublicTenant string `koanf:"publictenant"`
	// PublicTenantByHost resolves the tenant of such a request from the Host
	// header against tenants.domain: the multi-tenant deployment. It is
	// exclusive with PublicTenant. Without either, a public endpoint answers
	// 503 unless the application middleware put a tenant into the context.
	PublicTenantByHost bool `koanf:"publictenantbyhost"`
	// MaxBodyBytes bounds a request body. Default 1 MiB.
	MaxBodyBytes int64 `koanf:"maxbodybytes"`
	// Timeout bounds one endpoint invocation. Default 30s.
	Timeout time.Duration `koanf:"timeout"`
}

// GitConfig bounds repository imports of packages.
//
// Env prefix: "extruntime.git" (EXTRUNTIME_GIT_ALLOWEDSCHEMES, ...).
type GitConfig struct {
	// AllowedSchemes is the comma-separated list of the transports an import
	// may use: https, ssh and file. Default "https,ssh". "file" lets a caller
	// read any repository on the server; enable it only for trusted setups.
	AllowedSchemes string `koanf:"allowedschemes"`
	// AllowedHosts is the comma-separated list of hosts https and ssh imports
	// may use: an exact name ("git.example.com") or "*.example.com" for any
	// subdomain. Empty: any host whose addresses are public. Set it in
	// production so the import API cannot be pointed at arbitrary servers.
	AllowedHosts string `koanf:"allowedhosts"`
	// AllowPrivateNetworks lets https and ssh imports reach loopback,
	// link-local (cloud metadata), private and other non-public addresses.
	// Default false: the host name is resolved and every address checked, and
	// the transport connects to the addresses that were checked.
	AllowPrivateNetworks bool `koanf:"allowprivatenetworks"`
	// Binary is the git executable. Default "git".
	Binary string `koanf:"binary"`
	// Timeout bounds one import. Default 2m.
	Timeout time.Duration `koanf:"timeout"`
}

// Config holds the extension runtime settings.
//
// Env prefix: "extruntime" (EXTRUNTIME_ENABLED → extruntime.enabled,
// EXTRUNTIME_POOLSIZE → extruntime.poolsize, ...).
type Config struct {
	// Enabled turns the runtime on. Defaults to false.
	Enabled bool `koanf:"enabled"`
	// NodeBinary is the Node executable of the workers. Default "node".
	NodeBinary string `koanf:"nodebinary" default:"node"`
	// PoolSize is the number of Node worker processes serving invocations.
	// Default 2; valid range 1-64.
	PoolSize int `koanf:"poolsize" default:"2"`
	// AppRoot is the application's JavaScript root: the directory holding its
	// package.json and node_modules/@iota-uz/sdk. Default: the working
	// directory.
	AppRoot string `koanf:"approot"`
	// DistDir is the @iota-uz/sdk package directory the workers are taken from,
	// for deployments whose node_modules is not under AppRoot. Default:
	// $IOTA_SDK_DIST, else <AppRoot>/node_modules/@iota-uz/sdk. Its version
	// must be the Go SDK's release.
	DistDir string `koanf:"distdir"`
	// CacheDir is where artifacts are materialized for the workers and the UI
	// asset route. It is a cache rebuilt from the database, so it does not
	// need to persist. It is a private (0700) directory of the user running the
	// service. Default: <os.UserCacheDir()>/iota-extruntime, or
	// <os temp dir>/iota-extruntime-<uid> without a user cache directory.
	CacheDir string `koanf:"cachedir"`
	// WorkflowConcurrency is the number of workflow instances the durable
	// engine runs at once. Default: the engine's own.
	WorkflowConcurrency int `koanf:"workflowconcurrency"`
	// DisableBackground keeps the workflow engine, the event dispatcher and
	// the scheduler of this process off, for API replicas when a dedicated
	// worker process runs them. The management API, endpoints and RPC are
	// unaffected.
	DisableBackground bool `koanf:"disablebackground"`

	UI           UIConfig           `koanf:"ui"`
	Configurator ConfiguratorConfig `koanf:"configurator"`
	Endpoints    EndpointsConfig    `koanf:"endpoints"`
	Git          GitConfig          `koanf:"git"`
}

// ConfigPrefix returns the koanf prefix for extruntimeconfig ("extruntime").
func (Config) ConfigPrefix() string { return "extruntime" }

// Validate checks the settings. It is skipped while the runtime is disabled.
// Implements config.Validatable so config.Register invokes it automatically.
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.PoolSize < 1 || c.PoolSize > 64 {
		return fmt.Errorf("extruntimeconfig: poolsize must be between 1 and 64, got %d", c.PoolSize)
	}
	if strings.TrimSpace(c.NodeBinary) == "" {
		return fmt.Errorf("extruntimeconfig: nodebinary must not be empty")
	}
	if c.WorkflowConcurrency < 0 {
		return fmt.Errorf("extruntimeconfig: workflowconcurrency must not be negative, got %d", c.WorkflowConcurrency)
	}
	if c.Configurator.Enabled && strings.TrimSpace(c.UI.HostManifest) == "" {
		return fmt.Errorf("extruntimeconfig: configurator.enabled requires ui.hostmanifest (EXTRUNTIME_UI_HOSTMANIFEST): the configurator is a screen of the client host")
	}
	if p := strings.TrimSpace(c.Configurator.Path); p != "" && !strings.HasPrefix(p, "/") {
		return fmt.Errorf("extruntimeconfig: configurator.path must start with \"/\", got %q", p)
	}
	if t := strings.TrimSpace(c.Endpoints.PublicTenant); t != "" {
		id, err := uuid.Parse(t)
		if err != nil || id == uuid.Nil {
			return fmt.Errorf("extruntimeconfig: endpoints.publictenant must be a tenant UUID, got %q", t)
		}
		if c.Endpoints.PublicTenantByHost {
			return fmt.Errorf("extruntimeconfig: endpoints.publictenant and endpoints.publictenantbyhost are exclusive")
		}
	}
	if c.Endpoints.MaxBodyBytes < 0 {
		return fmt.Errorf("extruntimeconfig: endpoints.maxbodybytes must not be negative, got %d", c.Endpoints.MaxBodyBytes)
	}
	for _, s := range c.GitSchemes() {
		switch s {
		case "https", "ssh", "file":
		default:
			return fmt.Errorf("extruntimeconfig: git.allowedschemes: unsupported scheme %q (https, ssh or file)", s)
		}
	}
	for _, h := range c.GitHosts() {
		name := strings.TrimPrefix(h, "*.")
		if name == "" || strings.ContainsAny(name, "*/:@ ") {
			return fmt.Errorf("extruntimeconfig: git.allowedhosts: %q is not a host name (git.example.com or *.example.com)", h)
		}
	}
	return nil
}

// GitSchemes returns the allowed import schemes, lower-cased; nil selects the
// service's default.
func (c *Config) GitSchemes() []string {
	var out []string
	for _, s := range strings.Split(c.Git.AllowedSchemes, ",") {
		if s = strings.ToLower(strings.TrimSpace(s)); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// GitHosts returns the allowed import hosts, lower-cased; nil allows any host
// with public addresses.
func (c *Config) GitHosts() []string {
	var out []string
	for _, h := range strings.Split(c.Git.AllowedHosts, ",") {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			out = append(out, h)
		}
	}
	return out
}

// PublicTenantID returns the configured tenant of public endpoints, or
// uuid.Nil when none is set. Validate has checked the format.
func (c *Config) PublicTenantID() uuid.UUID {
	id, err := uuid.Parse(strings.TrimSpace(c.Endpoints.PublicTenant))
	if err != nil {
		return uuid.Nil
	}
	return id
}
