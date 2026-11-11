package originguard_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/originguard"
)

func req(method string, headers ...string) *http.Request {
	r := httptest.NewRequest(method, "https://erp.example.com/_ext/rpc", nil)
	for i := 0; i+1 < len(headers); i += 2 {
		r.Header.Set(headers[i], headers[i+1])
	}
	return r
}

func TestGuard(t *testing.T) {
	var zero *originguard.Guard
	trusted, err := originguard.New("https://admin.example.com")
	require.NoError(t, err)

	for _, g := range []*originguard.Guard{zero, {}, trusted} {
		require.NoError(t, g.Check(req("POST", "Sec-Fetch-Site", "same-origin")))
		require.NoError(t, g.Check(req("POST", "Sec-Fetch-Site", "none")))
		require.NoError(t, g.Check(req("POST")), "no browser headers: same-origin or not a browser")
		require.NoError(t, g.Check(req("POST", "Origin", "https://erp.example.com")))
		require.NoError(t, g.Check(req("GET", "Sec-Fetch-Site", "cross-site")), "safe methods are not state-changing")
		for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
			require.ErrorIs(t, g.Check(req(method, "Sec-Fetch-Site", "cross-site")), originguard.ErrCrossSite)
			require.ErrorIs(t, g.Check(req(method, "Sec-Fetch-Site", "same-site")), originguard.ErrCrossSite)
			require.ErrorIs(t, g.Check(req(method, "Origin", "https://evil.example.net")), originguard.ErrCrossSite, "old browsers: Origin differs from Host")
		}
	}
	require.NoError(t, trusted.Check(req("POST", "Sec-Fetch-Site", "cross-site", "Origin", "https://admin.example.com")))
	require.ErrorIs(t, trusted.Check(req("POST", "Sec-Fetch-Site", "cross-site", "Origin", "https://evil.example.net")), originguard.ErrCrossSite)

	_, err = originguard.New("not an origin")
	require.Error(t, err)
}
