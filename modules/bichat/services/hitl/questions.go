package hitl

import (
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// BuildQuestionData converts service interrupt questions to QuestionData for persistence.
func BuildQuestionData(checkpointID, agentName string, questions []bichatservices.Question) (*types.QuestionData, error) {
	const op serrors.Op = "hitl.BuildQuestionData"
	items := make([]types.QuestionDataItem, len(questions))
	for i, q := range questions {
		opts := make([]types.QuestionDataOption, len(q.Options))
		for j, o := range q.Options {
			opts[j] = types.QuestionDataOption{ID: o.ID, Label: o.Label}
		}
		qType := "single_choice"
		if q.Type == bichatservices.QuestionTypeMultipleChoice {
			qType = "multiple_choice"
		}
		items[i] = types.QuestionDataItem{
			ID:      q.ID,
			Text:    q.Text,
			Type:    qType,
			Options: opts,
		}
	}
	qd, err := types.NewQuestionData(checkpointID, agentName, items)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return qd, nil
}

// AgentQuestionsToServiceQuestions converts agent questions to service questions.
func AgentQuestionsToServiceQuestions(qs []agents.Question) []bichatservices.Question {
	if len(qs) == 0 {
		return nil
	}
	out := make([]bichatservices.Question, len(qs))
	for i, q := range qs {
		opts := make([]bichatservices.QuestionOption, len(q.Options))
		for j, o := range q.Options {
			opts[j] = bichatservices.QuestionOption{ID: o.ID, Label: o.Label}
		}
		qt := bichatservices.QuestionTypeSingleChoice
		if q.Type == agents.QuestionTypeMultipleChoice {
			qt = bichatservices.QuestionTypeMultipleChoice
		}
		out[i] = bichatservices.Question{
			ID:      q.ID,
			Text:    q.Text,
			Type:    qt,
			Options: opts,
		}
	}
	return out
}
