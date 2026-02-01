package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// QuestionOption represents a single option in a question.
type QuestionOption struct {
	Label       string `json:"label"`       // Display text (1-5 words, concise)
	Description string `json:"description"` // Explanation of what this option means
}

// UserQuestion represents a question posed to the user.
type UserQuestion struct {
	Question    string           `json:"question"`    // The complete question (must end with ?)
	Header      string           `json:"header"`      // Short label (max 12 chars) - displays as chip/tag
	MultiSelect bool             `json:"multiSelect"` // Allow multiple selections (default: false)
	Options     []QuestionOption `json:"options"`     // 2-4 options required
}

// QuestionMetadata represents optional metadata for the question request.
type QuestionMetadata struct {
	Source string `json:"source,omitempty"` // Optional tracking (e.g., "remember")
}

// InterruptData represents the data for an interrupt event.
// This is what gets saved in the checkpoint for HITL resumption.
type InterruptData struct {
	Type      string            `json:"type"`               // "ask_user_question"
	Questions []UserQuestion    `json:"questions"`          // The questions to ask
	Metadata  *QuestionMetadata `json:"metadata,omitempty"` // Optional metadata
}

// AskUserQuestionTool is a special tool that triggers a HITL (Human-in-the-Loop) interrupt.
// When called, the executor will pause execution, save a checkpoint, and wait for user input.
// This tool does NOT implement the standard Tool interface - it's handled specially by the executor.
// Instead, use NewAskUserQuestionHandler() to create an InterruptHandler.
type AskUserQuestionTool struct{}

// NewAskUserQuestionTool creates a tool that asks the user for clarification.
// NOTE: This tool should be registered with the executor's interrupt handler registry,
// not the regular tool registry. The executor will handle it specially.
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
	Questions []UserQuestion    `json:"questions"`
	Metadata  *QuestionMetadata `json:"metadata,omitempty"`
}

// Call executes the ask user question operation.
// This creates the question structure that will be used by the interrupt handler.
func (t *AskUserQuestionTool) Call(ctx context.Context, input string) (string, error) {
	const op serrors.Op = "AskUserQuestionTool.Call"

	// Parse input
	params, err := agents.ParseToolInput[askQuestionInput](input)
	if err != nil {
		return FormatToolError(
			ErrCodeInvalidRequest,
			fmt.Sprintf("failed to parse input: %v", err),
			HintCheckRequiredFields,
			"Provide questions array with valid structure",
		), serrors.E(op, err, "failed to parse input")
	}

	// Validate questions array
	if len(params.Questions) == 0 {
		return FormatToolError(
			ErrCodeInvalidRequest,
			"at least one question is required",
			HintCheckRequiredFields,
			"Provide 1-4 questions to ask the user",
		), serrors.E(op, "at least one question is required")
	}
	if len(params.Questions) > 4 {
		return FormatToolError(
			ErrCodeInvalidRequest,
			"maximum 4 questions allowed",
			"Limit to 4 questions per request",
			"Consider asking fewer, more focused questions",
		), serrors.E(op, "maximum 4 questions allowed")
	}

	// Validate each question
	for i, q := range params.Questions {
		if q.Question == "" {
			return FormatToolError(
				ErrCodeInvalidRequest,
				fmt.Sprintf("question[%d]: question text is required", i),
				HintCheckRequiredFields,
				"Each question must have question text ending with ?",
			), serrors.E(op, fmt.Sprintf("question[%d]: question text is required", i))
		}
		if q.Header == "" {
			return FormatToolError(
				ErrCodeInvalidRequest,
				fmt.Sprintf("question[%d]: header is required", i),
				HintCheckRequiredFields,
				"Each question must have a short header (max 12 chars)",
			), serrors.E(op, fmt.Sprintf("question[%d]: header is required", i))
		}
		if len(q.Header) > 12 {
			return FormatToolError(
				ErrCodeInvalidRequest,
				fmt.Sprintf("question[%d]: header exceeds 12 characters", i),
				HintCheckFieldFormat,
				"Headers must be 12 characters or less",
			), serrors.E(op, fmt.Sprintf("question[%d]: header exceeds 12 characters", i))
		}
		if len(q.Options) < 2 {
			return FormatToolError(
				ErrCodeInvalidRequest,
				fmt.Sprintf("question[%d]: at least 2 options required", i),
				HintCheckRequiredFields,
				"Each question must have 2-4 options",
			), serrors.E(op, fmt.Sprintf("question[%d]: at least 2 options required", i))
		}
		if len(q.Options) > 4 {
			return FormatToolError(
				ErrCodeInvalidRequest,
				fmt.Sprintf("question[%d]: maximum 4 options allowed", i),
				"Limit to 4 options per question",
				"Consider combining similar options",
			), serrors.E(op, fmt.Sprintf("question[%d]: maximum 4 options allowed", i))
		}

		// Validate each option
		for j, opt := range q.Options {
			if opt.Label == "" {
				return FormatToolError(
					ErrCodeInvalidRequest,
					fmt.Sprintf("question[%d].option[%d]: label is required", i, j),
					HintCheckRequiredFields,
					"Each option must have label and description",
				), serrors.E(op, fmt.Sprintf("question[%d].option[%d]: label is required", i, j))
			}
			if opt.Description == "" {
				return FormatToolError(
					ErrCodeInvalidRequest,
					fmt.Sprintf("question[%d].option[%d]: description is required", i, j),
					HintCheckRequiredFields,
					"Each option must have label and description",
				), serrors.E(op, fmt.Sprintf("question[%d].option[%d]: description is required", i, j))
			}
		}
	}

	// Create interrupt data
	interruptData := InterruptData{
		Type:      "ask_user_question",
		Questions: params.Questions,
		Metadata:  params.Metadata,
	}

	// Return interrupt signal as JSON
	// The executor will detect this and trigger an interrupt
	return agents.FormatToolOutput(interruptData)
}

// NewAskUserQuestionHandler creates an InterruptHandler for ask_user_question.
// This should be registered with the executor's interrupt handler registry.
//
// Example usage:
//
//	handler := tools.NewAskUserQuestionHandler()
//	executor.RegisterInterruptHandler(agents.ToolAskUserQuestion, handler)
func NewAskUserQuestionHandler() InterruptHandler {
	return &askUserQuestionHandler{}
}

// InterruptHandler handles HITL interrupts.
// This interface matches the one expected by the executor.
type InterruptHandler interface {
	// HandleInterrupt processes an interrupt and returns the data to save in the checkpoint.
	HandleInterrupt(ctx context.Context, toolName, input string) (json.RawMessage, error)

	// ResumeFromInterrupt processes the user's answer and returns the result to feed back to the agent.
	ResumeFromInterrupt(ctx context.Context, checkpointData json.RawMessage, answer string) (string, error)
}

// askUserQuestionHandler implements the InterruptHandler for ask_user_question.
type askUserQuestionHandler struct{}

// HandleInterrupt saves the question data for the checkpoint.
func (h *askUserQuestionHandler) HandleInterrupt(ctx context.Context, toolName, input string) (json.RawMessage, error) {
	const op serrors.Op = "askUserQuestionHandler.HandleInterrupt"

	// Parse the tool input to get the questions
	params, err := agents.ParseToolInput[askQuestionInput](input)
	if err != nil {
		return nil, serrors.E(op, err, "failed to parse input")
	}

	// Validate questions
	if len(params.Questions) == 0 {
		return nil, serrors.E(op, "at least one question is required")
	}

	// Build the interrupt data
	interruptData := InterruptData{
		Type:      "ask_user_question",
		Questions: params.Questions,
		Metadata:  params.Metadata,
	}

	// Save as checkpoint data
	data, err := json.Marshal(interruptData)
	if err != nil {
		return nil, serrors.E(op, err, "failed to marshal questions")
	}

	return data, nil
}

// ResumeFromInterrupt processes the user's answer and returns it to the agent.
// The answer should be a JSON object mapping question headers to selected options.
// For single-select questions: { "header1": "selected_label" }
// For multi-select questions: { "header2": ["label1", "label2"] }
func (h *askUserQuestionHandler) ResumeFromInterrupt(ctx context.Context, checkpointData json.RawMessage, answer string) (string, error) {
	const op serrors.Op = "askUserQuestionHandler.ResumeFromInterrupt"

	// Parse the saved interrupt data
	var interruptData InterruptData
	if err := json.Unmarshal(checkpointData, &interruptData); err != nil {
		return "", serrors.E(op, err, "failed to unmarshal interrupt data")
	}

	// Parse the user's answers
	var answers map[string]interface{}
	if err := json.Unmarshal([]byte(answer), &answers); err != nil {
		return "", serrors.E(op, err, "failed to parse user answers")
	}

	// Format the result to return to the agent
	result := map[string]interface{}{
		"questions": interruptData.Questions,
		"answers":   answers,
	}

	if interruptData.Metadata != nil {
		result["metadata"] = interruptData.Metadata
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", serrors.E(op, err, "failed to marshal result")
	}

	return string(data), nil
}
