package composition

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/health"
)

// SkipIfDisabled is the canonical one-line gate for optional Prefixed configs
// inside Component.Build. It returns true when the caller should return early
// because the feature's required fields were not supplied by the operator.
//
// Intended use:
//
//	func (c *component) Build(builder *composition.Builder) error {
//	    if composition.SkipIfDisabled[bichatconfig.Config](builder) {
//	        return nil
//	    }
//	    // ... register providers, controllers, etc.
//	}
//
// Side effect: a health.CapabilityProbe is registered into the shared
// CapabilityRegistry so /system/info shows the feature with its current
// state. Disabled features appear with Status=disabled; partially configured
// features (in lax mode only — strict mode prevents boot from reaching Build)
// appear with Status=down and a descriptive Message.
//
// Returns false (do not skip) when:
//   - T does not implement config.Configured (always-on feature)
//   - T.IsConfigured() returned true
//   - No config.Source is attached to the BuildContext (test harness)
//
// Panics when T is PartiallyConfigured — callers must not silently suppress
// misconfiguration. Strict mode prevents this panic by failing at Register;
// in lax mode the registry pre-downgrades partial to Disabled during Seal,
// so by the time SkipIfDisabled observes the state it is already Disabled.
// The panic is a defense-in-depth check for cases where a registry has not
// been sealed yet when Build runs.
func SkipIfDisabled[T config.Prefixed](builder *Builder) bool {
	if builder == nil {
		return false
	}
	buildCtx := &builder.context

	var zero T
	prefix := any(zero).(config.Prefixed).ConfigPrefix()

	state, cfg, err := ensureRegistered[T](buildCtx)
	if err != nil {
		// Partial config in strict mode: Register already errored. Propagate
		// through a panic so the caller surfaces the misconfiguration rather
		// than skipping the module silently. Strict mode prevents reaching
		// here at runtime — this is belt-and-braces.
		if logger := buildCtx.Logger(); logger != nil {
			logger.Errorf("composition: %s gate failed: %v", prefix, err)
		}
		panic(fmt.Sprintf("composition: %s gate: %v", prefix, err))
	}

	emitProbe(buildCtx, prefix, state, cfg)

	switch state {
	case config.StateDisabled:
		logSkip(buildCtx, prefix, reasonFor(cfg, "required fields not set"))
		return true
	case config.StatePartiallyConfigured:
		// Should only reach here if the registry is unsealed and lax mode let
		// partial slip through. Emit a loud warning and still skip so boot
		// doesn't fail, but make it visible.
		logSkip(buildCtx, prefix, reasonFor(cfg, "partially configured"))
		return true
	}
	return false
}

// IfConfigured invokes fn iff cfg implements config.Configured and reports
// IsConfigured. Used for sub-feature gating inside an already-active module
// (e.g. BiChat's Langfuse hook when the parent BiChat module is on but
// Langfuse credentials are absent).
//
// Unlike SkipIfDisabled, the gate subject does not need to be Prefixed —
// this variant operates on a concrete value extracted from the parent's
// already-loaded config. The key parameter is used for logging and as the
// CapabilityProbe key, so keep it stable and dot-delimited to match the
// enclosing config prefix convention.
//
// fn is invoked synchronously when the sub-feature is configured; its
// return is not captured. Callers that need an error channel should
// use the Prefixed form or register the error inside fn.
func IfConfigured[C any](builder *Builder, key string, cfg C, fn func()) {
	if builder == nil || fn == nil {
		return
	}
	if key == "" {
		panic("composition: IfConfigured requires a non-empty key")
	}

	buildCtx := &builder.context
	configured := true
	message := ""
	if c, ok := any(cfg).(config.Configured); ok {
		configured = c.IsConfigured()
		if !configured {
			message = reasonFor(cfg, "required fields not set")
		}
	} else if c, ok := any(&cfg).(config.Configured); ok {
		// Some configs implement IsConfigured on pointer receivers.
		configured = c.IsConfigured()
		if !configured {
			message = reasonFor(&cfg, "required fields not set")
		}
	}

	if !configured {
		emitSubFeatureProbe(buildCtx, key, config.StateDisabled, message)
		logSkip(buildCtx, key, message)
		return
	}

	emitSubFeatureProbe(buildCtx, key, config.StateActive, "")
	fn()
}

// GatedRegister invokes fn only when T is Active, letting middleware and
// other non-Component registration sites apply the same gate contract as
// Component.Build. Typical use is the telemetry Loki hook or a Twilio
// client factory that lives outside a module.
//
// Returns fn's error as-is when invoked; returns nil without calling fn
// when T is Disabled. Panics on PartiallyConfigured, matching
// SkipIfDisabled's contract.
func GatedRegister[T config.Prefixed](builder *Builder, fn func() error) error {
	if builder == nil {
		return errors.New("composition: GatedRegister: builder is nil")
	}
	if fn == nil {
		return errors.New("composition: GatedRegister: fn is nil")
	}
	buildCtx := &builder.context

	var zero T
	prefix := any(zero).(config.Prefixed).ConfigPrefix()

	state, cfg, err := ensureRegistered[T](buildCtx)
	if err != nil {
		return fmt.Errorf("composition: %s gate: %w", prefix, err)
	}

	emitProbe(buildCtx, prefix, state, cfg)

	switch state {
	case config.StateDisabled:
		logSkip(buildCtx, prefix, reasonFor(cfg, "required fields not set"))
		return nil
	case config.StatePartiallyConfigured:
		logSkip(buildCtx, prefix, reasonFor(cfg, "partially configured"))
		return nil
	}
	return fn()
}

// ensureRegistered registers T at its Prefixed prefix in the BuildContext's
// shared registry if not already, and returns the resolved FeatureState plus
// the typed pointer to the config value for downstream reason extraction.
// When no Source is attached, returns StateActive so gate helpers stay
// permissive in test harnesses.
func ensureRegistered[T config.Prefixed](buildCtx *BuildContext) (config.FeatureState, any, error) {
	if buildCtx.Source() == nil {
		var zero T
		return config.StateActive, &zero, nil
	}

	reg := buildCtx.Registry()
	var zero T
	if ptr, ok := config.Lookup[T](reg); ok {
		state, _ := reg.State(reflect.TypeOf(zero))
		return state, ptr, nil
	}

	ptr, err := config.Register[T](reg)
	if err != nil {
		return config.StateActive, nil, err
	}
	state, _ := reg.State(reflect.TypeOf(zero))
	return state, ptr, nil
}

// reasonFor returns cfg's DisabledReason when available, else fallback.
func reasonFor(cfg any, fallback string) string {
	if d, ok := cfg.(config.DisabledReason); ok {
		if r := d.DisabledReason(); r != "" {
			return r
		}
	}
	return fallback
}

// logSkip emits a single structured log line via the BuildContext's logger.
// Shape matches logSourceProvenance so ops can grep for feature-gating
// telemetry alongside provider telemetry.
func logSkip(buildCtx *BuildContext, feature, reason string) {
	logger := buildCtx.Logger()
	if logger == nil {
		return
	}
	logger.WithFields(map[string]any{
		"feature": feature,
		"reason":  reason,
	}).Info("feature skipped")
}

// emitProbe registers a static capability probe describing state for the
// top-level Prefixed config identified by prefix. The probe also records the
// provenance of the prefix by querying the source for the first key under
// it and attaching its Origin.
func emitProbe(buildCtx *BuildContext, prefix string, state config.FeatureState, cfg any) {
	registry := buildCtx.CapabilityRegistry()
	source := buildCtx.Source()
	reasonFallback := "required fields not set"
	if state == config.StatePartiallyConfigured {
		reasonFallback = "partially configured"
	}
	message := reasonFor(cfg, reasonFallback)
	origin := firstPrefixOrigin(source, prefix)

	registry.Register(health.CapabilityProbeFunc(func(_ context.Context) health.Capability {
		return buildCapability(prefix, state, message, origin)
	}))
}

// emitSubFeatureProbe is the IfConfigured variant: there is no Prefixed type
// to own the probe, just a stable key chosen by the caller.
func emitSubFeatureProbe(buildCtx *BuildContext, key string, state config.FeatureState, message string) {
	registry := buildCtx.CapabilityRegistry()
	origin := firstPrefixOrigin(buildCtx.Source(), key)
	registry.Register(health.CapabilityProbeFunc(func(_ context.Context) health.Capability {
		return buildCapability(key, state, message, origin)
	}))
}

func buildCapability(key string, state config.FeatureState, message, origin string) health.Capability {
	cap := health.Capability{
		Key:    key,
		Name:   key,
		Source: origin,
	}
	switch state {
	case config.StateActive:
		cap.Enabled = true
		cap.Status = health.StatusHealthy
	case config.StateDisabled:
		cap.Enabled = false
		cap.Status = health.StatusDisabled
		cap.Message = message
	case config.StatePartiallyConfigured:
		cap.Enabled = true
		cap.Status = health.StatusDown
		cap.Message = message
	default:
		cap.Status = health.StatusUnknown
	}
	return cap
}

// firstPrefixOrigin returns the provider name for any key observed under the
// given prefix, or "" when none. Used to populate Capability.Source with the
// provenance of the operator's input.
func firstPrefixOrigin(source config.Source, prefix string) string {
	if source == nil || prefix == "" {
		return ""
	}
	needle := prefix + "."
	for _, k := range source.Keys() {
		if len(k) >= len(needle) && k[:len(needle)] == needle {
			if origin, ok := source.Origin(k); ok {
				return origin
			}
		}
	}
	return ""
}
