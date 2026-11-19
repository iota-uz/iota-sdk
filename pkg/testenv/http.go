package testenv

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"time"
)

// Route is one HTTP control route, for hosts that register routes explicitly.
type Route struct {
	Method  string
	Pattern string
}

// Routes lists every route served by NewHandler. Control routes beyond the
// scenario routes answer missing_capability until their control is attached.
func Routes() []Route {
	return []Route{
		{http.MethodGet, "/__test__/scenarios"},
		{http.MethodGet, "/__test__/controls"},
		{http.MethodPost, "/__test__/scenarios/prepare"},
		{http.MethodPost, "/__test__/scopes"},
		{http.MethodDelete, "/__test__/scopes/{scopeId}"},
		{http.MethodGet, "/__test__/scopes/{scopeId}/manifest"},
		{http.MethodPost, "/__test__/scopes/{scopeId}/replay"},
		{http.MethodGet, "/__test__/clock"},
		{http.MethodPost, "/__test__/scopes/{scopeId}/clock/advance"},
		{http.MethodPost, "/__test__/scopes/{scopeId}/jobs"},
		{http.MethodGet, "/__test__/scopes/{scopeId}/executions/{executionId}"},
		{http.MethodPost, "/__test__/scopes/{scopeId}/executions/{executionId}/wait"},
		{http.MethodPost, "/__test__/scopes/{scopeId}/providers/{provider}/scripts"},
		{http.MethodGet, "/__test__/scopes/{scopeId}/providers/{provider}/calls"},
		{http.MethodPost, "/__test__/scopes/{scopeId}/providers/{provider}/gates/{gate}/release"},
		{http.MethodPost, "/__test__/scopes/{scopeId}/providers/{provider}/gates/{gate}/wait"},
		{http.MethodDelete, "/__test__/scopes/{scopeId}/providers"},
	}
}

// StatusCode maps a control error code to its HTTP status.
func StatusCode(code string) int {
	status := map[string]int{"invalid_input": 400, "unknown_scenario": 404, "version_mismatch": 409, "scope_conflict": 409, "missing_capability": 412, "timeout": 504}[code]
	if status == 0 {
		return 500
	}
	return status
}

// ControlsDescription is returned by GET /__test__/controls.
type ControlsDescription struct {
	EnvironmentID string           `json:"environmentId,omitempty"`
	Dedicated     bool             `json:"dedicated"`
	Clock         *string          `json:"clock,omitempty"`
	Build         BuildInfo        `json:"build"`
	Jobs          []JobDefinition  `json:"jobs"`
	Conditions    []string         `json:"conditions"`
	Providers     []map[string]any `json:"providers"`
	Resources     []string         `json:"resources"`
}

// Controls describes attached controls for discovery.
func (r *Registry) Controls() ControlsDescription {
	r.mu.Lock()
	description := ControlsDescription{EnvironmentID: r.environmentID, Dedicated: r.dedicated, Build: r.build, Jobs: []JobDefinition{}, Conditions: []string{}, Providers: []map[string]any{}, Resources: []string{}}
	jobs, providers, clock := r.jobs, r.providers, r.clock
	for _, resource := range r.resources {
		description.Resources = append(description.Resources, resource.name)
	}
	r.mu.Unlock()
	if clock != nil {
		now := clock.Now().Format(time.RFC3339Nano)
		description.Clock = &now
	}
	if jobs != nil {
		description.Jobs, description.Conditions = jobs.Definitions()
	}
	if providers != nil {
		description.Providers = providers.Definitions()
	}
	return description
}

func decode(w http.ResponseWriter, r *http.Request, limit int64, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return failure("invalid_input", err.Error())
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return failure("invalid_input", "expected one JSON object")
	}
	return nil
}

func NewHandler(registry *Registry, controlToken string) (http.Handler, error) {
	if registry == nil || len(controlToken) < 32 {
		return nil, failure("invalid_input", "registry and a control token of at least 32 bytes required")
	}
	mux := http.NewServeMux()
	write := func(w http.ResponseWriter, value any, err error) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if err != nil {
			var e *Error
			if !errors.As(err, &e) {
				e = &Error{Code: "execution_failed", Message: err.Error()}
			}
			w.WriteHeader(StatusCode(e.Code))
			value = e
		}
		if err := json.NewEncoder(w).Encode(value); err != nil {
			return
		}
	}
	jobs := func() (*Jobs, error) {
		registry.mu.Lock()
		defer registry.mu.Unlock()
		if registry.jobs == nil {
			return nil, failure("missing_capability", "job controls are not attached")
		}
		return registry.jobs, nil
	}
	providers := func() (*Providers, error) {
		registry.mu.Lock()
		defer registry.mu.Unlock()
		if registry.providers == nil {
			return nil, failure("missing_capability", "provider controls are not attached")
		}
		return registry.providers, nil
	}
	mux.HandleFunc("GET /__test__/scenarios", func(w http.ResponseWriter, r *http.Request) { write(w, registry.Definitions(), nil) })
	mux.HandleFunc("GET /__test__/controls", func(w http.ResponseWriter, r *http.Request) { write(w, registry.Controls(), nil) })
	mux.HandleFunc("POST /__test__/scopes", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			ScopeID string `json:"scopeId"`
		}
		if err := decode(w, r, 4096, &input); err != nil {
			write(w, nil, err)
			return
		}
		err := registry.ReserveScope(input.ScopeID)
		write(w, map[string]string{"scopeId": input.ScopeID}, err)
	})
	mux.HandleFunc("POST /__test__/scenarios/prepare", func(w http.ResponseWriter, r *http.Request) {
		var input Input
		if err := decode(w, r, 1<<20, &input); err != nil {
			write(w, nil, err)
			return
		}
		result, err := registry.Prepare(r.Context(), input)
		write(w, result, err)
	})
	mux.HandleFunc("DELETE /__test__/scopes/{scopeId}", func(w http.ResponseWriter, r *http.Request) {
		err := registry.Dispose(r.Context(), r.PathValue("scopeId"))
		write(w, map[string]bool{"disposed": err == nil}, err)
	})
	mux.HandleFunc("GET /__test__/scopes/{scopeId}/manifest", func(w http.ResponseWriter, r *http.Request) {
		manifest, err := registry.Manifest(r.Context(), r.PathValue("scopeId"))
		write(w, manifest, err)
	})
	mux.HandleFunc("POST /__test__/scopes/{scopeId}/replay", func(w http.ResponseWriter, r *http.Request) {
		var manifest Manifest
		if err := decode(w, r, 4<<20, &manifest); err != nil {
			write(w, nil, err)
			return
		}
		result, err := registry.Replay(r.Context(), r.PathValue("scopeId"), manifest)
		write(w, result, err)
	})
	mux.HandleFunc("GET /__test__/clock", func(w http.ResponseWriter, r *http.Request) {
		now, ok := registry.ClockNow()
		if !ok {
			write(w, nil, failure("missing_capability", "environment clock is not controlled"))
			return
		}
		write(w, map[string]string{"now": now.Format(time.RFC3339Nano)}, nil)
	})
	mux.HandleFunc("POST /__test__/scopes/{scopeId}/clock/advance", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			DurationMs int64 `json:"durationMs"`
		}
		if err := decode(w, r, 4096, &input); err != nil {
			write(w, nil, err)
			return
		}
		if input.DurationMs < 0 || input.DurationMs > math.MaxInt64/int64(time.Millisecond) {
			write(w, nil, failure("invalid_input", "durationMs must be a non-negative duration"))
			return
		}
		now, err := registry.AdvanceClock(r.PathValue("scopeId"), time.Duration(input.DurationMs)*time.Millisecond)
		write(w, map[string]string{"now": now.Format(time.RFC3339Nano)}, err)
	})
	mux.HandleFunc("POST /__test__/scopes/{scopeId}/jobs", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Job  string               `json:"job"`
			Refs map[string]EntityRef `json:"refs"`
		}
		if err := decode(w, r, 1<<20, &input); err != nil {
			write(w, nil, err)
			return
		}
		j, err := jobs()
		if err != nil {
			write(w, nil, err)
			return
		}
		id, err := j.Run(r.Context(), r.PathValue("scopeId"), input.Job, input.Refs)
		write(w, map[string]string{"executionId": id}, err)
	})
	mux.HandleFunc("GET /__test__/scopes/{scopeId}/executions/{executionId}", func(w http.ResponseWriter, r *http.Request) {
		j, err := jobs()
		if err != nil {
			write(w, nil, err)
			return
		}
		status, err := j.Status(r.PathValue("scopeId"), r.PathValue("executionId"))
		write(w, status, err)
	})
	mux.HandleFunc("POST /__test__/scopes/{scopeId}/executions/{executionId}/wait", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Condition string `json:"condition"`
			TimeoutMs int    `json:"timeoutMs"`
		}
		if err := decode(w, r, 4096, &input); err != nil {
			write(w, nil, err)
			return
		}
		j, err := jobs()
		if err != nil {
			write(w, nil, err)
			return
		}
		result, err := j.Wait(r.Context(), r.PathValue("scopeId"), r.PathValue("executionId"), input.Condition, input.TimeoutMs)
		write(w, result, err)
	})
	mux.HandleFunc("POST /__test__/scopes/{scopeId}/providers/{provider}/scripts", func(w http.ResponseWriter, r *http.Request) {
		var payload json.RawMessage
		if err := decode(w, r, 1<<20, &payload); err != nil {
			write(w, nil, err)
			return
		}
		p, err := providers()
		if err != nil {
			write(w, nil, err)
			return
		}
		err = p.Script(r.Context(), r.PathValue("scopeId"), r.PathValue("provider"), payload)
		write(w, map[string]bool{"scripted": err == nil}, err)
	})
	mux.HandleFunc("GET /__test__/scopes/{scopeId}/providers/{provider}/calls", func(w http.ResponseWriter, r *http.Request) {
		p, err := providers()
		if err != nil {
			write(w, nil, err)
			return
		}
		calls, err := p.Calls(r.PathValue("scopeId"), r.PathValue("provider"))
		write(w, map[string]any{"calls": calls}, err)
	})
	mux.HandleFunc("POST /__test__/scopes/{scopeId}/providers/{provider}/gates/{gate}/release", func(w http.ResponseWriter, r *http.Request) {
		p, err := providers()
		if err != nil {
			write(w, nil, err)
			return
		}
		err = p.Release(r.PathValue("scopeId"), r.PathValue("provider"), r.PathValue("gate"))
		write(w, map[string]bool{"released": err == nil}, err)
	})
	mux.HandleFunc("POST /__test__/scopes/{scopeId}/providers/{provider}/gates/{gate}/wait", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			TimeoutMs int `json:"timeoutMs"`
		}
		if err := decode(w, r, 4096, &input); err != nil {
			write(w, nil, err)
			return
		}
		if input.TimeoutMs < MinWaitTimeoutMs || input.TimeoutMs > MaxWaitTimeoutMs {
			write(w, nil, failure("invalid_input", "timeoutMs must be an integer from 1 to 120000"))
			return
		}
		p, err := providers()
		if err != nil {
			write(w, nil, err)
			return
		}
		ctx, cancel := contextWithTimeoutMs(r, input.TimeoutMs)
		defer cancel()
		err = p.WaitHeld(ctx, r.PathValue("scopeId"), r.PathValue("provider"), r.PathValue("gate"))
		write(w, map[string]bool{"held": err == nil}, err)
	})
	mux.HandleFunc("DELETE /__test__/scopes/{scopeId}/providers", func(w http.ResponseWriter, r *http.Request) {
		p, err := providers()
		if err != nil {
			write(w, nil, err)
			return
		}
		if err := registry.owned(r.PathValue("scopeId")); err != nil {
			write(w, nil, err)
			return
		}
		err = p.ForgetScope(r.Context(), r.PathValue("scopeId"))
		write(w, map[string]bool{"forgotten": err == nil}, err)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+controlToken)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if registry.environmentID != "" {
			w.Header().Set("X-Test-Environment-Id", registry.environmentID)
		}
		mux.ServeHTTP(w, r)
	}), nil
}
