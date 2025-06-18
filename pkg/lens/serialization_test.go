package lens

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboardConfig_MarshalJSON(t *testing.T) {
	config := &DashboardConfig{
		ID:          "test-dashboard",
		Name:        "Test Dashboard",
		Description: "A test dashboard",
		Version:     "1.0",
		Grid: GridConfig{
			Columns:   12,
			RowHeight: 60,
		},
		Panels: []PanelConfig{
			{
				ID:    "panel-1",
				Title: "Test Panel",
				Type:  ChartTypeLine,
			},
		},
		Variables: []Variable{
			{
				Name: "testVar",
				Type: "string",
			},
		},
	}

	data, err := config.MarshalJSON()
	require.NoError(t, err)
	assert.Contains(t, string(data), "test-dashboard")
	assert.Contains(t, string(data), "Test Dashboard")
}

func TestDashboardConfig_UnmarshalJSON(t *testing.T) {
	jsonData := `{
		"id": "test-dashboard",
		"name": "Test Dashboard",
		"description": "A test dashboard",
		"version": "1.0",
		"grid": {
			"columns": 12,
			"rowHeight": 60
		},
		"panels": [
			{
				"id": "panel-1",
				"title": "Test Panel",
				"type": "line"
			}
		],
		"variables": [
			{
				"name": "testVar",
				"type": "string"
			}
		]
	}`

	var config DashboardConfig
	err := config.UnmarshalJSON([]byte(jsonData))
	require.NoError(t, err)

	assert.Equal(t, "test-dashboard", config.ID)
	assert.Equal(t, "Test Dashboard", config.Name)
	assert.Equal(t, 12, config.Grid.Columns)
	assert.Len(t, config.Panels, 1)
	assert.Equal(t, "panel-1", config.Panels[0].ID)
	assert.Len(t, config.Variables, 1)
	assert.Equal(t, "testVar", config.Variables[0].Name)
}

func TestDashboardConfig_ToJSON(t *testing.T) {
	config := &DashboardConfig{
		ID:   "test-dashboard",
		Name: "Test Dashboard",
		Grid: GridConfig{
			Columns:   12,
			RowHeight: 60,
		},
	}

	jsonStr, err := config.ToJSON()
	require.NoError(t, err)
	assert.Contains(t, jsonStr, "test-dashboard")
	assert.Contains(t, jsonStr, "Test Dashboard")

	// Verify it's valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err)
}

func TestFromJSON_WithValidation(t *testing.T) {
	tests := []struct {
		name        string
		jsonStr     string
		expectError bool
	}{
		{
			name: "valid config",
			jsonStr: `{
				"id": "test-dashboard",
				"name": "Test Dashboard",
				"grid": {"columns": 12, "rowHeight": 60},
				"panels": [],
				"variables": []
			}`,
			expectError: false,
		},
		{
			name: "missing id",
			jsonStr: `{
				"name": "Test Dashboard",
				"grid": {"columns": 12, "rowHeight": 60},
				"panels": [],
				"variables": []
			}`,
			expectError: true,
		},
		{
			name: "missing name",
			jsonStr: `{
				"id": "test-dashboard",
				"grid": {"columns": 12, "rowHeight": 60},
				"panels": [],
				"variables": []
			}`,
			expectError: true,
		},
		{
			name:        "invalid json",
			jsonStr:     `{"invalid": json}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := FromJSON(tt.jsonStr)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
			}
		})
	}
}

func TestFromJSONUnsafe(t *testing.T) {
	// Should not validate, so missing required fields should still parse
	jsonStr := `{
		"description": "Test without required fields"
	}`

	config, err := FromJSONUnsafe(jsonStr)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "", config.ID)
	assert.Equal(t, "", config.Name)
	assert.Equal(t, "Test without required fields", config.Description)
}

func TestFromJSONBytes_WithValidation(t *testing.T) {
	validJSON := []byte(`{
		"id": "test-dashboard",
		"name": "Test Dashboard",
		"grid": {"columns": 12, "rowHeight": 60},
		"panels": [],
		"variables": []
	}`)

	config, err := FromJSONBytes(validJSON)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "test-dashboard", config.ID)
	assert.Equal(t, "Test Dashboard", config.Name)
}

func TestFromJSONBytesUnsafe(t *testing.T) {
	invalidJSON := []byte(`{
		"description": "Test without required fields"
	}`)

	config, err := FromJSONBytesUnsafe(invalidJSON)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "", config.ID)
	assert.Equal(t, "", config.Name)
}

func TestRoundTripSerialization(t *testing.T) {
	original := &DashboardConfig{
		ID:          "test-dashboard",
		Name:        "Test Dashboard",
		Description: "A test dashboard",
		Version:     "1.0",
		Grid: GridConfig{
			Columns:   12,
			RowHeight: 60,
			Breakpoints: map[string]int{
				"lg": 1200,
				"md": 996,
			},
		},
		Panels: []PanelConfig{
			{
				ID:    "panel-1",
				Title: "Test Panel",
				Type:  ChartTypeLine,
				Position: GridPosition{
					X: 0,
					Y: 0,
				},
				Dimensions: GridDimensions{
					Width:  6,
					Height: 4,
				},
				DataSource: DataSourceConfig{
					Type: "postgres",
					Ref:  "main",
				},
				Query: "SELECT * FROM test",
				Options: map[string]any{
					"color": "blue",
				},
			},
		},
		Variables: []Variable{
			{
				Name: "testVar",
				Type: "string",
			},
		},
	}

	// Serialize to JSON
	jsonStr, err := original.ToJSON()
	require.NoError(t, err)

	// Deserialize back
	restored, err := FromJSON(jsonStr)
	require.NoError(t, err)

	// Compare
	assert.Equal(t, original.ID, restored.ID)
	assert.Equal(t, original.Name, restored.Name)
	assert.Equal(t, original.Description, restored.Description)
	assert.Equal(t, original.Version, restored.Version)
	assert.Equal(t, original.Grid.Columns, restored.Grid.Columns)
	assert.Equal(t, original.Grid.RowHeight, restored.Grid.RowHeight)
	assert.Equal(t, len(original.Panels), len(restored.Panels))
	assert.Equal(t, original.Panels[0].ID, restored.Panels[0].ID)
	assert.Equal(t, original.Panels[0].Title, restored.Panels[0].Title)
	assert.Equal(t, original.Panels[0].Type, restored.Panels[0].Type)
}