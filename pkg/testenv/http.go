package testenv

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

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
			status := map[string]int{"invalid_input": 400, "unknown_scenario": 404, "version_mismatch": 409, "scope_conflict": 409, "missing_capability": 412, "timeout": 504}[e.Code]
			if status == 0 {
				status = 500
			}
			w.WriteHeader(status)
			value = e
		}
		if err := json.NewEncoder(w).Encode(value); err != nil {
			return
		}
	}
	mux.HandleFunc("GET /__test__/scenarios", func(w http.ResponseWriter, r *http.Request) { write(w, registry.Definitions(), nil) })
	mux.HandleFunc("POST /__test__/scopes", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			ScopeID string `json:"scopeId"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			write(w, nil, failure("invalid_input", err.Error()))
			return
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			write(w, nil, failure("invalid_input", "expected one JSON object"))
			return
		}
		err := registry.ReserveScope(input.ScopeID)
		write(w, map[string]string{"scopeId": input.ScopeID}, err)
	})
	mux.HandleFunc("POST /__test__/scenarios/prepare", func(w http.ResponseWriter, r *http.Request) {
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		d.DisallowUnknownFields()
		var input Input
		if err := d.Decode(&input); err != nil {
			write(w, nil, failure("invalid_input", err.Error()))
			return
		}
		var extra any
		if d.Decode(&extra) != io.EOF {
			write(w, nil, failure("invalid_input", "expected one JSON object"))
			return
		}
		result, err := registry.Prepare(r.Context(), input)
		write(w, result, err)
	})
	mux.HandleFunc("DELETE /__test__/scopes/{scopeId}", func(w http.ResponseWriter, r *http.Request) {
		err := registry.Dispose(r.Context(), r.PathValue("scopeId"))
		write(w, map[string]bool{"disposed": err == nil}, err)
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
