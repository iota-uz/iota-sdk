package application

import "testing"

func TestNormalizeCompositionProfileDefaultsToServer(t *testing.T) {
	t.Parallel()

	got, err := normalizeCompositionProfile("")
	if err != nil {
		t.Fatalf("normalizeCompositionProfile() error = %v", err)
	}
	if got != CompositionProfileServer {
		t.Fatalf("normalizeCompositionProfile() = %q, want %q", got, CompositionProfileServer)
	}
}

func TestNormalizeCompositionProfileHonorsBootstrap(t *testing.T) {
	t.Parallel()

	got, err := normalizeCompositionProfile(CompositionProfileBootstrap)
	if err != nil {
		t.Fatalf("normalizeCompositionProfile() error = %v", err)
	}
	if got != CompositionProfileBootstrap {
		t.Fatalf("normalizeCompositionProfile() = %q, want %q", got, CompositionProfileBootstrap)
	}
}
