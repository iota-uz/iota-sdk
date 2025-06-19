package lens

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLayoutEngine_CalculateLayout(t *testing.T) {
	engine := NewLayoutEngine()

	tests := []struct {
		name     string
		panels   []PanelConfig
		grid     GridConfig
		validate func(t *testing.T, layout Layout, err error)
	}{
		{
			name: "single_panel_layout",
			panels: []PanelConfig{
				{
					ID:         "panel1",
					Title:      "Test Panel",
					Type:       ChartTypeLine,
					Position:   GridPosition{X: 0, Y: 0},
					Dimensions: GridDimensions{Width: 6, Height: 4},
				},
			},
			grid: GridConfig{
				Columns:   12,
				RowHeight: 60,
			},
			validate: func(t *testing.T, layout Layout, err error) {
				require.NoError(t, err)
				assert.Equal(t, 12, layout.Grid.Columns)
				assert.Equal(t, 60, layout.Grid.RowHeight)
				assert.Equal(t, "lg", layout.Breakpoint)

				require.Len(t, layout.Panels, 1)
				panel := layout.Panels[0]
				assert.Equal(t, "panel1", panel.PanelID)
				assert.Equal(t, GridPosition{X: 0, Y: 0}, panel.Position)
				assert.Equal(t, GridDimensions{Width: 6, Height: 4}, panel.Dimensions)
				// CSS testing is now handled by the UI package
			},
		},
		{
			name: "multiple_panels_layout",
			panels: []PanelConfig{
				{
					ID:         "panel1",
					Type:       ChartTypeLine,
					Position:   GridPosition{X: 0, Y: 0},
					Dimensions: GridDimensions{Width: 6, Height: 4},
				},
				{
					ID:         "panel2",
					Type:       ChartTypeBar,
					Position:   GridPosition{X: 6, Y: 0},
					Dimensions: GridDimensions{Width: 6, Height: 4},
				},
				{
					ID:         "panel3",
					Type:       ChartTypePie,
					Position:   GridPosition{X: 0, Y: 4},
					Dimensions: GridDimensions{Width: 12, Height: 6},
				},
			},
			grid: GridConfig{
				Columns:   12,
				RowHeight: 60,
			},
			validate: func(t *testing.T, layout Layout, err error) {
				require.NoError(t, err)
				require.Len(t, layout.Panels, 3)

				// Check each panel has correct layout
				panelMap := make(map[string]PanelLayout)
				for _, panel := range layout.Panels {
					panelMap[panel.PanelID] = panel
				}

				panel1 := panelMap["panel1"]
				assert.Equal(t, GridPosition{X: 0, Y: 0}, panel1.Position)
				assert.Equal(t, GridDimensions{Width: 6, Height: 4}, panel1.Dimensions)

				panel2 := panelMap["panel2"]
				assert.Equal(t, GridPosition{X: 6, Y: 0}, panel2.Position)
				assert.Equal(t, GridDimensions{Width: 6, Height: 4}, panel2.Dimensions)

				panel3 := panelMap["panel3"]
				assert.Equal(t, GridPosition{X: 0, Y: 4}, panel3.Position)
				assert.Equal(t, GridDimensions{Width: 12, Height: 6}, panel3.Dimensions)
				// CSS testing is now handled by the UI package
			},
		},
		{
			name:   "empty_panels",
			panels: []PanelConfig{},
			grid: GridConfig{
				Columns:   12,
				RowHeight: 60,
			},
			validate: func(t *testing.T, layout Layout, err error) {
				require.NoError(t, err)
				assert.Empty(t, layout.Panels)
				assert.Equal(t, "lg", layout.Breakpoint)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout, err := engine.CalculateLayout(tt.panels, tt.grid)
			tt.validate(t, layout, err)
		})
	}
}

func TestLayoutEngine_DetectOverlaps(t *testing.T) {
	engine := NewLayoutEngine()

	tests := []struct {
		name         string
		panels       []PanelConfig
		expectErrors int
		validate     func(t *testing.T, errors []GridError)
	}{
		{
			name: "no_overlaps",
			panels: []PanelConfig{
				{
					ID:         "panel1",
					Position:   GridPosition{X: 0, Y: 0},
					Dimensions: GridDimensions{Width: 6, Height: 4},
				},
				{
					ID:         "panel2",
					Position:   GridPosition{X: 6, Y: 0},
					Dimensions: GridDimensions{Width: 6, Height: 4},
				},
			},
			expectErrors: 0,
			validate: func(t *testing.T, errors []GridError) {
				assert.Empty(t, errors)
			},
		},
		{
			name: "exact_position_overlap",
			panels: []PanelConfig{
				{
					ID:         "panel1",
					Position:   GridPosition{X: 0, Y: 0},
					Dimensions: GridDimensions{Width: 6, Height: 4},
				},
				{
					ID:         "panel2",
					Position:   GridPosition{X: 0, Y: 0},
					Dimensions: GridDimensions{Width: 4, Height: 3},
				},
			},
			expectErrors: 2, // Both exact position match AND area overlap are detected
			validate: func(t *testing.T, errors []GridError) {
				require.Len(t, errors, 2)
				// Should detect both exact position overlap and area overlap
				assert.Contains(t, errors[0].Message, "panels overlap")
				assert.Contains(t, errors[0].Panels, "panel1")
				assert.Contains(t, errors[0].Panels, "panel2")
				assert.Contains(t, errors[1].Message, "panels overlap")
				assert.Contains(t, errors[1].Panels, "panel1")
				assert.Contains(t, errors[1].Panels, "panel2")
			},
		},
		{
			name: "multiple_overlaps",
			panels: []PanelConfig{
				{
					ID:       "panel1",
					Position: GridPosition{X: 0, Y: 0},
				},
				{
					ID:       "panel2",
					Position: GridPosition{X: 0, Y: 0},
				},
				{
					ID:       "panel3",
					Position: GridPosition{X: 0, Y: 0},
				},
			},
			expectErrors: 3, // panel1-panel2, panel1-panel3, panel2-panel3
			validate: func(t *testing.T, errors []GridError) {
				assert.Len(t, errors, 3)
				for _, err := range errors {
					assert.Contains(t, err.Message, "panels overlap")
					assert.Len(t, err.Panels, 2)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := engine.DetectOverlaps(tt.panels)
			assert.Len(t, errors, tt.expectErrors)
			if tt.validate != nil {
				tt.validate(t, errors)
			}
		})
	}
}

func TestLayoutEngine_GetResponsiveLayout(t *testing.T) {
	engine := NewLayoutEngine()

	originalLayout := Layout{
		Grid: GridConfig{
			Columns:   12,
			RowHeight: 60,
		},
		Panels: []PanelLayout{
			{
				PanelID:    "panel1",
				Position:   GridPosition{X: 0, Y: 0},
				Dimensions: GridDimensions{Width: 6, Height: 4},
			},
		},
		Breakpoint: "lg",
	}

	tests := []struct {
		name       string
		layout     Layout
		breakpoint string
		validate   func(t *testing.T, result Layout)
	}{
		{
			name:       "change_to_mobile",
			layout:     originalLayout,
			breakpoint: "xs",
			validate: func(t *testing.T, result Layout) {
				assert.Equal(t, "xs", result.Breakpoint)
				assert.Equal(t, originalLayout.Grid, result.Grid)
				assert.Len(t, result.Panels, len(originalLayout.Panels))
			},
		},
		{
			name:       "change_to_tablet",
			layout:     originalLayout,
			breakpoint: "md",
			validate: func(t *testing.T, result Layout) {
				assert.Equal(t, "md", result.Breakpoint)
				assert.Equal(t, originalLayout.Grid, result.Grid)
			},
		},
		{
			name:       "keep_desktop",
			layout:     originalLayout,
			breakpoint: "lg",
			validate: func(t *testing.T, result Layout) {
				assert.Equal(t, "lg", result.Breakpoint)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.GetResponsiveLayout(tt.layout, tt.breakpoint)
			tt.validate(t, result)
		})
	}
}

func TestPanelLayout(t *testing.T) {
	t.Run("panel_layout_structure", func(t *testing.T) {
		layout := PanelLayout{
			PanelID:    "test-panel",
			Position:   GridPosition{X: 2, Y: 1},
			Dimensions: GridDimensions{Width: 8, Height: 6},
		}

		assert.Equal(t, "test-panel", layout.PanelID)
		assert.Equal(t, 2, layout.Position.X)
		assert.Equal(t, 1, layout.Position.Y)
		assert.Equal(t, 8, layout.Dimensions.Width)
		assert.Equal(t, 6, layout.Dimensions.Height)
		// CSS testing is now handled by the UI package
	})
}
