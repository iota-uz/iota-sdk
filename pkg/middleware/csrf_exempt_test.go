package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A prefix exempted by a module passes every CSRF middleware, however the application configured it;
// other paths stay protected.
func TestExemptFromCSRF(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := CSRF(nil)(ok)
	post := func(path string) int {
		r := httptest.NewRequest(http.MethodPost, "https://erp.test"+path, strings.NewReader(`{}`))
		r.Header.Set("Origin", "https://provider.example")
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	assert.Equal(t, http.StatusForbidden, post("/csrf-exempt-test/hook"), "a cross-site POST without a token is refused")
	ExemptFromCSRF("/csrf-exempt-test/", " ", "/")
	assert.Equal(t, http.StatusOK, post("/csrf-exempt-test/hook"))
	assert.Equal(t, http.StatusForbidden, post("/other"), "only the exempted prefix")
}
