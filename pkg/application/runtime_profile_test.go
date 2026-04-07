package application

import "testing"

func TestApplicationRuntimeProfileDefaultsToServer(t *testing.T) {
	t.Parallel()

	app, err := New(&ApplicationOptions{
		Bundle:             LoadBundle(),
		SupportedLanguages: []string{"en"},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if got := app.RuntimeProfile(); got != RuntimeProfileServer {
		t.Fatalf("RuntimeProfile() = %q, want %q", got, RuntimeProfileServer)
	}
}

func TestApplicationRuntimeProfileHonorsBootstrap(t *testing.T) {
	t.Parallel()

	app, err := New(&ApplicationOptions{
		Bundle:             LoadBundle(),
		SupportedLanguages: []string{"en"},
		RuntimeProfile:     RuntimeProfileBootstrap,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if got := app.RuntimeProfile(); got != RuntimeProfileBootstrap {
		t.Fatalf("RuntimeProfile() = %q, want %q", got, RuntimeProfileBootstrap)
	}
}
