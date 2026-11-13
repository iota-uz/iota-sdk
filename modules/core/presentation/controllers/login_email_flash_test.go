package controllers

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cookieByName(res *http.Response, name string) *http.Cookie {
	for _, c := range res.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestReadLoginEmailFlash(t *testing.T) {
	t.Parallel()

	t.Run("reads and expires a valid flash", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		req.AddCookie(&http.Cookie{Name: "email", Value: base64.URLEncoding.EncodeToString([]byte("user@example.com"))})
		rec := httptest.NewRecorder()

		assert.Equal(t, "user@example.com", readLoginEmailFlash(rec, req))
		cleared := cookieByName(rec.Result(), "email")
		require.NotNil(t, cleared)
		assert.Negative(t, cleared.MaxAge)
	})

	t.Run("expires an undecodable flash and prefills nothing", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		req.AddCookie(&http.Cookie{Name: "email", Value: "!!not-base64!!"})
		rec := httptest.NewRecorder()

		assert.Empty(t, readLoginEmailFlash(rec, req))
		cleared := cookieByName(rec.Result(), "email")
		require.NotNil(t, cleared)
		assert.Negative(t, cleared.MaxAge)
	})

	t.Run("falls back to the legacy query parameter", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/login?email=legacy@example.com", nil)
		assert.Equal(t, "legacy@example.com", readLoginEmailFlash(httptest.NewRecorder(), req))
	})
}

func TestSetLoginEmailFlash(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	setLoginEmailFlash(rec, "")
	assert.Nil(t, cookieByName(rec.Result(), "email"))

	rec = httptest.NewRecorder()
	setLoginEmailFlash(rec, "user@example.com")
	c := cookieByName(rec.Result(), "email")
	require.NotNil(t, c)
	decoded, err := base64.URLEncoding.DecodeString(c.Value)
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", string(decoded))
}
