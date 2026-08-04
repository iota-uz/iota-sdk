package config

// Configured is the lazy-loading gate interface for optional features.
//
// A config type implements Configured to signal that the feature it
// represents is opt-in: operators enable it implicitly by supplying the
// required fields (API key, URL, credentials). Leaving those fields unset
// keeps the feature disabled. No explicit X_ENABLED env var exists.
//
// Returning false puts the entry in one of two gated states depending on
// whether the operator touched the config prefix at all:
//
//   - No keys set under the prefix → StateDisabled. The feature is skipped
//     silently; a CapabilityProbe reports Status=disabled on /system/info.
//
//   - One or more keys set but IsConfigured still returns false →
//     StatePartiallyConfigured. This is almost always a typo or a half-
//     completed deployment, so the framework escalates: strict mode fails
//     boot with the missing-field reason; lax mode logs a warning and
//     downgrades to Disabled for the rest of boot.
//
// Configs with no required fields (appconfig, dbconfig, httpconfig, ...)
// are always-on and do not implement Configured. Opt-out security features
// (ratelimitconfig) expose enablement via their own Enabled *bool field
// and likewise skip Configured.
type Configured interface {
	IsConfigured() bool
}

// DisabledReason is an optional companion to Configured that supplies a
// human-readable explanation when a feature is disabled or partially
// configured. It surfaces in the gate log line and as the Message field
// of the emitted health.Capability.
//
// When absent the framework falls back to a generic "required fields not
// set" string, which is useful but less specific than a tailored message
// such as "OIDC_ISSUERURL and OIDC_CRYPTOKEY required".
type DisabledReason interface {
	DisabledReason() string
}
