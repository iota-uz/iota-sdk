package lens

// Validator validates dashboard configurations
type Validator interface {
	Validate(config *DashboardConfig) ValidationResult
	ValidatePanel(panel *PanelConfig, grid GridConfig) ValidationResult
	ValidateGrid(panels []PanelConfig, grid GridConfig) ValidationResult
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

// IsValid returns true if there are no validation errors
func (r ValidationResult) IsValid() bool {
	return r.Valid && len(r.Errors) == 0
}

// validator is the default implementation
type validator struct{}

// NewValidator creates a new validator
func NewValidator() Validator {
	return &validator{}
}

// Validate validates the entire dashboard configuration
func (v *validator) Validate(config *DashboardConfig) ValidationResult {
	result := ValidationResult{Valid: true}

	// Minimal implementation - just check required fields
	if config.ID == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "id",
			Message: "dashboard ID is required",
		})
	}

	if config.Name == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "name",
			Message: "dashboard name is required",
		})
	}

	return result
}

// ValidatePanel validates a single panel configuration
func (v *validator) ValidatePanel(panel *PanelConfig, grid GridConfig) ValidationResult {
	result := ValidationResult{Valid: true}

	// Minimal implementation
	if panel.ID == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "id",
			Message: "panel ID is required",
		})
	}

	return result
}

// ValidateGrid validates the grid layout
func (v *validator) ValidateGrid(panels []PanelConfig, grid GridConfig) ValidationResult {
	result := ValidationResult{Valid: true}

	// Minimal implementation
	if grid.Columns <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "grid.columns",
			Message: "grid columns must be positive",
		})
	}

	return result
}
