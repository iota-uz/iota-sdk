// Package hitl provides this package.
package hitl

import "github.com/iota-uz/iota-sdk/pkg/bichat/types"

func NormalizeAnswers(
	questions []types.QuestionDataItem,
	rawAnswers map[string]string,
) (map[string]string, map[string]types.Answer, error) {
	return types.NormalizeQuestionAnswers(questions, rawAnswers)
}
