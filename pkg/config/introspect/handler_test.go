package introspect_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config/introspect"
)

type secretStruct struct {
	Host     string `json:"host"`
	Password string `json:"password" secret:"true"`
}

func TestHandler_Forbidden(t *testing.T) {
	t.Parallel()

	h := introspect.Handler(
		func() any { return secretStruct{Host: "localhost", Password: "hunter2"} },
		nil, // nil authz → always deny
	)

	req := httptest.NewRequest(http.MethodGet, "/debug/config", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestHandler_AuthzFalse(t *testing.T) {
	t.Parallel()

	h := introspect.Handler(
		func() any { return secretStruct{Host: "localhost", Password: "hunter2"} },
		func(*http.Request) bool { return false },
	)

	req := httptest.NewRequest(http.MethodGet, "/debug/config", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestHandler_AuthzTrue_SecretRedacted(t *testing.T) {
	t.Parallel()

	h := introspect.Handler(
		func() any { return secretStruct{Host: "db.example.com", Password: "super-secret"} },
		func(*http.Request) bool { return true },
	)

	req := httptest.NewRequest(http.MethodGet, "/debug/config", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json content-type, got %q", ct)
	}

	body := rr.Body.String()

	// Secret field must be redacted
	if strings.Contains(body, "super-secret") {
		t.Errorf("body contains raw secret: %s", body)
	}

	// Non-secret field must be present
	if !strings.Contains(body, "db.example.com") {
		t.Errorf("body missing plain field: %s", body)
	}

	// Redaction marker must appear
	if !strings.Contains(body, "***") {
		t.Errorf("body missing redaction marker: %s", body)
	}

	// Response must be valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("response is not valid JSON: %v\nbody: %s", err, body)
	}
}

// TestHandlerFromMap_DeterministicKeyOrder verifies that HandlerFromMap produces
// byte-identical JSON output on successive calls (encoding/json sorts map keys).
func TestHandlerFromMap_DeterministicKeyOrder(t *testing.T) {
	t.Parallel()

	cfgs := map[string]any{
		"zebra":  secretStruct{Host: "z.local", Password: "pw-z"},
		"alpha":  secretStruct{Host: "a.local", Password: "pw-a"},
		"middle": secretStruct{Host: "m.local", Password: "pw-m"},
	}

	h := introspect.HandlerFromMap(cfgs, func(*http.Request) bool { return true })

	call := func() []byte {
		req := httptest.NewRequest(http.MethodGet, "/debug/config", nil)
		rr := httptest.NewRecorder()
		h(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		return rr.Body.Bytes()
	}

	first := call()
	second := call()
	if !bytes.Equal(first, second) {
		t.Errorf("handler output is non-deterministic:\nfirst:  %s\nsecond: %s", first, second)
	}

	// Also verify keys are lexicographically sorted in the output.
	var parsed map[string]any
	if err := json.Unmarshal(first, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// Re-marshal to get sorted keys, compare bytes.
	resorted, _ := json.MarshalIndent(parsed, "", "  ")
	if !bytes.Equal(first, resorted) {
		t.Errorf("keys are not sorted:\ngot:  %s\nwant: %s", first, resorted)
	}
}

func TestHandler_ZeroSecret_EmptyString(t *testing.T) {
	t.Parallel()

	// Zero-value secret fields should render as "" not "***".
	h := introspect.Handler(
		func() any { return secretStruct{Host: "localhost", Password: ""} },
		func(*http.Request) bool { return true },
	)

	req := httptest.NewRequest(http.MethodGet, "/debug/config", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	body := rr.Body.String()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("response is not valid JSON: %v\nbody: %s", err, body)
	}

	pw, ok := parsed["password"]
	if !ok {
		t.Fatal("password field missing from response")
	}
	if pw != "" {
		t.Errorf("expected empty string for zero-value secret, got %q", pw)
	}
}
