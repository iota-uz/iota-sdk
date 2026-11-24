// Package eskiz provides this package.
package eskiz

import (
	"context"
	"fmt"
	"net/http"

	eskizapi "github.com/iota-uz/eskiz"
)

type authRoundTripper struct {
	Base      http.RoundTripper
	Refresher *tokenRefresher
	// NoReplay: after a 401 the token is renewed for the next request, but the refused request is not
	// sent again and its 401 is the answer (a caller that may not send twice).
	NoReplay bool
}

func (rt *authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if rt.Refresher == nil {
		return nil, fmt.Errorf("token refresher is not initialized")
	}

	if rt.Base == nil {
		return nil, fmt.Errorf("base round tripper is not initialized")
	}

	token := rt.Refresher.CurrentToken()
	if token == "" {
		var err error
		token, err = rt.Refresher.RefreshToken(req.Context())
		if err != nil {
			return nil, fmt.Errorf("failed to get token: %w", err)
		}
	}

	ctx := context.WithValue(req.Context(), eskizapi.ContextAccessToken, token)
	req1 := req.Clone(ctx)
	req1.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := rt.Base.RoundTrip(req1)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	if rt.NoReplay {
		// Renewing the token is a request of its own; the refused one stays the answer.
		if _, err := rt.Refresher.RefreshToken(req.Context()); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to refresh token after 401: %w", err)
		}
		return resp, nil
	}

	if resp.Body != nil {
		err := resp.Body.Close()
		if err != nil {
			return nil, err
		}
	}

	token, err = rt.Refresher.RefreshToken(req.Context())
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token after 401: %w", err)
	}

	ctx = context.WithValue(req.Context(), eskizapi.ContextAccessToken, token)
	req2 := req.Clone(ctx)
	req2.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp2, err := rt.Base.RoundTrip(req2)
	if err != nil {
		return nil, fmt.Errorf("retry request failed: %w", err)
	}

	return resp2, nil
}
