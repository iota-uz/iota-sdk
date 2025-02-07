package llmproviders

import (
	"github.com/iota-uz/iota-sdk/modules/bichat/domain/entities/llm"
	"github.com/iota-uz/iota-sdk/pkg/mapping"

	"github.com/sashabaranov/go-openai"
)

func DomainFuncCallToOpenAI(fc llm.FunctionCall) openai.FunctionCall {
	return openai.FunctionCall{
		Name:      fc.Name,
		Arguments: fc.Arguments,
	}
}

func DomainImageURLToOpenAI(i llm.ChatMessageImageURL) openai.ChatMessageImageURL {
	return openai.ChatMessageImageURL{
		URL:    i.URL,
		Detail: openai.ImageURLDetail(i.Detail),
	}
}

func DomainMessagePartToOpenAI(m llm.ChatMessagePart) openai.ChatMessagePart {
	return openai.ChatMessagePart{
		Type:     openai.ChatMessagePartType(m.Type),
		Text:     m.Text,
		ImageURL: mapping.Pointer(DomainImageURLToOpenAI(*m.ImageURL)),
	}
}

func DomainToolCallToOpenAI(toolCalls []llm.ToolCall) []openai.ToolCall {
	result := make([]openai.ToolCall, 0, len(toolCalls))
	for _, t := range toolCalls {
		result = append(result, openai.ToolCall{
			Index:    t.Index,
			Type:     openai.ToolType(t.Type),
			Function: DomainFuncCallToOpenAI(t.Function),
		})
	}
	return result
}

func DomainFuncDefinitionToOpenAI(f llm.FunctionDefinition) openai.FunctionDefinition {
	return openai.FunctionDefinition{
		Name:        f.Name,
		Description: f.Description,
		Parameters:  f.Parameters,
	}
}

func DomainToolToOpenAI(t llm.Tool) openai.Tool {
	return openai.Tool{
		Type:     openai.ToolType(t.Type),
		Function: mapping.Pointer(DomainFuncDefinitionToOpenAI(*t.Function)),
	}
}

func DomainToOpenAIChatCompletionRequest(d llm.ChatCompletionRequest) openai.ChatCompletionRequest {
	tools := make([]openai.Tool, 0, len(d.Tools))
	for _, t := range d.Tools {
		tools = append(tools, DomainToolToOpenAI(t))
	}
	messages := make([]openai.ChatCompletionMessage, 0, len(d.Messages))
	for _, m := range d.Messages {
		messages = append(messages, DomainChatCompletionMessageToOpenAI(m))
	}
	return openai.ChatCompletionRequest{
		Model:       d.Model,
		Messages:    messages,
		Tools:       tools,
		MaxTokens:   d.MaxTokens,
		Temperature: d.Temperature,
		TopP:        d.TopP,
		N:           d.N,
		Stream:      d.Stream,
	}
}

func DomainChatCompletionMessageToOpenAI(message llm.ChatCompletionMessage) openai.ChatCompletionMessage {
	var funcCall *openai.FunctionCall
	if message.FunctionCall != nil {
		funcCall = mapping.Pointer(DomainFuncCallToOpenAI(*message.FunctionCall))
	}
	multiContent := make([]openai.ChatMessagePart, 0, len(message.MultiContent))
	for _, mc := range message.MultiContent {
		multiContent = append(multiContent, DomainMessagePartToOpenAI(mc))
	}
	return openai.ChatCompletionMessage{
		Name:         message.Name,
		Role:         message.Role,
		Content:      message.Content,
		Refusal:      message.Refusal,
		MultiContent: multiContent,
		FunctionCall: funcCall,
		ToolCalls:    DomainToolCallToOpenAI(message.ToolCalls),
		ToolCallID:   message.ToolCallID,
	}
}

func OpenAIToDomainFuncCall(fc openai.FunctionCall) llm.FunctionCall {
	return llm.FunctionCall{
		Name:      fc.Name,
		Arguments: fc.Arguments,
	}
}

func OpenAIToDomainImageURL(i openai.ChatMessageImageURL) llm.ChatMessageImageURL {
	return llm.ChatMessageImageURL{
		URL:    i.URL,
		Detail: llm.ImageURLDetail(i.Detail),
	}
}

func OpenAIToDomainMessagePart(m openai.ChatMessagePart) llm.ChatMessagePart {
	return llm.ChatMessagePart{
		Type:     llm.ChatMessagePartType(m.Type),
		Text:     m.Text,
		ImageURL: mapping.Pointer(OpenAIToDomainImageURL(*m.ImageURL)),
	}
}

func OpenAIToDomainToolCall(toolCalls []openai.ToolCall) []llm.ToolCall {
	result := make([]llm.ToolCall, 0, len(toolCalls))
	for _, t := range toolCalls {
		result = append(result, llm.ToolCall{
			Index:    t.Index,
			Type:     llm.ToolType(t.Type),
			Function: OpenAIToDomainFuncCall(t.Function),
		})
	}
	return result
}

func OpenAIChatCompletionMessageToDomain(message openai.ChatCompletionMessage) llm.ChatCompletionMessage {
	var funcCall *llm.FunctionCall
	if message.FunctionCall != nil {
		funcCall = mapping.Pointer(OpenAIToDomainFuncCall(*message.FunctionCall))
	}
	multiContent := make([]llm.ChatMessagePart, 0, len(message.MultiContent))
	for _, mc := range message.MultiContent {
		multiContent = append(multiContent, OpenAIToDomainMessagePart(mc))
	}
	return llm.ChatCompletionMessage{
		Name:         message.Name,
		Role:         message.Role,
		Content:      message.Content,
		Refusal:      message.Refusal,
		MultiContent: multiContent,
		FunctionCall: funcCall,
		ToolCalls:    OpenAIToDomainToolCall(message.ToolCalls),
		ToolCallID:   message.ToolCallID,
	}
}
