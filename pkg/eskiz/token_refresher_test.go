package eskiz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenRefresher_CurrentToken(t *testing.T) {
	refresher := &tokenRefresher{
		token: "test-token",
	}

	token := refresher.CurrentToken()
	assert.Equal(t, "test-token", token)
}

func TestTokenRefresher_RefreshToken_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	refresher := &tokenRefresher{}

	token, err := refresher.RefreshToken(ctx)

	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Empty(t, token)
}

func TestTokenRefresher_RefreshToken_TimeoutContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(2 * time.Millisecond)

	refresher := &tokenRefresher{}

	token, err := refresher.RefreshToken(ctx)

	require.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
	assert.Empty(t, token)
}

func TestTokenRefresher_RefreshTokenLocked_NilContext(t *testing.T) {
	refresher := &tokenRefresher{}

	token, err := refresher.refreshTokenLocked(nil) //nolint:staticcheck // Testing nil context behavior

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context cannot be nil")
	assert.Empty(t, token)
}

func TestTokenRefresher_RefreshTokenLocked_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	refresher := &tokenRefresher{}

	token, err := refresher.refreshTokenLocked(ctx)

	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Empty(t, token)
}

func TestTokenRefresher_RefreshTokenLocked_TimeoutContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(2 * time.Millisecond)

	refresher := &tokenRefresher{}

	token, err := refresher.refreshTokenLocked(ctx)

	require.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
	assert.Empty(t, token)
}

func TestTokenRefresher_Concurrent_Access(t *testing.T) {
	refresher := &tokenRefresher{}

	const goroutines = 10
	done := make(chan bool, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer func() { done <- true }()

			refresher.CurrentToken()
		}()
	}

	for i := 0; i < goroutines; i++ {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Goroutine did not complete within timeout")
		}
	}
}

func TestTokenRefresher_Thread_Safety(t *testing.T) {
	refresher := &tokenRefresher{}

	const goroutines = 5
	done := make(chan bool, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			refresher.mu.Lock()
			refresher.token = string(rune('a' + id))
			refresher.mu.Unlock()
		}(i)
	}

	for i := 0; i < goroutines; i++ {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Goroutine did not complete within timeout")
		}
	}

	token := refresher.CurrentToken()
	assert.Len(t, token, 1)
	assert.Contains(t, "abcde", token)
}
