package eskiz

import (
	"context"
	"errors"
	"sync"
	"time"

	eskizapi "github.com/iota-uz/eskiz"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	maxRetries = 3
	baseDelay  = time.Second
)

type tokenRefresher struct {
	client *eskizapi.APIClient
	cfg    Config

	mu    sync.Mutex
	token string
}

func (r *tokenRefresher) CurrentToken() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.token
}

func (r *tokenRefresher) RefreshToken(ctx context.Context) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.refreshTokenLocked(ctx)
}

func (r *tokenRefresher) refreshTokenLocked(ctx context.Context) (string, error) {
	const op serrors.Op = "TokenRefresher.RefreshToken"

	if ctx == nil {
		return "", serrors.E(op, serrors.KindValidation, "context cannot be nil")
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * baseDelay
			select {
			case <-ctx.Done():
				return "", serrors.E(op, ctx.Err())
			case <-time.After(delay):
			}
		}

		// Check context before attempting client operations
		if err := ctx.Err(); err != nil {
			return "", serrors.E(op, err)
		}

		if r.client == nil {
			return "", serrors.E(op, serrors.KindValidation, errors.New("client is not initialized"))
		}

		resp, httpResp, err := r.client.DefaultApi.
			Login(ctx).
			Email(r.cfg.Email()).
			Password(r.cfg.Password()).
			Execute()

		if httpResp != nil {
			_ = httpResp.Body.Close()
		}

		if err != nil {
			lastErr = err
			continue
		}

		data := resp.GetData()

		if data.Token == nil {
			lastErr = errors.New("access token is null in response from Eskiz auth API")
			continue
		}

		token := data.GetToken()
		if token == "" {
			lastErr = errors.New("received empty token from Eskiz auth API")
			continue
		}

		r.token = token
		return token, nil
	}

	return "", lastErr
}
