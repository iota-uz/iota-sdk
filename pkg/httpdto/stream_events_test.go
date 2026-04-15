package httpdto_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/httpdto"
)

func TestIsTerminal(t *testing.T) {
	t.Parallel()

	terminal := map[httpdto.StreamEventType]bool{
		httpdto.StreamEventDone:      true,
		httpdto.StreamEventCancelled: true,
		httpdto.StreamEventError:     true,
		httpdto.StreamEventFailed:    true,
	}

	for _, et := range httpdto.AllStreamEventTypes {
		want := terminal[et]
		got := httpdto.IsTerminal(et)
		if got != want {
			t.Errorf("IsTerminal(%q) = %v, want %v", et, got, want)
		}
	}
}

func TestIsTerminal_AllValuesAccountedFor(t *testing.T) {
	t.Parallel()

	// Every terminal event type must be in AllStreamEventTypes.
	terminalTypes := []httpdto.StreamEventType{
		httpdto.StreamEventDone,
		httpdto.StreamEventCancelled,
		httpdto.StreamEventError,
		httpdto.StreamEventFailed,
	}

	all := make(map[httpdto.StreamEventType]bool)
	for _, et := range httpdto.AllStreamEventTypes {
		all[et] = true
	}

	for _, tt := range terminalTypes {
		if !all[tt] {
			t.Errorf("terminal event type %q missing from AllStreamEventTypes", tt)
		}
	}
}
