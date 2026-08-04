package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuestionDataSubmitAnswersAcceptsFreeText(t *testing.T) {
	qd, err := NewQuestionData("checkpoint-1", "ali", []QuestionDataItem{
		{
			ID:   "period",
			Text: "Which period?",
			Type: "single_choice",
			Options: []QuestionDataOption{
				{ID: "ytd", Label: "Year to date"},
				{ID: "last12", Label: "Last 12 months"},
			},
		},
	})
	require.NoError(t, err)

	submitted, err := qd.SubmitAnswers(map[string]string{
		"period": "Show quarters for last year",
	})
	require.NoError(t, err)
	require.NotNil(t, submitted)
	assert.Equal(t, QuestionStatusAnswerSubmitted, submitted.Status)
	assert.Equal(t, "Show quarters for last year", submitted.Answers["period"])
}

func TestQuestionDataSubmitAnswersCanonicalizesOptionLabels(t *testing.T) {
	qd, err := NewQuestionData("checkpoint-1", "ali", []QuestionDataItem{
		{
			ID:   "slice",
			Text: "Which slice?",
			Type: "single_choice",
			Options: []QuestionDataOption{
				{ID: "all", Label: "All products"},
				{ID: "region", Label: "By region"},
			},
		},
	})
	require.NoError(t, err)

	submitted, err := qd.SubmitAnswers(map[string]string{
		"slice": "all products",
	})
	require.NoError(t, err)
	require.NotNil(t, submitted)
	assert.Equal(t, "all", submitted.Answers["slice"])
}
