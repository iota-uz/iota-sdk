package types

import (
	"fmt"
	"strings"
)

// NormalizeQuestionAnswers canonicalizes submitted answers against the
// question definitions while still preserving valid free-text responses.
func NormalizeQuestionAnswers(
	questions []QuestionDataItem,
	rawAnswers map[string]string,
) (map[string]string, map[string]Answer, error) {
	if len(rawAnswers) == 0 {
		return nil, nil, fmt.Errorf("%w: at least one answer is required", ErrQuestionDataInvalid)
	}

	questionsByID := make(map[string]QuestionDataItem, len(questions))
	for _, q := range questions {
		questionsByID[q.ID] = q
	}

	normalizedValues := make(map[string]string, len(rawAnswers))
	normalizedAnswers := make(map[string]Answer, len(rawAnswers))
	for questionID, rawAnswer := range rawAnswers {
		question, ok := questionsByID[questionID]
		if !ok {
			return nil, nil, fmt.Errorf("%w: unknown question id %q", ErrQuestionDataInvalid, questionID)
		}

		normalizedValue, normalizedAnswer, err := normalizeQuestionAnswer(question, rawAnswer)
		if err != nil {
			return nil, nil, err
		}

		normalizedValues[questionID] = normalizedValue
		normalizedAnswers[questionID] = normalizedAnswer
	}

	return normalizedValues, normalizedAnswers, nil
}

func normalizeQuestionAnswer(question QuestionDataItem, rawAnswer string) (string, Answer, error) {
	if question.Type == "multiple_choice" {
		parts := splitNormalizedAnswerParts(rawAnswer)
		if len(parts) == 0 {
			return "", Answer{}, fmt.Errorf("%w: empty answer for question %q", ErrQuestionDataInvalid, question.ID)
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
			return "", Answer{}, fmt.Errorf("%w: empty answer for question %q", ErrQuestionDataInvalid, question.ID)
		}
		if len(canonicalParts) == 1 {
			return canonicalParts[0], NewAnswer(canonicalParts[0]), nil
		}

		joined := strings.Join(canonicalParts, ", ")
		return joined, NewMultiAnswer(canonicalParts), nil
	}

	canonical := canonicalizeAnswerPart(rawAnswer, question.Options)
	if canonical == "" {
		return "", Answer{}, fmt.Errorf("%w: empty answer for question %q", ErrQuestionDataInvalid, question.ID)
	}
	return canonical, NewAnswer(canonical), nil
}

func splitNormalizedAnswerParts(value string) []string {
	parts := strings.Split(strings.TrimSpace(value), ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func canonicalizeAnswerPart(value string, options []QuestionDataOption) string {
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
