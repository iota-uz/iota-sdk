package composition

import (
	"strings"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/bichatconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/googleoauthconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/meiliconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/oidcconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/paymentsconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/ratelimitconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/redisconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/smtpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/telemetryconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/twilioconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/twofactorconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

// installAutoProviders registers providers for the core services already
// available on the build context's Application handle. Components can then
// take these as typed parameters in ProvideFunc / ContributeControllersFunc
// constructors, or call composition.Resolve[T] from inside a Contribute*
// closure — no dedicated accessor is needed.
//
// Each provider is registered only when the underlying value is non-nil.
// User providers always win: if a component declares its own provider for
// one of these keys, addBuilder drops the auto provider and installs the
// user one in its place.
//
// The set is intentionally small: only services that genuinely live one
// level above any component and are stable across the engine's lifetime.
func installAutoProviders(container *Container, ctx BuildContext) {
	if container == nil {
		return
	}

	// Application itself — supports the few legitimate cases (GraphQL
	// resolver root, the chat controller's websocket usage, etc.).
	if app := ctx.app; app != nil {
		registerAutoValue[application.Application](container, "auto:application", app)

		if ws := app.Websocket(); ws != nil {
			registerAutoValue[application.Huber](container, "auto:websocket", ws)
		}
		if sl := app.Spotlight(); sl != nil {
			registerAutoValue[spotlight.Service](container, "auto:spotlight", sl)
		}
	}

	if pool := ctx.db; pool != nil {
		registerAutoValue[*pgxpool.Pool](container, "auto:db", pool)
	}
	if bus := ctx.eventPublisher; bus != nil {
		registerAutoValue[eventbus.EventBus](container, "auto:eventbus", bus)
	}
	if bundle := ctx.bundle; bundle != nil {
		registerAutoValue[*i18n.Bundle](container, "auto:bundle", bundle)
	}
	if logger := ctx.logger; logger != nil {
		registerAutoValue[*logrus.Logger](container, "auto:logger", logger)
	}

	// Typed stdconfig auto-registration from config.Source.
	// When no Source is attached, consumers must call composition.ProvideConfig[T] explicitly.
	if src := ctx.source; src != nil {
		installStdconfigFromSource(container, src)
	}
}

// installStdconfigFromSource populates all stdconfig types from the new
// config.Source, using config.Register for unmarshal + optional Validate.
// Each registered *T is placed into the container under the pointer key so
// constructors can receive it directly.
func installStdconfigFromSource(container *Container, src config.Source) {
	reg := config.NewRegistry(src)

	if ptr, err := config.Register[dbconfig.Config](reg, "db"); err == nil {
		registerAutoValue[*dbconfig.Config](container, "auto:dbconfig", ptr)
	}
	if ptr, err := config.Register[httpconfig.Config](reg, "http"); err == nil {
		registerAutoValue[*httpconfig.Config](container, "auto:httpconfig", ptr)
	}
	if ptr, err := config.Register[smtpconfig.Config](reg, "smtp"); err == nil {
		registerAutoValue[*smtpconfig.Config](container, "auto:smtpconfig", ptr)
	}
	if ptr, err := config.Register[twilioconfig.Config](reg, "twilio"); err == nil {
		registerAutoValue[*twilioconfig.Config](container, "auto:twilioconfig", ptr)
	}
	if ptr, err := config.Register[oidcconfig.Config](reg, "oidc"); err == nil {
		registerAutoValue[*oidcconfig.Config](container, "auto:oidcconfig", ptr)
	}
	if ptr, err := config.Register[googleoauthconfig.Config](reg, "google"); err == nil {
		registerAutoValue[*googleoauthconfig.Config](container, "auto:googleoauthconfig", ptr)
	}
	if ptr, err := config.Register[ratelimitconfig.Config](reg, "ratelimit"); err == nil {
		registerAutoValue[*ratelimitconfig.Config](container, "auto:ratelimitconfig", ptr)
	}
	if ptr, err := config.Register[twofactorconfig.Config](reg, "twofactor"); err == nil {
		registerAutoValue[*twofactorconfig.Config](container, "auto:twofactorconfig", ptr)
	}
	if ptr, err := config.Register[telemetryconfig.Config](reg, "telemetry"); err == nil {
		registerAutoValue[*telemetryconfig.Config](container, "auto:telemetryconfig", ptr)
	}
	if ptr, err := config.Register[uploadsconfig.Config](reg, "uploads"); err == nil {
		registerAutoValue[*uploadsconfig.Config](container, "auto:uploadsconfig", ptr)
	}
	if ptr, err := config.Register[redisconfig.Config](reg, "redis"); err == nil {
		registerAutoValue[*redisconfig.Config](container, "auto:redisconfig", ptr)
	}
	if ptr, err := config.Register[meiliconfig.Config](reg, "meili"); err == nil {
		registerAutoValue[*meiliconfig.Config](container, "auto:meiliconfig", ptr)
	}
	if ptr, err := config.Register[paymentsconfig.Config](reg, "payments"); err == nil {
		registerAutoValue[*paymentsconfig.Config](container, "auto:paymentsconfig", ptr)
	}
	if ptr, err := config.Register[appconfig.Config](reg, "app"); err == nil {
		registerAutoValue[*appconfig.Config](container, "auto:appconfig", ptr)
	}
	if ptr, err := config.Register[bichatconfig.Config](reg, "bichat"); err == nil {
		registerAutoValue[*bichatconfig.Config](container, "auto:bichatconfig", ptr)
	}
}

// registerAutoValue installs a value-form provider for T directly on the
// container. Resolved upfront so factory closure machinery is bypassed. The
// `auto:` prefix on sourceName flags the entry as overridable by user
// providers via isAutoProvider.
//
// If a provider for T already exists (e.g. a user component registered its
// own before installAutoProviders ran, or a prior auto-registration touched
// the same key), registerAutoValue is a no-op. This preserves the
// "user-wins" contract regardless of call order: auto providers never
// clobber a pre-existing entry.
func registerAutoValue[T any](container *Container, sourceName string, value T) {
	key := keyFor(typeOf[T](), "")
	if _, exists := container.providers[key]; exists {
		return
	}
	entry := &providerEntry{
		key:           key,
		componentName: sourceName,
		displayName:   key.DisplayName(),
		factory: func(*Container) (any, error) {
			return value, nil
		},
		state: providerResolved,
		value: value,
	}
	container.providers[key] = entry
	container.providerOrder = append(container.providerOrder, entry)
}

// isAutoProvider reports whether an existing provider entry was installed by
// installAutoProviders. Used by addBuilder to allow user providers to override
// auto-injected core services without raising "duplicate provider".
func isAutoProvider(entry *providerEntry) bool {
	if entry == nil {
		return false
	}
	return strings.HasPrefix(entry.componentName, "auto:")
}
