//go:build !dev

package agentsignin

import (
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Falsely green if the test registers a different controller or tests only GET.
func TestProductionRoutesAbsent(t *testing.T) {
	c, err := NewController(Options{})
	require.NoError(t, err)
	require.Nil(t, c)
	r := mux.NewRouter()
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, Path, nil))
		require.Equal(t, http.StatusNotFound, w.Code)
	}
	_, err = NewController(Options{Enabled: true})
	require.Error(t, err)
}
