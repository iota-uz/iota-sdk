package eskiz

import (
	"context"
	"errors"
	"sync"
	"time"

	eskizapi "github.com/iota-uz/eskiz"
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
	if ctx == nil {
		return "", errors.New("context cannot be nil")
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * baseDelay
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
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
