package hitl

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAnswers_Scenarios(t *testing.T) {
	cases := []struct {
		name                string
		questions           []types.QuestionDataItem
		rawAnswers          map[string]string
		questionID          string
		expectedValue       string
		expectedAnswer      string
		expectedMultiAnswer []string
		expectedErr         string
	}{
		{
			name: "single choice canonicalizes by id and label",
			questions: []types.QuestionDataItem{{
				ID:   "q1",
				Type: "single_choice",
				Options: []types.QuestionDataOption{
					{ID: "opt_a", Label: "Option A"},
					{ID: "opt_b", Label: "Option B"},
				},
			}},
			rawAnswers:     map[string]string{"q1": "option b"},
			questionID:     "q1",
			expectedValue:  "opt_b",
			expectedAnswer: "opt_b",
		},
		{
			name: "single choice preserves free text",
			questions: []types.QuestionDataItem{{
				ID:   "q1",
				Type: "single_choice",
				Options: []types.QuestionDataOption{
					{ID: "opt_a", Label: "Option A"},
					{ID: "opt_b", Label: "Option B"},
				},
			}},
			rawAnswers:     map[string]string{"q1": "Custom free text"},
			questionID:     "q1",
			expectedValue:  "Custom free text",
			expectedAnswer: "Custom free text",
		},
		{
			name: "multiple choice dedupes canonical options",
			questions: []types.QuestionDataItem{{
				ID:   "q1",
				Type: "multiple_choice",
				Options: []types.QuestionDataOption{
					{ID: "opt_a", Label: "Option A"},
					{ID: "opt_b", Label: "Option B"},
				},
			}},
			rawAnswers:          map[string]string{"q1": "Option A, opt_a, option b"},
			questionID:          "q1",
			expectedValue:       "opt_a, opt_b",
			expectedMultiAnswer: []string{"opt_a", "opt_b"},
		},
		{
			name: "multiple choice preserves custom text",
			questions: []types.QuestionDataItem{{
				ID:   "q1",
				Type: "multiple_choice",
				Options: []types.QuestionDataOption{
					{ID: "opt_a", Label: "Option A"},
					{ID: "opt_b", Label: "Option B"},
				},
			}},
			rawAnswers:     map[string]string{"q1": "Something custom"},
			questionID:     "q1",
			expectedValue:  "Something custom",
			expectedAnswer: "Something custom",
		},
		{
			name: "unknown question id returns validation error",
			questions: []types.QuestionDataItem{{
				ID:   "q1",
				Type: "single_choice",
			}},
			rawAnswers:  map[string]string{"q-missing": "something"},
			expectedErr: "unknown question id",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			values, answers, err := NormalizeAnswers(tc.questions, tc.rawAnswers)

			if tc.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, tc.questionID)
			assert.Equal(t, tc.expectedValue, values[tc.questionID])
			if len(tc.expectedMultiAnswer) > 0 {
				assert.Equal(t, tc.expectedMultiAnswer, answers[tc.questionID].Strings())
				return
			}
			assert.Equal(t, tc.expectedAnswer, answers[tc.questionID].String())
		})
	}
}
