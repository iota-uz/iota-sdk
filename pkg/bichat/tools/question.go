package tools

import (
	"context"
	"encoding/json"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// QuestionType represents the type of user question.
type QuestionType string

const (
	QuestionTypeText           QuestionType = "text"
	QuestionTypeSingleChoice   QuestionType = "single_choice"
	QuestionTypeMultipleChoice QuestionType = "multiple_choice"
)

// QuestionChoice represents a choice for a question.
type QuestionChoice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value string `json:"value"`
}

// UserQuestion represents a question posed to the user.
type UserQuestion struct {
	ID       string           `json:"id"`
	Question string           `json:"question"`
	Type     QuestionType     `json:"type"`
	Choices  []QuestionChoice `json:"choices,omitempty"`
	Required bool             `json:"required"`
}

// InterruptData represents the data for an interrupt event.
// This is what gets saved in the checkpoint for HITL resumption.
type InterruptData struct {
	Type     string       `json:"type"`     // "ask_user_question"
	Question UserQuestion `json:"question"` // The question to ask
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
	return "Ask the user a clarifying question when requirements are ambiguous. " +
		"The agent will pause until the user provides an answer. " +
		"Use this sparingly, only when truly necessary."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *AskUserQuestionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "The clarifying question to ask the user",
			},
			"question_type": map[string]any{
				"type":        "string",
				"description": "Type of question: 'text', 'single_choice', 'multiple_choice'",
				"enum":        []string{"text", "single_choice", "multiple_choice"},
				"default":     "text",
			},
			"choices": map[string]any{
				"type":        "array",
				"description": "Available choices (for single_choice or multiple_choice)",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{
							"type":        "string",
							"description": "Unique identifier for the choice",
						},
						"label": map[string]any{
							"type":        "string",
							"description": "Display label for the choice",
						},
					},
					"required": []string{"id", "label"},
				},
			},
		},
		"required": []string{"question"},
	}
}

// askQuestionInput represents the parsed input parameters.
type askQuestionInput struct {
	Question     string `json:"question"`
	QuestionType string `json:"question_type,omitempty"`
	Choices      []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	} `json:"choices,omitempty"`
}

// Call executes the ask user question operation.
// This creates the question structure that will be used by the interrupt handler.
func (t *AskUserQuestionTool) Call(ctx context.Context, input string) (string, error) {
	const op serrors.Op = "AskUserQuestionTool.Call"

	// Parse input
	params, err := agents.ParseToolInput[askQuestionInput](input)
	if err != nil {
		return "", serrors.E(op, err, "failed to parse input")
	}

	if params.Question == "" {
		return "", serrors.E(op, "question parameter is required")
	}

	// Set defaults
	questionType := QuestionTypeText
	if params.QuestionType != "" {
		questionType = QuestionType(params.QuestionType)
	}

	// Convert choices
	var choices []QuestionChoice
	for _, choice := range params.Choices {
		if choice.ID != "" && choice.Label != "" {
			choices = append(choices, QuestionChoice{
				ID:    choice.ID,
				Label: choice.Label,
				Value: choice.ID,
			})
		}
	}

	// Create the user question
	userQuestion := UserQuestion{
		ID:       "q1", // Simple ID, could use UUID
		Question: params.Question,
		Type:     questionType,
		Choices:  choices,
		Required: true,
	}

	// Create interrupt data
	interruptData := InterruptData{
		Type:     "ask_user_question",
		Question: userQuestion,
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

	// Parse the tool input to get the question
	params, err := agents.ParseToolInput[askQuestionInput](input)
	if err != nil {
		return nil, serrors.E(op, err, "failed to parse input")
	}

	// Build the user question
	questionType := QuestionTypeText
	if params.QuestionType != "" {
		questionType = QuestionType(params.QuestionType)
	}

	var choices []QuestionChoice
	for _, choice := range params.Choices {
		if choice.ID != "" && choice.Label != "" {
			choices = append(choices, QuestionChoice{
				ID:    choice.ID,
				Label: choice.Label,
				Value: choice.ID,
			})
		}
	}

	userQuestion := UserQuestion{
		ID:       "q1",
		Question: params.Question,
		Type:     questionType,
		Choices:  choices,
		Required: true,
	}

	// Save as checkpoint data
	data, err := json.Marshal(userQuestion)
	if err != nil {
		return nil, serrors.E(op, err, "failed to marshal question")
	}

	return data, nil
}

// ResumeFromInterrupt processes the user's answer and returns it to the agent.
func (h *askUserQuestionHandler) ResumeFromInterrupt(ctx context.Context, checkpointData json.RawMessage, answer string) (string, error) {
	const op serrors.Op = "askUserQuestionHandler.ResumeFromInterrupt"

	// Parse the saved question
	var question UserQuestion
	if err := json.Unmarshal(checkpointData, &question); err != nil {
		return "", serrors.E(op, err, "failed to unmarshal question")
	}

	// Format the answer as a tool result
	result := map[string]interface{}{
		"question": question.Question,
		"answer":   answer,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", serrors.E(op, err, "failed to marshal answer")
	}

	return string(data), nil
}
