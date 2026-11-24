package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

// An exempted route passes the CSRF middleware of its router; the same path on another router, and
// other routes of the same router, stay protected.
func TestCSRFExemptRoute(t *testing.T) {
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	withHook := mux.NewRouter()
	withHook.Use(CSRF(nil))
	CSRFExemptRoute(withHook.HandleFunc("/hooks/{id}", ok).Methods(http.MethodPost))
	withHook.HandleFunc("/other", ok).Methods(http.MethodPost)

	another := mux.NewRouter()
	another.Use(CSRF(nil))
	another.HandleFunc("/hooks/{id}", ok).Methods(http.MethodPost)

	post := func(h http.Handler, path string) int {
		r := httptest.NewRequest(http.MethodPost, "https://erp.test"+path, strings.NewReader(`{}`))
		r.Header.Set("Origin", "https://provider.example")
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	assert.Equal(t, http.StatusOK, post(withHook, "/hooks/1"))
	assert.Equal(t, http.StatusForbidden, post(withHook, "/other"), "another route of the same router")
	assert.Equal(t, http.StatusForbidden, post(another, "/hooks/1"), "the same path on another router")
	assert.Equal(t, http.StatusForbidden, post(CSRF(nil)(http.HandlerFunc(ok)), "/hooks/1"), "no matched route: protected")
	CSRFExemptRoute(nil)
}
