package eskiz

import (
	"context"
	"fmt"
	eskizapi "github.com/iota-uz/eskiz"
	"net/http"
)

type authRoundTripper struct {
	Base      http.RoundTripper
	Refresher *tokenRefresher
}

func (rt *authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	token := rt.Refresher.CurrentToken()
	if token == "" {
		var err error
		token, err = rt.Refresher.RefreshToken(req.Context())
		if err != nil {
			return nil, fmt.Errorf("failed to get token: %w", err)
		}
	}

	ctx := context.WithValue(context.Background(), eskizapi.ContextAccessToken, token)
	req1 := req.Clone(ctx)
	req1.Header.Set("Authorization", fmt.Sprintf("%s %s", "Bearer", token))

	resp, err := rt.Base.RoundTrip(req1)
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		return resp, err
	}

	token, err = rt.Refresher.RefreshToken(req.Context())
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token after 401: %w", err)
	}

	ctx = context.WithValue(context.Background(), eskizapi.ContextAccessToken, token)
	req2 := req.Clone(ctx)
	req2.Header.Set("Authorization", fmt.Sprintf("%s %s", "Bearer", token))

	return rt.Base.RoundTrip(req2)

}
