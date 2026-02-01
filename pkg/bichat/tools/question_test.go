package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAskUserQuestionTool_Parameters(t *testing.T) {
	tool := NewAskUserQuestionTool()

	params := tool.Parameters()
	assert.NotNil(t, params)

	// Verify questions array is required
	required, ok := params["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, required, "questions")

	// Verify questions schema
	props, ok := params["properties"].(map[string]any)
	require.True(t, ok)

	questions, ok := props["questions"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "array", questions["type"])
	assert.Equal(t, 1, questions["minItems"])
	assert.Equal(t, 4, questions["maxItems"])
}

func TestAskUserQuestionTool_Call_Success(t *testing.T) {
	tool := NewAskUserQuestionTool()
	ctx := context.Background()

	input := `{
		"questions": [
			{
				"question": "What is your preferred time range?",
				"header": "Time Range",
				"multiSelect": false,
				"options": [
					{"label": "Last Week", "description": "Data from the previous 7 days"},
					{"label": "Last Month", "description": "Data from the previous 30 days"},
					{"label": "Last Quarter", "description": "Data from the previous 90 days"}
				]
			}
		]
	}`

	result, err := tool.Call(ctx, input)
	require.NoError(t, err)

	// Parse result as InterruptData
	var interruptData InterruptData
	err = json.Unmarshal([]byte(result), &interruptData)
	require.NoError(t, err)

	assert.Equal(t, "ask_user_question", interruptData.Type)
	assert.Len(t, interruptData.Questions, 1)

	q := interruptData.Questions[0]
	assert.Equal(t, "What is your preferred time range?", q.Question)
	assert.Equal(t, "Time Range", q.Header)
	assert.False(t, q.MultiSelect)
	assert.Len(t, q.Options, 3)
	assert.Equal(t, "Last Week", q.Options[0].Label)
	assert.Equal(t, "Data from the previous 7 days", q.Options[0].Description)
}

func TestAskUserQuestionTool_Call_MultipleQuestions(t *testing.T) {
	tool := NewAskUserQuestionTool()
	ctx := context.Background()

	input := `{
		"questions": [
			{
				"question": "Which metric?",
				"header": "Metric",
				"multiSelect": false,
				"options": [
					{"label": "Revenue", "description": "Total revenue"},
					{"label": "Profit", "description": "Net profit"}
				]
			},
			{
				"question": "Which region?",
				"header": "Region",
				"multiSelect": true,
				"options": [
					{"label": "North", "description": "Northern region"},
					{"label": "South", "description": "Southern region"},
					{"label": "East", "description": "Eastern region"}
				]
			}
		]
	}`

	result, err := tool.Call(ctx, input)
	require.NoError(t, err)

	var interruptData InterruptData
	err = json.Unmarshal([]byte(result), &interruptData)
	require.NoError(t, err)

	assert.Len(t, interruptData.Questions, 2)
	assert.Equal(t, "Metric", interruptData.Questions[0].Header)
	assert.False(t, interruptData.Questions[0].MultiSelect)
	assert.Equal(t, "Region", interruptData.Questions[1].Header)
	assert.True(t, interruptData.Questions[1].MultiSelect)
}

func TestAskUserQuestionTool_Call_WithMetadata(t *testing.T) {
	tool := NewAskUserQuestionTool()
	ctx := context.Background()

	input := `{
		"questions": [
			{
				"question": "Continue?",
				"header": "Confirm",
				"multiSelect": false,
				"options": [
					{"label": "Yes", "description": "Proceed"},
					{"label": "No", "description": "Cancel"}
				]
			}
		],
		"metadata": {
			"source": "remember"
		}
	}`

	result, err := tool.Call(ctx, input)
	require.NoError(t, err)

	var interruptData InterruptData
	err = json.Unmarshal([]byte(result), &interruptData)
	require.NoError(t, err)

	require.NotNil(t, interruptData.Metadata)
	assert.Equal(t, "remember", interruptData.Metadata.Source)
}

func TestAskUserQuestionTool_Call_ValidationErrors(t *testing.T) {
	tool := NewAskUserQuestionTool()
	ctx := context.Background()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "no questions",
			input: `{"questions": []}`,
		},
		{
			name:  "too many questions",
			input: `{"questions": [{},{},{},{},{}]}`, // 5 questions
		},
		{
			name: "missing header",
			input: `{
				"questions": [{
					"question": "Test?",
					"options": [
						{"label": "A", "description": "Option A"},
						{"label": "B", "description": "Option B"}
					]
				}]
			}`,
		},
		{
			name: "header too long",
			input: `{
				"questions": [{
					"question": "Test?",
					"header": "ThisIsWayTooLong",
					"options": [
						{"label": "A", "description": "Option A"},
						{"label": "B", "description": "Option B"}
					]
				}]
			}`,
		},
		{
			name: "too few options",
			input: `{
				"questions": [{
					"question": "Test?",
					"header": "Test",
					"options": [
						{"label": "A", "description": "Option A"}
					]
				}]
			}`,
		},
		{
			name: "too many options",
			input: `{
				"questions": [{
					"question": "Test?",
					"header": "Test",
					"options": [
						{"label": "A", "description": "A"},
						{"label": "B", "description": "B"},
						{"label": "C", "description": "C"},
						{"label": "D", "description": "D"},
						{"label": "E", "description": "E"}
					]
				}]
			}`,
		},
		{
			name: "missing option label",
			input: `{
				"questions": [{
					"question": "Test?",
					"header": "Test",
					"options": [
						{"description": "Option A"},
						{"label": "B", "description": "Option B"}
					]
				}]
			}`,
		},
		{
			name: "missing option description",
			input: `{
				"questions": [{
					"question": "Test?",
					"header": "Test",
					"options": [
						{"label": "A"},
						{"label": "B", "description": "Option B"}
					]
				}]
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Call(ctx, tt.input)
			assert.Error(t, err, "Expected validation error for: %s", tt.name)
		})
	}
}

func TestAskUserQuestionHandler_HandleInterrupt(t *testing.T) {
	handler := NewAskUserQuestionHandler()
	ctx := context.Background()

	input := `{
		"questions": [
			{
				"question": "Confirm action?",
				"header": "Confirm",
				"multiSelect": false,
				"options": [
					{"label": "Yes", "description": "Proceed"},
					{"label": "No", "description": "Cancel"}
				]
			}
		]
	}`

	data, err := handler.HandleInterrupt(ctx, "ask_user_question", input)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify checkpoint data can be unmarshaled
	var interruptData InterruptData
	err = json.Unmarshal(data, &interruptData)
	require.NoError(t, err)
	assert.Equal(t, "ask_user_question", interruptData.Type)
	assert.Len(t, interruptData.Questions, 1)
}

func TestAskUserQuestionHandler_ResumeFromInterrupt(t *testing.T) {
	handler := NewAskUserQuestionHandler()
	ctx := context.Background()

	// Checkpoint data
	checkpointData := json.RawMessage(`{
		"type": "ask_user_question",
		"questions": [
			{
				"question": "Select metric?",
				"header": "Metric",
				"multiSelect": false,
				"options": [
					{"label": "Revenue", "description": "Total revenue"},
					{"label": "Profit", "description": "Net profit"}
				]
			}
		]
	}`)

	// User answers
	answer := `{"Metric": "Revenue"}`

	result, err := handler.ResumeFromInterrupt(ctx, checkpointData, answer)
	require.NoError(t, err)

	// Parse result
	var resultData map[string]interface{}
	err = json.Unmarshal([]byte(result), &resultData)
	require.NoError(t, err)

	assert.Contains(t, resultData, "questions")
	assert.Contains(t, resultData, "answers")

	answers, ok := resultData["answers"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Revenue", answers["Metric"])
}
