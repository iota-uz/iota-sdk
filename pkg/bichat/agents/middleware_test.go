package agents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestWrap(t *testing.T) {
	t.Parallel()

	callOrder := []string{}
	baseFunc := func(ctx context.Context, req Request) (*Response, error) {
		callOrder = append(callOrder, "base")
		return &Response{Message: NewAssistantMessage("test", nil)}, nil
	}

	middleware1 := func(next ModelFunc) ModelFunc {
		return func(ctx context.Context, req Request) (*Response, error) {
			callOrder = append(callOrder, "middleware1-before")
			resp, err := next(ctx, req)
			callOrder = append(callOrder, "middleware1-after")
			return resp, err
		}
	}

	middleware2 := func(next ModelFunc) ModelFunc {
		return func(ctx context.Context, req Request) (*Response, error) {
			callOrder = append(callOrder, "middleware2-before")
			resp, err := next(ctx, req)
			callOrder = append(callOrder, "middleware2-after")
			return resp, err
		}
	}

	wrapped := Wrap(baseFunc, middleware1, middleware2)
	ctx := context.Background()
	req := Request{Messages: []Message{NewUserMessage("test")}}

	_, err := wrapped(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify execution order: middleware1 (outer) → middleware2 (inner) → base
	expected := []string{
		"middleware1-before",
		"middleware2-before",
		"base",
		"middleware2-after",
		"middleware1-after",
	}

	if len(callOrder) != len(expected) {
		t.Fatalf("expected %d calls, got %d", len(expected), len(callOrder))
	}

	for i, exp := range expected {
		if callOrder[i] != exp {
			t.Errorf("call %d: expected %s, got %s", i, exp, callOrder[i])
		}
	}
}

func TestWithLogging_Success(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetOutput(logrus.StandardLogger().Out) // Suppress output in tests

	baseFunc := func(ctx context.Context, req Request) (*Response, error) {
		return &Response{
			Message: NewAssistantMessage("test response", nil),
			Usage: TokenUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
			FinishReason: "stop",
		}, nil
	}

	wrapped := WithLogging(logger)(baseFunc)
	ctx := context.Background()
	req := Request{
		Messages: []Message{NewUserMessage("test")},
		Tools:    []Tool{},
	}

	resp, err := wrapped(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Usage.TotalTokens != 30 {
		t.Errorf("expected 30 total tokens, got %d", resp.Usage.TotalTokens)
	}
}

func TestWithLogging_Error(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetOutput(logrus.StandardLogger().Out)

	expectedErr := errors.New("test error")
	baseFunc := func(ctx context.Context, req Request) (*Response, error) {
		return nil, expectedErr
	}

	wrapped := WithLogging(logger)(baseFunc)
	ctx := context.Background()
	req := Request{Messages: []Message{NewUserMessage("test")}}

	resp, err := wrapped(ctx, req)
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	if resp != nil {
		t.Error("expected nil response on error")
	}
}

func TestExponentialBackoff_NextWait(t *testing.T) {
	t.Parallel()

	backoff := &ExponentialBackoff{
		BaseDelay: 100 * time.Millisecond,
		MaxDelay:  1 * time.Second,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 800 * time.Millisecond},
		{5, 1 * time.Second}, // Capped at MaxDelay
		{6, 1 * time.Second}, // Capped at MaxDelay
	}

	for _, tc := range tests {
		actual := backoff.NextWait(tc.attempt)
		if actual != tc.expected {
			t.Errorf("attempt %d: expected %v, got %v", tc.attempt, tc.expected, actual)
		}
	}
}

func TestWithRetry_Success(t *testing.T) {
	t.Parallel()

	callCount := 0
	baseFunc := func(ctx context.Context, req Request) (*Response, error) {
		callCount++
		return &Response{Message: NewAssistantMessage("success", nil)}, nil
	}

	backoff := &ExponentialBackoff{
		BaseDelay: 1 * time.Millisecond,
		MaxDelay:  10 * time.Millisecond,
	}
	wrapped := WithRetry(3, backoff)(baseFunc)
	ctx := context.Background()
	req := Request{Messages: []Message{NewUserMessage("test")}}

	resp, err := wrapped(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	if callCount != 1 {
		t.Errorf("expected 1 call on success, got %d", callCount)
	}
}

func TestWithRetry_EventualSuccess(t *testing.T) {
	t.Parallel()

	callCount := 0
	baseFunc := func(ctx context.Context, req Request) (*Response, error) {
		callCount++
		if callCount < 3 {
			return nil, errors.New("transient error")
		}
		return &Response{Message: NewAssistantMessage("success", nil)}, nil
	}

	backoff := &ExponentialBackoff{
		BaseDelay: 1 * time.Millisecond,
		MaxDelay:  10 * time.Millisecond,
	}
	wrapped := WithRetry(3, backoff)(baseFunc)
	ctx := context.Background()
	req := Request{Messages: []Message{NewUserMessage("test")}}

	resp, err := wrapped(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	if callCount != 3 {
		t.Errorf("expected 3 calls (2 failures + 1 success), got %d", callCount)
	}
}

func TestWithRetry_MaxAttemptsExceeded(t *testing.T) {
	t.Parallel()

	callCount := 0
	expectedErr := errors.New("persistent error")
	baseFunc := func(ctx context.Context, req Request) (*Response, error) {
		callCount++
		return nil, expectedErr
	}

	backoff := &ExponentialBackoff{
		BaseDelay: 1 * time.Millisecond,
		MaxDelay:  10 * time.Millisecond,
	}
	wrapped := WithRetry(3, backoff)(baseFunc)
	ctx := context.Background()
	req := Request{Messages: []Message{NewUserMessage("test")}}

	resp, err := wrapped(ctx, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if resp != nil {
		t.Error("expected nil response on error")
	}

	if callCount != 3 {
		t.Errorf("expected 3 calls (max attempts), got %d", callCount)
	}

	// Verify error message mentions max retries
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error to wrap %v", expectedErr)
	}
}

func TestWithRetry_ContextCanceled(t *testing.T) {
	t.Parallel()

	callCount := 0
	baseFunc := func(ctx context.Context, req Request) (*Response, error) {
		callCount++
		return nil, errors.New("transient error")
	}

	backoff := &ExponentialBackoff{
		BaseDelay: 100 * time.Millisecond, // Long delay to allow cancellation
		MaxDelay:  1 * time.Second,
	}
	wrapped := WithRetry(5, backoff)(baseFunc)

	ctx, cancel := context.WithCancel(context.Background())
	req := Request{Messages: []Message{NewUserMessage("test")}}

	// Cancel context after first failure
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	resp, err := wrapped(ctx, req)
	if err == nil {
		t.Fatal("expected error due to context cancellation")
	}

	if resp != nil {
		t.Error("expected nil response on error")
	}

	// Should have made at least 1 attempt before cancellation
	if callCount < 1 {
		t.Errorf("expected at least 1 call, got %d", callCount)
	}
}

func TestWithRateLimit(t *testing.T) {
	t.Parallel()

	limiter := &mockRateLimiter{
		waitCalls: 0,
	}

	callCount := 0
	baseFunc := func(ctx context.Context, req Request) (*Response, error) {
		callCount++
		return &Response{Message: NewAssistantMessage("test", nil)}, nil
	}

	wrapped := WithRateLimit(limiter)(baseFunc)
	ctx := context.Background()
	req := Request{Messages: []Message{NewUserMessage("test")}}

	resp, err := wrapped(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	if limiter.waitCalls != 1 {
		t.Errorf("expected 1 Wait call, got %d", limiter.waitCalls)
	}

	if callCount != 1 {
		t.Errorf("expected 1 base function call, got %d", callCount)
	}
}

func TestWithRateLimit_Error(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("rate limit error")
	limiter := &mockRateLimiter{
		waitError: expectedErr,
	}

	baseFunc := func(ctx context.Context, req Request) (*Response, error) {
		t.Error("base function should not be called if rate limit fails")
		return nil, nil
	}

	wrapped := WithRateLimit(limiter)(baseFunc)
	ctx := context.Background()
	req := Request{Messages: []Message{NewUserMessage("test")}}

	resp, err := wrapped(ctx, req)
	if err == nil {
		t.Fatal("expected error from rate limiter")
	}

	if resp != nil {
		t.Error("expected nil response on rate limit error")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error to wrap %v", expectedErr)
	}
}

func TestWithTracing(t *testing.T) {
	t.Parallel()

	tracer := &mockTracer{
		finishCalls: 0,
	}

	callCount := 0
	baseFunc := func(ctx context.Context, req Request) (*Response, error) {
		callCount++
		return &Response{Message: NewAssistantMessage("test", nil)}, nil
	}

	wrapped := WithTracing(tracer)(baseFunc)
	ctx := context.Background()
	req := Request{Messages: []Message{NewUserMessage("test")}}

	resp, err := wrapped(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	if tracer.startCalls != 1 {
		t.Errorf("expected 1 StartSpan call, got %d", tracer.startCalls)
	}

	if tracer.finishCalls != 1 {
		t.Errorf("expected 1 finish call, got %d", tracer.finishCalls)
	}

	if callCount != 1 {
		t.Errorf("expected 1 base function call, got %d", callCount)
	}
}

func TestWithTracing_Error(t *testing.T) {
	t.Parallel()

	tracer := &mockTracer{}
	expectedErr := errors.New("test error")

	baseFunc := func(ctx context.Context, req Request) (*Response, error) {
		return nil, expectedErr
	}

	wrapped := WithTracing(tracer)(baseFunc)
	ctx := context.Background()
	req := Request{Messages: []Message{NewUserMessage("test")}}

	resp, err := wrapped(ctx, req)
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	if resp != nil {
		t.Error("expected nil response on error")
	}

	if tracer.finishCalls != 1 {
		t.Errorf("expected 1 finish call even on error, got %d", tracer.finishCalls)
	}

	if tracer.finishError != expectedErr {
		t.Errorf("expected finish to receive error %v, got %v", expectedErr, tracer.finishError)
	}
}

func TestMiddlewareChain(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetOutput(logrus.StandardLogger().Out)

	limiter := &mockRateLimiter{}
	tracer := &mockTracer{}
	backoff := &ExponentialBackoff{
		BaseDelay: 1 * time.Millisecond,
		MaxDelay:  10 * time.Millisecond,
	}

	baseFunc := func(ctx context.Context, req Request) (*Response, error) {
		return &Response{
			Message: NewAssistantMessage("test", nil),
			Usage: TokenUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
			FinishReason: "stop",
		}, nil
	}

	// Apply full middleware chain
	wrapped := Wrap(baseFunc,
		WithLogging(logger),
		WithRetry(3, backoff),
		WithRateLimit(limiter),
		WithTracing(tracer),
	)

	ctx := context.Background()
	req := Request{Messages: []Message{NewUserMessage("test")}}

	resp, err := wrapped(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	// Verify all middleware was applied
	if limiter.waitCalls != 1 {
		t.Errorf("expected rate limiter to be called")
	}

	if tracer.startCalls != 1 {
		t.Errorf("expected tracer to be called")
	}
}

// mockRateLimiter is a test implementation of RateLimiter
type mockRateLimiter struct {
	waitCalls int
	waitError error
}

func (m *mockRateLimiter) Wait(ctx context.Context) error {
	m.waitCalls++
	return m.waitError
}

// mockTracer is a test implementation of Tracer
type mockTracer struct {
	startCalls  int
	finishCalls int
	finishError error
}

func (m *mockTracer) StartSpan(ctx context.Context, name string) (context.Context, func(error)) {
	m.startCalls++
	return ctx, func(err error) {
		m.finishCalls++
		m.finishError = err
	}
}
