package llmproviders

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// StubModel is a placeholder Model implementation for module initialization.
// Replace this with a real LLM provider (OpenAI, Anthropic, etc.) in production.
//
// To use a real model:
//   - Install provider SDK (e.g., github.com/sashabaranov/go-openai)
//   - Implement agents.Model interface with your provider
//   - Replace StubModel in module.go with your implementation
//
// Example with OpenAI:
//
//	import openai "github.com/sashabaranov/go-openai"
//
//	client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
//	model := &YourOpenAIModel{client: client, modelName: "gpt-4"}
type StubModel struct {
	modelName string
	provider  string
}

// NewStubModel creates a placeholder model for development/testing.
// This model returns placeholder responses and should NOT be used in production.
func NewStubModel() agents.Model {
	return &StubModel{
		modelName: "stub-model",
		provider:  "stub",
	}
}

// Generate returns a placeholder response.
// In production, this would call the actual LLM API.
func (m *StubModel) Generate(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error) {
	// This is a stub - returns a placeholder response
	return &agents.Response{
		Message: types.Message{
			Role:    types.RoleAssistant,
			Content: "This is a stub response. Please configure a real LLM model (OpenAI, Anthropic, etc.) to get actual AI responses.",
		},
		Usage: types.TokenUsage{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
		},
		FinishReason: "stop",
	}, nil
}

// Stream returns a placeholder streaming response.
// In production, this would stream from the actual LLM API.
func (m *StubModel) Stream(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) types.Generator[agents.Chunk] {
	chunks := []agents.Chunk{
		{Delta: "This is a stub streaming response. ", Done: false},
		{Delta: "Please configure a real LLM model.", Done: false},
		{
			Delta:        "",
			FinishReason: "stop",
			Done:         true,
			Usage: &types.TokenUsage{
				PromptTokens:     0,
				CompletionTokens: 0,
				TotalTokens:      0,
			},
		},
	}

	return types.NewGenerator(ctx, func(ctx context.Context, yield func(agents.Chunk) bool) error {
		for _, chunk := range chunks {
			if !yield(chunk) {
				return nil
			}
		}
		return nil
	})
}

// Info returns model metadata.
func (m *StubModel) Info() agents.ModelInfo {
	return agents.ModelInfo{
		Name:         m.modelName,
		Provider:     m.provider,
		Capabilities: []agents.Capability{}, // No capabilities for stub
	}
}

// HasCapability always returns false for stub model.
func (m *StubModel) HasCapability(capability agents.Capability) bool {
	return false
}

// Example: Real OpenAI Model Implementation
// ==========================================
//
// Here's how to implement a real model with OpenAI:
//
// type OpenAIModel struct {
//     client    *openai.Client
//     modelName string
//     maxTokens int
// }
//
// func NewOpenAIModel(apiKey string) agents.Model {
//     return &OpenAIModel{
//         client:    openai.NewClient(apiKey),
//         modelName: "gpt-4",
//         maxTokens: 4096,
//     }
// }
//
// func (m *OpenAIModel) Generate(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error) {
//     // Convert agents.Request to openai.ChatCompletionRequest
//     messages := make([]openai.ChatCompletionMessage, len(req.Messages))
//     for i, msg := range req.Messages {
//         messages[i] = openai.ChatCompletionMessage{
//             Role:    string(msg.Role),
//             Content: msg.Content,
//         }
//     }
//
//     // Call OpenAI API
//     resp, err := m.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
//         Model:     m.modelName,
//         Messages:  messages,
//         MaxTokens: m.maxTokens,
//     })
//     if err != nil {
//         return nil, err
//     }
//
//     // Convert response
//     return &agents.Response{
//         Message: types.Message{
//             Role:    types.Role(resp.Choices[0].Message.Role),
//             Content: resp.Choices[0].Message.Content,
//         },
//         Usage: types.TokenUsage{
//             PromptTokens:     resp.Usage.PromptTokens,
//             CompletionTokens: resp.Usage.CompletionTokens,
//             TotalTokens:      resp.Usage.TotalTokens,
//         },
//         FinishReason: resp.Choices[0].FinishReason,
//     }, nil
// }
//
// Similar pattern for Stream(), Info(), and HasCapability()
