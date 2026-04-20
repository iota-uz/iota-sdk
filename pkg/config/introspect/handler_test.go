package introspect_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/introspect"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
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
	resorted, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if !bytes.Equal(first, resorted) {
		t.Errorf("keys are not sorted:\ngot:  %s\nwant: %s", first, resorted)
	}
}

func TestHandlerWithOrigins_PlainMode(t *testing.T) {
	t.Parallel()

	src, err := config.Build(static.New(map[string]any{"db.host": "localhost"}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	h := introspect.HandlerWithOrigins(
		func() any { return secretStruct{Host: "plain"} },
		src,
		func(*http.Request) bool { return true },
	)

	req := httptest.NewRequest(http.MethodGet, "/debug/config", nil) // no ?origins=1
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	// Should be plain JSON, not the {values, origins} wrapper.
	if strings.HasPrefix(body, `{"values":`) {
		t.Errorf("plain mode should not include values/origins wrapper: %s", body)
	}
	if !strings.Contains(body, "plain") {
		t.Errorf("body missing host field: %s", body)
	}
}

func TestHandlerWithOrigins_OriginsMode(t *testing.T) {
	t.Parallel()

	src, err := config.Build(
		static.New(map[string]any{"db.host": "myhost", "db.port": "5432"}),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	h := introspect.HandlerWithOrigins(
		func() any { return secretStruct{Host: "snap", Password: "secret"} },
		src,
		func(*http.Request) bool { return true },
	)

	req := httptest.NewRequest(http.MethodGet, "/debug/config?origins=1", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.HasPrefix(body, `{"values":`) {
		t.Errorf("origins mode should have {values,...} wrapper: %s", body)
	}

	var parsed struct {
		Values  map[string]any    `json:"values"`
		Origins map[string]string `json:"origins"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("response is not valid JSON: %v\nbody: %s", err, body)
	}

	// values should be present and secrets redacted.
	if parsed.Values == nil {
		t.Error("values field is nil")
	}
	if pw, ok := parsed.Values["password"]; ok && pw == "secret" {
		t.Error("password not redacted in values")
	}

	// origins should contain known keys from the source.
	if len(parsed.Origins) == 0 {
		t.Error("origins map is empty")
	}
	for key, origin := range parsed.Origins {
		if origin == "" {
			t.Errorf("empty origin for key %q", key)
		}
	}
}

func TestHandlerWithOrigins_Forbidden(t *testing.T) {
	t.Parallel()

	src, _ := config.Build(static.New(nil))
	h := introspect.HandlerWithOrigins(
		func() any { return struct{}{} },
		src,
		func(*http.Request) bool { return false },
	)

	req := httptest.NewRequest(http.MethodGet, "/debug/config?origins=1", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// TestHandlerWithOrigins_MalformedRedact verifies that if config.Redact returns
// a non-JSON string (e.g. due to an unmarshallable type), HandlerWithOrigins
// still returns valid JSON with null for the values field.
func TestHandlerWithOrigins_MalformedRedact(t *testing.T) {
	t.Parallel()

	// A struct with a channel field causes json.MarshalIndent to fail inside
	// config.Redact, which returns a "<redact error: ...>" string — not valid JSON.
	type badSnapshot struct {
		Ch chan int
	}

	src, _ := config.Build(static.New(map[string]any{"x": "1"}))
	h := introspect.HandlerWithOrigins(
		func() any { return badSnapshot{Ch: make(chan int)} },
		src,
		func(*http.Request) bool { return true },
	)

	req := httptest.NewRequest(http.MethodGet, "/debug/config?origins=1", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	body := rr.Body.Bytes()
	if !json.Valid(body) {
		t.Fatalf("response is not valid JSON: %s", body)
	}

	var parsed struct {
		Values  any               `json:"values"`
		Origins map[string]string `json:"origins"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	// values must be null when Redact produced malformed JSON.
	if parsed.Values != nil {
		t.Errorf("expected values=null for malformed Redact output, got %v", parsed.Values)
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
