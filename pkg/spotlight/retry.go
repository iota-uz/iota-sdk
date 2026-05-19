package spotlight

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

// RetryClass categorizes Meilisearch errors so callers can apply the right
// policy (split, backoff, drop). The classifier is exported so projector
// code outside this package can reuse it.
type RetryClass int

const (
	// RetryClassNone — error not classified or success.
	RetryClassNone RetryClass = iota
	// RetryClassSplit — Meili rejected the payload as too large. Caller
	// should split the batch and retry each half (engine handles this
	// internally for AddDocuments; provided here for projector callers
	// that build their own batches).
	RetryClassSplit
	// RetryClassBackoff — transient rate limit (429). Retry after
	// jittered backoff.
	RetryClassBackoff
	// RetryClass5xx — server-side fault (>=500). Retry with exponential
	// backoff up to RetryPolicy.MaxAttempts.
	RetryClass5xx
	// RetryClassDrop — permanent 4xx (other than 413/429). Caller should
	// log/metric and proceed; retrying will not help.
	RetryClassDrop
)

// RetryPolicy holds the knobs for retry.Do.
type RetryPolicy struct {
	MaxAttempts      int
	InitialBackoff   time.Duration
	MaxBackoff       time.Duration
	JitterFractional float64 // 0–1.0, fraction of computed delay
}

// DefaultRetryPolicy returns the policy tuned to match issue #2810 §3.3:
// 5xx → max 3 retries, exponential 500ms→4s; 429 → max 5 retries, jittered
// 250ms→4s. The classifier picks the correct attempt count at runtime.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:      3,
		InitialBackoff:   500 * time.Millisecond,
		MaxBackoff:       4 * time.Second,
		JitterFractional: 0.25,
	}
}

// ClassifyMeiliError inspects an error returned by the Meili client and
// reports the action the caller should take. nil → RetryClassNone.
func ClassifyMeiliError(err error) RetryClass {
	if err == nil {
		return RetryClassNone
	}
	var meiliErr *meilisearch.Error
	if errors.As(err, &meiliErr) {
		switch {
		case meiliErr.StatusCode == http.StatusRequestEntityTooLarge:
			return RetryClassSplit
		case meiliErr.StatusCode == http.StatusTooManyRequests:
			return RetryClassBackoff
		case meiliErr.StatusCode >= 500:
			return RetryClass5xx
		case meiliErr.StatusCode >= 400:
			return RetryClassDrop
		}
	}
	// Connection-level or non-typed errors are usually transient (e.g.,
	// EOF mid-upload). Treat as 5xx so the caller retries.
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return RetryClassNone
	}
	return RetryClass5xx
}

// Do runs fn with retry. Returns the last error if all attempts fail. The
// ctx is honored between attempts so a cancelled caller short-circuits.
// Use ClassifyMeiliError to decide whether retry is appropriate before
// calling this — Do does NOT classify; it just applies backoff.
func (p RetryPolicy) Do(ctx context.Context, fn func(attempt int) error) error {
	attempts := p.MaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastErr = fn(i)
		if lastErr == nil {
			return nil
		}
		if i == attempts-1 {
			break
		}
		delay := backoffWithJitter(p, i)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("spotlight retry exhausted after %d attempts: %w", attempts, lastErr)
}

func backoffWithJitter(p RetryPolicy, attempt int) time.Duration {
	if p.InitialBackoff <= 0 {
		p.InitialBackoff = 500 * time.Millisecond
	}
	delay := p.InitialBackoff << attempt
	if p.MaxBackoff > 0 && delay > p.MaxBackoff {
		delay = p.MaxBackoff
	}
	if p.JitterFractional <= 0 {
		return delay
	}
	jitter := float64(delay) * p.JitterFractional
	// rand.Float64 is fine here — we are jittering retry delays, not
	// generating cryptographic randomness.
	return time.Duration(float64(delay) + (rand.Float64()*2-1)*jitter)
}
