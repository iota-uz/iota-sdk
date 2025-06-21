package lens

import "fmt"

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// EvaluationError represents an evaluation error
type EvaluationError struct {
	PanelID string
	Message string
	Cause   error
}

func (e EvaluationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("panel %s: %s: %v", e.PanelID, e.Message, e.Cause)
	}
	return fmt.Sprintf("panel %s: %s", e.PanelID, e.Message)
}

// GridError represents a grid layout error
type GridError struct {
	Message string
	Panels  []string
}

func (e GridError) Error() string {
	return fmt.Sprintf("grid error: %s (panels: %v)", e.Message, e.Panels)
}
