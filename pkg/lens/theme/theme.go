// Package theme is the single source of truth for the Lens design system v2
// tokens. Render targets (render/apex options, templ style blocks) and any
// consumer that needs a Lens color, radius, or typography value should read
// it from here rather than hardcoding hexes.
//
// The CSS custom properties emitted by render/templ.LensThemeStyles() mirror
// these constants one-to-one; keep the two in sync when changing a token.
package theme

// Surfaces.
const (
	// BgPage is the page background behind cards.
	BgPage = "#F6F7F9"
	// BgCard is the card surface background.
	BgCard = "#FFFFFF"
	// BgInset is the recessed surface used for insets (segmented controls,
	// chips, hover rows).
	BgInset = "#F8FAFC"
)

// Hairlines.
const (
	// Border is the default hairline color for card and control borders.
	Border = "#E2E8F0"
	// BorderStrong is a higher-contrast hairline (tooltips, emphasis).
	BorderStrong = "#CBD5E1"
	// Divider is the faintest hairline, used inside cards (header rules,
	// table row separators).
	Divider = "#F1F5F9"
)

// Text.
const (
	// TextStrong is the highest-emphasis text color (headline values).
	TextStrong = "#0F172A"
	// Text is the default body text color.
	Text = "#334155"
	// TextMuted is secondary text (labels, captions).
	TextMuted = "#64748B"
	// TextFaint is the lowest-emphasis text (hints, zero states, table
	// headers).
	TextFaint = "#94A3B8"
)

// Accent ramp (blue).
const (
	Accent700 = "#1D4ED8"
	Accent600 = "#2255D6"
	Accent500 = "#2563EB"
	Accent300 = "#93C5FD"
	Accent100 = "#DBEAFE"
	Accent50  = "#EFF6FF"
)

// Status colors.
const (
	// Positive marks favorable deltas and success states.
	Positive = "#059669"
	// Negative marks unfavorable deltas and error states.
	Negative = "#DC2626"
	// Warning marks caution states.
	Warning = "#D97706"
)

// Geometry (px).
const (
	// RadiusCard is the border radius of cards.
	RadiusCard = 8
	// RadiusControl is the border radius of controls (segmented controls,
	// inputs).
	RadiusControl = 6
	// RadiusBadge is the border radius of badges and chips.
	RadiusBadge = 4
)

// Typography.
const (
	// FontFamily is the Lens font stack.
	FontFamily = "'Inter',ui-sans-serif,system-ui,'Helvetica Neue',Arial,sans-serif"
	// AxisFontSizePx is the font size for chart axis labels.
	AxisFontSizePx = 11
)

// Behavior.
const (
	// DebounceMs is the standard debounce for filter inputs.
	DebounceMs = 500
)

// ShadowCard is the resting card shadow.
const ShadowCard = "0 1px 2px rgba(15,23,42,.04)"
