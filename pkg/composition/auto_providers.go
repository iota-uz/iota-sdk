package composition

import (
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

// installAutoProviders registers providers for the core services already
// available on the build context's Application handle. Components can then
// `composition.Use[T]()` for these without needing `RequireApplication`.
//
// Each provider is registered only when the underlying value is non-nil. User
// providers always win — if a component declares its own provider for one of
// these keys, addBuilder catches the duplicate and reports it.
//
// The set is intentionally small: only services that genuinely live one level
// above any component, are stable across the engine's lifetime, and were
// previously the most common reasons to call `RequireApplication`.
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
	if cfg := ctx.config; cfg != nil {
		registerAutoValue[*configuration.Configuration](container, "auto:config", cfg)
	}
	return nil
}

// registerAutoValue installs a value-form provider for T directly on the
// container. Resolved upfront so factory closure machinery is bypassed. The
// `auto:` prefix on sourceName flags the entry as overridable by user
// providers via isAutoProvider.
func registerAutoValue[T any](container *Container, sourceName string, value T) {
	key := keyFor(typeOf[T](), "")
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
	return len(entry.componentName) >= 5 && entry.componentName[:5] == "auto:"
}
