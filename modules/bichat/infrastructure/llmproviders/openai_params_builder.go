package llmproviders

import (
	"context"
	"strings"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// buildResponseParams converts agents.Request to OpenAI Responses API parameters.
func (m *OpenAIModel) buildResponseParams(ctx context.Context, req agents.Request, config agents.GenerateConfig) responses.ResponseNewParams {
	params := responses.ResponseNewParams{
		Model: m.modelName,
		Store: openai.Bool(true),
	}

	inputItems := m.buildInputItemsWithContext(ctx, req.Messages)
	params.Input = responses.ResponseNewParamsInputUnion{
		OfInputItemList: inputItems,
	}
	if req.PreviousResponseID != nil {
		previousResponseID := strings.TrimSpace(*req.PreviousResponseID)
		if previousResponseID != "" {
			params.PreviousResponseID = openai.String(previousResponseID)
		}
	}

	if config.MaxTokens != nil {
		params.MaxOutputTokens = openai.Int(int64(*config.MaxTokens))
	}

	if config.Temperature != nil {
		params.Temperature = openai.Float(*config.Temperature)
	}

	if config.JSONMode && m.HasCapability(agents.CapabilityJSONMode) {
		params.Text = responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigUnionParam{
				OfJSONObject: &shared.ResponseFormatJSONObjectParam{},
			},
		}
	}

	var tools []responses.ToolUnionParam
	var includes []responses.ResponseIncludable
	hasWebSearch := false
	hasCodeInterpreter := false
	codeInterpreterFileIDs := make([]string, 0)

	for _, tool := range req.Tools {
		if tool.Name() == "code_interpreter" {
			hasCodeInterpreter = true
			break
		}
	}
	if hasCodeInterpreter {
		codeInterpreterFileIDs = m.resolveCodeInterpreterFileIDs(ctx)
	}

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
			tools = append(tools, responses.ToolParamOfCodeInterpreter(
				responses.ToolCodeInterpreterContainerCodeInterpreterContainerAutoParam{
					MemoryLimit: m.getCodeInterpreterMemoryLimit(),
					FileIDs:     codeInterpreterFileIDs,
				},
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

// SetCodeInterpreterMemoryLimit updates the default memory limit for OpenAI code_interpreter containers.
func (m *OpenAIModel) SetCodeInterpreterMemoryLimit(limit string) error {
	const op serrors.Op = "OpenAIModel.SetCodeInterpreterMemoryLimit"
	normalized, ok := normalizeCodeInterpreterMemoryLimit(limit)
	if !ok {
		return serrors.E(op, serrors.KindValidation, "invalid code interpreter memory limit: must be one of 1g, 4g, 16g, 64g")
	}
	m.mu.Lock()
	m.codeInterpreterMemoryLimit = normalized
	m.mu.Unlock()
	return nil
}

// SetCodeInterpreterArtifactSource configures the session artifact source and storage
// used to upload attachments to OpenAI Files for code_interpreter.
func (m *OpenAIModel) SetCodeInterpreterArtifactSource(repo domain.ChatRepository, fileStorage CodeInterpreterArtifactStorage) {
	m.mu.Lock()
	if m.client != nil && repo != nil && fileStorage != nil {
		m.artifactResolver = NewOpenAICodeInterpreterArtifactResolver(m.client, repo, fileStorage, m.logger)
	} else {
		m.artifactResolver = nil
	}
	m.mu.Unlock()
}

func (m *OpenAIModel) getCodeInterpreterMemoryLimit() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.codeInterpreterMemoryLimit
}
