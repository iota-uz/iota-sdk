package spotlight

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/meilisearch/meilisearch-go"
	"github.com/stretchr/testify/require"
)

func TestClassifyMeiliError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   error
		want RetryClass
	}{
		{nil, RetryClassNone},
		{&meilisearch.Error{StatusCode: http.StatusRequestEntityTooLarge}, RetryClassSplit},
		{&meilisearch.Error{StatusCode: http.StatusTooManyRequests}, RetryClassBackoff},
		{&meilisearch.Error{StatusCode: http.StatusBadGateway}, RetryClass5xx},
		{&meilisearch.Error{StatusCode: http.StatusBadRequest}, RetryClassDrop},
		{errors.New("unexpected EOF"), RetryClass5xx},
		{context.DeadlineExceeded, RetryClassNone},
		{context.Canceled, RetryClassNone},
	}
	for _, tc := range cases {
		got := ClassifyMeiliError(tc.in)
		require.Equalf(t, tc.want, got, "ClassifyMeiliError(%v)", tc.in)
	}
}

func TestRetryPolicy_DoStopsOnSuccessAndReportsFailure(t *testing.T) {
	t.Parallel()
	p := RetryPolicy{MaxAttempts: 3, InitialBackoff: 1 * time.Millisecond, MaxBackoff: 1 * time.Millisecond}
	calls := 0
	err := p.Do(context.Background(), func(attempt int) error {
		calls++
		if attempt == 1 {
			return nil
		}
		return errors.New("transient")
	})
	require.NoError(t, err)
	require.Equal(t, 2, calls)

	calls = 0
	err = p.Do(context.Background(), func(attempt int) error {
		calls++
		return errors.New("hard fail")
	})
	require.Error(t, err)
	require.Equal(t, 3, calls)
	require.Contains(t, err.Error(), "retry exhausted after 3 attempts")
}

func TestRetryPolicy_DoHonorsCancel(t *testing.T) {
	t.Parallel()
	p := RetryPolicy{MaxAttempts: 5, InitialBackoff: 50 * time.Millisecond, MaxBackoff: 50 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := p.Do(ctx, func(attempt int) error {
		return errors.New("never")
	})
	require.ErrorIs(t, err, context.Canceled)
}
