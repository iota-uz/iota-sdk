// Package config provides a source-agnostic, DI-native configuration system.
//
// # Invariants
//
//   - A [Source] is immutable after [Build] returns. There is no Reload, Watch,
//     OnChange, or Subscribe API — and none will be added. To pick up new
//     configuration values, restart the process.
//   - No package-level globals. Callers compose providers, call [Build] once at
//     bootstrap, and pass the resulting [Source] to [NewRegistry].
//   - Hot-reload is explicitly excluded from the design.
//
// # Typical usage
//
//	import (
//	    "github.com/iota-uz/iota-sdk/pkg/config"
//	    "github.com/iota-uz/iota-sdk/pkg/config/providers/env"
//	    "github.com/iota-uz/iota-sdk/pkg/config/providers/yamlfile"
//	    "github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
//	)
//
//	src, err := config.Build(
//	    env.New(".env", ".env.local"),
//	    yamlfile.New("config.yaml"),
//	)
//	r := config.NewRegistry(src)
//	dbCfg, err := config.Register[dbconfig.Config](r)
//
// # Lazy-loading contract (optional features)
//
// Optional features (bichat, oidc, twilio, meili, smtp, telemetry.loki,
// payments, googleoauth, redis) opt in *implicitly*: operators supply the
// fields the feature needs (API key, URL, credentials) and the feature lights
// up. Leaving those fields unset keeps the feature disabled. No
// `X_ENABLED=true` ceremony exists — doing so would double the surface area
// operators must manage.
//
// The contract rests on two interfaces, [Configured] and [DisabledReason],
// and one state machine, [FeatureState]:
//
//   - If a config type does not implement [Configured], the framework treats
//     it as always-on (appropriate for non-optional core configs like
//     appconfig, dbconfig, httpconfig, …).
//   - If IsConfigured reports true, the feature is Active: Validate runs,
//     the module wires up, and /system/info shows Status=healthy.
//   - If IsConfigured reports false and the operator set NO keys under the
//     config's prefix, the feature is Disabled: Validate is skipped, the
//     module early-returns from Build, and /system/info shows
//     Status=disabled with the DisabledReason message.
//   - If IsConfigured reports false but the operator set SOME keys under the
//     prefix, the feature is PartiallyConfigured — almost always a typo or
//     half-finished deployment. [StrictYes] mode (default in production)
//     fails boot with a message naming the missing canonical field.
//     [StrictLax] mode (default in non-production) downgrades to Disabled
//     and logs a warning. Override via APP_STRICT_CONFIG=true|false or
//     [Registry.SetStrict].
//
// Module-side, the gate is a one-liner:
//
//	func (c *component) Build(builder *composition.Builder) error {
//	    if composition.SkipIfDisabled[bichatconfig.Config](builder) {
//	        return nil
//	    }
//	    // ... register providers / controllers ...
//	}
//
// Sub-feature gating (parent on, inner feature still optional) uses
// [composition.IfConfigured] on the sub-struct. Non-Component registration
// sites (telemetry hooks, middleware) use [composition.GatedRegister].
//
// All three helpers emit a [health.CapabilityProbe] into the shared
// [health.CapabilityRegistry], so /system/info reflects every optional
// feature's state automatically — no per-module health wiring.
package config
