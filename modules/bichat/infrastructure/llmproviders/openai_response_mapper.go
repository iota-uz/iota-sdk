package llmproviders

import (
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/openai/openai-go/v3/responses"
)

// mapResponse converts a Responses API Response to agents.Response.
func (m *OpenAIModel) mapResponse(resp *responses.Response) (*agents.Response, error) {
	var content string
	var thinking string
	var toolCalls []types.ToolCall
	var citations []types.Citation
	var codeResults []types.CodeInterpreterResult
	var fileAnnotations []types.FileAnnotation
	var webSearchSources []string

	for _, item := range resp.Output {
		switch item.Type {
		case "web_search_call":
			for _, src := range item.Action.Sources {
				webSearchSources = append(webSearchSources, src.URL)
			}
		case "message":
			for _, part := range item.Content {
				if part.Type == "output_text" {
					content += part.Text

					for _, ann := range part.Annotations {
						if ann.Type == "url_citation" {
							c := types.Citation{
								Type:       "web",
								Title:      ann.Title,
								URL:        ann.URL,
								Excerpt:    "", // API url_citation does not expose excerpt in SDK; can be set when available
								StartIndex: int(ann.StartIndex),
								EndIndex:   int(ann.EndIndex),
							}
							citations = append(citations, c)
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
				switch out.Type {
				case "logs":
					result.Logs = out.Logs
					result.Outputs = append(result.Outputs, types.CodeInterpreterGeneratedOutput{
						Type: "logs",
						Logs: out.Logs,
					})
				case "image":
					if strings.TrimSpace(out.URL) != "" {
						result.Outputs = append(result.Outputs, types.CodeInterpreterGeneratedOutput{
							Type: "image",
							URL:  out.URL,
						})
					}
				}
			}
			codeResults = append(codeResults, result)

		case "reasoning":
			for _, s := range item.Summary {
				if thinking != "" {
					thinking += "\n"
				}
				thinking += s.Text
			}
		}
	}

	// Enrich citations from web_search_call sources when annotation URL is missing
	for i := range citations {
		if citations[i].URL == "" && i < len(webSearchSources) {
			citations[i].URL = webSearchSources[i]
		}
	}

	finishReason := "stop"
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}
	if resp.Status == "incomplete" {
		finishReason = "length"
	}

	msgOpts := []types.MessageOption{}
	if len(toolCalls) > 0 {
		msgOpts = append(msgOpts, types.WithToolCalls(toolCalls...))
	}
	if len(citations) > 0 {
		msgOpts = append(msgOpts, types.WithCitations(citations...))
	}
	msg := types.AssistantMessage(content, msgOpts...)

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
		Thinking:               thinking,
		CodeInterpreterResults: codeResults,
		FileAnnotations:        fileAnnotations,
		ProviderResponseID:     resp.ID,
	}, nil
}
