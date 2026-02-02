package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/bichat/permissions"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// StreamController handles Server-Sent Events (SSE) for streaming chat responses.
type StreamController struct {
	app         application.Application
	chatService services.ChatService
}

// NewStreamController creates a new stream controller.
func NewStreamController(
	app application.Application,
	chatService services.ChatService,
) *StreamController {
	return &StreamController{
		app:         app,
		chatService: chatService,
	}
}

// Key returns the controller key for dependency injection.
func (c *StreamController) Key() string {
	return "bichat.StreamController"
}

// Register registers HTTP routes for SSE streaming.
func (c *StreamController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(c.app),
		middleware.ProvideLocalizer(c.app),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}

	subRouter := r.PathPrefix("/bi-chat").Subrouter()
	subRouter.Use(commonMiddleware...)

	// Stream route
	subRouter.HandleFunc("/stream", c.StreamMessage).Methods("POST")
}

// StreamMessage handles SSE streaming for a message.
//
// Request Body:
//
//	{
//	  "sessionId": "uuid",
//	  "content": "Show me revenue for Q1",
//	  "attachments": []
//	}
//
// Response: Server-Sent Events stream
func (c *StreamController) StreamMessage(w http.ResponseWriter, r *http.Request) {
	const op serrors.Op = "StreamController.StreamMessage"

	// 1. Check for Flusher support
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// 2. Get user
	user, err := composables.UseUser(r.Context())
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 3. Parse request
	type streamRequest struct {
		SessionID   uuid.UUID           `json:"sessionId"`
		Content     string              `json:"content"`
		Attachments []domain.Attachment `json:"attachments"`
	}

	var req streamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 4. Validate session access
	session, err := c.chatService.GetSession(r.Context(), req.SessionID)
	if err != nil {
		if errors.Is(err, persistence.ErrSessionNotFound) {
			http.Error(w, "Session not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Check permission (user owns session or has read_all permission)
	if session.UserID != int64(user.ID()) && composables.CanUser(r.Context(), permissions.BiChatReadAll) != nil {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// 5. Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// 6. Stream chunks
	err = c.chatService.SendMessageStream(r.Context(), services.SendMessageRequest{
		SessionID:   req.SessionID,
		UserID:      int64(user.ID()),
		Content:     req.Content,
		Attachments: req.Attachments,
	}, func(chunk services.StreamChunk) {
		// Handle context cancellation
		select {
		case <-r.Context().Done():
			return
		default:
		}

		// Serialize chunk to JSON
		data, err := json.Marshal(chunk)
		if err != nil {
			c.sendSSEEvent(w, flusher, "error", map[string]string{
				"type":  "error",
				"error": "Failed to serialize chunk",
			})
			return
		}

		// Write SSE event based on chunk type
		eventType := "chunk"
		if chunk.Type == services.ChunkTypeError {
			eventType = "error"
		} else if chunk.Type == services.ChunkTypeDone {
			eventType = "done"
		}

		c.sendSSEEvent(w, flusher, eventType, json.RawMessage(data))
	})

	if err != nil {
		// Log actual error server-side
		logger := configuration.Use().Logger()
		logger.WithError(serrors.E(op, err)).Error("Stream error")

		// Send sanitized error to client
		c.sendSSEEvent(w, flusher, "error", map[string]string{
			"type":  "error",
			"error": "An error occurred while processing your request",
		})
		return
	}

	// Send done event
	c.sendSSEEvent(w, flusher, "done", map[string]string{
		"type": "done",
	})
}

// Helper methods

// sendSSEEvent sends an SSE event with the given name and data
func (c *StreamController) sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, event string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		// If we can't marshal, send a plain error
		fmt.Fprintf(w, "event: error\n")
		fmt.Fprintf(w, "data: {\"type\":\"error\",\"error\":\"Failed to serialize data\"}\n\n")
		flusher.Flush()
		return
	}

	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
	flusher.Flush()
}

// sendSSEComment sends an SSE comment for keep-alive
func (c *StreamController) sendSSEComment(w http.ResponseWriter, flusher http.Flusher, comment string) {
	fmt.Fprintf(w, ": %s\n\n", comment)
	flusher.Flush()
}
