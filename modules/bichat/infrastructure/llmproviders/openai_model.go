package llmproviders

import (
	"context"
	"os"
	"strings"
	"sync"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/logging"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// OpenAIModel implements the agents.Model interface using OpenAI's Responses API.
// It provides both blocking and streaming modes with native tool support.
type OpenAIModel struct {
	mu                           sync.RWMutex
	client                       *openai.Client
	modelName                    string
	logger                       logging.Logger
	codeInterpreterMemoryLimit   string
	codeInterpreterArtifactLimit int
	artifactResolver             CodeInterpreterArtifactResolver
	imageUploadResolver          OpenAIImageUploadLookup
}

// OpenAIModelOption configures an OpenAIModel.
type OpenAIModelOption func(*OpenAIModel)

const (
	defaultCodeInterpreterMemoryLimit = "4g"
	defaultCodeInterpreterFileLimit   = 20
)

// WithLogger sets the logger for the OpenAI model.
func WithLogger(logger logging.Logger) OpenAIModelOption {
	return func(m *OpenAIModel) {
		m.logger = logger
	}
}

// WithCodeInterpreterMemoryLimit sets the default memory limit for OpenAI code_interpreter containers.
// Allowed values: "1g", "4g", "16g", "64g".
func WithCodeInterpreterMemoryLimit(limit string) OpenAIModelOption {
	return func(m *OpenAIModel) {
		if normalized, ok := normalizeCodeInterpreterMemoryLimit(limit); ok {
			m.mu.Lock()
			m.codeInterpreterMemoryLimit = normalized
			m.mu.Unlock()
			return
		}
		m.logger.Warn(context.Background(), "invalid code_interpreter memory limit option ignored", map[string]any{
			"provided_limit": limit,
			"allowed_values": []string{"1g", "4g", "16g", "64g"},
		})
	}
}

// WithCodeInterpreterArtifactSource configures artifact lookup and file storage so
// uploaded session artifacts can be passed to code_interpreter as file_ids.
func WithCodeInterpreterArtifactSource(repo domain.ChatRepository, fileStorage CodeInterpreterArtifactStorage) OpenAIModelOption {
	return func(m *OpenAIModel) {
		m.mu.Lock()
		if m.client != nil && repo != nil && fileStorage != nil {
			m.artifactResolver = NewOpenAICodeInterpreterArtifactResolver(m.client, repo, fileStorage, m.logger)
		} else {
			m.artifactResolver = nil
		}
		m.mu.Unlock()
	}
}

// WithCodeInterpreterArtifactLimit sets the max number of session artifacts uploaded
// to OpenAI and attached to code_interpreter per request.
func WithCodeInterpreterArtifactLimit(limit int) OpenAIModelOption {
	return func(m *OpenAIModel) {
		if limit > 0 {
			m.mu.Lock()
			m.codeInterpreterArtifactLimit = limit
			m.mu.Unlock()
		}
	}
}

// WithImageUploadResolver overrides upload lookup used for image attachments.
func WithImageUploadResolver(resolver OpenAIImageUploadLookup) OpenAIModelOption {
	return func(m *OpenAIModel) {
		m.mu.Lock()
		m.imageUploadResolver = resolver
		m.mu.Unlock()
	}
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
		client:                       &client,
		modelName:                    modelName,
		logger:                       logging.NewNoOpLogger(),
		codeInterpreterMemoryLimit:   defaultCodeInterpreterMemoryLimit,
		codeInterpreterArtifactLimit: defaultCodeInterpreterFileLimit,
		imageUploadResolver:          newCoreOpenAIImageUploadLookup(),
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.imageUploadResolver == nil {
		m.imageUploadResolver = newCoreOpenAIImageUploadLookup()
	}
	return m, nil
}

// Generate sends a blocking request to the OpenAI Responses API.
func (m *OpenAIModel) Generate(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error) {
	const op serrors.Op = "OpenAIModel.Generate"

	config := agents.ApplyGenerateOptions(opts...)
	params := m.buildResponseParams(ctx, req, config)

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
	params := m.buildResponseParams(ctx, req, config)

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
					itemID := functionCallItemKey(event.Item, event.ItemID)
					if itemID == "" {
						m.logger.Warn(genCtx, "skipping function_call output_item.done without item id", map[string]any{
							"call_id": event.Item.CallID,
							"name":    event.Item.Name,
						})
						continue
					}
					if a, ok := toolCallAccum[itemID]; ok {
						a.callID = event.Item.CallID
						if a.name == "" {
							a.name = event.Item.Name
						}
						if a.args == "" {
							a.args = event.Item.Arguments
						}
					} else {
						toolCallAccum[itemID] = &toolCallAccumEntry{
							id:     itemID,
							callID: event.Item.CallID,
							name:   event.Item.Name,
							args:   event.Item.Arguments,
						}
						toolCallOrder = append(toolCallOrder, itemID)
					}

					readyToolCalls := m.buildReadyToolCallsFromAccum(toolCallAccum, toolCallOrder)
					if len(readyToolCalls) > 0 {
						if !yield(agents.Chunk{ToolCalls: readyToolCalls}) {
							return nil
						}
					}
				}

			case "response.completed":
				resp := event.Response
				agentResp, err := m.mapResponse(&resp)
				if err != nil {
					return serrors.E(op, err, "failed to map completed response")
				}

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
					ProviderResponseID:     agentResp.ProviderResponseID,
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

func normalizeCodeInterpreterMemoryLimit(limit string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(limit))
	switch normalized {
	case "1g", "4g", "16g", "64g":
		return normalized, true
	default:
		return "", false
	}
}
