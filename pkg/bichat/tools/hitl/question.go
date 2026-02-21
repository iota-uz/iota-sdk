package hitl

import (
	"context"
	"fmt"
	tools "github.com/iota-uz/iota-sdk/pkg/bichat/tools"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// Internal types for parsing tool input JSON (not part of public API)
// These match the JSON schema but convert to canonical types.AskUserQuestionPayload

// questionOptionInput represents an option in the tool input JSON.
type questionOptionInput struct {
	ID          string `json:"id,omitempty"` // Stable identifier (auto-generated if missing)
	Label       string `json:"label"`        // Display text
	Description string `json:"description"`  // Explanation
}

// userQuestionInput represents a question in the tool input JSON.
type userQuestionInput struct {
	ID          string                `json:"id,omitempty"` // Stable identifier (auto-generated if missing)
	Question    string                `json:"question"`     // The complete question (must end with ?)
	Header      string                `json:"header"`       // Short label (max 12 chars)
	MultiSelect bool                  `json:"multiSelect"`  // Allow multiple selections
	Options     []questionOptionInput `json:"options"`      // 2-4 options required
}

// questionMetadataInput represents optional metadata in the tool input JSON.
type questionMetadataInput struct {
	Source string `json:"source,omitempty"`
}

// AskUserQuestionTool is a tool that triggers a HITL (Human-in-the-Loop) interrupt.
// When called, the executor will pause execution, save a checkpoint, and wait for user input.
// The executor handles interrupt detection and checkpointing automatically.
type AskUserQuestionTool struct{}

// NewAskUserQuestionTool creates a tool that asks the user for clarification.
// Register this tool with the agent's tool registry. The executor will automatically
// detect when this tool is called and trigger an interrupt event.
func NewAskUserQuestionTool() agents.Tool {
	return &AskUserQuestionTool{}
}

// Name returns the tool name (must match agents.ToolAskUserQuestion constant).
func (t *AskUserQuestionTool) Name() string {
	return agents.ToolAskUserQuestion
}

// Description returns the tool description for the LLM.
func (t *AskUserQuestionTool) Description() string {
	return "Ask the user one or more clarifying questions when requirements are ambiguous. " +
		"The agent will pause until the user provides answers. " +
		"Use this sparingly, only when truly necessary. " +
		"You can ask 1-4 questions at once, each with 2-4 options."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *AskUserQuestionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"questions": map[string]any{
				"type":        "array",
				"description": "List of questions to ask the user (1-4 questions)",
				"minItems":    1,
				"maxItems":    4,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{
							"type":        "string",
							"description": "Stable identifier for the question (auto-generated if not provided)",
						},
						"question": map[string]any{
							"type":        "string",
							"description": "The complete question (must end with ?)",
							"pattern":     ".*\\?$",
						},
						"header": map[string]any{
							"type":        "string",
							"description": "Short label (max 12 chars) - displays as chip/tag",
							"maxLength":   12,
						},
						"multiSelect": map[string]any{
							"type":        "boolean",
							"description": "Allow multiple selections (default: false)",
							"default":     false,
						},
						"options": map[string]any{
							"type":        "array",
							"description": "Available options (2-4 required)",
							"minItems":    2,
							"maxItems":    4,
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"id": map[string]any{
										"type":        "string",
										"description": "Stable identifier for the option (auto-generated if not provided)",
									},
									"label": map[string]any{
										"type":        "string",
										"description": "Display text (1-5 words, concise)",
									},
									"description": map[string]any{
										"type":        "string",
										"description": "Explanation of what this option means",
									},
								},
								"required": []string{"label", "description"},
							},
						},
					},
					"required": []string{"question", "header", "options"},
				},
			},
			"metadata": map[string]any{
				"type":        "object",
				"description": "Optional metadata for tracking",
				"properties": map[string]any{
					"source": map[string]any{
						"type":        "string",
						"description": "Optional tracking (e.g., 'remember')",
					},
				},
			},
		},
		"required": []string{"questions"},
	}
}

// askQuestionInput represents the parsed input parameters.
type askQuestionInput struct {
	Questions []userQuestionInput    `json:"questions"`
	Metadata  *questionMetadataInput `json:"metadata,omitempty"`
}

// Call executes the ask user question operation.
// This creates the question structure that will be used by the interrupt handler.
func (t *AskUserQuestionTool) Call(ctx context.Context, input string) (string, error) {
	// Parse input
	params, err := agents.ParseToolInput[askQuestionInput](input)
	if err != nil {
		return tools.FormatToolError(
			tools.ErrCodeInvalidRequest,
			fmt.Sprintf("failed to parse input: %v", err),
			tools.HintCheckRequiredFields,
			"Provide questions array with valid structure",
		), nil
	}

	// Validate questions array
	if len(params.Questions) == 0 {
		return tools.FormatToolError(
			tools.ErrCodeInvalidRequest,
			"at least one question is required",
			tools.HintCheckRequiredFields,
			"Provide 1-4 questions to ask the user",
		), nil
	}
	if len(params.Questions) > 4 {
		return tools.FormatToolError(
			tools.ErrCodeInvalidRequest,
			"maximum 4 questions allowed",
			"Limit to 4 questions per request",
			"Consider asking fewer, more focused questions",
		), nil
	}

	// Validate each question
	for i, q := range params.Questions {
		if q.Question == "" {
			return tools.FormatToolError(
				tools.ErrCodeInvalidRequest,
				fmt.Sprintf("question[%d]: question text is required", i),
				tools.HintCheckRequiredFields,
				"Each question must have question text ending with ?",
			), nil
		}
		if q.Header == "" {
			return tools.FormatToolError(
				tools.ErrCodeInvalidRequest,
				fmt.Sprintf("question[%d]: header is required", i),
				tools.HintCheckRequiredFields,
				"Each question must have a short header (max 12 chars)",
			), nil
		}
		if len(q.Header) > 12 {
			return tools.FormatToolError(
				tools.ErrCodeInvalidRequest,
				fmt.Sprintf("question[%d]: header exceeds 12 characters", i),
				tools.HintCheckFieldFormat,
				"Headers must be 12 characters or less",
			), nil
		}
		if len(q.Options) < 2 {
			return tools.FormatToolError(
				tools.ErrCodeInvalidRequest,
				fmt.Sprintf("question[%d]: at least 2 options required", i),
				tools.HintCheckRequiredFields,
				"Each question must have 2-4 options",
			), nil
		}
		if len(q.Options) > 4 {
			return tools.FormatToolError(
				tools.ErrCodeInvalidRequest,
				fmt.Sprintf("question[%d]: maximum 4 options allowed", i),
				"Limit to 4 options per question",
				"Consider combining similar options",
			), nil
		}

		// Validate each option
		for j, opt := range q.Options {
			if opt.Label == "" {
				return tools.FormatToolError(
					tools.ErrCodeInvalidRequest,
					fmt.Sprintf("question[%d].option[%d]: label is required", i, j),
					tools.HintCheckRequiredFields,
					"Each option must have label and description",
				), nil
			}
			if opt.Description == "" {
				return tools.FormatToolError(
					tools.ErrCodeInvalidRequest,
					fmt.Sprintf("question[%d].option[%d]: description is required", i, j),
					tools.HintCheckRequiredFields,
					"Each option must have label and description",
				), nil
			}
		}
	}

	// Generate IDs for questions and options if not provided
	canonicalQuestions := make([]types.AskUserQuestion, 0, len(params.Questions))
	questionIDs := make(map[string]bool)

	for i, q := range params.Questions {
		// Generate question ID if missing
		qid := q.ID
		if qid == "" {
			qid = fmt.Sprintf("q%d", i+1)
		}
		if questionIDs[qid] {
			return tools.FormatToolError(
				tools.ErrCodeInvalidRequest,
				fmt.Sprintf("duplicate question ID: %s", qid),
				tools.HintCheckFieldFormat,
				"Question IDs must be unique",
			), nil
		}
		questionIDs[qid] = true

		// Convert options with IDs
		canonicalOptions := make([]types.QuestionOption, 0, len(q.Options))
		optionIDs := make(map[string]bool)
		for j, opt := range q.Options {
			// Generate option ID if missing
			oid := opt.ID
			if oid == "" {
				oid = fmt.Sprintf("%s_opt%d", qid, j+1)
			}
			if optionIDs[oid] {
				return tools.FormatToolError(
					tools.ErrCodeInvalidRequest,
					fmt.Sprintf("question[%d]: duplicate option ID: %s", i, oid),
					tools.HintCheckFieldFormat,
					"Option IDs must be unique within a question",
				), nil
			}
			optionIDs[oid] = true

			canonicalOptions = append(canonicalOptions, types.QuestionOption{
				ID:          oid,
				Label:       opt.Label,
				Description: opt.Description,
			})
		}

		canonicalQuestions = append(canonicalQuestions, types.AskUserQuestion{
			ID:          qid,
			Question:    q.Question,
			Header:      q.Header,
			MultiSelect: q.MultiSelect,
			Options:     canonicalOptions,
		})
	}

	// Create canonical interrupt payload
	var metadata *types.QuestionMetadata
	if params.Metadata != nil {
		metadata = &types.QuestionMetadata{
			Source: params.Metadata.Source,
		}
	}

	payload := types.AskUserQuestionPayload{
		Type:      types.InterruptTypeAskUserQuestion,
		Questions: canonicalQuestions,
		Metadata:  metadata,
	}

	// Return interrupt signal as JSON
	// The executor will detect this and trigger an interrupt
	return agents.FormatToolOutput(payload)
}
