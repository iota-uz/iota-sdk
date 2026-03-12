package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// InterruptType identifies the type of interrupt.
type InterruptType string

const (
	// InterruptTypeAskUserQuestion indicates a HITL interrupt waiting for user answers.
	InterruptTypeAskUserQuestion InterruptType = "ASK_USER_QUESTION"
)

// AskUserQuestion represents a single question in a HITL interrupt.
type AskUserQuestion struct {
	ID          string           `json:"id"`          // Stable identifier for the question (required, unique)
	Question    string           `json:"question"`    // The complete question text (must end with ?)
	Header      string           `json:"header"`      // Short label (max 12 chars) - displays as chip/tag
	MultiSelect bool             `json:"multiSelect"` // Allow multiple selections (default: false)
	Options     []QuestionOption `json:"options"`     // 2-4 options required
}

// QuestionOption represents a single option in a question.
type QuestionOption struct {
	ID          string `json:"id"`          // Stable identifier for the option (required, unique within question)
	Label       string `json:"label"`       // Display text (1-5 words, concise)
	Description string `json:"description"` // Explanation of what this option means
}

// QuestionMetadata represents optional metadata for the question request.
type QuestionMetadata struct {
	Source string `json:"source,omitempty"` // Optional tracking (e.g., "remember")
}

// AskUserQuestionPayload represents the canonical HITL interrupt payload.
// This is stored in InterruptEvent.Data and used for resume validation.
type AskUserQuestionPayload struct {
	Type      InterruptType     `json:"type"`               // "ASK_USER_QUESTION"
	Questions []AskUserQuestion `json:"questions"`          // The questions to ask (with stable IDs)
	Metadata  *QuestionMetadata `json:"metadata,omitempty"` // Optional metadata
}

// Answer represents a user's answer to a question.
// Supports both single-select (string) and multi-select ([]string).
type Answer struct {
	Value json.RawMessage `json:"value"` // JSON-encoded: string for single-select, []string for multi-select
}

// String returns the answer as a string (for single-select).
// Returns empty string if multi-select or invalid.
func (a Answer) String() string {
	var s string
	if err := json.Unmarshal(a.Value, &s); err != nil {
		return ""
	}
	return s
}

// Strings returns the answer as a string slice (for multi-select).
// Returns nil if single-select or invalid.
func (a Answer) Strings() []string {
	var ss []string
	if err := json.Unmarshal(a.Value, &ss); err != nil {
		return nil
	}
	return ss
}

// NewAnswer creates an Answer from a string (single-select).
func NewAnswer(s string) Answer {
	value, err := json.Marshal(s)
	if err != nil {
		return Answer{Value: []byte("null")}
	}
	return Answer{Value: value}
}

// NewMultiAnswer creates an Answer from a string slice (multi-select).
func NewMultiAnswer(ss []string) Answer {
	value, err := json.Marshal(ss)
	if err != nil {
		return Answer{Value: []byte("null")}
	}
	return Answer{Value: value}
}

// QuestionStatus represents the lifecycle state of a HITL question.
type QuestionStatus string

const (
	QuestionStatusPending         QuestionStatus = "PENDING"
	QuestionStatusAnswerSubmitted QuestionStatus = "ANSWER_SUBMITTED"
	QuestionStatusRejectSubmitted QuestionStatus = "REJECT_SUBMITTED"
	QuestionStatusAnswered        QuestionStatus = "ANSWERED"
	QuestionStatusRejected        QuestionStatus = "REJECTED"
)

var (
	ErrInvalidQuestionTransition = errors.New("invalid question status transition")
	ErrQuestionDataInvalid       = errors.New("invalid question data")
)

// QuestionData holds the HITL question state stored in a message's question_data JSONB column.
// It is the single source of truth for whether a question is pending, durably submitted,
// answered, or rejected.
type QuestionData struct {
	CheckpointID string             `json:"checkpointId"`
	Status       QuestionStatus     `json:"status"`
	AgentName    string             `json:"agentName"`
	Questions    []QuestionDataItem `json:"questions"`
	Answers      map[string]string  `json:"answers,omitempty"`
}

// QuestionDataItem represents a single question in the HITL payload.
type QuestionDataItem struct {
	ID      string               `json:"id"`
	Text    string               `json:"text"`
	Type    string               `json:"type"` // "single_choice" or "multiple_choice"
	Options []QuestionDataOption `json:"options"`
}

// QuestionDataOption represents a selectable option for a question.
type QuestionDataOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// NewQuestionData constructs a QuestionData in pending state.
// Returns error if invariants are violated.
func NewQuestionData(checkpointID, agentName string, questions []QuestionDataItem) (*QuestionData, error) {
	checkpointID = strings.TrimSpace(checkpointID)
	if checkpointID == "" {
		return nil, fmt.Errorf("%w: checkpointID required", ErrQuestionDataInvalid)
	}
	if len(questions) == 0 {
		return nil, fmt.Errorf("%w: at least one question required", ErrQuestionDataInvalid)
	}
	seenQuestionIDs := make(map[string]struct{}, len(questions))
	for _, q := range questions {
		qID := strings.TrimSpace(q.ID)
		if qID == "" {
			return nil, fmt.Errorf("%w: question id is required", ErrQuestionDataInvalid)
		}
		if _, exists := seenQuestionIDs[qID]; exists {
			return nil, fmt.Errorf("%w: duplicated question id %q", ErrQuestionDataInvalid, qID)
		}
		seenQuestionIDs[qID] = struct{}{}
		if len(q.Options) < 2 || len(q.Options) > 4 {
			return nil, fmt.Errorf("%w: question %q must have between 2 and 4 options", ErrQuestionDataInvalid, q.ID)
		}
		seenOptionIDs := make(map[string]struct{}, len(q.Options))
		for _, opt := range q.Options {
			optID := strings.TrimSpace(opt.ID)
			if optID == "" {
				return nil, fmt.Errorf("%w: option id is required in question %q", ErrQuestionDataInvalid, q.ID)
			}
			if _, exists := seenOptionIDs[optID]; exists {
				return nil, fmt.Errorf("%w: duplicated option id %q in question %q", ErrQuestionDataInvalid, optID, q.ID)
			}
			seenOptionIDs[optID] = struct{}{}
		}
	}
	return &QuestionData{
		CheckpointID: checkpointID,
		Status:       QuestionStatusPending,
		AgentName:    agentName,
		Questions:    questions,
	}, nil
}

func (qd *QuestionData) normalizeAnswers(answers map[string]string) (map[string]string, error) {
	if len(answers) == 0 {
		return nil, fmt.Errorf("%w: at least one answer is required", ErrQuestionDataInvalid)
	}

	questionsByID := make(map[string]QuestionDataItem, len(qd.Questions))
	for _, q := range qd.Questions {
		questionsByID[q.ID] = q
	}
	for answerQID, rawAnswer := range answers {
		question, ok := questionsByID[answerQID]
		if !ok {
			return nil, fmt.Errorf("%w: unknown question id %q", ErrQuestionDataInvalid, answerQID)
		}
		allowed := make(map[string]struct{}, len(question.Options))
		for _, opt := range question.Options {
			allowed[opt.ID] = struct{}{}
		}
		candidates := []string{strings.TrimSpace(rawAnswer)}
		if question.Type == "multiple_choice" {
			parts := strings.Split(rawAnswer, ",")
			candidates = make([]string, 0, len(parts))
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					candidates = append(candidates, trimmed)
				}
			}
		}
		if len(candidates) == 0 {
			return nil, fmt.Errorf("%w: empty answer for question %q", ErrQuestionDataInvalid, answerQID)
		}
		for _, candidate := range candidates {
			if _, exists := allowed[candidate]; !exists {
				return nil, fmt.Errorf("%w: invalid option %q for question %q", ErrQuestionDataInvalid, candidate, answerQID)
			}
		}
	}

	normalized := make(map[string]string, len(answers))
	for key, value := range answers {
		normalized[key] = value
	}
	return normalized, nil
}

// SubmitAnswers durably records validated answers while the continuation run is still pending.
func (qd *QuestionData) SubmitAnswers(answers map[string]string) (*QuestionData, error) {
	if qd.Status != QuestionStatusPending {
		return nil, fmt.Errorf("%w: cannot submit answers from status %q", ErrInvalidQuestionTransition, qd.Status)
	}
	normalized, err := qd.normalizeAnswers(answers)
	if err != nil {
		return nil, err
	}

	next := *qd
	next.Status = QuestionStatusAnswerSubmitted
	next.Answers = normalized
	return &next, nil
}

// Answer transitions from pending to answered.
func (qd *QuestionData) Answer(answers map[string]string) (*QuestionData, error) {
	switch qd.Status {
	case QuestionStatusPending, QuestionStatusAnswerSubmitted:
	default:
		return nil, fmt.Errorf("%w: cannot answer from status %q", ErrInvalidQuestionTransition, qd.Status)
	}
	normalized, err := qd.normalizeAnswers(answers)
	if err != nil {
		return nil, err
	}
	next := *qd
	next.Status = QuestionStatusAnswered
	next.Answers = normalized
	return &next, nil
}

// SubmitReject durably records that the user rejected the pending question
// while the continuation run is still pending.
func (qd *QuestionData) SubmitReject() (*QuestionData, error) {
	if qd.Status != QuestionStatusPending {
		return nil, fmt.Errorf("%w: cannot submit rejection from status %q", ErrInvalidQuestionTransition, qd.Status)
	}
	next := *qd
	next.Status = QuestionStatusRejectSubmitted
	return &next, nil
}

// Reject transitions from pending to rejected.
func (qd *QuestionData) Reject() (*QuestionData, error) {
	switch qd.Status {
	case QuestionStatusPending, QuestionStatusRejectSubmitted:
	default:
		return nil, fmt.Errorf("%w: cannot reject from status %q", ErrInvalidQuestionTransition, qd.Status)
	}
	next := *qd
	next.Status = QuestionStatusRejected
	return &next, nil
}

// IsPending returns true if the question is in pending state.
func (qd *QuestionData) IsPending() bool {
	return qd != nil && qd.Status == QuestionStatusPending
}
