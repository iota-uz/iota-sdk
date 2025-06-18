package validation

import "github.com/iota-uz/iota-sdk/pkg/lens"

// Validator validates dashboard configurations
type Validator interface {
	Validate(config *lens.DashboardConfig) ValidationResult
	ValidatePanel(panel *lens.PanelConfig, grid lens.GridConfig) ValidationResult
	ValidateGrid(panels []lens.PanelConfig, grid lens.GridConfig) ValidationResult
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
type validator struct {
	rules []ValidationRule
}

// NewValidator creates a new validator
func NewValidator() Validator {
	return &validator{
		rules: []ValidationRule{
			&DashboardValidationRule{},
			&PanelValidationRule{},
			&GridValidationRule{},
		},
	}
}

// Validate validates the entire dashboard configuration
func (v *validator) Validate(config *lens.DashboardConfig) ValidationResult {
	result := ValidationResult{Valid: true}
	
	for _, rule := range v.rules {
		if dashboardRule, ok := rule.(*DashboardValidationRule); ok {
			ruleResult := dashboardRule.ValidateDashboard(config)
			if !ruleResult.IsValid() {
				result.Valid = false
				result.Errors = append(result.Errors, ruleResult.Errors...)
			}
		}
	}
	
	// Validate all panels
	for _, panel := range config.Panels {
		panelResult := v.ValidatePanel(&panel, config.Grid)
		if !panelResult.IsValid() {
			result.Valid = false
			result.Errors = append(result.Errors, panelResult.Errors...)
		}
	}
	
	// Validate grid layout
	gridResult := v.ValidateGrid(config.Panels, config.Grid)
	if !gridResult.IsValid() {
		result.Valid = false
		result.Errors = append(result.Errors, gridResult.Errors...)
	}
	
	return result
}

// ValidatePanel validates a single panel configuration
func (v *validator) ValidatePanel(panel *lens.PanelConfig, grid lens.GridConfig) ValidationResult {
	result := ValidationResult{Valid: true}
	
	for _, rule := range v.rules {
		if panelRule, ok := rule.(*PanelValidationRule); ok {
			ruleResult := panelRule.ValidatePanel(panel, grid)
			if !ruleResult.IsValid() {
				result.Valid = false
				result.Errors = append(result.Errors, ruleResult.Errors...)
			}
		}
	}
	
	return result
}

// ValidateGrid validates the grid layout
func (v *validator) ValidateGrid(panels []lens.PanelConfig, grid lens.GridConfig) ValidationResult {
	result := ValidationResult{Valid: true}
	
	for _, rule := range v.rules {
		if gridRule, ok := rule.(*GridValidationRule); ok {
			ruleResult := gridRule.ValidateGrid(panels, grid)
			if !ruleResult.IsValid() {
				result.Valid = false
				result.Errors = append(result.Errors, ruleResult.Errors...)
			}
		}
	}
	
	return result
}