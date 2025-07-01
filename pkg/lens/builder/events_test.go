package builder

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPanelBuilder_OnClick(t *testing.T) {
	action := lens.ActionConfig{
		Type: lens.ActionTypeNavigation,
		Navigation: &lens.NavigationAction{
			URL:    "/dashboard/{value}",
			Target: "_blank",
		},
	}

	panel := NewPanel().
		ID("test-panel").
		Title("Test Panel").
		OnClick(action).
		Build()

	require.NotNil(t, panel.Events)
	require.NotNil(t, panel.Events.Click)
	assert.Equal(t, lens.ActionTypeNavigation, panel.Events.Click.Action.Type)
	assert.Equal(t, "/dashboard/{value}", panel.Events.Click.Action.Navigation.URL)
	assert.Equal(t, "_blank", panel.Events.Click.Action.Navigation.Target)
}

func TestPanelBuilder_OnDataPointClick(t *testing.T) {
	action := lens.ActionConfig{
		Type: lens.ActionTypeDrillDown,
		DrillDown: &lens.DrillDownAction{
			Dashboard: "detail-dashboard",
			Filters: map[string]string{
				"category": "{categoryName}",
			},
		},
	}

	panel := NewPanel().
		ID("data-panel").
		OnDataPointClick(action).
		Build()

	require.NotNil(t, panel.Events)
	require.NotNil(t, panel.Events.DataPoint)
	assert.Equal(t, lens.ActionTypeDrillDown, panel.Events.DataPoint.Action.Type)
	assert.Equal(t, "detail-dashboard", panel.Events.DataPoint.Action.DrillDown.Dashboard)
}

func TestPanelBuilder_OnLegendClick(t *testing.T) {
	action := lens.ActionConfig{
		Type: lens.ActionTypeModal,
		Modal: &lens.ModalAction{
			Title:   "Legend Details",
			Content: "Series: {seriesName}",
		},
	}

	panel := NewPanel().
		ID("legend-panel").
		OnLegendClick(action).
		Build()

	require.NotNil(t, panel.Events)
	require.NotNil(t, panel.Events.Legend)
	assert.Equal(t, lens.ActionTypeModal, panel.Events.Legend.Action.Type)
	assert.Equal(t, "Legend Details", panel.Events.Legend.Action.Modal.Title)
}

func TestPanelBuilder_OnMarkerClick(t *testing.T) {
	action := lens.ActionConfig{
		Type: lens.ActionTypeCustom,
		Custom: &lens.CustomAction{
			Function: "handleMarkerClick",
		},
	}

	panel := NewPanel().
		ID("marker-panel").
		OnMarkerClick(action).
		Build()

	require.NotNil(t, panel.Events)
	require.NotNil(t, panel.Events.Marker)
	assert.Equal(t, lens.ActionTypeCustom, panel.Events.Marker.Action.Type)
	assert.Equal(t, "handleMarkerClick", panel.Events.Marker.Action.Custom.Function)
}

func TestPanelBuilder_OnXAxisLabelClick(t *testing.T) {
	action := lens.ActionConfig{
		Type: lens.ActionTypeNavigation,
		Navigation: &lens.NavigationAction{
			URL: "/category/{label}",
		},
	}

	panel := NewPanel().
		ID("xaxis-panel").
		OnXAxisLabelClick(action).
		Build()

	require.NotNil(t, panel.Events)
	require.NotNil(t, panel.Events.XAxisLabel)
	assert.Equal(t, lens.ActionTypeNavigation, panel.Events.XAxisLabel.Action.Type)
	assert.Equal(t, "/category/{label}", panel.Events.XAxisLabel.Action.Navigation.URL)
}

func TestPanelBuilder_OnNavigate(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		target         []string
		expectedTarget string
	}{
		{
			name:           "navigate with default target",
			url:            "/dashboard/sales",
			target:         nil,
			expectedTarget: "_self",
		},
		{
			name:           "navigate with blank target",
			url:            "/external/link",
			target:         []string{"_blank"},
			expectedTarget: "_blank",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewPanel().ID("nav-panel")

			if tt.target != nil {
				builder = builder.OnNavigate(tt.url, tt.target...)
			} else {
				builder = builder.OnNavigate(tt.url)
			}

			panel := builder.Build()

			require.NotNil(t, panel.Events)
			require.NotNil(t, panel.Events.Click)
			assert.Equal(t, lens.ActionTypeNavigation, panel.Events.Click.Action.Type)
			assert.Equal(t, tt.url, panel.Events.Click.Action.Navigation.URL)
			assert.Equal(t, tt.expectedTarget, panel.Events.Click.Action.Navigation.Target)
		})
	}
}

func TestPanelBuilder_OnDrillDown(t *testing.T) {
	filters := map[string]string{
		"region":   "{region}",
		"category": "{categoryName}",
	}

	tests := []struct {
		name              string
		filters           map[string]string
		dashboard         []string
		expectedDashboard string
	}{
		{
			name:              "drill-down without dashboard",
			filters:           filters,
			dashboard:         nil,
			expectedDashboard: "",
		},
		{
			name:              "drill-down with dashboard",
			filters:           filters,
			dashboard:         []string{"detail-view"},
			expectedDashboard: "detail-view",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewPanel().ID("drill-panel")

			if tt.dashboard != nil {
				builder = builder.OnDrillDown(tt.filters, tt.dashboard...)
			} else {
				builder = builder.OnDrillDown(tt.filters)
			}

			panel := builder.Build()

			require.NotNil(t, panel.Events)
			require.NotNil(t, panel.Events.Click)
			assert.Equal(t, lens.ActionTypeDrillDown, panel.Events.Click.Action.Type)
			assert.Equal(t, tt.expectedDashboard, panel.Events.Click.Action.DrillDown.Dashboard)
			assert.Equal(t, filters, panel.Events.Click.Action.DrillDown.Filters)
		})
	}
}

func TestPanelBuilder_OnModal(t *testing.T) {
	title := "Product Details"
	content := "Product: {label}"

	tests := []struct {
		name        string
		title       string
		content     string
		url         []string
		expectedURL string
	}{
		{
			name:        "modal without URL",
			title:       title,
			content:     content,
			url:         nil,
			expectedURL: "",
		},
		{
			name:        "modal with URL",
			title:       title,
			content:     content,
			url:         []string{"/api/product/{label}"},
			expectedURL: "/api/product/{label}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewPanel().ID("modal-panel")

			if tt.url != nil {
				builder = builder.OnModal(tt.title, tt.content, tt.url...)
			} else {
				builder = builder.OnModal(tt.title, tt.content)
			}

			panel := builder.Build()

			require.NotNil(t, panel.Events)
			require.NotNil(t, panel.Events.Click)
			assert.Equal(t, lens.ActionTypeModal, panel.Events.Click.Action.Type)
			assert.Equal(t, tt.title, panel.Events.Click.Action.Modal.Title)
			assert.Equal(t, tt.content, panel.Events.Click.Action.Modal.Content)
			assert.Equal(t, tt.expectedURL, panel.Events.Click.Action.Modal.URL)
		})
	}
}

func TestPanelBuilder_OnCustom(t *testing.T) {
	function := "myCustomHandler"

	tests := []struct {
		name              string
		function          string
		variables         []map[string]string
		expectedVariables map[string]string
	}{
		{
			name:              "custom without variables",
			function:          function,
			variables:         nil,
			expectedVariables: map[string]string{},
		},
		{
			name:     "custom with variables",
			function: function,
			variables: []map[string]string{
				{
					"param1": "{label}",
					"param2": "{value}",
				},
			},
			expectedVariables: map[string]string{
				"param1": "{label}",
				"param2": "{value}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewPanel().ID("custom-panel")

			if tt.variables != nil {
				builder = builder.OnCustom(tt.function, tt.variables...)
			} else {
				builder = builder.OnCustom(tt.function)
			}

			panel := builder.Build()

			require.NotNil(t, panel.Events)
			require.NotNil(t, panel.Events.Click)
			assert.Equal(t, lens.ActionTypeCustom, panel.Events.Click.Action.Type)
			assert.Equal(t, tt.function, panel.Events.Click.Action.Custom.Function)
			assert.Equal(t, tt.expectedVariables, panel.Events.Click.Action.Custom.Variables)
		})
	}
}

func TestPanelBuilder_MultipleEventHandlers(t *testing.T) {
	clickAction := lens.ActionConfig{
		Type:       lens.ActionTypeNavigation,
		Navigation: &lens.NavigationAction{URL: "/click"},
	}

	dataPointAction := lens.ActionConfig{
		Type:  lens.ActionTypeModal,
		Modal: &lens.ModalAction{Title: "Data Point"},
	}

	legendAction := lens.ActionConfig{
		Type:      lens.ActionTypeDrillDown,
		DrillDown: &lens.DrillDownAction{Dashboard: "legend-drill"},
	}

	panel := NewPanel().
		ID("multi-event-panel").
		OnClick(clickAction).
		OnDataPointClick(dataPointAction).
		OnLegendClick(legendAction).
		Build()

	require.NotNil(t, panel.Events)

	// Verify click event
	require.NotNil(t, panel.Events.Click)
	assert.Equal(t, lens.ActionTypeNavigation, panel.Events.Click.Action.Type)

	// Verify data point event
	require.NotNil(t, panel.Events.DataPoint)
	assert.Equal(t, lens.ActionTypeModal, panel.Events.DataPoint.Action.Type)

	// Verify legend event
	require.NotNil(t, panel.Events.Legend)
	assert.Equal(t, lens.ActionTypeDrillDown, panel.Events.Legend.Action.Type)
}

func TestPanelBuilder_EventsInitialization(t *testing.T) {
	// Test that events are properly initialized when first event is added
	panel := NewPanel().
		ID("init-panel").
		OnNavigate("/test").
		Build()

	require.NotNil(t, panel.Events)
	require.NotNil(t, panel.Events.Click)
	assert.Equal(t, lens.ActionTypeNavigation, panel.Events.Click.Action.Type)
	assert.Equal(t, "/test", panel.Events.Click.Action.Navigation.URL)
	assert.Equal(t, "_self", panel.Events.Click.Action.Navigation.Target)
}

func TestConvenienceChartBuilders_WithEvents(t *testing.T) {
	// Test that convenience builders work with event methods
	panel := BarChart().
		ID("bar-with-events").
		Title("Sales by Region").
		OnNavigate("/region/{label}").
		Build()

	assert.Equal(t, "bar-with-events", panel.ID)
	assert.Equal(t, "Sales by Region", panel.Title)
	assert.Equal(t, lens.ChartTypeBar, panel.Type)
	require.NotNil(t, panel.Events)
	require.NotNil(t, panel.Events.Click)
	assert.Equal(t, lens.ActionTypeNavigation, panel.Events.Click.Action.Type)
}
