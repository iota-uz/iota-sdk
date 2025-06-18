package lens

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluator_Evaluate(t *testing.T) {
	evaluator := NewEvaluator()

	tests := []struct {
		name        string
		config      *DashboardConfig
		context     EvaluationContext
		expectError bool
		validate    func(t *testing.T, result *EvaluatedDashboard)
	}{
		{
			name: "basic_dashboard_evaluation",
			config: &DashboardConfig{
				ID:   "test-dashboard",
				Name: "Test Dashboard",
				Grid: GridConfig{
					Columns:   12,
					RowHeight: 60,
				},
				Panels: []PanelConfig{
					{
						ID:    "panel1",
						Title: "Test Panel",
						Type:  ChartTypeLine,
						Position: GridPosition{X: 0, Y: 0},
						Dimensions: GridDimensions{Width: 6, Height: 4},
						Query: "SELECT * FROM test",
						DataSource: DataSourceConfig{
							Type: "postgres",
							Ref:  "main",
						},
						Options: map[string]any{
							"color": "blue",
						},
					},
				},
				Variables: []Variable{
					{
						Name:    "timeRange",
						Type:    "timeRange",
						Default: "last7days",
					},
				},
			},
			context: EvaluationContext{
				TimeRange: TimeRange{
					Start: time.Now().Add(-7 * 24 * time.Hour),
					End:   time.Now(),
				},
				Variables: map[string]any{
					"department": "sales",
				},
				User: UserContext{
					ID:    "user1",
					Roles: []string{"admin"},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *EvaluatedDashboard) {
				assert.Equal(t, "test-dashboard", result.Config.ID)
				assert.Equal(t, "Test Dashboard", result.Config.Name)
				assert.Equal(t, 12, result.Layout.Grid.Columns)
				assert.Equal(t, "lg", result.Layout.Breakpoint)
				
				require.Len(t, result.Panels, 1)
				panel := result.Panels[0]
				assert.Equal(t, "panel1", panel.Config.ID)
				assert.Equal(t, "SELECT * FROM test", panel.ResolvedQuery)
				assert.Equal(t, "main", panel.DataSourceRef)
				assert.Equal(t, ChartTypeLine, panel.RenderConfig.ChartType)
				assert.Equal(t, 30, panel.RenderConfig.RefreshRate)
				assert.Contains(t, panel.RenderConfig.ChartOptions, "color")
				
				assert.Empty(t, result.Errors)
			},
		},
		{
			name: "empty_dashboard",
			config: &DashboardConfig{
				ID:   "empty-dashboard",
				Name: "Empty Dashboard",
				Grid: GridConfig{
					Columns:   12,
					RowHeight: 60,
				},
				Panels:    []PanelConfig{},
				Variables: []Variable{},
			},
			context: EvaluationContext{
				Variables: map[string]any{},
			},
			expectError: false,
			validate: func(t *testing.T, result *EvaluatedDashboard) {
				assert.Equal(t, "empty-dashboard", result.Config.ID)
				assert.Empty(t, result.Panels)
				assert.Empty(t, result.Errors)
			},
		},
		{
			name: "multiple_panels",
			config: &DashboardConfig{
				ID:   "multi-panel-dashboard",
				Name: "Multi Panel Dashboard",
				Grid: GridConfig{
					Columns:   12,
					RowHeight: 60,
				},
				Panels: []PanelConfig{
					{
						ID:    "panel1",
						Title: "Line Chart",
						Type:  ChartTypeLine,
						Position: GridPosition{X: 0, Y: 0},
						Dimensions: GridDimensions{Width: 6, Height: 4},
					},
					{
						ID:    "panel2",
						Title: "Bar Chart",
						Type:  ChartTypeBar,
						Position: GridPosition{X: 6, Y: 0},
						Dimensions: GridDimensions{Width: 6, Height: 4},
					},
				},
			},
			context: EvaluationContext{},
			expectError: false,
			validate: func(t *testing.T, result *EvaluatedDashboard) {
				require.Len(t, result.Panels, 2)
				
				panel1 := result.Panels[0]
				assert.Equal(t, "panel1", panel1.Config.ID)
				assert.Equal(t, ChartTypeLine, panel1.RenderConfig.ChartType)
				
				panel2 := result.Panels[1]
				assert.Equal(t, "panel2", panel2.Config.ID)
				assert.Equal(t, ChartTypeBar, panel2.RenderConfig.ChartType)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.config, tt.context)
			
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			
			require.NoError(t, err)
			require.NotNil(t, result)
			
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestEvaluationContext(t *testing.T) {
	t.Run("evaluation_context_creation", func(t *testing.T) {
		start := time.Now().Add(-24 * time.Hour)
		end := time.Now()
		
		ctx := EvaluationContext{
			TimeRange: TimeRange{
				Start: start,
				End:   end,
			},
			Variables: map[string]any{
				"department": "engineering",
				"threshold":  100,
			},
			User: UserContext{
				ID:    "user123",
				Roles: []string{"admin", "viewer"},
			},
		}
		
		assert.Equal(t, start, ctx.TimeRange.Start)
		assert.Equal(t, end, ctx.TimeRange.End)
		assert.Equal(t, "engineering", ctx.Variables["department"])
		assert.Equal(t, 100, ctx.Variables["threshold"])
		assert.Equal(t, "user123", ctx.User.ID)
		assert.Contains(t, ctx.User.Roles, "admin")
		assert.Contains(t, ctx.User.Roles, "viewer")
	})
}

func TestEvaluatedPanel(t *testing.T) {
	t.Run("evaluated_panel_structure", func(t *testing.T) {
		panel := EvaluatedPanel{
			Config: PanelConfig{
				ID:    "test-panel",
				Title: "Test Panel",
				Type:  ChartTypePie,
			},
			ResolvedQuery: "SELECT count(*) FROM users WHERE active = true",
			DataSourceRef: "primary_db",
			RenderConfig: RenderConfig{
				ChartType: ChartTypePie,
				ChartOptions: map[string]any{
					"legend": true,
					"colors": []string{"#FF6384", "#36A2EB"},
				},
				RefreshRate: 60,
			},
		}
		
		assert.Equal(t, "test-panel", panel.Config.ID)
		assert.Equal(t, ChartTypePie, panel.Config.Type)
		assert.Equal(t, "SELECT count(*) FROM users WHERE active = true", panel.ResolvedQuery)
		assert.Equal(t, "primary_db", panel.DataSourceRef)
		assert.Equal(t, ChartTypePie, panel.RenderConfig.ChartType)
		assert.Equal(t, 60, panel.RenderConfig.RefreshRate)
		assert.True(t, panel.RenderConfig.ChartOptions["legend"].(bool))
	})
}