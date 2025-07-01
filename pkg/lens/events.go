package lens

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// EventContext represents the context information passed to event handlers
type EventContext struct {
	PanelID      string                 `json:"panelId"`
	ChartType    ChartType              `json:"chartType"`
	DataPoint    *DataPointContext      `json:"dataPoint,omitempty"`
	SeriesIndex  *int                   `json:"seriesIndex,omitempty"`
	DataIndex    *int                   `json:"dataIndex,omitempty"`
	Label        string                 `json:"label,omitempty"`
	Value        interface{}            `json:"value,omitempty"`
	SeriesName   string                 `json:"seriesName,omitempty"`
	CategoryName string                 `json:"categoryName,omitempty"`
	Variables    map[string]interface{} `json:"variables,omitempty"`
	CustomData   map[string]interface{} `json:"customData,omitempty"`
}

// DataPointContext represents the context of a clicked data point
type DataPointContext struct {
	X           interface{} `json:"x"`
	Y           interface{} `json:"y"`
	SeriesIndex int         `json:"seriesIndex"`
	DataIndex   int         `json:"dataIndex"`
	Label       string      `json:"label"`
	Value       interface{} `json:"value"`
	Color       string      `json:"color,omitempty"`
}

// EventHandler interface for handling chart events
type EventHandler interface {
	HandleEvent(ctx context.Context, eventCtx *EventContext, action ActionConfig) (*EventResult, error)
}

// EventResult represents the result of an event handling operation
type EventResult struct {
	Type     EventResultType `json:"type"`
	Data     interface{}     `json:"data,omitempty"`
	Redirect *RedirectResult `json:"redirect,omitempty"`
	Modal    *ModalResult    `json:"modal,omitempty"`
	Update   *UpdateResult   `json:"update,omitempty"`
	Error    *EventError     `json:"error,omitempty"`
}

// EventResultType represents the type of event result
type EventResultType string

const (
	EventResultTypeRedirect EventResultType = "redirect"
	EventResultTypeModal    EventResultType = "modal"
	EventResultTypeUpdate   EventResultType = "update"
	EventResultTypeError    EventResultType = "error"
	EventResultTypeSuccess  EventResultType = "success"
)

// RedirectResult represents a redirect response
type RedirectResult struct {
	URL    string `json:"url"`
	Target string `json:"target,omitempty"`
}

// ModalResult represents a modal response
type ModalResult struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	URL     string `json:"url,omitempty"`
}

// UpdateResult represents a dashboard update response
type UpdateResult struct {
	PanelID   string                 `json:"panelId,omitempty"`
	Variables map[string]interface{} `json:"variables,omitempty"`
	Filters   map[string]interface{} `json:"filters,omitempty"`
}

// EventError represents an event handling error
type EventError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// DefaultEventHandler is the default implementation of EventHandler
type DefaultEventHandler struct{}

// NewEventHandler creates a new default event handler
func NewEventHandler() EventHandler {
	return &DefaultEventHandler{}
}

// HandleEvent processes an event based on the action configuration
func (h *DefaultEventHandler) HandleEvent(ctx context.Context, eventCtx *EventContext, action ActionConfig) (*EventResult, error) {
	switch action.Type {
	case ActionTypeNavigation:
		return h.handleNavigation(ctx, eventCtx, action.Navigation)
	case ActionTypeDrillDown:
		return h.handleDrillDown(ctx, eventCtx, action.DrillDown)
	case ActionTypeModal:
		return h.handleModal(ctx, eventCtx, action.Modal)
	case ActionTypeCustom:
		return h.handleCustom(ctx, eventCtx, action.Custom)
	default:
		return nil, fmt.Errorf("unsupported action type: %s", action.Type)
	}
}

// handleNavigation processes navigation actions
func (h *DefaultEventHandler) handleNavigation(ctx context.Context, eventCtx *EventContext, nav *NavigationAction) (*EventResult, error) {
	if nav == nil {
		return nil, fmt.Errorf("navigation action is nil")
	}

	finalURL, err := h.buildURL(nav.URL, nav.Variables, eventCtx)
	if err != nil {
		return &EventResult{
			Type: EventResultTypeError,
			Error: &EventError{
				Code:    "URL_BUILD_ERROR",
				Message: fmt.Sprintf("Failed to build URL: %v", err),
			},
		}, nil
	}

	target := nav.Target
	if target == "" {
		target = "_self"
	}

	return &EventResult{
		Type: EventResultTypeRedirect,
		Redirect: &RedirectResult{
			URL:    finalURL,
			Target: target,
		},
	}, nil
}

// handleDrillDown processes drill-down actions
func (h *DefaultEventHandler) handleDrillDown(ctx context.Context, eventCtx *EventContext, drillDown *DrillDownAction) (*EventResult, error) {
	if drillDown == nil {
		return nil, fmt.Errorf("drillDown action is nil")
	}

	filters := make(map[string]interface{})
	for key, value := range drillDown.Filters {
		expandedValue, err := h.expandVariables(value, eventCtx)
		if err != nil {
			return &EventResult{
				Type: EventResultTypeError,
				Error: &EventError{
					Code:    "VARIABLE_EXPANSION_ERROR",
					Message: fmt.Sprintf("Failed to expand variable %s: %v", key, err),
				},
			}, nil
		}
		filters[key] = expandedValue
	}

	variables := make(map[string]interface{})
	for key, value := range drillDown.Variables {
		expandedValue, err := h.expandVariables(value, eventCtx)
		if err != nil {
			return &EventResult{
				Type: EventResultTypeError,
				Error: &EventError{
					Code:    "VARIABLE_EXPANSION_ERROR",
					Message: fmt.Sprintf("Failed to expand variable %s: %v", key, err),
				},
			}, nil
		}
		variables[key] = expandedValue
	}

	return &EventResult{
		Type: EventResultTypeUpdate,
		Update: &UpdateResult{
			PanelID:   eventCtx.PanelID,
			Filters:   filters,
			Variables: variables,
		},
	}, nil
}

// handleModal processes modal actions
func (h *DefaultEventHandler) handleModal(ctx context.Context, eventCtx *EventContext, modal *ModalAction) (*EventResult, error) {
	if modal == nil {
		return nil, fmt.Errorf("modal action is nil")
	}

	title, err := h.expandVariables(modal.Title, eventCtx)
	if err != nil {
		return &EventResult{
			Type: EventResultTypeError,
			Error: &EventError{
				Code:    "VARIABLE_EXPANSION_ERROR",
				Message: fmt.Sprintf("Failed to expand modal title: %v", err),
			},
		}, nil
	}

	content := ""
	if modal.Content != "" {
		content, err = h.expandVariables(modal.Content, eventCtx)
		if err != nil {
			return &EventResult{
				Type: EventResultTypeError,
				Error: &EventError{
					Code:    "VARIABLE_EXPANSION_ERROR",
					Message: fmt.Sprintf("Failed to expand modal content: %v", err),
				},
			}, nil
		}
	}

	modalURL := ""
	if modal.URL != "" {
		modalURL, err = h.buildURL(modal.URL, modal.Variables, eventCtx)
		if err != nil {
			return &EventResult{
				Type: EventResultTypeError,
				Error: &EventError{
					Code:    "URL_BUILD_ERROR",
					Message: fmt.Sprintf("Failed to build modal URL: %v", err),
				},
			}, nil
		}
	}

	return &EventResult{
		Type: EventResultTypeModal,
		Modal: &ModalResult{
			Title:   title,
			Content: content,
			URL:     modalURL,
		},
	}, nil
}

// handleCustom processes custom JavaScript actions
func (h *DefaultEventHandler) handleCustom(ctx context.Context, eventCtx *EventContext, custom *CustomAction) (*EventResult, error) {
	if custom == nil {
		return nil, fmt.Errorf("custom action is nil")
	}

	variables := make(map[string]interface{})
	for key, value := range custom.Variables {
		expandedValue, err := h.expandVariables(value, eventCtx)
		if err != nil {
			return &EventResult{
				Type: EventResultTypeError,
				Error: &EventError{
					Code:    "VARIABLE_EXPANSION_ERROR",
					Message: fmt.Sprintf("Failed to expand variable %s: %v", key, err),
				},
			}, nil
		}
		variables[key] = expandedValue
	}

	return &EventResult{
		Type: EventResultTypeSuccess,
		Data: map[string]interface{}{
			"function":  custom.Function,
			"variables": variables,
			"context":   eventCtx,
		},
	}, nil
}

// buildURL constructs a URL with variable substitution
func (h *DefaultEventHandler) buildURL(baseURL string, variables map[string]string, eventCtx *EventContext) (string, error) {
	finalURL := baseURL

	for key, value := range variables {
		expandedValue, err := h.expandVariables(value, eventCtx)
		if err != nil {
			return "", fmt.Errorf("failed to expand variable %s: %w", key, err)
		}

		placeholder := fmt.Sprintf("{%s}", key)
		finalURL = strings.ReplaceAll(finalURL, placeholder, url.QueryEscape(expandedValue))
	}

	return finalURL, nil
}

// expandVariables expands template variables using event context
func (h *DefaultEventHandler) expandVariables(template string, eventCtx *EventContext) (string, error) {
	result := template

	replacements := map[string]interface{}{
		"{panelId}":      eventCtx.PanelID,
		"{chartType}":    eventCtx.ChartType,
		"{label}":        eventCtx.Label,
		"{value}":        eventCtx.Value,
		"{seriesName}":   eventCtx.SeriesName,
		"{categoryName}": eventCtx.CategoryName,
	}

	if eventCtx.SeriesIndex != nil {
		replacements["{seriesIndex}"] = *eventCtx.SeriesIndex
	}
	if eventCtx.DataIndex != nil {
		replacements["{dataIndex}"] = *eventCtx.DataIndex
	}

	if eventCtx.DataPoint != nil {
		replacements["{dataPoint.x}"] = eventCtx.DataPoint.X
		replacements["{dataPoint.y}"] = eventCtx.DataPoint.Y
		replacements["{dataPoint.label}"] = eventCtx.DataPoint.Label
		replacements["{dataPoint.value}"] = eventCtx.DataPoint.Value
		replacements["{dataPoint.color}"] = eventCtx.DataPoint.Color
	}

	for key, value := range eventCtx.Variables {
		varKey := fmt.Sprintf("{var.%s}", key)
		replacements[varKey] = value
	}

	for key, value := range eventCtx.CustomData {
		dataKey := fmt.Sprintf("{data.%s}", key)
		replacements[dataKey] = value
	}

	for placeholder, value := range replacements {
		if value != nil {
			result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
		}
	}

	return result, nil
}
