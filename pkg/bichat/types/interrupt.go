package types

import (
	"encoding/json"
	"errors"
	"fmt"
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
	QuestionStatusPending  QuestionStatus = "PENDING"
	QuestionStatusAnswered QuestionStatus = "ANSWERED"
	QuestionStatusRejected QuestionStatus = "REJECTED"
)

var (
	ErrInvalidQuestionTransition = errors.New("invalid question status transition")
	ErrQuestionDataInvalid       = errors.New("invalid question data")
)

// QuestionData holds the HITL question state stored in a message's question_data JSONB column.
// It is the single source of truth for whether a question is pending, answered, or rejected.
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
	if checkpointID == "" {
		return nil, fmt.Errorf("%w: checkpointID required", ErrQuestionDataInvalid)
	}
	if len(questions) == 0 {
		return nil, fmt.Errorf("%w: at least one question required", ErrQuestionDataInvalid)
	}
	for _, q := range questions {
		if len(q.Options) < 2 {
			return nil, fmt.Errorf("%w: question %q must have at least 2 options", ErrQuestionDataInvalid, q.ID)
		}
	}
	return &QuestionData{
		CheckpointID: checkpointID,
		Status:       QuestionStatusPending,
		AgentName:    agentName,
		Questions:    questions,
	}, nil
}

// Answer transitions from pending to answered.
func (qd *QuestionData) Answer(answers map[string]string) (*QuestionData, error) {
	if qd.Status != QuestionStatusPending {
		return nil, fmt.Errorf("%w: cannot answer from status %q", ErrInvalidQuestionTransition, qd.Status)
	}
	next := *qd
	next.Status = QuestionStatusAnswered
	next.Answers = answers
	return &next, nil
}

// Reject transitions from pending to rejected.
func (qd *QuestionData) Reject() (*QuestionData, error) {
	if qd.Status != QuestionStatusPending {
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
