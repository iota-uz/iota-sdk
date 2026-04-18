package config

import (
	"errors"
	"strings"
	"testing"
)

// --- test fixtures ----------------------------------------------------------

// optionalCfg is a Configured optional feature keyed under "opt". It is
// configured iff Required is non-empty. Implements DisabledReason.
type optionalCfg struct {
	Required string `koanf:"required"`
	Flavour  string `koanf:"flavour"`
}

func (optionalCfg) ConfigPrefix() string      { return "opt" }
func (c *optionalCfg) IsConfigured() bool     { return c.Required != "" }
func (c *optionalCfg) DisabledReason() string { return "opt.required not set" }

// strictValidator fails Validate. Used to confirm Validate is skipped for
// Disabled entries and runs for Active entries.
type strictValidator struct {
	Required string `koanf:"required"`
}

func (strictValidator) ConfigPrefix() string  { return "strict" }
func (c *strictValidator) IsConfigured() bool { return c.Required != "" }
func (c *strictValidator) Validate() error    { return errors.New("strictValidator always fails") }

// --- Source.HasPrefix -------------------------------------------------------

func TestSource_HasPrefix(t *testing.T) {
	t.Parallel()

	src := buildSource(t, map[string]any{
		"opt.required":      "yes",
		"other.thing":       "x",
		"namespace.key.sub": "v",
	})

	cases := []struct {
		prefix string
		want   bool
	}{
		{"opt", true},
		{"other", true},
		{"namespace", true},
		{"namespace.key", true},
		{"missing", false},
		{"", true},                 // empty prefix: true iff any keys
		{"namespace.key.sub", false}, // exact match without trailing "." doesn't count
	}

	for _, tc := range cases {
		got := src.HasPrefix(tc.prefix)
		if got != tc.want {
			t.Errorf("HasPrefix(%q): got %v, want %v", tc.prefix, got, tc.want)
		}
	}
}

func TestSource_HasPrefix_EmptySource(t *testing.T) {
	t.Parallel()

	src := buildSource(t, nil)
	if src.HasPrefix("") {
		t.Error("empty source with empty prefix should return false")
	}
	if src.HasPrefix("any") {
		t.Error("empty source should never match a prefix")
	}
}

// --- FeatureState resolution ------------------------------------------------

func TestState_ActiveWhenNotConfigured_Interface_Absent(t *testing.T) {
	t.Parallel()

	src := buildSource(t, map[string]any{"server.host": "h"})
	r := NewRegistry(src)
	r.SetStrict(StrictLax)

	if _, err := RegisterAt[serverConfig](r, "server"); err != nil {
		t.Fatalf("RegisterAt: %v", err)
	}

	st, ok := StateOf[serverConfig](r)
	if !ok {
		t.Fatal("State: expected ok=true for registered type")
	}
	if st != StateActive {
		t.Errorf("State: got %s, want active", st)
	}
}

func TestState_DisabledWhenUnconfiguredAndPrefixClean(t *testing.T) {
	t.Parallel()

	// No keys under "opt" at all → StateDisabled.
	src := buildSource(t, map[string]any{"other.k": "v"})
	r := NewRegistry(src)
	r.SetStrict(StrictLax)

	if _, err := Register[optionalCfg](r); err != nil {
		t.Fatalf("Register: %v", err)
	}

	st, _ := StateOf[optionalCfg](r)
	if st != StateDisabled {
		t.Errorf("State: got %s, want disabled", st)
	}
}

func TestState_PartiallyConfiguredWhenSomeKeysSet_LaxMode(t *testing.T) {
	t.Parallel()

	// opt.flavour set but opt.required missing → partial.
	src := buildSource(t, map[string]any{"opt.flavour": "spicy"})
	r := NewRegistry(src)
	r.SetStrict(StrictLax)

	if _, err := Register[optionalCfg](r); err != nil {
		t.Fatalf("Register (lax): %v", err)
	}

	st, _ := StateOf[optionalCfg](r)
	if st != StatePartiallyConfigured {
		t.Errorf("State: got %s, want partially_configured", st)
	}
}

func TestState_PartialConfig_StrictMode_RegisterErrors(t *testing.T) {
	t.Parallel()

	src := buildSource(t, map[string]any{"opt.flavour": "spicy"})
	r := NewRegistry(src)
	r.SetStrict(StrictYes)

	_, err := Register[optionalCfg](r)
	if err == nil {
		t.Fatal("Register should error in strict mode on partial config")
	}
	if !strings.Contains(err.Error(), "partially configured") {
		t.Errorf("error should mention 'partially configured': %v", err)
	}
	if !strings.Contains(err.Error(), "opt.required not set") {
		t.Errorf("error should carry DisabledReason: %v", err)
	}

	// Entry must NOT be stored when Register errors.
	if _, ok := Lookup[optionalCfg](r); ok {
		t.Error("partial-config strict-mode Register should not store entry")
	}
}

func TestState_ActiveWhenConfigured(t *testing.T) {
	t.Parallel()

	src := buildSource(t, map[string]any{"opt.required": "yes"})
	r := NewRegistry(src)
	r.SetStrict(StrictYes) // strict should be fine when fully configured

	if _, err := Register[optionalCfg](r); err != nil {
		t.Fatalf("Register: %v", err)
	}

	st, _ := StateOf[optionalCfg](r)
	if st != StateActive {
		t.Errorf("State: got %s, want active", st)
	}
}

// --- Validate skip for Disabled --------------------------------------------

func TestRegister_ValidateSkipped_WhenDisabled(t *testing.T) {
	t.Parallel()

	// strictValidator.Validate always fails. With no keys set the state is
	// Disabled, so Validate must be skipped and Register must succeed.
	src := buildSource(t, map[string]any{"other.k": "v"})
	r := NewRegistry(src)
	r.SetStrict(StrictLax)

	if _, err := Register[strictValidator](r); err != nil {
		t.Errorf("Register on Disabled entry should skip Validate; got err: %v", err)
	}

	st, _ := StateOf[strictValidator](r)
	if st != StateDisabled {
		t.Errorf("State: got %s, want disabled", st)
	}
}

func TestRegister_ValidateRuns_WhenActive(t *testing.T) {
	t.Parallel()

	// Fields set → Active → Validate runs and fails.
	src := buildSource(t, map[string]any{"strict.required": "yes"})
	r := NewRegistry(src)
	r.SetStrict(StrictLax)

	_, err := Register[strictValidator](r)
	if err == nil {
		t.Fatal("Register should error when Active entry fails Validate")
	}
	if !strings.Contains(err.Error(), "strictValidator always fails") {
		t.Errorf("error should propagate Validate message: %v", err)
	}
}

// --- Seal matrix ------------------------------------------------------------

func TestSeal_StrictYes_PartiallyConfiguredJoinedIntoError(t *testing.T) {
	t.Parallel()

	// Use StrictLax so RegisterAt stores the entry as PartiallyConfigured,
	// then bump to StrictYes and seal. Verifies that Seal re-reads strict
	// mode and treats partial entries as errors.
	src := buildSource(t, map[string]any{"opt.flavour": "spicy"})
	r := NewRegistry(src)
	r.SetStrict(StrictLax)
	if _, err := Register[optionalCfg](r); err != nil {
		t.Fatalf("Register (lax): %v", err)
	}

	r.SetStrict(StrictYes)
	err := r.Seal()
	if err == nil {
		t.Fatal("Seal should error in strict mode with partial entry present")
	}
	if !strings.Contains(err.Error(), "partially configured") {
		t.Errorf("Seal error should name partial-config reason: %v", err)
	}
}

func TestSeal_StrictLax_PartiallyConfiguredDowngradesToDisabled(t *testing.T) {
	t.Parallel()

	src := buildSource(t, map[string]any{"opt.flavour": "spicy"})
	r := NewRegistry(src)
	r.SetStrict(StrictLax)
	if _, err := Register[optionalCfg](r); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if err := r.Seal(); err != nil {
		t.Errorf("Seal (lax) should not error on partial entry; got: %v", err)
	}

	// Post-Seal state: downgraded to Disabled.
	st, _ := StateOf[optionalCfg](r)
	if st != StateDisabled {
		t.Errorf("post-Seal state: got %s, want disabled (lax downgrade)", st)
	}
}
