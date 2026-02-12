package agents

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// Model is the primary interface for LLM interactions.
// It replaces LLMClient and provides capability-aware generation.
//
// Model implementations handle:
//   - API authentication and configuration
//   - Request/response serialization
//   - Provider-specific features (thinking, JSON mode, etc.)
//
// The same logical model (e.g., claude-3.5-sonnet) can have multiple
// implementations for different providers (Anthropic, Bedrock, Vertex).
//
// Example:
//
//	model := openai.NewModel(client, openai.ModelConfig{
//	    Name:      "gpt-5.2-2025-12-11",
//	    MaxTokens: 4096,
//	})
//
//	resp, err := model.Generate(ctx, req,
//	    WithMaxTokens(1000),
//	    WithReasoningEffort(ReasoningHigh),
//	)
type Model interface {
	// Generate sends a completion request and returns the full response.
	// This is a blocking call that waits for the complete response.
	// Options can customize the request (e.g., WithMaxTokens, WithReasoningEffort).
	Generate(ctx context.Context, req Request, opts ...GenerateOption) (*Response, error)

	// Stream sends a streaming completion request.
	// Returns a Generator that yields Chunk objects as they arrive.
	// Use this for real-time UI updates and progressive response display.
	// Returns an error immediately if the stream cannot be created.
	//
	// Example:
	//   gen, err := model.Stream(ctx, req)
	//   if err != nil { return err }
	//   defer gen.Close()
	//   for {
	//       chunk, err := gen.Next(ctx)
	//       if err == types.ErrGeneratorDone { break }
	//       if err != nil { return err }
	//       fmt.Print(chunk.Delta)
	//   }
	Stream(ctx context.Context, req Request, opts ...GenerateOption) (types.Generator[Chunk], error)

	// Info returns model metadata (name, provider, capabilities).
	// This is used for observability, logging, and capability checks.
	Info() ModelInfo

	// HasCapability checks if this model supports a specific capability.
	// Use this before passing capability-specific options to Generate.
	//
	// Example:
	//   if model.HasCapability(CapabilityThinking) {
	//       resp, _ := model.Generate(ctx, req, WithReasoningEffort(ReasoningHigh))
	//   }
	HasCapability(capability Capability) bool

	// Pricing returns pricing information for this model.
	// Used for cost tracking and budgeting.
	Pricing() ModelPricing
}

// ModelInfo describes a model for discovery and observability.
// It contains metadata about the model including its capabilities.
type ModelInfo struct {
	// Name is the model identifier (e.g., "gpt-5.2-2025-12-11", "claude-3-5-sonnet").
	Name string

	// Provider is the service provider (e.g., "openai", "anthropic", "bedrock", "vertex").
	// This allows the same model to be served via different providers.
	Provider string

	// ContextWindow is the provider/model max context length in tokens.
	// Zero means unknown/not declared by the model implementation.
	ContextWindow int

	// Capabilities lists supported features for this model.
	// Check capabilities before using capability-specific options.
	Capabilities []Capability
}

// String returns the full model identifier in "provider/name" format.
func (m ModelInfo) String() string {
	return fmt.Sprintf("%s/%s", m.Provider, m.Name)
}

// HasCapability checks if this model info contains a specific capability.
func (m ModelInfo) HasCapability(capability Capability) bool {
	for _, c := range m.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// Capability represents a model feature.
// Different models support different capabilities, and options
// should only be used when the model supports them.
type Capability string

const (
	// CapabilityStreaming indicates the model supports streaming responses.
	CapabilityStreaming Capability = "streaming"

	// CapabilityTools indicates the model supports function/tool calling.
	CapabilityTools Capability = "tools"

	// CapabilityVision indicates the model can process images.
	CapabilityVision Capability = "vision"

	// CapabilityThinking indicates the model supports extended thinking/reasoning.
	// Use WithReasoningEffort to control thinking depth.
	CapabilityThinking Capability = "thinking"

	// CapabilityJSONMode indicates the model can output JSON format.
	// Use WithJSONMode to enable.
	CapabilityJSONMode Capability = "json_mode"

	// CapabilityJSONSchema indicates the model supports JSON with schema validation.
	// Use WithJSONSchema to enable with a specific schema.
	CapabilityJSONSchema Capability = "json_schema"
)

// ModelPricing contains pricing information for LLM tokens.
// All prices are per 1 million tokens in the specified currency.
type ModelPricing struct {
	// Currency is the currency code (e.g., "USD", "EUR")
	Currency string

	// InputPer1M is the price per 1 million input tokens
	InputPer1M float64

	// OutputPer1M is the price per 1 million output tokens
	OutputPer1M float64

	// CacheWritePer1M is the price per 1 million cache write tokens
	// (for prompt caching). Zero if cache write pricing not applicable.
	CacheWritePer1M float64

	// CacheReadPer1M is the price per 1 million cache read tokens
	// (for prompt caching). Zero if cache read pricing not applicable.
	CacheReadPer1M float64
}

// CalculateCost computes the total cost from token usage.
// Returns the total cost in the pricing's currency.
func (p ModelPricing) CalculateCost(usage types.TokenUsage) float64 {
	inputCost := (float64(usage.PromptTokens) / 1_000_000) * p.InputPer1M
	outputCost := (float64(usage.CompletionTokens) / 1_000_000) * p.OutputPer1M
	cacheWriteCost := (float64(usage.CacheWriteTokens) / 1_000_000) * p.CacheWritePer1M
	cacheReadCost := (float64(usage.CacheReadTokens) / 1_000_000) * p.CacheReadPer1M

	return inputCost + outputCost + cacheWriteCost + cacheReadCost
}

// GenerateConfig holds configuration for Generate/Stream requests.
// This is populated by GenerateOption functions.
type GenerateConfig struct {
	// MaxTokens limits the response length.
	MaxTokens *int

	// ReasoningEffort controls thinking depth for models that support it.
	// Only effective when model has CapabilityThinking.
	ReasoningEffort *ReasoningEffort

	// JSONSchema enables structured JSON output with schema validation.
	// Only effective when model has CapabilityJSONSchema.
	JSONSchema any

	// JSONMode enables JSON output without schema validation.
	// Only effective when model has CapabilityJSONMode.
	JSONMode bool

	// Temperature controls randomness (0.0-2.0).
	// Lower values (0.3) are more deterministic and focused.
	// Higher values (1.0+) are more random and creative.
	Temperature *float64
}

// ReasoningEffort controls thinking depth for models that support extended thinking.
type ReasoningEffort string

const (
	// ReasoningLow uses minimal thinking for faster responses.
	ReasoningLow ReasoningEffort = "low"

	// ReasoningMedium uses moderate thinking (default for thinking-enabled requests).
	ReasoningMedium ReasoningEffort = "medium"

	// ReasoningHigh uses maximum thinking for complex problems.
	ReasoningHigh ReasoningEffort = "high"
)

// GenerateOption configures a Generate/Stream request.
type GenerateOption func(*GenerateConfig)

// WithMaxTokens sets the maximum completion tokens.
func WithMaxTokens(n int) GenerateOption {
	return func(c *GenerateConfig) {
		c.MaxTokens = &n
	}
}

// WithReasoningEffort sets thinking depth (requires CapabilityThinking).
// If the model doesn't support thinking, this option is ignored.
func WithReasoningEffort(effort ReasoningEffort) GenerateOption {
	return func(c *GenerateConfig) {
		c.ReasoningEffort = &effort
	}
}

// WithJSONSchema enables structured JSON output with schema validation.
// Requires CapabilityJSONSchema. If not supported, this option is ignored.
//
// The schema should be a JSON Schema object or a struct that can be
// converted to JSON Schema.
func WithJSONSchema(schema any) GenerateOption {
	return func(c *GenerateConfig) {
		c.JSONSchema = schema
	}
}

// WithJSONMode enables JSON output without schema validation.
// Requires CapabilityJSONMode. If not supported, this option is ignored.
func WithJSONMode() GenerateOption {
	return func(c *GenerateConfig) {
		c.JSONMode = true
	}
}

// WithTemperature sets the temperature for generation (0.0-2.0).
// Lower values produce more deterministic outputs.
func WithTemperature(temp float64) GenerateOption {
	return func(cfg *GenerateConfig) {
		cfg.Temperature = &temp
	}
}

// ApplyGenerateOptions applies options to a GenerateConfig.
func ApplyGenerateOptions(opts ...GenerateOption) GenerateConfig {
	var config GenerateConfig
	for _, opt := range opts {
		opt(&config)
	}
	return config
}

// Request is the input for Generate/Stream.
// This is similar to CompletionRequest but without the model field
// (the model is determined by the Model implementation itself).
type Request struct {
	// Messages is the conversation history.
	Messages []types.Message

	// Tools is the list of tools available to the model.
	Tools []Tool

	// PreviousResponseID is a provider continuity token for multi-turn state.
	// For OpenAI Responses API this maps to previous_response_id.
	PreviousResponseID *string
}

// Response is the output from Generate.
// It contains the assistant's response and metadata about the completion.
type Response struct {
	// Message is the assistant's response message.
	// It may contain tool calls if the model decided to use tools.
	Message types.Message

	// Usage tracks token consumption for this request.
	Usage types.TokenUsage

	// FinishReason indicates why the model stopped generating.
	// Common values: "stop", "tool_calls", "length", "content_filter".
	FinishReason string

	// Thinking contains the model's reasoning process.
	// Only populated when using WithReasoningEffort on models with CapabilityThinking.
	Thinking string

	// CodeInterpreterResults holds results from code interpreter executions.
	// Populated when code_interpreter tool is used.
	CodeInterpreterResults []types.CodeInterpreterResult

	// FileAnnotations holds references to files generated by code interpreter.
	// These files need to be downloaded and persisted.
	FileAnnotations []types.FileAnnotation

	// ProviderResponseID is the provider's response identifier for continuity.
	// For OpenAI Responses API this is the "resp_*" response ID.
	ProviderResponseID string
}

// Chunk is a streaming response piece.
// Chunks are yielded as the model generates tokens.
type Chunk struct {
	// Delta is the text content delta (partial content).
	Delta string

	// ToolCalls contains tool call deltas (accumulated incrementally).
	// Tool calls are built up across multiple chunks.
	ToolCalls []types.ToolCall

	// Usage is the final token usage (only present on the last chunk).
	Usage *types.TokenUsage

	// FinishReason is the reason why the model stopped generating.
	// Only present on the last chunk (when Done is true).
	// Common values: "stop", "length", "tool_calls", "content_filter".
	FinishReason string

	// Done indicates this is the final chunk.
	Done bool

	// Citations contains web search citations (only present on the final chunk).
	Citations []types.Citation

	// CodeInterpreterResults contains code execution results (only present on the final chunk).
	CodeInterpreterResults []types.CodeInterpreterResult

	// FileAnnotations contains file references from code interpreter (only present on the final chunk).
	FileAnnotations []types.FileAnnotation

	// ProviderResponseID is the provider's response identifier (only present on the final chunk).
	ProviderResponseID string
}
