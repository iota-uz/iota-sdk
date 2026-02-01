package llmproviders

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sashabaranov/go-openai"
)

// OpenAIModel implements the agents.Model interface using OpenAI's API.
// It provides both blocking and streaming completion modes with tool calling support.
type OpenAIModel struct {
	client    *openai.Client
	modelName string
}

// NewOpenAIModel creates a new OpenAI model from environment variables.
// It reads OPENAI_API_KEY (required) and OPENAI_MODEL (optional, defaults to gpt-4).
// Returns an error if OPENAI_API_KEY is not set.
func NewOpenAIModel() (agents.Model, error) {
	const op serrors.Op = "llmproviders.NewOpenAIModel"

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, serrors.E(op, "OPENAI_API_KEY environment variable is required")
	}

	modelName := os.Getenv("OPENAI_MODEL")
	if modelName == "" {
		modelName = openai.GPT4 // Default to gpt-4
	}

	client := openai.NewClient(apiKey)

	return &OpenAIModel{
		client:    client,
		modelName: modelName,
	}, nil
}

// Generate sends a blocking completion request to OpenAI.
// It supports tool calling, JSON mode, and temperature control via options.
func (m *OpenAIModel) Generate(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error) {
	const op serrors.Op = "OpenAIModel.Generate"

	// Apply options
	config := agents.ApplyGenerateOptions(opts...)

	// Build OpenAI request
	oaiReq := m.buildChatCompletionRequest(req, config)

	// Call OpenAI API
	resp, err := m.client.CreateChatCompletion(ctx, oaiReq)
	if err != nil {
		return nil, serrors.E(op, err, "OpenAI API request failed")
	}

	// Validate response
	if len(resp.Choices) == 0 {
		return nil, serrors.E(op, "OpenAI returned empty response")
	}

	choice := resp.Choices[0]

	// Build message with content and tool calls
	msg := types.Message{
		Role:    types.RoleAssistant,
		Content: choice.Message.Content,
	}

	// Convert tool calls
	if len(choice.Message.ToolCalls) > 0 {
		msg.ToolCalls = make([]types.ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			msg.ToolCalls[i] = types.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}
		}
	}

	return &agents.Response{
		Message: msg,
		Usage: types.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
		FinishReason: string(choice.FinishReason),
	}, nil
}

// Stream sends a streaming completion request to OpenAI.
// Returns a Generator that yields Chunk objects as tokens arrive.
func (m *OpenAIModel) Stream(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) types.Generator[agents.Chunk] {
	config := agents.ApplyGenerateOptions(opts...)

	return types.NewGenerator(ctx, func(genCtx context.Context, yield func(agents.Chunk) bool) error {
		const op serrors.Op = "OpenAIModel.Stream"

		// Build OpenAI streaming request
		oaiReq := m.buildChatCompletionRequest(req, config)

		// Create stream
		stream, err := m.client.CreateChatCompletionStream(genCtx, oaiReq)
		if err != nil {
			return serrors.E(op, err, "failed to create OpenAI stream")
		}
		defer stream.Close()

		// Accumulate tool calls across chunks
		toolCallsMap := make(map[int]*types.ToolCall)
		var totalUsage *types.TokenUsage

		// Stream chunks
		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				// Send final chunk with usage if available
				if totalUsage != nil {
					if !yield(agents.Chunk{
						Done:  true,
						Usage: totalUsage,
					}) {
						return nil // Generator cancelled
					}
				}
				return nil // Stream complete
			}
			if err != nil {
				return serrors.E(op, err, "stream receive failed")
			}

			if len(response.Choices) == 0 {
				continue
			}

			choice := response.Choices[0]
			chunk := agents.Chunk{
				Delta:        choice.Delta.Content,
				FinishReason: string(choice.FinishReason),
			}

			// Handle tool call deltas
			if len(choice.Delta.ToolCalls) > 0 {
				for _, tcDelta := range choice.Delta.ToolCalls {
					// Index can be nil in some cases, use 0 as default
					index := 0
					if tcDelta.Index != nil {
						index = *tcDelta.Index
					}

					if toolCallsMap[index] == nil {
						toolCallsMap[index] = &types.ToolCall{
							ID:   tcDelta.ID,
							Name: tcDelta.Function.Name,
						}
					}
					// Accumulate arguments
					toolCallsMap[index].Arguments += tcDelta.Function.Arguments
				}

				// Convert map to slice for chunk
				chunk.ToolCalls = make([]types.ToolCall, 0, len(toolCallsMap))
				for i := 0; i < len(toolCallsMap); i++ {
					if tc := toolCallsMap[i]; tc != nil {
						chunk.ToolCalls = append(chunk.ToolCalls, *tc)
					}
				}
			}

			// Check for usage (sent in final chunk by OpenAI)
			if response.Usage != nil && response.Usage.TotalTokens > 0 {
				totalUsage = &types.TokenUsage{
					PromptTokens:     response.Usage.PromptTokens,
					CompletionTokens: response.Usage.CompletionTokens,
					TotalTokens:      response.Usage.TotalTokens,
				}
				chunk.Usage = totalUsage
			}

			// Mark as done if finish reason present
			if chunk.FinishReason != "" {
				chunk.Done = true
			}

			// Yield chunk
			if !yield(chunk) {
				return nil // Generator cancelled
			}

			if chunk.Done {
				return nil // Stream complete
			}
		}
	}, types.WithBufferSize(10)) // Buffer up to 10 chunks
}

// Info returns model metadata including capabilities.
func (m *OpenAIModel) Info() agents.ModelInfo {
	return agents.ModelInfo{
		Name:     m.modelName,
		Provider: "openai",
		Capabilities: []agents.Capability{
			agents.CapabilityStreaming,
			agents.CapabilityTools,
			agents.CapabilityJSONMode,
		},
	}
}

// HasCapability checks if this model supports a specific capability.
func (m *OpenAIModel) HasCapability(capability agents.Capability) bool {
	return m.Info().HasCapability(capability)
}

// buildChatCompletionRequest converts agents.Request to OpenAI format.
func (m *OpenAIModel) buildChatCompletionRequest(req agents.Request, config agents.GenerateConfig) openai.ChatCompletionRequest {
	// Convert messages
	messages := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = openai.ChatCompletionMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}

		// Add tool call ID for tool messages
		if msg.ToolCallID != nil {
			messages[i].ToolCallID = *msg.ToolCallID
		}

		// Add tool calls for assistant messages
		if len(msg.ToolCalls) > 0 {
			messages[i].ToolCalls = make([]openai.ToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				messages[i].ToolCalls[j] = openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				}
			}
		}
	}

	// Build base request
	oaiReq := openai.ChatCompletionRequest{
		Model:    m.modelName,
		Messages: messages,
	}

	// Apply max tokens
	if config.MaxTokens != nil {
		oaiReq.MaxTokens = *config.MaxTokens
	}

	// Apply temperature
	if config.Temperature != nil {
		oaiReq.Temperature = float32(*config.Temperature)
	}

	// Apply JSON mode
	if config.JSONMode && m.HasCapability(agents.CapabilityJSONMode) {
		oaiReq.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		}
	}

	// Convert tools
	if len(req.Tools) > 0 {
		oaiReq.Tools = make([]openai.Tool, len(req.Tools))
		for i, tool := range req.Tools {
			oaiReq.Tools[i] = openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        tool.Name(),
					Description: tool.Description(),
					Parameters:  tool.Parameters(),
				},
			}
		}
	}

	return oaiReq
}
