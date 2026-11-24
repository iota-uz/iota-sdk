// Package automations is the automations module: its permissions, its section of the settings hub and
// its pages, starting with «Integrations» (/settings/integrations), where a tenant administrator
// connects accounts to the connectors the application registered.
//
// modules.Components lists it without options. An application that registers its connectors (or
// a usage index, or a host manifest) replaces that entry with its own:
//
//	components := slices.DeleteFunc(modules.Components(), func(c composition.Component) bool {
//	    return c.Descriptor().Name == "automations"
//	})
//	components = append(components, automations.NewComponent(automations.WithConnectors(telegram.New())))
//
// or registers connectors on the resolved connectors.Directory before serving requests.
//
// The component provides the connector Directory, the catalog Registry with the built-in node types
// (set, code, http, start_automation, run_automation and the actions modules contribute through
// pkg/automation/builtin/actions), the encrypted ConnectionStore
// (keys from AUTOMATION_CONNECTIONS_KEYS, platform credentials from AUTOMATION_PLATFORM_CREDENTIALS),
// the OAuth2 helper and the host.SecretResolver the extension runtime serves secrets.get with.
package automations

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	icons "github.com/iota-uz/icons/phosphor"

	"github.com/iota-uz/iota-sdk/modules/automations/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/modules/automations/presentation/integrations"
	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/automationconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/extruntimeconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/settings"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

//go:embed presentation/locales/*.json
var localeFiles embed.FS

// SettingsPageIntegrations is the settings hub page key of the integrations page.
const SettingsPageIntegrations = "automations.integrations"

// Option configures the component.
type Option func(*component)

// WithUsageIndex is where connections are used (the automation engine's index over graph versions,
// wired by #1339). Default: a usage index the container provides, else none — nothing is in use and
// deleting is never blocked.
func WithUsageIndex(u connectors.UsageIndex) Option {
	return func(c *component) { c.usage = u }
}

// WithConnectors registers connectors in the directory. Code that composes later may register more
// on the resolved connectors.Directory before the application serves requests.
func WithConnectors(list ...connectors.Connector) Option {
	return func(c *component) { c.connectors = append(c.connectors, list...) }
}

// WithHostManifest is the application's client host bundle manifest the integrations screen is served
// with (the application merged the screen's feature catalog into its bundle). Default: the module's own
// embedded bundle, served at /assets/automations; without it, the manifest at
// EXTRUNTIME_UI_HOSTMANIFEST.
func WithHostManifest(m clienthost.Manifest) Option {
	return func(c *component) { c.host = &m }
}

// WithAutomationsPath is the path of the automations pages that uses link to (default /automations).
func WithAutomationsPath(p string) Option {
	return func(c *component) { c.automationsPath = p }
}

// NewComponent is the automations module.
func NewComponent(opts ...Option) composition.Component {
	c := &component{}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

type component struct {
	usage           connectors.UsageIndex
	connectors      []connectors.Connector
	host            *clienthost.Manifest
	automationsPath string
}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "automations"}
}

func (c *component) LocaleFS() []*embed.FS { return []*embed.FS{&localeFiles} }

// settings is the configuration the component reads: the automation section and, for the screen's
// host bundle and the OAuth2 redirect, the extension runtime's UI and the HTTP origin.
type settingsCfg struct {
	automation automationconfig.Config
	hostPath   string
	origin     string
}

func (c *component) readConfig(b *composition.Builder) (settingsCfg, error) {
	var out settingsCfg
	bc := b.Context()
	if bc.Source() == nil {
		return out, nil
	}
	reg := config.NewRegistry(bc.Source())
	a, err := config.Register[automationconfig.Config](reg)
	if err != nil {
		return out, err
	}
	out.automation = *a
	ext, err := config.Register[extruntimeconfig.Config](reg)
	if err != nil {
		return out, err
	}
	out.hostPath = strings.TrimSpace(ext.UI.HostManifest)
	// The OAuth2 redirect is built from the configured origin, never from request headers alone.
	httpCfg, err := config.Register[httpconfig.Config](reg)
	if err != nil {
		return out, err
	}
	appCfg, err := config.Register[appconfig.Config](reg)
	if err != nil {
		return out, err
	}
	out.origin = httpCfg.Origin(appCfg)
	return out, nil
}

func (c *component) Build(b *composition.Builder) error {
	const op = serrors.Op("automations.Build")
	cfg, err := c.readConfig(b)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	keys, err := cfg.automation.Keyring()
	if err != nil && !errors.Is(err, connectors.ErrNotConfigured) {
		return serrors.Wrap(op, err)
	}
	platformCreds, err := cfg.automation.PlatformCredentials()
	if err != nil {
		return serrors.Wrap(op, err)
	}
	hostManifest := c.host
	var assets http.Handler
	if hostManifest == nil {
		bundle, err := integrations.DefaultBundle()
		if err == nil {
			hostManifest, assets = &bundle.Manifest, bundle.Handler
		} else if logger := b.Context().Logger(); logger != nil {
			logger.Warnf("automations: the embedded integrations bundle is unavailable: %v", err)
		}
	}
	if hostManifest == nil && cfg.hostPath != "" {
		m, err := loadManifest(cfg.hostPath)
		if err != nil {
			return serrors.Wrap(op, err)
		}
		hostManifest = &m
	}

	// The registry's availability is the store, which is built after the directory that registers into
	// the registry: it is resolved on the first catalog read.
	// The built-in node types are in every tenant's catalog; an application that provides its own
	// registry registers them with builtin.Register.
	composition.ProvideDefault[catalog.Registry](b, func(container *composition.Container) (catalog.Registry, error) {
		reg := catalog.NewRegistry(lazyAvailability{container: container})
		if err := registerBuiltins(reg, container); err != nil {
			return nil, err
		}
		return reg, nil
	})
	// The actions of the application's native set (actions.Native) join the catalog when it starts.
	composition.ContributeHooks(b, func(container *composition.Container) ([]composition.Hook, error) {
		return []composition.Hook{actionsHook(container)}, nil
	})
	composition.ProvideDefault[connectors.Directory](b, func(container *composition.Container) (connectors.Directory, error) {
		reg, err := composition.Resolve[catalog.Registry](container)
		if err != nil {
			return nil, err
		}
		dir := connectors.NewDirectory(reg)
		for _, conn := range c.connectors {
			if err := dir.Register(conn); err != nil {
				return nil, err
			}
		}
		return dir, nil
	})
	composition.ProvideDefault[connectors.PlatformCredentials](b, func(container *composition.Container) (connectors.PlatformCredentials, error) {
		dir, err := composition.Resolve[connectors.Directory](container)
		if err != nil {
			return nil, err
		}
		return connectors.NewPlatformCredentials(dir, platformCreds), nil
	})
	composition.ProvideDefault[connectors.PlatformGrants](b, func(container *composition.Container) (connectors.PlatformGrants, error) {
		db, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		return connectors.NewPlatformGrants(db), nil
	})
	composition.ProvideDefault[connectors.Store](b, func(container *composition.Container) (connectors.Store, error) {
		db, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		usage, err := c.usageIndex(container)
		if err != nil {
			return nil, err
		}
		platform, err := composition.Resolve[connectors.PlatformCredentials](container)
		if err != nil {
			return nil, err
		}
		grants, err := composition.Resolve[connectors.PlatformGrants](container)
		if err != nil {
			return nil, err
		}
		return connectors.NewStore(db, keys, usage, platform, grants), nil
	})
	composition.Provide[connectors.ConnectionStore](b, func(container *composition.Container) (connectors.ConnectionStore, error) {
		return composition.Resolve[connectors.Store](container)
	})
	composition.Provide[connectors.CredentialSource](b, func(container *composition.Container) (connectors.CredentialSource, error) {
		return composition.Resolve[connectors.Store](container)
	})
	composition.ProvideDefault[connectors.OAuth2](b, func(container *composition.Container) (connectors.OAuth2, error) {
		st, err := composition.Resolve[connectors.Store](container)
		if err != nil {
			return nil, err
		}
		dir, err := composition.Resolve[connectors.Directory](container)
		if err != nil {
			return nil, err
		}
		platform, err := composition.Resolve[connectors.PlatformCredentials](container)
		if err != nil {
			return nil, err
		}
		return connectors.NewOAuth2(st, dir, platform)
	})
	// The extension runtime resolves the resolver when its configuration names none: package secrets
	// "<connection name>.<field>" come from the tenant's connections.
	composition.ProvideDefault[host.SecretResolver](b, func(container *composition.Container) (host.SecretResolver, error) {
		st, err := composition.Resolve[connectors.Store](container)
		if err != nil {
			return nil, err
		}
		dir, err := composition.Resolve[connectors.Directory](container)
		if err != nil {
			return nil, err
		}
		return connectors.SecretResolver(st, dir, nil), nil
	})
	// Required dependencies are the constructor's parameters; only the optional usage index is looked
	// up through the container.
	composition.ProvideFunc(b, func(
		container *composition.Container,
		db *pgxpool.Pool,
		dir connectors.Directory,
		st connectors.Store,
		platform connectors.PlatformCredentials,
		grants connectors.PlatformGrants,
		oauth connectors.OAuth2,
	) (*services.IntegrationsService, error) {
		usage, err := c.usageIndex(container)
		if err != nil {
			return nil, err
		}
		return services.NewIntegrationsService(services.IntegrationsDeps{
			Directory: dir, Store: st, Usage: usage, Platform: platform, Grants: grants, OAuth: oauth,
			States: persistence.NewOAuthStates(db), Checks: persistence.NewConnectionChecks(db), Requests: persistence.NewRequests(db),
		}), nil
	})

	if !b.Context().HasCapability(composition.CapabilityAPI) {
		return nil
	}
	messageKeys, err := MessageKeys()
	if err != nil {
		return serrors.Wrap(op, err)
	}
	composition.ContributeControllersFunc(b, func(svc *services.IntegrationsService) ([]application.Controller, error) {
		ctrl, err := integrations.NewController(integrations.Config{
			Service: svc, Host: hostManifest, Assets: assets, AutomationsPath: c.automationsPath, Configured: keys != nil,
			Origin: cfg.origin, MessageKeys: messageKeys, Logger: slog.Default(),
		})
		if err != nil {
			return nil, err
		}
		return []application.Controller{ctrl}, nil
	})
	if hostManifest != nil {
		composition.AddSettings(b, settings.OrganizationSection(IntegrationsSettingsPage()))
	} else if logger := b.Context().Logger(); logger != nil {
		logger.Info("automations: the integrations page is not mounted: no client host manifest (EXTRUNTIME_UI_HOSTMANIFEST or automations.WithHostManifest)")
	}
	return nil
}

// IntegrationsSettingsPage is the settings hub entry of the integrations page.
func IntegrationsSettingsPage() types.SettingsPage {
	return types.SettingsPage{
		Key:         SettingsPageIntegrations,
		Name:        "Settings.Hub.Pages.Integrations.Name",
		Description: "Settings.Hub.Pages.Integrations.Summary",
		Href:        integrations.DefaultPath,
		Icon:        icons.PlugsConnected(icons.Props{Size: "20"}),
		Order:       60,
		Permissions: []permission.Permission{permissions.ConnectionsManage},
	}
}

func loadManifest(path string) (clienthost.Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return clienthost.Manifest{}, serrors.Wrap("automations.loadManifest", err)
	}
	return clienthost.ParseManifest(raw)
}

// lazyAvailability resolves the connection store when the catalog is first read.
type lazyAvailability struct{ container *composition.Container }

func (l lazyAvailability) Connected(ctx context.Context, tenantID uuid.UUID) (map[catalog.ConnectorKind][]catalog.ConnectionRef, error) {
	st, err := composition.Resolve[connectors.Store](l.container)
	if err != nil {
		return nil, err
	}
	return st.Connected(ctx, tenantID)
}

// messagePrefix is the namespace of the screen's strings in the module's locale files.
const messagePrefix = "Automations.Integrations."

// MessageKeys are the locale keys the integrations screen reads: every key below
// Automations.Integrations of the English locale file.
func MessageKeys() ([]string, error) {
	raw, err := localeFiles.ReadFile("presentation/locales/en.json")
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	var keys []string
	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				p := k
				if prefix != "" {
					p = prefix + "." + k
				}
				walk(p, child)
			}
		case string:
			if strings.HasPrefix(prefix, messagePrefix) {
				keys = append(keys, prefix)
			}
		}
	}
	walk("", doc)
	sort.Strings(keys)
	return keys, nil
}

// usageIndex is the index given with WithUsageIndex, else the one the container provides, else nil
// (no usage: "used in" is empty and deletion is not blocked). A provided index that fails to build
// fails the build: a nested missing dependency must not read as "no index" and unblock deletion.
func (c *component) usageIndex(container *composition.Container) (connectors.UsageIndex, error) {
	if c.usage != nil {
		return c.usage, nil
	}
	if !composition.Has[connectors.UsageIndex](container) {
		return nil, nil //nolint:nilnil // no index is an outcome: usage is then empty
	}
	return composition.Resolve[connectors.UsageIndex](container)
}
