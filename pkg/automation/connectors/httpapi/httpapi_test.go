package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/httpapi"
)

func creds(kv ...string) connectors.Credentials {
	c := connectors.Credentials{"base_url": "https://api.example.com/v1"}
	for i := 0; i+1 < len(kv); i += 2 {
		c[kv[i]] = kv[i+1]
	}
	return c
}

func TestRegistersInADirectoryAndTheCatalog(t *testing.T) {
	c := httpapi.New()
	require.NoError(t, connectors.NewDirectory(catalog.NewRegistry(nil)).Register(c))
	assert.Equal(t, catalog.KindHTTPAPI, c.Kind())
	d := c.Descriptor()
	assert.Equal(t, connectors.CategoryOther, d.Category)
	assert.Equal(t, connectors.AuthCustom, d.Auth)
	for _, lang := range []string{"en", "ru", "uz"} {
		assert.NotEmpty(t, d.Title[lang])
		assert.NotEmpty(t, d.Description[lang])
	}
	secrets := connectors.SecretFields(d)
	assert.True(t, secrets["token"])
	assert.True(t, secrets["password"])
	assert.False(t, secrets["base_url"])
	access := c.(connectors.CodeAccessor).CodeAccess()
	assert.Equal(t, []string{"get", "post", "put", "patch", "delete"}, access.MethodNames())
	assert.True(t, access.Fetch)
	assert.Empty(t, access.BaseURL)
	assert.Empty(t, access.Operations)
	_, operates := c.(connectors.CodeOperator)
	assert.False(t, operates)
}

func TestAuthorize(t *testing.T) {
	c := httpapi.New().(connectors.CodeAccessor)
	ctx := context.Background()
	cases := []struct {
		name  string
		creds connectors.Credentials
		check func(t *testing.T, h http.Header)
	}{
		{"none", creds(), func(t *testing.T, h http.Header) { assert.Empty(t, h.Get("Authorization")) }},
		{"none, explicit", creds("auth", "none", "token", "ignored"), func(t *testing.T, h http.Header) { assert.Empty(t, h) }},
		{"bearer", creds("auth", "bearer", "token", "tok-123"), func(t *testing.T, h http.Header) {
			assert.Equal(t, "Bearer tok-123", h.Get("Authorization"))
		}},
		{"header", creds("auth", "header", "header_name", "X-Api-Key", "token", "key-9"), func(t *testing.T, h http.Header) {
			assert.Equal(t, "key-9", h.Get("X-Api-Key"))
			assert.Empty(t, h.Get("Authorization"))
		}},
		{"basic", creds("auth", "basic", "username", "ann", "password", "p:w"), func(t *testing.T, h http.Header) {
			r := &http.Request{Header: h}
			u, p, ok := r.BasicAuth()
			assert.True(t, ok)
			assert.Equal(t, "ann", u)
			assert.Equal(t, "p:w", p)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			require.NoError(t, c.Authorize(ctx, tc.creds, h))
			tc.check(t, h)
		})
	}
	t.Run("replaces the authorization the code set", func(t *testing.T) {
		h := http.Header{"Authorization": {"Bearer from-code"}}
		require.NoError(t, c.Authorize(ctx, creds("auth", "bearer", "token", "tok-123"), h))
		assert.Equal(t, []string{"Bearer tok-123"}, h.Values("Authorization"))
	})
}

func TestAuthorizeRefusesBadSettingsWithoutLeaking(t *testing.T) {
	c := httpapi.New().(connectors.CodeAccessor)
	for name, cr := range map[string]connectors.Credentials{
		"bearer without a token": creds("auth", "bearer"),
		"header without a name":  creds("auth", "header", "token", "SECRET-TOKEN"),
		"a forbidden header":     creds("auth", "header", "header_name", "Host", "token", "SECRET-TOKEN"),
		"a bad header":           creds("auth", "header", "header_name", "X Key", "token", "SECRET-TOKEN"),
		"basic without a login":  creds("auth", "basic", "password", "SECRET-PASS"),
		"a colon in the login":   creds("auth", "basic", "username", "a:b", "password", "SECRET-PASS"),
		"an unknown mode":        creds("auth", "digest", "token", "SECRET-TOKEN"),
		"a line break":           creds("auth", "bearer", "token", "SECRET\r\nX-Evil: 1"),
	} {
		err := c.Authorize(context.Background(), cr, http.Header{})
		require.Error(t, err, name)
		assert.NotContains(t, err.Error(), "SECRET", name)
	}
}

func TestCodeBaseURL(t *testing.T) {
	c := httpapi.New().(connectors.CodeBaseURLResolver)
	u, err := c.CodeBaseURL(creds())
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com/v1", u)

	for name, base := range map[string]string{
		"http":        "http://api.example.com/v1",
		"credentials": "https://user:hunter2@api.example.com",
		"query":       "https://api.example.com/v1?key=hunter2",
		"fragment":    "https://api.example.com/#hunter2",
		"no host":     "https:///hunter2",
		"empty":       "",
		"not a URL":   "https://exa mple.com/hunter2",
	} {
		_, err := c.CodeBaseURL(connectors.Credentials{"base_url": base})
		require.Error(t, err, name)
		assert.NotContains(t, err.Error(), "hunter2", name)
		assert.NotContains(t, err.Error(), "example.com", name, "the URL is not echoed")
	}
}

func TestTestValidatesWithoutTheNetwork(t *testing.T) {
	c := httpapi.New()
	id, err := c.Test(context.Background(), creds("auth", "bearer", "token", "tok"))
	require.NoError(t, err)
	assert.Equal(t, "api.example.com", id.Display)
	assert.Equal(t, "api.example.com", id.External)
	assert.Empty(t, id.Kind, "an untyped identity")

	_, err = c.Test(context.Background(), creds("auth", "bearer"))
	require.Error(t, err)
	_, err = c.Test(context.Background(), connectors.Credentials{"base_url": "http://" + strings.Repeat("a", 10)})
	require.Error(t, err)
}
