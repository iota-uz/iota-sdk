package lens

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidator_Validate(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name         string
		config       *DashboardConfig
		expectValid  bool
		expectErrors []string
	}{
		{
			name: "valid_basic_config",
			config: &DashboardConfig{
				ID:   "test-dashboard",
				Name: "Test Dashboard",
				Grid: GridConfig{
					Columns:   12,
					RowHeight: 60,
				},
			},
			expectValid:  true,
			expectErrors: nil,
		},
		{
			name: "missing_id",
			config: &DashboardConfig{
				Name: "Test Dashboard",
			},
			expectValid:  false,
			expectErrors: []string{"dashboard ID is required"},
		},
		{
			name: "missing_name",
			config: &DashboardConfig{
				ID: "test-dashboard",
			},
			expectValid:  false,
			expectErrors: []string{"dashboard name is required"},
		},
		{
			name:         "missing_both_id_and_name",
			config:       &DashboardConfig{},
			expectValid:  false,
			expectErrors: []string{"dashboard ID is required", "dashboard name is required"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.config)

			assert.Equal(t, tt.expectValid, result.IsValid())

			if tt.expectErrors != nil {
				require.Len(t, result.Errors, len(tt.expectErrors))
				for i, expectedMsg := range tt.expectErrors {
					assert.Contains(t, result.Errors[i].Message, expectedMsg)
				}
			} else {
				assert.Empty(t, result.Errors)
			}
		})
	}
}

func TestValidator_ValidatePanel(t *testing.T) {
	validator := NewValidator()
	grid := GridConfig{Columns: 12, RowHeight: 60}

	tests := []struct {
		name         string
		panel        *PanelConfig
		expectValid  bool
		expectErrors []string
	}{
		{
			name: "valid_panel",
			panel: &PanelConfig{
				ID:         "test-panel",
				Title:      "Test Panel",
				Type:       ChartTypeLine,
				Position:   GridPosition{X: 0, Y: 0},
				Dimensions: GridDimensions{Width: 6, Height: 4},
			},
			expectValid:  true,
			expectErrors: nil,
		},
		{
			name: "missing_panel_id",
			panel: &PanelConfig{
				Title: "Test Panel",
				Type:  ChartTypeLine,
			},
			expectValid:  false,
			expectErrors: []string{"panel ID is required"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidatePanel(tt.panel, grid)

			assert.Equal(t, tt.expectValid, result.IsValid())

			if tt.expectErrors != nil {
				require.Len(t, result.Errors, len(tt.expectErrors))
				for i, expectedMsg := range tt.expectErrors {
					assert.Contains(t, result.Errors[i].Message, expectedMsg)
				}
			} else {
				assert.Empty(t, result.Errors)
			}
		})
	}
}

func TestValidator_ValidateGrid(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name         string
		panels       []PanelConfig
		grid         GridConfig
		expectValid  bool
		expectErrors []string
	}{
		{
			name: "valid_grid",
			panels: []PanelConfig{
				{
					ID:         "panel1",
					Position:   GridPosition{X: 0, Y: 0},
					Dimensions: GridDimensions{Width: 6, Height: 4},
				},
			},
			grid: GridConfig{
				Columns:   12,
				RowHeight: 60,
			},
			expectValid:  true,
			expectErrors: nil,
		},
		{
			name:   "invalid_grid_columns_zero",
			panels: []PanelConfig{},
			grid: GridConfig{
				Columns:   0,
				RowHeight: 60,
			},
			expectValid:  false,
			expectErrors: []string{"grid columns must be positive"},
		},
		{
			name:   "invalid_grid_columns_negative",
			panels: []PanelConfig{},
			grid: GridConfig{
				Columns:   -1,
				RowHeight: 60,
			},
			expectValid:  false,
			expectErrors: []string{"grid columns must be positive"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateGrid(tt.panels, tt.grid)

			assert.Equal(t, tt.expectValid, result.IsValid())

			if tt.expectErrors != nil {
				require.Len(t, result.Errors, len(tt.expectErrors))
				for i, expectedMsg := range tt.expectErrors {
					assert.Contains(t, result.Errors[i].Message, expectedMsg)
				}
			} else {
				assert.Empty(t, result.Errors)
			}
		})
	}
}

func TestValidationResult_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		result ValidationResult
		expect bool
	}{
		{
			name: "valid_with_no_errors",
			result: ValidationResult{
				Valid:  true,
				Errors: []ValidationError{},
			},
			expect: true,
		},
		{
			name: "invalid_flag_set",
			result: ValidationResult{
				Valid:  false,
				Errors: []ValidationError{},
			},
			expect: false,
		},
		{
			name: "has_errors",
			result: ValidationResult{
				Valid: true,
				Errors: []ValidationError{
					{Field: "test", Message: "error"},
				},
			},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, tt.result.IsValid())
		})
	}
}
