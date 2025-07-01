package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

// LensEventsController handles chart event requests from the UI
type LensEventsController struct {
	app          application.Application
	eventHandler lens.EventHandler
}

// NewLensEventsController creates a new lens events controller
func NewLensEventsController(app application.Application) application.Controller {
	return &LensEventsController{
		app:          app,
		eventHandler: lens.NewEventHandler(),
	}
}

func (c *LensEventsController) Key() string {
	return "/api/lens/events"
}

func (c *LensEventsController) Register(r *mux.Router) {
	router := r.PathPrefix("/api/lens/events").Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.ProvideUser(),
		middleware.ProvideLocalizer(c.app.Bundle()),
	)

	// Handle chart events
	router.HandleFunc("/chart/{panelId}", c.HandleChartEvent).Methods(http.MethodPost)
}

// ChartEventRequest represents the request payload for chart events
type ChartEventRequest struct {
	PanelID      string                 `json:"panelId"`
	EventType    string                 `json:"eventType"`
	ChartType    string                 `json:"chartType"`
	ActionConfig lens.ActionConfig      `json:"actionConfig"`
	DataPoint    *lens.DataPointContext `json:"dataPoint,omitempty"`
	SeriesIndex  *int                   `json:"seriesIndex,omitempty"`
	DataIndex    *int                   `json:"dataIndex,omitempty"`
	Label        string                 `json:"label,omitempty"`
	Value        interface{}            `json:"value,omitempty"`
	SeriesName   string                 `json:"seriesName,omitempty"`
	CategoryName string                 `json:"categoryName,omitempty"`
	Variables    map[string]interface{} `json:"variables,omitempty"`
	CustomData   map[string]interface{} `json:"customData,omitempty"`
}

// ChartEventResponse represents the response for chart events
type ChartEventResponse struct {
	Success bool              `json:"success"`
	Result  *lens.EventResult `json:"result,omitempty"`
	Error   string            `json:"error,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// HandleChartEvent processes chart click events via HTMX
func (c *LensEventsController) HandleChartEvent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	panelID := vars["panelId"]

	if panelID == "" {
		http.Error(w, "Panel ID is required", http.StatusBadRequest)
		return
	}

	var req ChartEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate panel ID matches
	if req.PanelID != panelID {
		http.Error(w, "Panel ID mismatch", http.StatusBadRequest)
		return
	}

	// Create event context
	eventCtx := &lens.EventContext{
		PanelID:      req.PanelID,
		ChartType:    lens.ChartType(req.ChartType),
		DataPoint:    req.DataPoint,
		SeriesIndex:  req.SeriesIndex,
		DataIndex:    req.DataIndex,
		Label:        req.Label,
		Value:        req.Value,
		SeriesName:   req.SeriesName,
		CategoryName: req.CategoryName,
		Variables:    req.Variables,
		CustomData:   req.CustomData,
	}

	// Handle the event
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := c.eventHandler.HandleEvent(ctx, eventCtx, req.ActionConfig)
	if err != nil {
		c.sendErrorResponse(w, fmt.Sprintf("Failed to handle event: %v", err))
		return
	}

	// Send appropriate response based on result type
	c.sendEventResponse(w, result)
}

// sendEventResponse sends the appropriate response based on event result type
func (c *LensEventsController) sendEventResponse(w http.ResponseWriter, result *lens.EventResult) {
	response := ChartEventResponse{
		Success: true,
		Result:  result,
		Headers: make(map[string]string),
	}

	switch result.Type {
	case lens.EventResultTypeRedirect:
		if result.Redirect != nil {
			// For HTMX redirects, use HX-Redirect header
			if result.Redirect.Target == "_blank" {
				// For new windows, use JavaScript
				response.Headers["HX-Trigger"] = fmt.Sprintf(`{"openWindow": {"url": "%s"}}`, result.Redirect.URL)
			} else {
				// For same window redirects
				response.Headers["HX-Redirect"] = result.Redirect.URL
			}
		}

	case lens.EventResultTypeModal:
		if result.Modal != nil {
			// Trigger modal via HX-Trigger
			modalData := map[string]interface{}{
				"title":   result.Modal.Title,
				"content": result.Modal.Content,
				"url":     result.Modal.URL,
			}
			modalJSON, err := json.Marshal(modalData)
			if err != nil {
				log.Printf("Failed to marshal modal data: %v", err)
				c.sendErrorResponse(w, "Failed to process modal data")
				return
			}
			response.Headers["HX-Trigger"] = fmt.Sprintf(`{"showModal": %s}`, string(modalJSON))
		}

	case lens.EventResultTypeUpdate:
		if result.Update != nil {
			// Trigger dashboard update via HX-Trigger
			updateData := map[string]interface{}{
				"panelId":   result.Update.PanelID,
				"variables": result.Update.Variables,
				"filters":   result.Update.Filters,
			}
			updateJSON, err := json.Marshal(updateData)
			if err != nil {
				log.Printf("Failed to marshal update data: %v", err)
				c.sendErrorResponse(w, "Failed to process update data")
				return
			}
			response.Headers["HX-Trigger"] = fmt.Sprintf(`{"updateDashboard": %s}`, string(updateJSON))
		}

	case lens.EventResultTypeError:
		if result.Error != nil {
			c.sendErrorResponse(w, result.Error.Message)
			return
		}

	case lens.EventResultTypeSuccess:
		// For custom functions, send the function data to client
		if result.Data != nil {
			if data, ok := result.Data.(map[string]interface{}); ok {
				if function, exists := data["function"]; exists {
					response.Headers["HX-Trigger"] = fmt.Sprintf(`{"customFunction": {"function": "%s", "variables": %v, "context": %v}}`,
						function, data["variables"], data["context"])
				}
			}
		}
	}

	// Set headers
	for key, value := range response.Headers {
		w.Header().Set(key, value)
	}

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// sendErrorResponse sends an error response
func (c *LensEventsController) sendErrorResponse(w http.ResponseWriter, message string) {
	response := ChartEventResponse{
		Success: false,
		Error:   message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode error response: %v", err)
	}
}
