package types

import "encoding/json"

// InterruptType identifies the type of interrupt.
type InterruptType string

const (
	// InterruptTypeAskUserQuestion indicates a HITL interrupt waiting for user answers.
	InterruptTypeAskUserQuestion InterruptType = "ask_user_question"
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
	Type      InterruptType     `json:"type"`               // "ask_user_question"
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
	value, _ := json.Marshal(s)
	return Answer{Value: value}
}

// NewMultiAnswer creates an Answer from a string slice (multi-select).
func NewMultiAnswer(ss []string) Answer {
	value, _ := json.Marshal(ss)
	return Answer{Value: value}
}
