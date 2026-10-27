package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/jobs"
	"github.com/stretchr/testify/assert"
)

func TestGet_MissingJobStopsHTMXPolling(t *testing.T) {
	c := &JobController{runner: jobs.NewRunner(jobs.NewMemoryStore(0), jobs.NewRegistry(), jobs.RunnerOptions{})}
	for _, tc := range []struct {
		name   string
		htmx   bool
		status int
	}{
		{"plain", false, 404}, {"htmx", true, 286},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/jobs/missing", nil)
			r = mux.SetURLVars(r, map[string]string{"id": uuid.NewString()})
			if tc.htmx {
				r.Header.Set("Hx-Request", "true")
			}
			w := httptest.NewRecorder()
			c.Get(r, w)
			assert.Equal(t, tc.status, w.Code)
			if tc.htmx {
				assert.Empty(t, w.Body.String())
			}
		})
	}
}
