package llmproviders

import (
	"context"
	"os"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/logging"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// OpenAIModel implements the agents.Model interface using OpenAI's Responses API.
// It provides both blocking and streaming modes with native tool support
// including web_search and code_interpreter (handled by the API).
type OpenAIModel struct {
	client    *openai.Client
	modelName string
	logger    logging.Logger
}

// OpenAIModelOption configures an OpenAIModel.
type OpenAIModelOption func(*OpenAIModel)

// WithLogger sets the logger for the OpenAI model.
func WithLogger(logger logging.Logger) OpenAIModelOption {
	return func(m *OpenAIModel) {
		m.logger = logger
	}
}

// toolCallAccumEntry accumulates streaming function call data.
type toolCallAccumEntry struct {
	id   string
	name string
	args string
}

// NewOpenAIModel creates a new OpenAI model from environment variables.
// It reads OPENAI_API_KEY (required) and OPENAI_MODEL (optional, defaults to gpt-5.2-2025-12-11).
func NewOpenAIModel(opts ...OpenAIModelOption) (agents.Model, error) {
	const op serrors.Op = "llmproviders.NewOpenAIModel"

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, serrors.E(op, "OPENAI_API_KEY environment variable is required")
	}

	modelName := os.Getenv("OPENAI_MODEL")
	if modelName == "" {
		modelName = "gpt-5.2-2025-12-11"
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))

	m := &OpenAIModel{
		client:    &client,
		modelName: modelName,
		logger:    logging.NewNoOpLogger(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m, nil
}

// Generate sends a blocking request to the OpenAI Responses API.
func (m *OpenAIModel) Generate(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error) {
	const op serrors.Op = "OpenAIModel.Generate"

	config := agents.ApplyGenerateOptions(opts...)
	params := m.buildResponseParams(req, config)

	resp, err := m.client.Responses.New(ctx, params)
	if err != nil {
		return nil, serrors.E(op, err, "OpenAI API request failed")
	}

	return m.mapResponse(resp)
}

// Stream sends a streaming request to the OpenAI Responses API.
func (m *OpenAIModel) Stream(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (types.Generator[agents.Chunk], error) {
	const op serrors.Op = "OpenAIModel.Stream"

	config := agents.ApplyGenerateOptions(opts...)
	params := m.buildResponseParams(req, config)

	stream := m.client.Responses.NewStreaming(ctx, params)

	return types.NewGenerator(ctx, func(genCtx context.Context, yield func(agents.Chunk) bool) error {
		defer func() {
			if err := stream.Close(); err != nil {
				m.logger.Warn(genCtx, "stream close error", map[string]any{
					"error": err.Error(),
					"model": m.modelName,
				})
			}
		}()

		// Accumulate function call arguments per call ID
		toolCallAccum := make(map[string]*toolCallAccumEntry)
		var toolCallOrder []string

		for stream.Next() {
			event := stream.Current()

			switch event.Type {
			case "response.output_text.delta":
				if !yield(agents.Chunk{Delta: event.Delta}) {
					return nil
				}

			case "response.function_call_arguments.delta":
				callID := event.ItemID
				if _, ok := toolCallAccum[callID]; !ok {
					toolCallAccum[callID] = &toolCallAccumEntry{id: callID}
					toolCallOrder = append(toolCallOrder, callID)
				}
				toolCallAccum[callID].args += event.Delta

			case "response.function_call_arguments.done":
				callID := event.ItemID
				if a, ok := toolCallAccum[callID]; ok {
					a.name = event.Name
					a.args = event.Arguments
				}

			case "response.output_item.done":
				if event.Item.Type == "function_call" {
					callID := event.Item.CallID
					if _, ok := toolCallAccum[callID]; !ok {
						toolCallAccum[callID] = &toolCallAccumEntry{
							id:   callID,
							name: event.Item.Name,
							args: event.Item.Arguments,
						}
						toolCallOrder = append(toolCallOrder, callID)
					} else {
						a := toolCallAccum[callID]
						if a.name == "" {
							a.name = event.Item.Name
						}
						if a.args == "" {
							a.args = event.Item.Arguments
						}
					}
				}

			case "response.completed":
				resp := event.Response
				agentResp, err := m.mapResponse(&resp)
				if err != nil {
					return serrors.E(op, err, "failed to map completed response")
				}

				// Build final chunk with all accumulated tool calls
				toolCalls := m.buildToolCallsFromAccum(toolCallAccum, toolCallOrder)
				if len(agentResp.Message.ToolCalls()) > 0 && len(toolCalls) == 0 {
					toolCalls = agentResp.Message.ToolCalls()
				}

				chunk := agents.Chunk{
					Done:                   true,
					ToolCalls:              toolCalls,
					Usage:                  &agentResp.Usage,
					FinishReason:           agentResp.FinishReason,
					Citations:              agentResp.Message.Citations(),
					CodeInterpreterResults: agentResp.CodeInterpreterResults,
					FileAnnotations:        agentResp.FileAnnotations,
				}
				if !yield(chunk) {
					return nil
				}
				return nil
			}
		}

		if err := stream.Err(); err != nil {
			return serrors.E(op, err, "stream error")
		}

		return nil
	}, types.WithBufferSize(10)), nil
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

// Pricing returns pricing information for this OpenAI model.
func (m *OpenAIModel) Pricing() agents.ModelPricing {
	pricing := map[string]agents.ModelPricing{
		"gpt-5.2-2025-12-11": {
			Currency:        "USD",
			InputPer1M:      1.75,
			OutputPer1M:     14.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0.18,
		},
		"gpt-4o": {
			Currency:        "USD",
			InputPer1M:      2.50,
			OutputPer1M:     10.00,
			CacheWritePer1M: 1.25,
			CacheReadPer1M:  0.625,
		},
		"gpt-4o-mini": {
			Currency:        "USD",
			InputPer1M:      0.150,
			OutputPer1M:     0.600,
			CacheWritePer1M: 0.075,
			CacheReadPer1M:  0.038,
		},
		"gpt-4-turbo": {
			Currency:        "USD",
			InputPer1M:      10.00,
			OutputPer1M:     30.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0,
		},
		"gpt-4": {
			Currency:        "USD",
			InputPer1M:      30.00,
			OutputPer1M:     60.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0,
		},
	}

	if p, exists := pricing[m.modelName]; exists {
		return p
	}

	return agents.ModelPricing{
		Currency:        "USD",
		InputPer1M:      1.75,
		OutputPer1M:     14.00,
		CacheWritePer1M: 0,
		CacheReadPer1M:  0.18,
	}
}

// buildResponseParams converts agents.Request to OpenAI Responses API parameters.
func (m *OpenAIModel) buildResponseParams(req agents.Request, config agents.GenerateConfig) responses.ResponseNewParams {
	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(m.modelName),
	}

	// Build input items from messages
	inputItems := m.buildInputItems(req.Messages)
	params.Input = responses.ResponseNewParamsInputUnion{
		OfInputItemList: inputItems,
	}

	// Apply max tokens
	if config.MaxTokens != nil {
		params.MaxOutputTokens = openai.Int(int64(*config.MaxTokens))
	}

	// Apply temperature
	if config.Temperature != nil {
		params.Temperature = openai.Float(*config.Temperature)
	}

	// Apply JSON mode
	if config.JSONMode && m.HasCapability(agents.CapabilityJSONMode) {
		params.Text = responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigUnionParam{
				OfJSONObject: &shared.ResponseFormatJSONObjectParam{},
			},
		}
	}

	// Convert tools — detect native tools vs function tools
	var tools []responses.ToolUnionParam
	var includes []responses.ResponseIncludable
	hasWebSearch := false
	hasCodeInterpreter := false

	for _, tool := range req.Tools {
		switch tool.Name() {
		case "web_search":
			hasWebSearch = true
			tools = append(tools, responses.ToolUnionParam{
				OfWebSearch: &responses.WebSearchToolParam{
					Type: responses.WebSearchToolTypeWebSearch,
				},
			})
		case "code_interpreter":
			hasCodeInterpreter = true
			tools = append(tools, responses.ToolParamOfCodeInterpreter(
				responses.ToolCodeInterpreterContainerCodeInterpreterContainerAutoParam{},
			))
		default:
			tools = append(tools, responses.ToolUnionParam{
				OfFunction: &responses.FunctionToolParam{
					Name:        tool.Name(),
					Description: openai.String(tool.Description()),
					Parameters:  tool.Parameters(),
				},
			})
		}
	}

	if len(tools) > 0 {
		params.Tools = tools
	}

	// Request additional output data for native tools
	if hasWebSearch {
		includes = append(includes, responses.ResponseIncludableWebSearchCallActionSources)
	}
	if hasCodeInterpreter {
		includes = append(includes, responses.ResponseIncludableCodeInterpreterCallOutputs)
	}
	if len(includes) > 0 {
		params.Include = includes
	}

	return params
}

// buildInputItems converts types.Message slice to Responses API input items.
func (m *OpenAIModel) buildInputItems(messages []types.Message) responses.ResponseInputParam {
	items := make(responses.ResponseInputParam, 0, len(messages))

	for _, msg := range messages {
		switch msg.Role() {
		case types.RoleSystem:
			items = append(items, responses.ResponseInputItemParamOfMessage(
				msg.Content(),
				responses.EasyInputMessageRoleDeveloper,
			))

		case types.RoleUser:
			if len(msg.Attachments()) > 0 {
				// Build multipart content with text + images
				parts := make(responses.ResponseInputMessageContentListParam, 0, 1+len(msg.Attachments()))
				if msg.Content() != "" {
					parts = append(parts, responses.ResponseInputContentParamOfInputText(msg.Content()))
				}
				for _, attachment := range msg.Attachments() {
					parts = append(parts, responses.ResponseInputContentUnionParam{
						OfInputImage: &responses.ResponseInputImageParam{
							ImageURL: openai.String(attachment.FilePath),
							Detail:   responses.ResponseInputImageDetailLow,
						},
					})
				}
				items = append(items, responses.ResponseInputItemParamOfMessage(
					parts,
					responses.EasyInputMessageRoleUser,
				))
			} else {
				items = append(items, responses.ResponseInputItemParamOfMessage(
					msg.Content(),
					responses.EasyInputMessageRoleUser,
				))
			}

		case types.RoleAssistant:
			// Emit text content as an assistant message (if present)
			if msg.Content() != "" {
				items = append(items, responses.ResponseInputItemParamOfMessage(
					msg.Content(),
					responses.EasyInputMessageRoleAssistant,
				))
			}
			// Emit each tool call as a separate function_call input item
			for _, tc := range msg.ToolCalls() {
				items = append(items, responses.ResponseInputItemParamOfFunctionCall(
					tc.Arguments,
					tc.ID,
					tc.Name,
				))
			}

		case types.RoleTool:
			if msg.ToolCallID() != nil {
				items = append(items, responses.ResponseInputItemParamOfFunctionCallOutput(
					*msg.ToolCallID(),
					msg.Content(),
				))
			}
		}
	}

	return items
}

// mapResponse converts a Responses API Response to agents.Response.
func (m *OpenAIModel) mapResponse(resp *responses.Response) (*agents.Response, error) {
	var content string
	var toolCalls []types.ToolCall
	var citations []types.Citation
	var codeResults []types.CodeInterpreterResult
	var fileAnnotations []types.FileAnnotation

	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				if part.Type == "output_text" {
					content += part.Text

					// Extract citations from annotations
					for _, ann := range part.Annotations {
						if ann.Type == "url_citation" {
							citations = append(citations, types.Citation{
								Type:       "web",
								Title:      ann.Title,
								URL:        ann.URL,
								StartIndex: int(ann.StartIndex),
								EndIndex:   int(ann.EndIndex),
							})
						}
						if ann.Type == "container_file_citation" {
							fileAnnotations = append(fileAnnotations, types.FileAnnotation{
								Type:        ann.Type,
								ContainerID: ann.ContainerID,
								FileID:      ann.FileID,
								Filename:    ann.Filename,
								StartIndex:  int(ann.StartIndex),
								EndIndex:    int(ann.EndIndex),
							})
						}
					}
				}
			}

		case "function_call":
			toolCalls = append(toolCalls, types.ToolCall{
				ID:        item.CallID,
				Name:      item.Name,
				Arguments: item.Arguments,
			})

		case "code_interpreter_call":
			result := types.CodeInterpreterResult{
				ID:          item.ID,
				Code:        item.Code,
				ContainerID: item.ContainerID,
				Status:      item.Status,
			}
			for _, out := range item.Outputs {
				if out.Type == "logs" {
					result.Logs = out.Logs
				}
			}
			codeResults = append(codeResults, result)
		}
	}

	// Determine finish reason
	finishReason := "stop"
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}
	if resp.Status == "incomplete" {
		finishReason = "length"
	}

	// Build message
	msgOpts := []types.MessageOption{}
	if len(toolCalls) > 0 {
		msgOpts = append(msgOpts, types.WithToolCalls(toolCalls...))
	}
	if len(citations) > 0 {
		msgOpts = append(msgOpts, types.WithCitations(citations...))
	}
	msg := types.AssistantMessage(content, msgOpts...)

	// Build usage
	usage := types.TokenUsage{
		PromptTokens:     int(resp.Usage.InputTokens),
		CompletionTokens: int(resp.Usage.OutputTokens),
		TotalTokens:      int(resp.Usage.TotalTokens),
	}
	if resp.Usage.InputTokensDetails.CachedTokens > 0 {
		usage.CacheReadTokens = int(resp.Usage.InputTokensDetails.CachedTokens)
	}

	return &agents.Response{
		Message:                msg,
		Usage:                  usage,
		FinishReason:           finishReason,
		CodeInterpreterResults: codeResults,
		FileAnnotations:        fileAnnotations,
	}, nil
}

// buildToolCallsFromAccum converts accumulated tool call data to types.ToolCall slice.
func (m *OpenAIModel) buildToolCallsFromAccum(accum map[string]*toolCallAccumEntry, order []string) []types.ToolCall {
	if len(accum) == 0 {
		return nil
	}
	calls := make([]types.ToolCall, 0, len(accum))
	for _, callID := range order {
		if a, ok := accum[callID]; ok {
			calls = append(calls, types.ToolCall{
				ID:        a.id,
				Name:      a.name,
				Arguments: a.args,
			})
		}
	}
	return calls
}
