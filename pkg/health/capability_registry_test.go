package health

import (
	"context"
	"testing"
)

// stubProbe returns a fixed Capability whenever Probe is called.
type stubProbe struct {
	cap Capability
}

func (s stubProbe) Probe(_ context.Context) Capability { return s.cap }

func TestList_NoDedup_WhenKeysDistinct(t *testing.T) {
	t.Parallel()

	r := NewCapabilityRegistry()
	r.Register(stubProbe{cap: Capability{Key: "a", Status: StatusHealthy}})
	r.Register(stubProbe{cap: Capability{Key: "b", Status: StatusHealthy}})
	r.Register(stubProbe{cap: Capability{Key: "c", Status: StatusHealthy}})

	probes := r.List()
	if len(probes) != 3 {
		t.Fatalf("expected 3 probes, got %d", len(probes))
	}

	keys := make([]string, 0, len(probes))
	for _, p := range probes {
		keys = append(keys, p.Probe(context.Background()).Key)
	}
	want := []string{"a", "b", "c"}
	for i, k := range keys {
		if k != want[i] {
			t.Errorf("order at %d: got %q, want %q", i, k, want[i])
		}
	}
}

func TestList_LastWinsByKey(t *testing.T) {
	t.Parallel()

	r := NewCapabilityRegistry()
	r.Register(stubProbe{cap: Capability{Key: "bichat", Status: StatusDisabled, Message: "earliest"}})
	r.Register(stubProbe{cap: Capability{Key: "other", Status: StatusHealthy}})
	r.Register(stubProbe{cap: Capability{Key: "bichat", Status: StatusHealthy, Message: "latest"}})

	probes := r.List()
	if len(probes) != 2 {
		t.Fatalf("expected 2 probes after dedup, got %d", len(probes))
	}

	// Surviving order: "other" (position 1 preserved) then the later "bichat".
	first := probes[0].Probe(context.Background())
	second := probes[1].Probe(context.Background())

	if first.Key != "other" {
		t.Errorf("first survivor: got %q, want %q", first.Key, "other")
	}
	if second.Key != "bichat" {
		t.Errorf("second survivor: got %q, want %q", second.Key, "bichat")
	}
	if second.Message != "latest" {
		t.Errorf("second survivor message: got %q, want %q", second.Message, "latest")
	}
}

func TestList_EmptyKey_NeverDeduped(t *testing.T) {
	t.Parallel()

	r := NewCapabilityRegistry()
	r.Register(stubProbe{cap: Capability{Key: "", Status: StatusDown, Message: "probe1"}})
	r.Register(stubProbe{cap: Capability{Key: "", Status: StatusDown, Message: "probe2"}})

	probes := r.List()
	if len(probes) != 2 {
		t.Errorf("empty-Key probes must not be deduped; got %d, want 2", len(probes))
	}
}

func TestList_NilProbe_Skipped(t *testing.T) {
	t.Parallel()

	r := NewCapabilityRegistry()
	r.Register(nil)
	r.Register(stubProbe{cap: Capability{Key: "k"}})

	if got := len(r.List()); got != 1 {
		t.Errorf("nil Register should no-op: got %d probes, want 1", got)
	}
}
