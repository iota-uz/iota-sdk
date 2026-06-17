package services

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBuildDebugTrace_IncludesTraceMetadata(t *testing.T) {
	sessionID := uuid.New()
	traceID := uuid.New().String()
	langfuseBaseURL := "https://langfuse.local/"

	trace := buildDebugTrace(sessionID, traceID, nil, nil, 123, "", "", "", "", "", "", "", "", time.Now(), langfuseBaseURL)
	if trace == nil {
		t.Fatalf("expected debug trace")
	}
	if trace.TraceID != traceID {
		t.Fatalf("expected trace id %q, got %q", traceID, trace.TraceID)
	}
	expectedURL := "https://langfuse.local/trace/" + traceID
	if trace.TraceURL != expectedURL {
		t.Fatalf("expected trace url %q, got %q", expectedURL, trace.TraceURL)
	}
	if trace.SessionID != sessionID.String() {
		t.Fatalf("expected session id %q, got %q", sessionID.String(), trace.SessionID)
	}
}

func TestBuildDebugTrace_WithoutMetricsStillReturnsTraceReference(t *testing.T) {
	sessionID := uuid.New()

	trace := buildDebugTrace(sessionID, "", nil, nil, 0, "", "", "", "", "", "", "", "", time.Now(), "")
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
		expected   string
		shouldHave string
	}{
		{
			name:     "base url with trailing slash",
			baseURL:  "https://base.example.com",
			expected: "https://base.example.com/trace/trace%2Fwith%20special",
		},
		{
			name:     "base url with trailing slash stripped",
			baseURL:  "https://host.example.com/",
			expected: "https://host.example.com/trace/trace%2Fwith%20special",
		},
		{
			name:     "preserve base path",
			baseURL:  "https://host.example.com/langfuse",
			expected: "https://host.example.com/langfuse/trace/trace%2Fwith%20special",
		},
		{
			name:     "reject invalid scheme",
			baseURL:  "ftp://host.example.com",
			expected: "",
		},
		{
			name:     "reject malformed url",
			baseURL:  "://bad-url",
			expected: "",
		},
		{
			name:       "strip query and fragment",
			baseURL:    "https://host.example.com/root?x=1#frag",
			expected:   "https://host.example.com/root/trace/trace%2Fwith%20special",
			shouldHave: "/root/trace/",
		},
		{
			name:     "empty base url returns empty",
			baseURL:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildLangfuseTraceURL(traceID, tt.baseURL)
			if got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
			if tt.shouldHave != "" && !strings.Contains(got, tt.shouldHave) {
				t.Fatalf("expected %q to contain %q", got, tt.shouldHave)
			}
		})
	}
}
