package services

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestBuildDebugTrace_IncludesTraceMetadata(t *testing.T) {
	t.Setenv("LANGFUSE_BASE_URL", "https://langfuse.local/")
	sessionID := uuid.New()

	trace := buildDebugTrace(sessionID, nil, nil, 123)
	if trace == nil {
		t.Fatalf("expected debug trace")
	}
	if trace.TraceID != sessionID.String() {
		t.Fatalf("expected trace id %q, got %q", sessionID.String(), trace.TraceID)
	}
	expectedURL := "https://langfuse.local/trace/" + sessionID.String()
	if trace.TraceURL != expectedURL {
		t.Fatalf("expected trace url %q, got %q", expectedURL, trace.TraceURL)
	}
}

func TestBuildDebugTrace_WithoutMetricsStillReturnsTraceReference(t *testing.T) {
	sessionID := uuid.New()

	trace := buildDebugTrace(sessionID, nil, nil, 0)
	if trace == nil {
		t.Fatalf("expected debug trace with trace metadata")
	}
	if trace.TraceID != sessionID.String() {
		t.Fatalf("expected trace id %q, got %q", sessionID.String(), trace.TraceID)
	}
}

func TestBuildLangfuseTraceURL_Hardening(t *testing.T) {
	traceID := "trace/with special"
	tests := []struct {
		name       string
		baseURL    string
		hostURL    string
		expected   string
		shouldHave string
	}{
		{
			name:     "prefer base url over host",
			baseURL:  "https://base.example.com",
			hostURL:  "https://host.example.com",
			expected: "https://base.example.com/trace/trace%2Fwith%20special",
		},
		{
			name:     "fallback to host",
			baseURL:  "",
			hostURL:  "https://host.example.com/",
			expected: "https://host.example.com/trace/trace%2Fwith%20special",
		},
		{
			name:     "preserve base path",
			baseURL:  "https://host.example.com/langfuse",
			hostURL:  "",
			expected: "https://host.example.com/langfuse/trace/trace%2Fwith%20special",
		},
		{
			name:     "reject invalid scheme",
			baseURL:  "ftp://host.example.com",
			hostURL:  "",
			expected: "",
		},
		{
			name:     "reject malformed url",
			baseURL:  "://bad-url",
			hostURL:  "",
			expected: "",
		},
		{
			name:       "strip query and fragment",
			baseURL:    "https://host.example.com/root?x=1#frag",
			hostURL:    "",
			expected:   "https://host.example.com/root/trace/trace%2Fwith%20special",
			shouldHave: "/root/trace/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LANGFUSE_BASE_URL", tt.baseURL)
			t.Setenv("LANGFUSE_HOST", tt.hostURL)

			got := buildLangfuseTraceURL(traceID)
			if got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
			if tt.shouldHave != "" && !strings.Contains(got, tt.shouldHave) {
				t.Fatalf("expected %q to contain %q", got, tt.shouldHave)
			}
		})
	}
}
