package testkit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/testenv"
	"github.com/stretchr/testify/require"
)

func TestScenarioControllerMountsEveryControlRoute(t *testing.T) {
	// Falsely green if the controller listed only the original scenario routes:
	// job, provider, clock and manifest controls would 404 in SDK ERP.
	var reached []string
	controller := &scenarioController{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = append(reached, r.Method+" "+r.URL.Path)
	})}
	router := mux.NewRouter()
	controller.Register(router)
	descriptor := controller.Descriptor()
	require.Len(t, descriptor.Routes, len(testenv.Routes()))
	for _, route := range testenv.Routes() {
		path := strings.NewReplacer("{scopeId}", "env-1", "{executionId}", "x", "{provider}", "p", "{gate}", "g").Replace(route.Pattern)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(route.Method, path, nil))
		require.Contains(t, reached, route.Method+" "+path)
	}
}
