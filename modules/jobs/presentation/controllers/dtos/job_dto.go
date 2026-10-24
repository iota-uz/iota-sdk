// Package dtos provides this package.
package dtos

import (
	"encoding/json"
	"net/http"
	"strings"
)

// CreateJobDTO carries the enqueue request. Params is an optional JSON object
// of handler-specific parameters.
type CreateJobDTO struct {
	Kind   string
	Params map[string]any
}

// ParseCreateJobDTO reads kind and params from a form body. params may be
// passed either as a JSON object string or as individual form fields under
// the params. prefix.
func ParseCreateJobDTO(r *http.Request) (*CreateJobDTO, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	dto := &CreateJobDTO{
		Kind:   r.FormValue("kind"),
		Params: map[string]any{},
	}
	if raw := r.FormValue("params"); raw != "" {
		params := map[string]any{}
		if err := json.Unmarshal([]byte(raw), &params); err != nil {
			return nil, err
		}
		dto.Params = params
	}
	for key, values := range r.Form {
		if len(values) == 0 {
			continue
		}
		if prefix, ok := strings.CutPrefix(key, "params."); ok {
			if len(values) == 1 {
				dto.Params[prefix] = values[0]
			} else {
				dto.Params[prefix] = values
			}
		}
	}
	return dto, nil
}
