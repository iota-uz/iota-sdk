package agents

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// ModelFunc is the signature for Model.Generate.
// Middleware wraps ModelFunc to add cross-cutting concerns like logging,
// retries, rate limiting, and tracing.
//
// Example:
//
//	generateFunc := func(ctx context.Context, req Request) (*Response, error) {
//	    return model.Generate(ctx, req)
//	}
//
//	wrapped := WithLogging(logger)(generateFunc)
//	resp, err := wrapped(ctx, req)
type ModelFunc func(ctx context.Context, req Request) (*Response, error)

// Middleware wraps a ModelFunc to add additional behavior.
// Middleware follows the standard Go middleware pattern:
//
//	middleware := func(next ModelFunc) ModelFunc {
//	    return func(ctx context.Context, req Request) (*Response, error) {
//	        // Before logic
//	        resp, err := next(ctx, req) // Call next in chain
//	        // After logic
//	        return resp, err
//	    }
//	}
//
// Common middleware use cases:
//   - Logging: Log requests/responses for observability
//   - Retries: Retry on transient errors with exponential backoff
//   - Rate limiting: Prevent exceeding API quotas
//   - Tracing: Add distributed tracing spans
//   - Metrics: Record latency, token usage, error rates
//   - Validation: Validate requests before sending
type Middleware func(next ModelFunc) ModelFunc

// Wrap applies a chain of middleware to a ModelFunc.
// Middleware is applied in order: first middleware wraps second, etc.
//
// Example:
//
//	generateFunc := func(ctx context.Context, req Request) (*Response, error) {
//	    return model.Generate(ctx, req)
//	}
//
//	wrapped := Wrap(generateFunc,
//	    WithLogging(logger),           // Applied first (outermost)
//	    WithRetry(3, exponentialBackoff), // Applied second
//	    WithRateLimit(rateLimiter),    // Applied third (innermost)
//	)
//
//	// Execution order:
//	// 1. WithLogging logs request
//	// 2. WithRetry checks retry attempts
//	// 3. WithRateLimit checks rate limit
//	// 4. Actual model.Generate() call
//	// 5. WithRateLimit updates rate limit
//	// 6. WithRetry handles errors/retries
//	// 7. WithLogging logs response
func Wrap(fn ModelFunc, middleware ...Middleware) ModelFunc {
	// Apply middleware in reverse order so first middleware is outermost
	for i := len(middleware) - 1; i >= 0; i-- {
		fn = middleware[i](fn)
	}
	return fn
}

// WithLogging adds request/response logging.
// Logs include request size, response tokens, latency, and errors.
//
// Example:
//
//	wrapped := WithLogging(logger)(generateFunc)
//	resp, err := wrapped(ctx, req)
//
// Logged fields:
//   - message_count: Number of messages in request
//   - tool_count: Number of tools available
//   - duration_ms: Request latency in milliseconds
//   - prompt_tokens: Input tokens consumed
//   - completion_tokens: Output tokens generated
//   - total_tokens: Total tokens consumed
//   - finish_reason: Why the model stopped generating
//   - error: Error message if request failed
func WithLogging(logger *logrus.Logger) Middleware {
	return func(next ModelFunc) ModelFunc {
		return func(ctx context.Context, req Request) (*Response, error) {
			start := time.Now()

			// Log request
			logger.WithFields(logrus.Fields{
				"message_count": len(req.Messages),
				"tool_count":    len(req.Tools),
			}).Debug("Model.Generate request started")

			// Call next in chain
			resp, err := next(ctx, req)

			duration := time.Since(start)

			// Log response/error
			fields := logrus.Fields{
				"duration_ms": duration.Milliseconds(),
			}

			if err != nil {
				fields["error"] = err.Error()
				logger.WithFields(fields).Error("Model.Generate request failed")
				return nil, err
			}

			fields["prompt_tokens"] = resp.Usage.PromptTokens
			fields["completion_tokens"] = resp.Usage.CompletionTokens
			fields["total_tokens"] = resp.Usage.TotalTokens
			fields["finish_reason"] = resp.FinishReason

			logger.WithFields(fields).Info("Model.Generate request completed")

			return resp, nil
		}
	}
}

// BackoffStrategy calculates wait time before retry attempt.
// Implementations can provide exponential backoff, linear backoff,
// or custom strategies.
type BackoffStrategy interface {
	// NextWait returns the duration to wait before the next retry attempt.
	// attempt is 1-based (1 for first retry, 2 for second, etc).
	NextWait(attempt int) time.Duration
}

// ExponentialBackoff implements exponential backoff with jitter.
// Wait time doubles with each retry: base, 2*base, 4*base, etc.
//
// Example:
//
//	backoff := &ExponentialBackoff{
//	    BaseDelay: 100 * time.Millisecond,
//	    MaxDelay:  5 * time.Second,
//	}
//	wrapped := WithRetry(3, backoff)(generateFunc)
type ExponentialBackoff struct {
	// BaseDelay is the initial wait time before first retry.
	BaseDelay time.Duration

	// MaxDelay is the maximum wait time (prevents unbounded growth).
	MaxDelay time.Duration
}

// NextWait returns the duration to wait before the next retry attempt.
// Uses exponential backoff: base * 2^(attempt-1).
func (b *ExponentialBackoff) NextWait(attempt int) time.Duration {
	delay := b.BaseDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay > b.MaxDelay {
			return b.MaxDelay
		}
	}
	return delay
}

// WithRetry adds retry logic with exponential backoff.
// Retries on errors, up to maxAttempts total attempts (including initial).
//
// Example:
//
//	backoff := &ExponentialBackoff{
//	    BaseDelay: 100 * time.Millisecond,
//	    MaxDelay:  5 * time.Second,
//	}
//	wrapped := WithRetry(3, backoff)(generateFunc)
//	resp, err := wrapped(ctx, req)
//	// Will retry up to 2 times (3 total attempts) with exponential backoff
//
// Notes:
//   - Only retries on errors (not on successful responses)
//   - Does not distinguish between transient vs permanent errors
//   - Consider using context deadline to prevent unbounded retries
func WithRetry(maxAttempts int, backoff BackoffStrategy) Middleware {
	return func(next ModelFunc) ModelFunc {
		return func(ctx context.Context, req Request) (*Response, error) {
			var lastErr error

			for attempt := 1; attempt <= maxAttempts; attempt++ {
				resp, err := next(ctx, req)
				if err == nil {
					return resp, nil
				}

				lastErr = err

				// Don't sleep after last attempt
				if attempt < maxAttempts {
					// Check context before sleeping
					if ctx.Err() != nil {
						return nil, fmt.Errorf("retry canceled: %w", ctx.Err())
					}

					wait := backoff.NextWait(attempt)
					time.Sleep(wait)
				}
			}

			return nil, fmt.Errorf("max retries (%d) exceeded: %w", maxAttempts, lastErr)
		}
	}
}

// RateLimiter controls the rate of requests.
// Implementations can use token bucket, leaky bucket, or other algorithms.
type RateLimiter interface {
	// Wait blocks until request is allowed, or context is canceled.
	// Returns error if context is canceled while waiting.
	Wait(ctx context.Context) error
}

// WithRateLimit adds rate limiting to prevent exceeding API quotas.
// Uses the provided RateLimiter implementation to control request rate.
//
// Example:
//
//	import "golang.org/x/time/rate"
//
//	limiter := rate.NewLimiter(rate.Every(time.Second), 10) // 10 req/sec
//	wrapped := WithRateLimit(limiter)(generateFunc)
//	resp, err := wrapped(ctx, req)
//
// Notes:
//   - Blocks until rate limit allows the request
//   - Returns error if context is canceled while waiting
//   - Integrates with golang.org/x/time/rate or custom implementations
func WithRateLimit(limiter RateLimiter) Middleware {
	return func(next ModelFunc) ModelFunc {
		return func(ctx context.Context, req Request) (*Response, error) {
			// Wait for rate limit
			if err := limiter.Wait(ctx); err != nil {
				return nil, fmt.Errorf("rate limit wait failed: %w", err)
			}

			// Proceed with request
			return next(ctx, req)
		}
	}
}

// Tracer represents a distributed tracing backend.
// Implementations can integrate with OpenTelemetry, Jaeger, Datadog, etc.
type Tracer interface {
	// StartSpan creates a new tracing span.
	// Returns a context with the span attached and a finish function.
	// The finish function should be called when the operation completes.
	//
	// Example:
	//   ctx, finish := tracer.StartSpan(ctx, "model.generate")
	//   defer finish(err)
	StartSpan(ctx context.Context, name string) (context.Context, func(error))
}

// WithTracing adds distributed tracing spans.
// Creates a span for each Generate call with timing and error information.
//
// Example:
//
//	import "go.opentelemetry.io/otel"
//
//	tracer := &OpenTelemetryTracer{tracer: otel.Tracer("bichat")}
//	wrapped := WithTracing(tracer)(generateFunc)
//	resp, err := wrapped(ctx, req)
//
// Notes:
//   - Span includes duration, error status, and context propagation
//   - Integrates with OpenTelemetry, Jaeger, or custom backends
//   - Useful for debugging latency and error propagation
func WithTracing(tracer Tracer) Middleware {
	return func(next ModelFunc) ModelFunc {
		return func(ctx context.Context, req Request) (*Response, error) {
			ctx, finish := tracer.StartSpan(ctx, "model.generate")
			resp, err := next(ctx, req)
			finish(err)
			return resp, err
		}
	}
}
