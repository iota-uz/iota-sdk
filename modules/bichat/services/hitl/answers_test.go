package hitl

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAnswers_CanonicalizesByIDAndLabel(t *testing.T) {
	questions := []types.QuestionDataItem{{
		ID:   "q1",
		Type: "single_choice",
		Options: []types.QuestionDataOption{
			{ID: "opt_a", Label: "Option A"},
			{ID: "opt_b", Label: "Option B"},
		},
	}}

	values, answers := NormalizeAnswers(questions, map[string]string{"q1": "option b"})
	require.Equal(t, "opt_b", values["q1"])
	require.Equal(t, "opt_b", answers["q1"].String())
}

func TestNormalizeAnswers_MultiChoiceDedupes(t *testing.T) {
	questions := []types.QuestionDataItem{{
		ID:   "q1",
		Type: "multiple_choice",
		Options: []types.QuestionDataOption{
			{ID: "opt_a", Label: "Option A"},
			{ID: "opt_b", Label: "Option B"},
		},
	}}

	values, answers := NormalizeAnswers(questions, map[string]string{"q1": "Option A, opt_a, option b"})
	require.Equal(t, "opt_a, opt_b", values["q1"])
	require.Equal(t, []string{"opt_a", "opt_b"}, answers["q1"].Strings())
}
