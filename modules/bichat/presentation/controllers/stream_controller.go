package controllers

import (
	"net/http"

	"github.com/iota-uz/iota-sdk/pkg/application"
)

// StreamController handles Server-Sent Events (SSE) for streaming chat responses.
// TODO: Implement when Phase 1 (Agent Framework) is complete.
//
// Responsibilities:
//   - Establish SSE connection
//   - Stream message chunks in real-time
//   - Handle connection lifecycle (keep-alive, close)
//   - Error handling and recovery
//   - Tenant isolation
//
// SSE Event Format:
//
//	event: chunk
//	data: {"type":"content","content":"Hello","timestamp":"2025-01-31T..."}
//
//	event: chunk
//	data: {"type":"citation","citation":{...},"timestamp":"2025-01-31T..."}
//
//	event: chunk
//	data: {"type":"usage","usage":{...},"timestamp":"2025-01-31T..."}
//
//	event: done
//	data: {"type":"done","timestamp":"2025-01-31T..."}
type StreamController struct {
	app application.Application
	// chatService services.ChatService // TODO: Uncomment when Phase 1 is complete
}

// NewStreamController creates a new stream controller.
// TODO: Update constructor signature when Phase 1 is complete.
func NewStreamController(app application.Application) *StreamController {
	return &StreamController{
		app: app,
	}
}

// RegisterRoutes registers HTTP routes for SSE streaming.
// TODO: Implement when Phase 1 is complete.
//
// Routes:
//
//	POST /bichat/stream - Start streaming a message response
func (c *StreamController) RegisterRoutes() {
	// TODO: Implement route registration
	// Example:
	// router := c.app.Router()
	// subRouter := router.PathPrefix("/bichat").Subrouter()
	//
	// // Apply auth middleware
	// subRouter.Use(middleware.Authorize())
	// subRouter.Use(middleware.RequirePermission(permissions.BIChatAccess))
	//
	// // Stream route
	// subRouter.HandleFunc("/stream", c.StreamMessage).Methods("POST")
}

// StreamMessage handles SSE streaming for a message.
// TODO: Implement when Phase 1 is complete.
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
	// TODO: Implement SSE streaming
	// Example implementation:
	//
	// 1. Parse request
	// type streamRequest struct {
	//     SessionID   uuid.UUID `json:"sessionId"`
	//     Content     string    `json:"content"`
	//     Attachments []domain.Attachment `json:"attachments"`
	// }
	// var req streamRequest
	// if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
	//     http.Error(w, "Invalid request", http.StatusBadRequest)
	//     return
	// }
	//
	// 2. Validate session access
	// session, err := c.chatService.GetSession(r.Context(), req.SessionID)
	// if err != nil {
	//     http.Error(w, "Session not found", http.StatusNotFound)
	//     return
	// }
	// user := composables.UseUser(r.Context())
	// if session.UserID != user.ID && !composables.CanUser(r.Context(), permissions.BIChatReadAll) {
	//     http.Error(w, "Access denied", http.StatusForbidden)
	//     return
	// }
	//
	// 3. Set SSE headers
	// w.Header().Set("Content-Type", "text/event-stream")
	// w.Header().Set("Cache-Control", "no-cache")
	// w.Header().Set("Connection", "keep-alive")
	// w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
	//
	// 4. Create flusher
	// flusher, ok := w.(http.Flusher)
	// if !ok {
	//     http.Error(w, "Streaming not supported", http.StatusInternalServerError)
	//     return
	// }
	//
	// 5. Stream chunks
	// err = c.chatService.SendMessageStream(r.Context(), services.SendMessageRequest{
	//     SessionID:   req.SessionID,
	//     UserID:      user.ID,
	//     Content:     req.Content,
	//     Attachments: req.Attachments,
	// }, func(chunk services.StreamChunk) {
	//     // Serialize chunk to JSON
	//     data, _ := json.Marshal(chunk)
	//
	//     // Write SSE event
	//     fmt.Fprintf(w, "event: chunk\n")
	//     fmt.Fprintf(w, "data: %s\n\n", data)
	//     flusher.Flush()
	// })
	//
	// if err != nil {
	//     // Send error event
	//     errorData, _ := json.Marshal(map[string]string{
	//         "type":  "error",
	//         "error": err.Error(),
	//     })
	//     fmt.Fprintf(w, "event: error\n")
	//     fmt.Fprintf(w, "data: %s\n\n", errorData)
	//     flusher.Flush()
	//     return
	// }
	//
	// // Send done event
	// fmt.Fprintf(w, "event: done\n")
	// fmt.Fprintf(w, "data: {\"type\":\"done\"}\n\n")
	// flusher.Flush()

	http.Error(w, "Not implemented - Phase 1 pending", http.StatusNotImplemented)
}

// TODO: Implement sendSSEEvent when Phase 1 is complete.
// Helper to send SSE events - currently unused but reserved for future implementation.
//
// func (c *StreamController) sendSSEEvent(w http.ResponseWriter, event string, data interface{}) error {
// 	flusher, _ := w.(http.Flusher)
// 	jsonData, err := json.Marshal(data)
// 	if err != nil {
// 		return err
// 	}
// 	fmt.Fprintf(w, "event: %s\n", event)
// 	fmt.Fprintf(w, "data: %s\n\n", jsonData)
// 	flusher.Flush()
// 	return nil
// }

// TODO: Implement sendSSEComment when Phase 1 is complete.
// Helper to send SSE comments for keep-alive - currently unused but reserved for future implementation.
//
// func (c *StreamController) sendSSEComment(w http.ResponseWriter, comment string) {
// 	flusher, _ := w.(http.Flusher)
// 	fmt.Fprintf(w, ": %s\n\n", comment)
// 	flusher.Flush()
// }
