package evaluation

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/lens"
)

func TestEvaluator_Evaluate(t *testing.T) {
	evaluator := NewEvaluator()

	tests := []struct {
		name        string
		config      *lens.DashboardConfig
		context     *EvaluationContext
		expectError bool
		validate    func(t *testing.T, result *EvaluatedDashboard)
	}{
		{
			name: "successful_evaluation_with_interpolation",
			config: &lens.DashboardConfig{
				ID:      "test-dashboard",
				Name:    "Test Dashboard",
				Version: "1.0.0",
				Grid: lens.GridConfig{
					Columns:   12,
					RowHeight: 60,
				},
				Panels: []lens.PanelConfig{
					{
						ID:         "panel1",
						Title:      "CPU Usage",
						Type:       lens.ChartTypeLine,
						Position:   lens.GridPosition{X: 0, Y: 0},
						Dimensions: lens.GridDimensions{Width: 6, Height: 4},
						Query:      "SELECT timestamp, cpu_percent FROM metrics WHERE host = :host AND time >= :timeRange.start",
						DataSource: lens.DataSourceConfig{
							Type: "postgres",
							Ref:  "main",
						},
					},
				},
			},
			context: &EvaluationContext{
				TimeRange: lens.TimeRange{
					Start: time.Now().Add(-1 * time.Hour),
					End:   time.Now(),
				},
				Variables: map[string]any{
					"host": "server1",
					"timeRange": lens.TimeRange{
						Start: time.Now().Add(-1 * time.Hour),
						End:   time.Now(),
					},
				},
				Options: EvaluationOptions{
					InterpolateVariables: true,
					CalculateLayout:      true,
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *EvaluatedDashboard) {
				t.Helper()
				assert.Equal(t, "test-dashboard", result.Config.ID)
				require.Len(t, result.Panels, 1)
				panel := result.Panels[0]
				assert.Contains(t, panel.ResolvedQuery, "server1")
				assert.Contains(t, panel.ResolvedQuery, "WHERE host = server1")
				assert.NotEmpty(t, result.Layout.Panels)
			},
		},
		{
			name: "nested_variable_access",
			config: &lens.DashboardConfig{
				ID:      "nested-vars-dashboard",
				Name:    "Nested Variables Dashboard",
				Version: "1.0.0",
				Grid: lens.GridConfig{
					Columns:   12,
					RowHeight: 60,
				},
				Panels: []lens.PanelConfig{
					{
						ID:         "panel1",
						Title:      "Nested Vars Panel",
						Type:       lens.ChartTypeLine,
						Position:   lens.GridPosition{X: 0, Y: 0},
						Dimensions: lens.GridDimensions{Width: 6, Height: 4},
						Query:      "SELECT * FROM logs WHERE level = :filters.level AND app = :filters.app",
						DataSource: lens.DataSourceConfig{
							Type: "postgres",
							Ref:  "main",
						},
					},
				},
			},
			context: &EvaluationContext{
				Variables: map[string]any{
					"filters": map[string]any{
						"level": "error",
						"app":   "web-server",
					},
				},
				Options: EvaluationOptions{
					InterpolateVariables: true,
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *EvaluatedDashboard) {
				t.Helper()
				require.Len(t, result.Panels, 1)
				panel := result.Panels[0]
				assert.Contains(t, panel.ResolvedQuery, "level = error")
				assert.Contains(t, panel.ResolvedQuery, "app = web-server")
			},
		},
		{
			name: "invalid_variable_reference",
			config: &lens.DashboardConfig{
				ID:      "invalid-vars-dashboard",
				Name:    "Invalid Variables Dashboard",
				Version: "1.0.0",
				Grid: lens.GridConfig{
					Columns:   12,
					RowHeight: 60,
				},
				Panels: []lens.PanelConfig{
					{
						ID:         "panel1",
						Title:      "Invalid Vars Panel",
						Type:       lens.ChartTypeLine,
						Position:   lens.GridPosition{X: 0, Y: 0},
						Dimensions: lens.GridDimensions{Width: 6, Height: 4},
						Query:      "SELECT * FROM logs WHERE level = :nonexistent",
						DataSource: lens.DataSourceConfig{
							Type: "postgres",
							Ref:  "main",
						},
					},
				},
			},
			context: &EvaluationContext{
				Variables: map[string]any{
					"existing": "value",
				},
				Options: EvaluationOptions{
					InterpolateVariables: true,
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *EvaluatedDashboard) {
				t.Helper()
				require.Len(t, result.Panels, 1)
				panel := result.Panels[0]
				assert.NotEmpty(t, panel.Errors)
				assert.Equal(t, "SELECT * FROM logs WHERE level = :nonexistent", panel.ResolvedQuery)
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

func TestQueryProcessor_InterpolateQuery(t *testing.T) {
	processor := NewQueryProcessor()

	tests := []struct {
		name        string
		query       string
		context     *EvaluationContext
		expected    string
		expectError bool
	}{
		{
			name:  "simple_variable_interpolation",
			query: "SELECT * FROM users WHERE id = :userId",
			context: &EvaluationContext{
				Variables: map[string]any{
					"userId": 123,
				},
			},
			expected:    "SELECT * FROM users WHERE id = 123",
			expectError: false,
		},
		{
			name:  "multiple_variables",
			query: "SELECT * FROM logs WHERE level = :level AND app = :app",
			context: &EvaluationContext{
				Variables: map[string]any{
					"level": "error",
					"app":   "web-server",
				},
			},
			expected:    "SELECT * FROM logs WHERE level = error AND app = web-server",
			expectError: false,
		},
		{
			name:  "nested_property_access",
			query: "SELECT * FROM metrics WHERE timestamp >= :timeRange.start AND timestamp <= :timeRange.end",
			context: &EvaluationContext{
				Variables: map[string]any{
					"timeRange": lens.TimeRange{
						Start: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
						End:   time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
					},
				},
			},
			expected: "SELECT * FROM metrics WHERE timestamp >= 2023-01-01T00:00:00Z AND timestamp <= 2023-01-02T00:00:00Z",
		},
		{
			name:  "map_property_access",
			query: "SELECT * FROM events WHERE type = :filters.type AND status = :filters.status",
			context: &EvaluationContext{
				Variables: map[string]any{
					"filters": map[string]any{
						"type":   "click",
						"status": "processed",
					},
				},
			},
			expected: "SELECT * FROM events WHERE type = click AND status = processed",
		},
		{
			name:  "nonexistent_variable",
			query: "SELECT * FROM users WHERE id = :nonExistent",
			context: &EvaluationContext{
				Variables: map[string]any{
					"userId": 123,
				},
			},
			expectError: true,
		},
		{
			name:  "invalid_nested_property",
			query: "SELECT * FROM metrics WHERE time = :timeRange.invalid",
			context: &EvaluationContext{
				Variables: map[string]any{
					"timeRange": lens.TimeRange{
						Start: time.Now(),
						End:   time.Now(),
					},
				},
			},
			expectError: true,
		},
		{
			name:  "property_access_on_non_object",
			query: "SELECT * FROM users WHERE id = :userId.property",
			context: &EvaluationContext{
				Variables: map[string]any{
					"userId": 123,
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.InterpolateQuery(tt.query, tt.context)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQueryProcessor_ValidateQuery(t *testing.T) {
	processor := NewQueryProcessor()

	tests := []struct {
		name           string
		query          string
		dataSourceType string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "safe_select_query",
			query:          "SELECT * FROM users WHERE id = 123",
			dataSourceType: "postgres",
			expectError:    false,
		},
		{
			name:           "safe_select_with_joins",
			query:          "SELECT u.name, p.title FROM users u JOIN posts p ON u.id = p.user_id",
			dataSourceType: "postgres",
			expectError:    false,
		},
		{
			name:           "dangerous_drop_table",
			query:          "DROP TABLE users",
			dataSourceType: "postgres",
			expectError:    true,
			errorContains:  "drop table",
		},
		{
			name:           "dangerous_delete",
			query:          "DELETE FROM users WHERE id = 1",
			dataSourceType: "postgres",
			expectError:    true,
			errorContains:  "delete from",
		},
		{
			name:           "dangerous_truncate",
			query:          "TRUNCATE TABLE users",
			dataSourceType: "postgres",
			expectError:    true,
			errorContains:  "truncate",
		},
		{
			name:           "dangerous_alter",
			query:          "ALTER TABLE users ADD COLUMN email VARCHAR(255)",
			dataSourceType: "postgres",
			expectError:    true,
			errorContains:  "alter table",
		},
		{
			name:           "dangerous_create_table",
			query:          "CREATE TABLE temp AS SELECT * FROM users",
			dataSourceType: "postgres",
			expectError:    true,
			errorContains:  "create table",
		},
		{
			name:           "dangerous_insert",
			query:          "INSERT INTO users (name) VALUES ('test')",
			dataSourceType: "postgres",
			expectError:    true,
			errorContains:  "insert into",
		},
		{
			name:           "dangerous_update",
			query:          "UPDATE users SET name = 'test' WHERE id = 1",
			dataSourceType: "postgres",
			expectError:    true,
			errorContains:  "update",
		},
		{
			name:           "dangerous_sql_comment",
			query:          "SELECT * FROM users -- DROP TABLE users",
			dataSourceType: "postgres",
			expectError:    true,
			errorContains:  "--",
		},
		{
			name:           "dangerous_semicolon",
			query:          "SELECT * FROM users; DROP TABLE users",
			dataSourceType: "postgres",
			expectError:    true,
			errorContains:  ";",
		},
		{
			name:           "case_insensitive_detection",
			query:          "Select * from users where id = 1; DROP TABLE users",
			dataSourceType: "postgres",
			expectError:    true,
			errorContains:  ";",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.ValidateQuery(tt.query, tt.dataSourceType)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestLayoutEngine_CalculateLayout(t *testing.T) {
	engine := NewLayoutEngine()

	tests := []struct {
		name        string
		panels      []lens.PanelConfig
		grid        lens.GridConfig
		expectError bool
		validate    func(t *testing.T, layout *Layout)
	}{
		{
			name: "simple_grid_layout",
			panels: []lens.PanelConfig{
				{
					ID:         "panel1",
					Title:      "Panel 1",
					Position:   lens.GridPosition{X: 0, Y: 0},
					Dimensions: lens.GridDimensions{Width: 6, Height: 4},
				},
				{
					ID:         "panel2",
					Title:      "Panel 2",
					Position:   lens.GridPosition{X: 6, Y: 0},
					Dimensions: lens.GridDimensions{Width: 6, Height: 4},
				},
			},
			grid: lens.GridConfig{
				Columns:   12,
				RowHeight: 60,
			},
			expectError: false,
			validate: func(t *testing.T, layout *Layout) {
				t.Helper()
				assert.Equal(t, 12, layout.Grid.Columns)
				assert.Equal(t, 60, layout.Grid.RowHeight)
				assert.Equal(t, BreakpointLG, layout.Breakpoint)
				assert.Len(t, layout.Panels, 2)

				panel1Layout := layout.Panels[0]
				assert.Equal(t, "panel1", panel1Layout.PanelID)
				assert.Equal(t, "1 / 1 / 5 / 7", panel1Layout.CSS.GridArea)

				panel2Layout := layout.Panels[1]
				assert.Equal(t, "panel2", panel2Layout.PanelID)
				assert.Equal(t, "1 / 7 / 5 / 13", panel2Layout.CSS.GridArea)
			},
		},
		{
			name: "complex_grid_layout",
			panels: []lens.PanelConfig{
				{
					ID:         "header",
					Title:      "Header Panel",
					Position:   lens.GridPosition{X: 0, Y: 0},
					Dimensions: lens.GridDimensions{Width: 12, Height: 2},
				},
				{
					ID:         "sidebar",
					Title:      "Sidebar Panel",
					Position:   lens.GridPosition{X: 0, Y: 2},
					Dimensions: lens.GridDimensions{Width: 3, Height: 8},
				},
				{
					ID:         "main",
					Title:      "Main Panel",
					Position:   lens.GridPosition{X: 3, Y: 2},
					Dimensions: lens.GridDimensions{Width: 9, Height: 8},
				},
			},
			grid: lens.GridConfig{
				Columns:   12,
				RowHeight: 60,
			},
			expectError: false,
			validate: func(t *testing.T, layout *Layout) {
				t.Helper()
				assert.Len(t, layout.Panels, 3)

				headerLayout := layout.Panels[0]
				assert.Equal(t, "header", headerLayout.PanelID)
				assert.Equal(t, "1 / 1 / 3 / 13", headerLayout.CSS.GridArea)

				sidebarLayout := layout.Panels[1]
				assert.Equal(t, "sidebar", sidebarLayout.PanelID)
				assert.Equal(t, "3 / 1 / 11 / 4", sidebarLayout.CSS.GridArea)

				mainLayout := layout.Panels[2]
				assert.Equal(t, "main", mainLayout.PanelID)
				assert.Equal(t, "3 / 4 / 11 / 13", mainLayout.CSS.GridArea)
			},
		},
		{
			name:   "empty_panels",
			panels: []lens.PanelConfig{},
			grid: lens.GridConfig{
				Columns:   12,
				RowHeight: 60,
			},
			expectError: false,
			validate: func(t *testing.T, layout *Layout) {
				t.Helper()
				assert.Empty(t, layout.Panels)
				assert.Equal(t, 12, layout.Grid.Columns)
				assert.Equal(t, 60, layout.Grid.RowHeight)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout, err := engine.CalculateLayout(tt.panels, tt.grid)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, layout)

			if tt.validate != nil {
				tt.validate(t, layout)
			}
		})
	}
}

func TestRenderMapper_MapToRenderConfig(t *testing.T) {
	mapper := NewRenderMapper()

	tests := []struct {
		name        string
		panel       *lens.PanelConfig
		context     *EvaluationContext
		expectError bool
		validate    func(t *testing.T, config *RenderConfig)
	}{
		{
			name: "line_chart_config",
			panel: &lens.PanelConfig{
				ID:         "line-chart",
				Title:      "Line Chart",
				Type:       lens.ChartTypeLine,
				Position:   lens.GridPosition{X: 0, Y: 0},
				Dimensions: lens.GridDimensions{Width: 6, Height: 4},
				Options: map[string]any{
					"color": "blue",
				},
			},
			context: &EvaluationContext{},
			validate: func(t *testing.T, config *RenderConfig) {
				t.Helper()
				assert.Equal(t, lens.ChartTypeLine, config.ChartType)
				assert.Equal(t, "/api/panels/line-chart/data", config.DataEndpoint)
				assert.Equal(t, 30*time.Second, config.RefreshRate)
				assert.Contains(t, config.ChartOptions, "stroke")
				assert.Contains(t, config.ChartOptions, "color")
				assert.Equal(t, "1 / 1 / 5 / 7", config.GridCSS.GridArea)
				assert.Equal(t, "every 30s", config.HTMXConfig.Trigger)
				assert.Equal(t, "#panel-line-chart", config.HTMXConfig.Target)
			},
		},
		{
			name: "bar_chart_config",
			panel: &lens.PanelConfig{
				ID:         "bar-chart",
				Title:      "Bar Chart",
				Type:       lens.ChartTypeBar,
				Position:   lens.GridPosition{X: 6, Y: 0},
				Dimensions: lens.GridDimensions{Width: 6, Height: 4},
			},
			context: &EvaluationContext{},
			validate: func(t *testing.T, config *RenderConfig) {
				t.Helper()
				assert.Equal(t, lens.ChartTypeBar, config.ChartType)
				assert.Contains(t, config.ChartOptions, "plotOptions")
				plotOptions := config.ChartOptions["plotOptions"].(map[string]any)
				assert.Contains(t, plotOptions, "bar")
			},
		},
		{
			name: "pie_chart_config",
			panel: &lens.PanelConfig{
				ID:         "pie-chart",
				Title:      "Pie Chart",
				Type:       lens.ChartTypePie,
				Position:   lens.GridPosition{X: 0, Y: 4},
				Dimensions: lens.GridDimensions{Width: 12, Height: 6},
			},
			context: &EvaluationContext{},
			validate: func(t *testing.T, config *RenderConfig) {
				t.Helper()
				assert.Equal(t, lens.ChartTypePie, config.ChartType)
				assert.Contains(t, config.ChartOptions, "legend")
				legend := config.ChartOptions["legend"].(map[string]any)
				assert.Equal(t, "bottom", legend["position"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := mapper.MapToRenderConfig(tt.panel, tt.context)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, config)

			if tt.validate != nil {
				tt.validate(t, config)
			}
		})
	}
}

func BenchmarkQueryProcessor_InterpolateQuery(b *testing.B) {
	processor := NewQueryProcessor()
	query := "SELECT * FROM metrics WHERE host = :host AND time >= :timeRange.start AND time <= :timeRange.end AND app = :filters.app"

	ctx := &EvaluationContext{
		Variables: map[string]any{
			"host": "server1",
			"timeRange": lens.TimeRange{
				Start: time.Now().Add(-1 * time.Hour),
				End:   time.Now(),
			},
			"filters": map[string]any{
				"app": "web-server",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := processor.InterpolateQuery(query, ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLayoutEngine_CalculateLayout(b *testing.B) {
	engine := NewLayoutEngine()

	panels := make([]lens.PanelConfig, 20)
	for i := 0; i < 20; i++ {
		panels[i] = lens.PanelConfig{
			ID:    fmt.Sprintf("panel-%d", i),
			Title: fmt.Sprintf("Panel %d", i),
			Position: lens.GridPosition{
				X: i % 12,
				Y: i / 12,
			},
			Dimensions: lens.GridDimensions{
				Width:  3,
				Height: 4,
			},
		}
	}

	grid := lens.GridConfig{
		Columns:   12,
		RowHeight: 60,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.CalculateLayout(panels, grid)
		if err != nil {
			b.Fatal(err)
		}
	}
}
