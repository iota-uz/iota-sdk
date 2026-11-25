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
//	components = append(components, automations.NewComponent(automations.WithConnectors(myConnector)))
//
// or registers connectors on the resolved connectors.Directory before serving requests. The SDK's AI, SMS, MCP,
// email and Telegram bot connectors are registered by the module itself unless the application registered its own of the kind.
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
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	icons "github.com/iota-uz/icons/phosphor"

	"github.com/iota-uz/iota-sdk/modules/automations/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/modules/automations/presentation/attachments"
	"github.com/iota-uz/iota-sdk/modules/automations/presentation/graphs"
	"github.com/iota-uz/iota-sdk/modules/automations/presentation/integrations"
	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/modules/automations/tasks"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/notify"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/jev"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/mcp"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/openai"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/sms"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/automationconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/eskizconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/extruntimeconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/jevconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/openaiconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/smtpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/twilioconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
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

// WithUsageIndex is where connections are used. Default: a usage index the container provides, else
// the automation engine's index over graph versions (engine.Runtime.UsageIndex), else none — nothing is
// in use and deleting is never blocked.
func WithUsageIndex(u connectors.UsageIndex) Option {
	return func(c *component) { c.usage = u }
}

// WithConnectors registers connectors in the directory. Code that composes later may register more
// on the resolved connectors.Directory before the application serves requests.
func WithConnectors(list ...connectors.Connector) Option {
	return func(c *component) { c.connectors = append(c.connectors, list...) }
}

// WithoutMCP leaves out the MCP connector (mcp.New) the component registers when the application
// registers no connector of kind mcp itself.
func WithoutMCP() Option {
	return func(c *component) { c.noMCP = true }
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
	engineTune      []func(*engine.Config)
	// subjectLinks are the pages of subjects by entity (WithSubjectLink); links is their registry, built
	// over the configured origin: the run view's «Open <subject>» and a run context's subject.url.
	subjectLinks map[string]engine.SubjectURLResolver
	links        *engine.LinkRegistry
	noEngine     bool
	noBackground bool
	noMCP        bool
	noEmail      bool
	noTelegram   bool
	subjectNames tasks.SubjectResolver
}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "automations"}
}

func (c *component) LocaleFS() []*embed.FS { return []*embed.FS{&localeFiles} }

// settings is the configuration the component reads: the automation section and, for the screen's
// host bundle and the OAuth2 redirect, the extension runtime's UI and the HTTP origin.
type settingsCfg struct {
	automation automationconfig.Config
	// openai and jev are the platform keys of the AI connectors the environment configures
	// (automation.openai.*, automation.jev.*).
	openai openaiconfig.Config
	jev    jevconfig.Config
	// sms are the platform credentials of the SMS connectors the environment configures.
	sms map[catalog.ConnectorKind]connectors.PlatformCredential
	// smtp is the application's own SMTP (SMTP_*): the platform credential of the email connector's smtp kind.
	smtp smtpconfig.Config
	// uploads is where the email connector keeps the attachments of incoming messages (its private namespace).
	uploads  uploadsconfig.Config
	hostPath string
	origin   string
	// telegramToken is the platform bot of the Telegram connector (APP_TELEGRAMBOTTOKEN).
	telegramToken string
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
	openaiCfg, err := config.Register[openaiconfig.Config](reg)
	if err != nil {
		return out, err
	}
	out.openai = *openaiCfg
	jevCfg, err := config.Register[jevconfig.Config](reg)
	if err != nil {
		return out, err
	}
	out.jev = *jevCfg
	eskizCfg, err := config.Register[eskizconfig.Config](reg)
	if err != nil {
		return out, err
	}
	twilioCfg, err := config.Register[twilioconfig.Config](reg)
	if err != nil {
		return out, err
	}
	out.sms = sms.PlatformCredentials(eskizCfg, twilioCfg)
	smtpCfg, err := config.Register[smtpconfig.Config](reg)
	if err != nil {
		return out, err
	}
	out.smtp = *smtpCfg
	uploadsCfg, err := config.Register[uploadsconfig.Config](reg)
	if err != nil {
		return out, err
	}
	out.uploads = *uploadsCfg
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
	out.telegramToken = strings.TrimSpace(appCfg.TelegramBotToken)
	return out, nil
}

func (c *component) Build(b *composition.Builder) error {
	const op = serrors.Op("automations.Build")
	cfg, err := c.readConfig(b)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if c.links, err = c.newLinks(cfg.origin); err != nil {
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
	// The platform keys of the AI and SMS connectors come from their own settings; an explicit
	// platform.credentials entry of the kind wins.
	for _, creds := range []map[catalog.ConnectorKind]connectors.PlatformCredential{
		openai.PlatformCredentials(&cfg.openai), jev.PlatformCredentials(&cfg.jev), cfg.sms,
	} {
		for kind, cred := range creds {
			if _, set := platformCreds[kind]; !set {
				platformCreds[kind] = cred
			}
		}
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
		reg := catalog.NewRegistry(lazyAvailability{container: container}, catalog.WithParticipants(func(event string) []string {
			return participantsOf(container, event)
		}))
		if c.noEngine {
			return reg, nil
		}
		var natives *native.Set
		if composition.Has[*native.Set](container) {
			if natives, err = composition.Resolve[*native.Set](container); err != nil {
				return nil, err
			}
		}
		if err := registerEngineCatalog(reg, natives, slog.Default()); err != nil {
			return nil, err
		}
		if err := registerBuiltins(reg, container); err != nil {
			return nil, err
		}
		// The human node: its form annotation names a task form of the application.
		forms, err := composition.Resolve[*tasks.Forms](container)
		if err != nil {
			return nil, err
		}
		if err := reg.RegisterNode(tasks.HumanNode(forms)); err != nil {
			return nil, err
		}
		return reg, nil
	})
	// The actions of the application's native set (actions.Native) join the catalog when it starts.
	composition.ContributeHooks(b, func(container *composition.Container) ([]composition.Hook, error) {
		return []composition.Hook{actionsHook(container)}, nil
	})
	c.buildEngine(b)
	c.buildTasks(b)
	composition.ProvideDefault[connectors.Directory](b, func(container *composition.Container) (connectors.Directory, error) {
		reg, err := composition.Resolve[catalog.Registry](container)
		if err != nil {
			return nil, err
		}
		dir := connectors.NewDirectory(reg)
		registered := map[catalog.ConnectorKind]bool{}
		for _, conn := range c.connectors {
			if err := dir.Register(conn); err != nil {
				return nil, err
			}
			registered[conn.Kind()] = true
		}
		// The SDK's own AI, SMS, MCP, email and Telegram connectors, unless the application registered its own of the kind.
		status := lazyStatus{container: container}
		sdk := []connectors.Connector{
			openai.New(
				openai.WithDefaultModel(cfg.openai.DefaultModel), openai.WithConnectionStatus(status),
				openai.WithImages(services.UploadImages{Uploads: func() (services.UploadReader, error) { return resolveUploads(container) }}),
			),
			jev.New(jev.WithPlatformConfig(&cfg.jev), jev.WithConnectionStatus(status), jev.WithProviders(lazyProviders{container: container})),
			sms.NewEskiz(sms.WithConnectionStatus(status)), sms.NewTwilio(sms.WithConnectionStatus(status)),
		}
		if !c.noMCP {
			sdk = append(sdk, mcp.New())
		}
		if !c.noEmail {
			sdk = append(sdk, emailConnectors(container, cfg.smtp, cfg.uploads)...)
		}
		if !c.noTelegram {
			sdk = append(sdk, c.newTelegram(container, cfg))
		}
		for _, conn := range sdk {
			if registered[conn.Kind()] {
				continue
			}
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
		return connectors.NewPlatformCredentials(dir, withConnectorCredentials(platformCreds, dir.List())), nil
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
		st := connectors.NewStore(db, keys, usage, platform, grants)
		// A connector that refreshes its connections' tokens itself (MCP) gets the store.
		dir, err := composition.Resolve[connectors.Directory](container)
		if err != nil {
			return nil, err
		}
		connectors.BindStore(dir, st)
		return st, nil
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

	// A connector with a platform credential that needs setting up (a webhook) does it at start.
	composition.ContributeHooks(b, func(container *composition.Container) ([]composition.Hook, error) {
		return []composition.Hook{{Name: "automations.platform-connectors", Start: func(ctx context.Context) (composition.StopFn, error) {
			dir, err := composition.Resolve[connectors.Directory](container)
			if err != nil {
				return nil, err
			}
			platform, err := composition.Resolve[connectors.PlatformCredentials](container)
			if err != nil {
				return nil, err
			}
			db, dbErr := composition.Resolve[*pgxpool.Pool](container)
			warn := func(kind catalog.ConnectorKind, format string, err error) {
				if logger := b.Context().Logger(); logger != nil {
					logger.Warnf(format, kind, err)
				}
			}
			if err := startPlatformConnectors(ctx, dir, platform, db, dbErr, warn); err != nil {
				return nil, err
			}
			if dbErr != nil {
				return nil, nil //nolint:nilnil,nilerr // without the database there is no reservation to keep; the start warned
			}
			return keepPlatformLeases(dir, platform, db, warn), nil
		}}}, nil
	})

	c.contributeNotifyNodes(b, cfg.origin)

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
	// The attachments of incoming email are served by a route of the module, to the users of their tenant.
	composition.ContributeControllers(b, func(container *composition.Container) ([]application.Controller, error) {
		uploads, err := composition.Resolve[*coreservices.UploadService](container)
		if err != nil {
			return nil, nil //nolint:nilerr // an application without the SDK's upload service has no attachments to serve
		}
		return []application.Controller{attachments.NewController(uploads)}, nil
	})
	c.buildPages(b)
	if hostManifest != nil {
		composition.AddSettings(b, settings.OrganizationSection(IntegrationsSettingsPage()))
	} else if logger := b.Context().Logger(); logger != nil {
		logger.Info("automations: the integrations page is not mounted: no client host manifest (EXTRUNTIME_UI_HOSTMANIFEST or automations.WithHostManifest)")
	}
	return nil
}

// contributeNotifyNodes registers the notification nodes of graphs (notify, send_to_contact) in the catalog
// registry and their notification definition automations.notify.v1 in the core catalog. The nodes send
// through core's AudienceSender, so the core notification stack must be part of the composition.
func (c *component) contributeNotifyNodes(b *composition.Builder, origin string) {
	if u, err := url.Parse(origin); err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		origin = "" // only paths of the application are links of a notice then
	}
	notify.Contribute(b)
	composition.ContributeHooks(b, func(container *composition.Container) ([]composition.Hook, error) {
		reg, err := composition.Resolve[catalog.Registry](container)
		if err != nil {
			return nil, err
		}
		sender, err := composition.Resolve[notifications.AudienceSender](container)
		if err != nil {
			return nil, err
		}
		definitions, err := composition.Resolve[*notifications.Catalog](container)
		if err != nil {
			return nil, err
		}
		db, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		return nil, notify.Register(reg, notify.Config{Sender: sender, Catalog: definitions, Pool: db, BaseURL: origin})
	})
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

// participantsOf are the recipient keys of the notification definition of an event: the participants a notify
// node may name when a graph is triggered by it.
func participantsOf(container *composition.Container, event string) []string {
	definitions, err := composition.Resolve[*notifications.Catalog](container)
	if err != nil {
		return nil
	}
	def, ok := definitions.Get(event)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(def.RecipientKeys))
	for _, r := range def.RecipientKeys {
		keys = append(keys, r.Key)
	}
	return keys
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

// PlatformWithheld forwards catalog.PlatformOffers of the store: without it the registry never learns that a
// tenant's platform key was switched off, and a node that left its connection out would end with
// connection_unavailable instead of unavailable.
func (l lazyAvailability) PlatformWithheld(ctx context.Context, tenantID uuid.UUID) (map[catalog.ConnectorKind]bool, error) {
	st, err := composition.Resolve[connectors.Store](l.container)
	if err != nil {
		return nil, err
	}
	if po, ok := st.(catalog.PlatformOffers); ok {
		return po.PlatformWithheld(ctx, tenantID)
	}
	return map[catalog.ConnectorKind]bool{}, nil // a store that does not offer platform keys withholds none
}

// lazyProviders tells the provider of a Jev connection (its provider credential field), resolving the
// stores when it is first needed: the platform credential's from the configuration, a tenant connection's
// from its credentials.
type lazyProviders struct{ container *composition.Container }

func (l lazyProviders) Provider(ctx context.Context, tenantID uuid.UUID, connectionID string) (string, error) {
	if kind, ok := strings.CutPrefix(connectionID, catalog.PlatformPrefix); ok {
		platform, err := composition.Resolve[connectors.PlatformCredentials](l.container)
		if err != nil {
			return "", err
		}
		creds, _, ok := platform.Platform(catalog.ConnectorKind(kind))
		if !ok {
			return "", connectors.ErrNotFound
		}
		return creds["provider"], nil
	}
	id, err := uuid.Parse(connectionID)
	if err != nil {
		return "", connectors.ErrNotFound
	}
	st, err := composition.Resolve[connectors.Store](l.container)
	if err != nil {
		return "", err
	}
	_, creds, err := st.Get(ctx, tenantID, id)
	if err != nil {
		return "", err
	}
	return creds["provider"], nil
}

// resolveUploads is the uploads service of the core module, when the application has it: the files an
// LLM node's images are read from.
func resolveUploads(container *composition.Container) (services.UploadReader, error) {
	if !composition.Has[*coreservices.UploadService](container) {
		return nil, errors.New("automations: the uploads service is not available")
	}
	svc, err := composition.Resolve[*coreservices.UploadService](container)
	if err != nil {
		return nil, err
	}
	return coreUploads{svc: svc}, nil
}

// lazyStatus marks connections needs_reauth through the store, which is built after the directory.
type lazyStatus struct{ container *composition.Container }

func (l lazyStatus) SetStatus(ctx context.Context, tenantID, id uuid.UUID, st connectors.ConnectionStatus, reason string) error {
	store, err := composition.Resolve[connectors.ConnectionStore](l.container)
	if err != nil {
		return err
	}
	return store.SetStatus(ctx, tenantID, id, st, reason)
}

// messagePrefix is the namespace of the integrations screen's strings in the module's locale files.
const messagePrefix = "Automations.Integrations."

// graphsPrefix is the namespace of the automations screens' strings.
const graphsPrefix = "Automations.Graphs."

// MessageKeys are the locale keys the integrations screen reads: every key below
// Automations.Integrations of the English locale file.
func MessageKeys() ([]string, error) { return messageKeysUnder(messagePrefix) }

// GraphsMessageKeys are the locale keys the automations screens read: every key below
// Automations.Graphs of the English locale file.
func GraphsMessageKeys() ([]string, error) { return messageKeysUnder(graphsPrefix) }

func messageKeysUnder(messagePrefix string) ([]string, error) {
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
	if composition.Has[connectors.UsageIndex](container) {
		return composition.Resolve[connectors.UsageIndex](container)
	}
	// The engine's index over graph versions: a connection an enabled graph uses is not deleted.
	rt, err := c.runtime(container)
	if err != nil {
		return nil, err
	}
	if rt != nil {
		return rt.UsageIndex(), nil
	}
	return nil, nil //nolint:nilnil // no index is an outcome: usage is then empty
}

// buildPages mounts the automations section (/automations: the list and the editor) and the
// connectors' webhook endpoint when the composition runs the engine.
func (c *component) buildPages(b *composition.Builder) {
	composition.ContributeControllers(b, func(container *composition.Container) ([]application.Controller, error) {
		rt, err := c.runtime(container)
		if err != nil {
			return nil, err
		}
		if rt == nil {
			if logger := b.Context().Logger(); logger != nil {
				logger.Info("automations: the automations pages are not mounted: the composition runs no automation engine (wiring.NewComponent)")
			}
			return nil, nil
		}
		db, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		reg, err := composition.Resolve[catalog.Registry](container)
		if err != nil {
			return nil, err
		}
		store, err := composition.Resolve[connectors.ConnectionStore](container)
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
		keys, err := GraphsMessageKeys()
		if err != nil {
			return nil, err
		}
		host, assets := c.host, http.Handler(nil)
		if host == nil {
			bundle, err := graphs.DefaultBundle()
			if err == nil {
				host, assets = &bundle.Manifest, bundle.Handler
			} else if logger := b.Context().Logger(); logger != nil {
				logger.Warnf("automations: the embedded automations bundle is unavailable: %v", err)
			}
		}
		path := c.automationsPath
		if path == "" {
			path = graphs.DefaultPath
		}
		var tools mcp.ToolLister
		if conn, ok := dir.Get(mcp.Kind); ok {
			tools, _ = conn.(mcp.ToolLister)
		}
		ctrl, err := graphs.NewController(graphs.Config{
			Engine:  graphs.Engine{Service: rt.Service(), Authoring: rt.Authoring(), Operations: rt.Operations()},
			Catalog: reg, Users: persistence.NewUserNames(db), Host: host, Assets: assets, Path: path,
			IntegrationsPath: integrations.DefaultPath, MessageKeys: keys, Logger: slog.Default(), MCPTools: tools, Links: c.links,
			Options: connectionOptions{store: store, dir: dir},
		})
		if err != nil {
			return nil, err
		}
		hooks := graphs.NewHooksController(engine.InboundHandler(engine.InboundConfig{
			Connections: store, Registry: reg, Inbound: rt.Inbound(), DB: db, Connectors: dir, Platform: platform,
		}))
		out := []application.Controller{ctrl, hooks}
		inboxCtrl, err := c.inboxController(container)
		if err != nil {
			return nil, err
		}
		if inboxCtrl != nil {
			out = append(out, inboxCtrl)
		}
		return out, nil
	})
}

// platformProvider is a connector that brings a platform credential of its own kind from the application's
// configuration (the email connector: SMTP_*).
type platformProvider interface {
	PlatformCredential() (connectors.PlatformCredential, bool)
}

// withConnectorCredentials adds the platform credentials the connectors bring themselves, for a kind the
// automation configuration (creds) does not name.
func withConnectorCredentials(creds map[catalog.ConnectorKind]connectors.PlatformCredential, list []connectors.Connector) map[catalog.ConnectorKind]connectors.PlatformCredential {
	out := make(map[catalog.ConnectorKind]connectors.PlatformCredential, len(creds)+len(list))
	for k, v := range creds {
		out[k] = v
	}
	for _, c := range list {
		pp, ok := c.(platformProvider)
		if !ok {
			continue
		}
		if _, set := out[c.Kind()]; set {
			continue
		}
		if pc, ok := pp.PlatformCredential(); ok && len(pc.Credentials) > 0 {
			out[c.Kind()] = pc
		}
	}
	return out
}

// startPlatformConnectors sets up the platform credential of every connector that has one: the account it names
// is reserved against the tenants' connections (a credential whose account a tenant's connection holds is
// withdrawn; one that cannot be withdrawn stops the start, ErrPlatformNotWithdrawable), and a connector that sets its
// credential up (a webhook) does it only with the reservation in place, under the account's lock: two instances
// that start together (a rolling update) set the webhook one after the other, and a tenant's save of the account
// waits. dbErr is the failure to reach the database: no reservation can be made or checked then, so no webhook is
// set. The reservation is a lease (connectors.PlatformLease) that renewPlatformConnectors keeps.
func startPlatformConnectors(ctx context.Context, dir connectors.Directory, platform connectors.PlatformCredentials, db *pgxpool.Pool, dbErr error, warn func(kind catalog.ConnectorKind, format string, err error)) error {
	for _, conn := range dir.List() {
		starter, isStarter := conn.(connectors.PlatformStarter)
		ex, exclusive := conn.(connectors.ExclusiveIdentity)
		creds, _, has := platform.Platform(conn.Kind())
		if !has {
			// No platform credential is configured (any more): the account it held is released once no
			// instance runs with it (its lease ran out). Its webhook cannot be removed without its token,
			// which is gone with the configuration.
			if exclusive && dbErr == nil {
				if err := connectors.PrunePlatformIdentities(ctx, db, conn.Kind()); err != nil {
					warn(conn.Kind(), "automations: the platform reservations of %s were not pruned: %v", err)
				}
			}
			continue
		}
		if !exclusive {
			if isStarter {
				if err := starter.StartPlatform(ctx, creds); err != nil {
					// The application starts without it: the platform credential works for sending.
					warn(conn.Kind(), "automations: the platform credential of %s was not set up: %v", err)
				}
			}
			continue
		}
		if _, ok := platform.(connectors.PlatformWithdrawer); !ok {
			// The reservation of an exclusive account is a lease: once it ran out (the database was unreachable
			// longer than the lease) a tenant's connection may take the account, and the credential must then stop
			// being served. A set that cannot withdraw it cannot honour that: the application does not start with it.
			return fmt.Errorf("automations: the platform credential of %s: %w", conn.Kind(), ErrPlatformNotWithdrawable)
		}
		if dbErr != nil {
			if isStarter {
				// Without the reservation the webhook could replace one a tenant's connection holds: the
				// platform credential keeps working for sending, and is set up at the next start.
				warn(conn.Kind(), "automations: the platform credential of %s was not set up: %v", dbErr)
			}
			continue
		}
		// The platform credential in effect (the configuration's, else the connector's own) is the one that is
		// registered, verified and reserved: an account a tenant's connection holds cannot be the platform's
		// too, and is then not offered to tenants as a platform credential.
		var start func(context.Context) error
		if isStarter {
			start = func(ctx context.Context) error { return starter.StartPlatform(ctx, creds) }
		}
		err := connectors.StartPlatformIdentity(ctx, db, conn.Kind(), ex.ExternalID(creds), start)
		switch {
		case connectors.IsPlatformStartError(err):
			// The application starts without it: the platform credential works for sending.
			warn(conn.Kind(), "automations: the platform credential of %s was not set up: %v", err)
		case errors.Is(err, connectors.ErrIdentityTaken):
			// A tenant's connection holds the account: the credential is served to nobody.
			withdrawPlatform(platform, conn.Kind())
			warn(conn.Kind(), "automations: the platform credential of %s is withdrawn: %v", err)
			continue
		case err != nil:
			// The reservation could not be checked (the database failed): no webhook is set, and the credential keeps
			// sending; the renewal reserves it once the database answers.
			warn(conn.Kind(), "automations: the platform credential of %s was not set up: %v", err)
			continue
		}
		// Accounts a previous platform credential held are released once no instance runs with them.
		if err := connectors.PrunePlatformIdentities(ctx, db, conn.Kind()); err != nil {
			warn(conn.Kind(), "automations: the platform reservations of %s were not pruned: %v", err)
		}
	}
	return nil
}

// ErrPlatformNotWithdrawable is a platform credential of an exclusive account (a Telegram bot) in a
// connectors.PlatformCredentials that is not a connectors.PlatformWithdrawer: its reservation is a lease, and a
// credential whose account a tenant takes after the lease ran out must stop being served.
var ErrPlatformNotWithdrawable = errors.New("a platform credential of an exclusive account needs a PlatformCredentials that can withdraw it")

// withdrawPlatform takes the platform credential of a kind out of service; false when the set cannot withdraw.
func withdrawPlatform(platform connectors.PlatformCredentials, kind catalog.ConnectorKind) bool {
	w, ok := platform.(connectors.PlatformWithdrawer)
	if ok {
		w.Withdraw(kind)
	}
	return ok
}

// renewPlatformConnectors renews the lease of the platform credential's reservation of every exclusive connector
// this instance runs with, and releases the reservations whose lease ran out (accounts no instance runs with any
// more). An account a tenant's connection took meanwhile (the lease ran out while the database was unreachable)
// withdraws the credential, as at the start; one that cannot be withdrawn is logged.
func renewPlatformConnectors(ctx context.Context, dir connectors.Directory, platform connectors.PlatformCredentials, db *pgxpool.Pool, warn func(kind catalog.ConnectorKind, format string, err error)) {
	for _, conn := range dir.List() {
		ex, exclusive := conn.(connectors.ExclusiveIdentity)
		if !exclusive {
			continue
		}
		if creds, _, has := platform.Platform(conn.Kind()); has {
			err := connectors.ReservePlatformIdentity(ctx, db, conn.Kind(), ex.ExternalID(creds))
			switch {
			case errors.Is(err, connectors.ErrIdentityTaken):
				// The start refuses a set that cannot withdraw (ErrPlatformNotWithdrawable), so the credential
				// stops being served here.
				withdrawPlatform(platform, conn.Kind())
				warn(conn.Kind(), "automations: the platform credential of %s is withdrawn: %v", err)
			case err != nil:
				warn(conn.Kind(), "automations: the platform reservation of %s was not renewed: %v", err)
			}
		}
		if err := connectors.PrunePlatformIdentities(ctx, db, conn.Kind()); err != nil {
			warn(conn.Kind(), "automations: the platform reservations of %s were not pruned: %v", err)
		}
	}
}

// keepPlatformLeases renews the platform reservations every quarter of the lease until stop is called.
func keepPlatformLeases(dir connectors.Directory, platform connectors.PlatformCredentials, db *pgxpool.Pool, warn func(kind catalog.ConnectorKind, format string, err error)) composition.StopFn {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		t := time.NewTicker(connectors.PlatformLease / 4)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				renewPlatformConnectors(ctx, dir, platform, db, warn)
			}
		}
	}()
	return func(context.Context) error {
		cancel()
		<-done
		return nil
	}
}
