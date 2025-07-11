package eskiz

import (
	"context"
	"errors"
	eskizapi "github.com/iota-uz/eskiz"
	"sync"
	"time"
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
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * baseDelay
			time.Sleep(delay)
		}

		resp, _, err := r.client.DefaultApi.
			Login(ctx).
			Email(r.cfg.Email()).
			Password(r.cfg.Password()).
			Execute()

		if err != nil {
			lastErr = err
			continue
		}

		if resp == nil {
			lastErr = errors.New("received nil response from Eskiz auth API")
			continue
		}

		data := resp.GetData()

		if data.Token == nil {
			lastErr = errors.New("access token is null in response")
			continue
		}

		r.token = data.GetToken()

		return r.token, nil
	}

	return "", lastErr
}
