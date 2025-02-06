package llmproviders

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/llm"
	"github.com/sashabaranov/go-openai"
)

type OpenAIProvider struct {
	client *openai.Client
}

func NewOpenAIProvider(authToken string) *OpenAIProvider {
	return &OpenAIProvider{
		client: openai.NewClient(authToken),
	}
}

func (p *OpenAIProvider) CreateChatCompletionStream(ctx context.Context, request llm.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return p.client.CreateChatCompletionStream(ctx, DomainToOpenAIChatCompletionRequest(request))
}
