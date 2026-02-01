package agents

import "errors"

var (
	// ErrMaxIterations is returned when the agent exceeds max iterations.
	// This prevents infinite loops in the ReAct cycle.
	ErrMaxIterations = errors.New("bichat: max iterations reached")

	// ErrAgentNotFound is returned when a requested agent doesn't exist
	// in the registry.
	ErrAgentNotFound = errors.New("bichat: agent not found")

	// ErrCheckpointNotFound is returned when a checkpoint doesn't exist
	// during resume operations.
	ErrCheckpointNotFound = errors.New("bichat: checkpoint not found")

	// ErrCheckpointSaveFailed is returned when checkpoint save fails
	// during interrupt handling. This is a hard error because
	// without a checkpoint, the HITL flow cannot resume.
	ErrCheckpointSaveFailed = errors.New("bichat: checkpoint save failed")

	// ErrToolNotFound is returned when a requested tool doesn't exist
	// in the tool registry.
	ErrToolNotFound = errors.New("bichat: tool not found")

	// ErrGeneratorClosed is returned when trying to use a closed generator.
	ErrGeneratorClosed = errors.New("bichat: generator closed")

	// ErrModelNotFound is returned when a requested model doesn't exist
	// in the model registry.
	ErrModelNotFound = errors.New("bichat: model not found")

	// ErrContextOverflow is returned when context exceeds token limit.
	// This can be raised during prompt compilation or model execution.
	ErrContextOverflow = errors.New("bichat: context exceeds token limit")

	// ErrSessionNotFound is returned when a requested session doesn't exist.
	ErrSessionNotFound = errors.New("bichat: session not found")

	// ErrRateLimited is returned when rate limited by provider.
	ErrRateLimited = errors.New("bichat: rate limited by provider")

	// ErrProviderError is returned for provider-specific errors.
	ErrProviderError = errors.New("bichat: provider returned error")
)

// ProviderError wraps provider-specific errors with context.
// This provides structured information about provider failures
// to enable better error handling and retry logic.
type ProviderError struct {
	// Provider is the provider name (e.g., "openai", "anthropic")
	Provider string
	// StatusCode is the HTTP status code from the provider API
	StatusCode int
	// Message is the error message from the provider
	Message string
	// Retryable indicates if the error is transient and can be retried
	Retryable bool
	// Err is the underlying error
	Err error
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	return e.Message
}

// Unwrap returns the underlying error.
func (e *ProviderError) Unwrap() error {
	return e.Err
}

// ContextOverflowError provides details about context overflow.
// This helps callers understand the overflow and take appropriate action
// (truncation, compaction, error reporting, etc.).
type ContextOverflowError struct {
	// Requested is the number of tokens requested
	Requested int
	// Available is the number of tokens available
	Available int
	// Message is a human-readable error message
	Message string
}

// Error implements the error interface.
func (e *ContextOverflowError) Error() string {
	return e.Message
}
