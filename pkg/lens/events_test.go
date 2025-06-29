package lens

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventHandler_HandleEvent(t *testing.T) {
	handler := NewEventHandler()

	tests := []struct {
		name        string
		eventCtx    *EventContext
		action      ActionConfig
		expectType  EventResultType
		expectError bool
	}{
		{
			name: "navigation action",
			eventCtx: &EventContext{
				PanelID:   "test-panel",
				ChartType: ChartTypeBar,
				Label:     "Test Label",
				Value:     100,
			},
			action: ActionConfig{
				Type: ActionTypeNavigation,
				Navigation: &NavigationAction{
					URL:    "/dashboard/{label}",
					Target: "_blank",
					Variables: map[string]string{
						"label": "{label}",
					},
				},
			},
			expectType:  EventResultTypeRedirect,
			expectError: false,
		},
		{
			name: "drill-down action",
			eventCtx: &EventContext{
				PanelID:      "test-panel",
				ChartType:    ChartTypePie,
				CategoryName: "Revenue",
				Value:        5000,
			},
			action: ActionConfig{
				Type: ActionTypeDrillDown,
				DrillDown: &DrillDownAction{
					Dashboard: "detailed-dashboard",
					Filters: map[string]string{
						"category": "{categoryName}",
						"value":    "{value}",
					},
				},
			},
			expectType:  EventResultTypeUpdate,
			expectError: false,
		},
		{
			name: "modal action",
			eventCtx: &EventContext{
				PanelID:    "test-panel",
				ChartType:  ChartTypeLine,
				SeriesName: "Sales",
				Label:      "Q1 2024",
			},
			action: ActionConfig{
				Type: ActionTypeModal,
				Modal: &ModalAction{
					Title:   "Sales Details for {label}",
					Content: "Series: {seriesName}",
					Variables: map[string]string{
						"label":      "{label}",
						"seriesName": "{seriesName}",
					},
				},
			},
			expectType:  EventResultTypeModal,
			expectError: false,
		},
		{
			name: "custom action",
			eventCtx: &EventContext{
				PanelID:   "test-panel",
				ChartType: ChartTypeArea,
				Label:     "Custom Data",
			},
			action: ActionConfig{
				Type: ActionTypeCustom,
				Custom: &CustomAction{
					Function: "myCustomFunction",
					Variables: map[string]string{
						"data": "{label}",
					},
				},
			},
			expectType:  EventResultTypeSuccess,
			expectError: false,
		},
		{
			name: "unsupported action type",
			eventCtx: &EventContext{
				PanelID: "test-panel",
			},
			action: ActionConfig{
				Type: "unsupported",
			},
			expectType:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := handler.HandleEvent(ctx, tt.eventCtx, tt.action)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.expectType, result.Type)
		})
	}
}

func TestEventHandler_NavigationAction(t *testing.T) {
	handler := &DefaultEventHandler{}
	ctx := context.Background()

	eventCtx := &EventContext{
		PanelID:      "test-panel",
		Label:        "Test Product",
		Value:        250,
		SeriesName:   "Electronics",
		CategoryName: "Q1",
	}

	navigation := &NavigationAction{
		URL:    "/product/{label}?category={categoryName}&value={value}",
		Target: "_blank",
		Variables: map[string]string{
			"label":        "{label}",
			"categoryName": "{categoryName}",
			"value":        "{value}",
		},
	}

	result, err := handler.handleNavigation(ctx, eventCtx, navigation)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, EventResultTypeRedirect, result.Type)
	require.NotNil(t, result.Redirect)
	assert.Equal(t, "_blank", result.Redirect.Target)
	assert.Contains(t, result.Redirect.URL, "Test+Product")
	assert.Contains(t, result.Redirect.URL, "Q1")
	assert.Contains(t, result.Redirect.URL, "250")
}

func TestEventHandler_DrillDownAction(t *testing.T) {
	handler := &DefaultEventHandler{}
	ctx := context.Background()

	eventCtx := &EventContext{
		PanelID:      "test-panel",
		CategoryName: "Sales",
		SeriesName:   "Q1 2024",
		Value:        1000,
	}

	drillDown := &DrillDownAction{
		Dashboard: "sales-detail",
		Filters: map[string]string{
			"period":   "{seriesName}",
			"category": "{categoryName}",
			"amount":   "{value}",
		},
		Variables: map[string]string{
			"selectedPeriod": "{seriesName}",
		},
	}

	result, err := handler.handleDrillDown(ctx, eventCtx, drillDown)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, EventResultTypeUpdate, result.Type)
	require.NotNil(t, result.Update)
	assert.Equal(t, "test-panel", result.Update.PanelID)
	assert.Equal(t, "Q1 2024", result.Update.Filters["period"])
	assert.Equal(t, "Sales", result.Update.Filters["category"])
	assert.Equal(t, "1000", result.Update.Filters["amount"])
	assert.Equal(t, "Q1 2024", result.Update.Variables["selectedPeriod"])
}

func TestEventHandler_ModalAction(t *testing.T) {
	handler := &DefaultEventHandler{}
	ctx := context.Background()

	eventCtx := &EventContext{
		PanelID:    "test-panel",
		Label:      "Revenue Analysis",
		SeriesName: "Q4 Results",
		Value:      50000,
	}

	modal := &ModalAction{
		Title:   "Details for {label}",
		Content: "Series: {seriesName}, Value: {value}",
		URL:     "/modal/{seriesName}",
		Variables: map[string]string{
			"label":      "{label}",
			"seriesName": "{seriesName}",
			"value":      "{value}",
		},
	}

	result, err := handler.handleModal(ctx, eventCtx, modal)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, EventResultTypeModal, result.Type)
	require.NotNil(t, result.Modal)
	assert.Equal(t, "Details for Revenue Analysis", result.Modal.Title)
	assert.Equal(t, "Series: Q4 Results, Value: 50000", result.Modal.Content)
	assert.Contains(t, result.Modal.URL, "Q4+Results")
}

func TestEventHandler_CustomAction(t *testing.T) {
	handler := &DefaultEventHandler{}
	ctx := context.Background()

	eventCtx := &EventContext{
		PanelID: "test-panel",
		Label:   "Custom Event",
		Value:   123,
	}

	custom := &CustomAction{
		Function: "handleCustomEvent",
		Variables: map[string]string{
			"eventLabel": "{label}",
			"eventValue": "{value}",
		},
	}

	result, err := handler.handleCustom(ctx, eventCtx, custom)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, EventResultTypeSuccess, result.Type)
	require.NotNil(t, result.Data)

	data, ok := result.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "handleCustomEvent", data["function"])

	variables, ok := data["variables"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Custom Event", variables["eventLabel"])
	assert.Equal(t, "123", variables["eventValue"])
}

func TestEventHandler_ExpandVariables(t *testing.T) {
	handler := &DefaultEventHandler{}

	eventCtx := &EventContext{
		PanelID:      "test-panel",
		ChartType:    ChartTypeBar,
		Label:        "Product A",
		Value:        150,
		SeriesName:   "Electronics",
		CategoryName: "Q2",
		Variables: map[string]interface{}{
			"timeRange": "30d",
			"region":    "North America",
		},
		CustomData: map[string]interface{}{
			"department": "Sales",
			"budget":     50000,
		},
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "simple label replacement",
			template: "Product: {label}",
			expected: "Product: Product A",
		},
		{
			name:     "multiple replacements",
			template: "{seriesName} - {categoryName}: {value}",
			expected: "Electronics - Q2: 150",
		},
		{
			name:     "variable replacement",
			template: "Time range: {var.timeRange}, Region: {var.region}",
			expected: "Time range: 30d, Region: North America",
		},
		{
			name:     "custom data replacement",
			template: "Department: {data.department}, Budget: {data.budget}",
			expected: "Department: Sales, Budget: 50000",
		},
		{
			name:     "mixed replacements",
			template: "{label} in {var.region} ({data.department})",
			expected: "Product A in North America (Sales)",
		},
		{
			name:     "no replacements",
			template: "Static text without variables",
			expected: "Static text without variables",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.expandVariables(tt.template, eventCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEventHandler_BuildURL(t *testing.T) {
	handler := &DefaultEventHandler{}

	eventCtx := &EventContext{
		PanelID:      "sales-panel",
		Label:        "Product ABC",
		Value:        999,
		CategoryName: "Electronics & Gadgets",
	}

	baseURL := "/dashboard/product?name={productName}&category={cat}&amount={amt}"
	variables := map[string]string{
		"productName": "{label}",
		"cat":         "{categoryName}",
		"amt":         "{value}",
	}

	result, err := handler.buildURL(baseURL, variables, eventCtx)

	require.NoError(t, err)
	expected := "/dashboard/product?name=Product+ABC&category=Electronics+%26+Gadgets&amount=999"
	assert.Equal(t, expected, result)
}

func TestEventHandler_NilActions(t *testing.T) {
	handler := &DefaultEventHandler{}
	ctx := context.Background()
	eventCtx := &EventContext{PanelID: "test"}

	// Test nil navigation action
	result, err := handler.handleNavigation(ctx, eventCtx, nil)
	require.Error(t, err)
	assert.Nil(t, result)

	// Test nil drill-down action
	result, err = handler.handleDrillDown(ctx, eventCtx, nil)
	require.Error(t, err)
	assert.Nil(t, result)

	// Test nil modal action
	result, err = handler.handleModal(ctx, eventCtx, nil)
	require.Error(t, err)
	assert.Nil(t, result)

	// Test nil custom action
	result, err = handler.handleCustom(ctx, eventCtx, nil)
	require.Error(t, err)
	assert.Nil(t, result)
}
