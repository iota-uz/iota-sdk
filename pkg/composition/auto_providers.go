package composition

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/bichatconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/googleoauthconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/cookies"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/headers"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/pagination"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/session"
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
	"github.com/iota-uz/iota-sdk/pkg/health"
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
func installAutoProviders(container *Container, ctx BuildContext) error {
	if container == nil {
		return nil
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

	// Expose the shared health.CapabilityRegistry as an auto-provider so
	// controllers (system_info) can read the gate-emitted probes via DI.
	// Lazy-init on access via (&ctx).CapabilityRegistry() ensures we always
	// hand out the same instance that gate helpers wrote into.
	registerAutoValue[health.CapabilityRegistry](container, "auto:capabilityregistry", (&ctx).CapabilityRegistry())

	// Typed stdconfig auto-registration from config.Source.
	// When no Source is attached, consumers must call composition.ProvideConfig[T] explicitly.
	if src := ctx.source; src != nil {
		if err := installStdconfigFromSource(container, &ctx); err != nil {
			return err
		}
	}
	return nil
}

// registerAuto registers a Prefixed config type T into the registry, then places
// the result into the container. Returns an error if registration fails so the
// caller can surface configuration problems rather than silently skipping them.
func registerAuto[T config.Prefixed](r *config.Registry, container *Container, name string) error {
	ptr, err := config.Register[T](r)
	if err != nil {
		return fmt.Errorf("auto register %T: %w", *new(T), err)
	}
	registerAutoValue[*T](container, name, ptr)
	return nil
}

// installStdconfigFromSource populates all stdconfig types from the attached
// config.Source, using the BuildContext's shared registry (so component gate
// helpers see the same entries) and placing each *T into the container under
// the pointer key for constructor auto-wiring.
// Returns the first registration error encountered, if any.
func installStdconfigFromSource(container *Container, ctx *BuildContext) error {
	reg := ctx.Registry()

	registrations := []func() error{
		func() error { return registerAuto[dbconfig.Config](reg, container, "auto:dbconfig") },
		func() error { return registerAuto[httpconfig.Config](reg, container, "auto:httpconfig") },
		func() error { return registerAuto[headers.Config](reg, container, "auto:headersconfig") },
		func() error { return registerAuto[cookies.Config](reg, container, "auto:cookiesconfig") },
		func() error { return registerAuto[session.Config](reg, container, "auto:sessionconfig") },
		func() error { return registerAuto[pagination.Config](reg, container, "auto:paginationconfig") },
		func() error { return registerAuto[smtpconfig.Config](reg, container, "auto:smtpconfig") },
		func() error { return registerAuto[twilioconfig.Config](reg, container, "auto:twilioconfig") },
		func() error { return registerAuto[oidcconfig.Config](reg, container, "auto:oidcconfig") },
		func() error {
			return registerAuto[googleoauthconfig.Config](reg, container, "auto:googleoauthconfig")
		},
		func() error {
			return registerAuto[ratelimitconfig.Config](reg, container, "auto:ratelimitconfig")
		},
		func() error {
			return registerAuto[twofactorconfig.Config](reg, container, "auto:twofactorconfig")
		},
		func() error {
			return registerAuto[telemetryconfig.Config](reg, container, "auto:telemetryconfig")
		},
		func() error {
			return registerAuto[uploadsconfig.Config](reg, container, "auto:uploadsconfig")
		},
		func() error { return registerAuto[redisconfig.Config](reg, container, "auto:redisconfig") },
		func() error { return registerAuto[meiliconfig.Config](reg, container, "auto:meiliconfig") },
		func() error {
			return registerAuto[paymentsconfig.Config](reg, container, "auto:paymentsconfig")
		},
		func() error { return registerAuto[appconfig.Config](reg, container, "auto:appconfig") },
		func() error {
			return registerAuto[bichatconfig.Config](reg, container, "auto:bichatconfig")
		},
	}

	for _, register := range registrations {
		if err := register(); err != nil {
			return err
		}
	}

	if err := reg.Seal(); err != nil {
		return fmt.Errorf("seal config registry: %w", err)
	}

	logSourceProvenance(ctx.Source())
	return nil
}

// logSourceProvenance emits a one-shot startup summary of which provider
// supplied each top-level config prefix, so ops can see "db.* came from env,
// meili.* came from static" without hitting /debug/config.
func logSourceProvenance(src config.Source) {
	counts := make(map[string]map[string]int)
	for _, key := range src.Keys() {
		prefix := key
		if idx := strings.Index(key, "."); idx >= 0 {
			prefix = key[:idx]
		}
		origin, _ := src.Origin(key)
		if counts[prefix] == nil {
			counts[prefix] = make(map[string]int)
		}
		counts[prefix][origin]++
	}
	summary := make([]string, 0, len(counts))
	for prefix, origins := range counts {
		parts := make([]string, 0, len(origins))
		for origin, n := range origins {
			parts = append(parts, fmt.Sprintf("%s=%d", origin, n))
		}
		sort.Strings(parts)
		summary = append(summary, fmt.Sprintf("%s[%s]", prefix, strings.Join(parts, ",")))
	}
	sort.Strings(summary)
	slog.Info("config loaded", "provenance", strings.Join(summary, " "))
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
