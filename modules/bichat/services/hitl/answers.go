package hitl

import (
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

func NormalizeAnswers(
	questions []types.QuestionDataItem,
	rawAnswers map[string]string,
) (map[string]string, map[string]types.Answer, error) {
	normalizedValues := make(map[string]string, len(rawAnswers))
	normalizedAnswers := make(map[string]types.Answer, len(rawAnswers))
	questionsByID := make(map[string]types.QuestionDataItem, len(questions))
	for _, q := range questions {
		questionsByID[q.ID] = q
	}

	for questionID, answerValue := range rawAnswers {
		question, ok := questionsByID[questionID]
		if !ok {
			return nil, nil, fmt.Errorf("unknown question id: %s", questionID)
		}

		if question.Type == "multiple_choice" {
			parts := splitAnswerParts(answerValue)
			if len(parts) == 0 {
				trimmed := strings.TrimSpace(answerValue)
				normalizedValues[questionID] = trimmed
				normalizedAnswers[questionID] = types.NewAnswer(trimmed)
				continue
			}

			canonicalParts := make([]string, 0, len(parts))
			seen := make(map[string]struct{}, len(parts))
			for _, part := range parts {
				canonical := canonicalizeAnswerPart(part, question.Options)
				if canonical == "" {
					continue
				}
				if _, exists := seen[canonical]; exists {
					continue
				}
				seen[canonical] = struct{}{}
				canonicalParts = append(canonicalParts, canonical)
			}

			if len(canonicalParts) == 0 {
				trimmed := strings.TrimSpace(answerValue)
				normalizedValues[questionID] = trimmed
				normalizedAnswers[questionID] = types.NewAnswer(trimmed)
			} else if len(canonicalParts) == 1 {
				normalizedValues[questionID] = canonicalParts[0]
				normalizedAnswers[questionID] = types.NewAnswer(canonicalParts[0])
			} else {
				normalizedValues[questionID] = strings.Join(canonicalParts, ", ")
				normalizedAnswers[questionID] = types.NewMultiAnswer(canonicalParts)
			}
			continue
		}

		canonical := canonicalizeAnswerPart(answerValue, question.Options)
		normalizedValues[questionID] = canonical
		normalizedAnswers[questionID] = types.NewAnswer(canonical)
	}

	return normalizedValues, normalizedAnswers, nil
}

func splitAnswerParts(value string) []string {
	parts := strings.Split(strings.TrimSpace(value), ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func canonicalizeAnswerPart(value string, options []types.QuestionDataOption) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	for _, opt := range options {
		if strings.EqualFold(strings.TrimSpace(opt.ID), trimmed) {
			return opt.ID
		}
	}
	for _, opt := range options {
		if strings.EqualFold(strings.TrimSpace(opt.Label), trimmed) {
			return opt.ID
		}
	}

	return trimmed
}
