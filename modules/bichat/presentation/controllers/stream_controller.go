package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/httpdto"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// StreamController handles Server-Sent Events (SSE) for streaming chat responses.
type StreamController struct {
	app               application.Application
	chatService       bichatservices.ChatService
	attachmentService bichatservices.AttachmentService
	opts              ControllerOptions
}

const maxStreamRequestBodyBytes int64 = 32 << 20 // 32 MiB

// NewStreamController creates a new stream controller.
func NewStreamController(
	app application.Application,
	chatService bichatservices.ChatService,
	attachmentService bichatservices.AttachmentService,
	opts ...ControllerOption,
) *StreamController {
	return &StreamController{
		app:               app,
		chatService:       chatService,
		attachmentService: attachmentService,
		opts:              applyControllerOptions(opts...),
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

	subRouter := r.PathPrefix(c.opts.BasePath).Subrouter()
	subRouter.Use(commonMiddleware...)

	// Stream route
	subRouter.HandleFunc("/stream", c.StreamMessage).Methods("POST")
	subRouter.HandleFunc("/stream/stop", c.StopStream).Methods("POST")
	subRouter.HandleFunc("/stream/status", c.StreamStatus).Methods("GET")
	subRouter.HandleFunc("/stream/resume", c.ResumeStream).Methods("POST")
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

	// 3. Enforce access permission early (avoid DB work for forbidden users)
	if c.opts.RequireAccessPermission != nil {
		if err := composables.CanUser(r.Context(), c.opts.RequireAccessPermission); err != nil {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
	}

	// 4. Parse request
	type streamRequest struct {
		SessionID   uuid.UUID             `json:"sessionId"`
		Content     string                `json:"content"`
		Attachments []AttachmentUploadDTO `json:"attachments"`
		DebugMode   bool                  `json:"debugMode"`
		ReplaceFrom *uuid.UUID            `json:"replaceFromMessageId,omitempty"`
	}

	var req streamRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxStreamRequestBodyBytes)
	defer func() { _ = r.Body.Close() }()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "Request too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	domainAttachments, err := convertAttachmentDTOs(r.Context(), req.Attachments)
	if err != nil {
		message := "Invalid attachments"
		errorText := err.Error()
		if strings.Contains(errorText, "uploadId is required") || strings.Contains(errorText, "uploadId not found") {
			message = fmt.Sprintf("Invalid attachments: %s; upload artifacts first", errorText)
		}
		http.Error(w, message, http.StatusBadRequest)
		return
	}

	// 5. Validate session access
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
	if session.UserID() != int64(user.ID()) && composables.CanUser(r.Context(), c.opts.ReadAllPermission) != nil {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// 6. Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// 7. Stream chunks
	ctx := r.Context()
	if req.DebugMode {
		ctx = bichatservices.WithDebugMode(ctx, true)
	}

	err = c.chatService.SendMessageStream(ctx, bichatservices.SendMessageRequest{
		SessionID:            req.SessionID,
		UserID:               int64(user.ID()),
		Content:              req.Content,
		Attachments:          domainAttachments,
		DebugMode:            req.DebugMode,
		ReplaceFromMessageID: req.ReplaceFrom,
	}, func(chunk bichatservices.StreamChunk) {
		// Handle context cancellation
		select {
		case <-r.Context().Done():
			return
		default:
		}

		payload := httpdto.StreamChunkPayload{
			Type:         string(chunk.Type),
			Content:      chunk.Content,
			Citation:     chunk.Citation,
			Usage:        chunk.Usage,
			GenerationMs: chunk.GenerationMs,
			Timestamp:    chunk.Timestamp.UnixMilli(),
		}
		if chunk.Tool != nil {
			toolPayload := &httpdto.ToolEventPayload{
				CallID:     chunk.Tool.CallID,
				Name:       chunk.Tool.Name,
				AgentName:  chunk.Tool.AgentName,
				Arguments:  chunk.Tool.Arguments,
				Result:     chunk.Tool.Result,
				DurationMs: chunk.Tool.DurationMs,
			}
			if chunk.Tool.Error != nil {
				toolPayload.Error = chunk.Tool.Error.Error()
			}
			payload.Tool = toolPayload
		}
		if chunk.Interrupt != nil {
			questions := make([]httpdto.InterruptQuestionPayload, 0, len(chunk.Interrupt.Questions))
			for _, q := range chunk.Interrupt.Questions {
				options := make([]httpdto.InterruptQuestionOptionPayload, 0, len(q.Options))
				for _, opt := range q.Options {
					options = append(options, httpdto.InterruptQuestionOptionPayload{
						ID:    opt.ID,
						Label: opt.Label,
					})
				}
				questions = append(questions, httpdto.InterruptQuestionPayload{
					ID:      q.ID,
					Text:    q.Text,
					Type:    string(q.Type),
					Options: options,
				})
			}
			payload.Interrupt = &httpdto.InterruptEventPayload{
				CheckpointID:       chunk.Interrupt.CheckpointID,
				AgentName:          chunk.Interrupt.AgentName,
				ProviderResponseID: chunk.Interrupt.ProviderResponseID,
				Questions:          questions,
			}
		}
		if chunk.Error != nil {
			// Avoid leaking internal errors to the client.
			if chunk.Type == bichatservices.ChunkTypeError {
				payload.Error = "An error occurred while processing your request"
			} else {
				payload.Error = chunk.Error.Error()
			}
		}
		if chunk.RunID != "" {
			payload.RunID = chunk.RunID
		}
		if chunk.Snapshot != nil {
			payload.Snapshot = &httpdto.StreamSnapshotPayload{
				PartialContent:  chunk.Snapshot.PartialContent,
				PartialMetadata: chunk.Snapshot.PartialMetadata,
			}
		}

		// Event name is for SSE clients that care; our frontend reads `data:` lines.
		eventName := payload.Type
		if eventName == "" {
			eventName = "chunk"
		}
		c.sendSSEEvent(w, flusher, eventName, payload)
	})

	if err != nil {
		// Log actual error server-side
		logger := configuration.Use().Logger()
		entry := logger.WithError(serrors.E(op, err))
		entry.Error("Stream error")

		// Send sanitized error to client
		c.sendSSEEvent(w, flusher, "error", httpdto.StreamChunkPayload{
			Type:      "error",
			Error:     "An error occurred while processing your request",
			Timestamp: time.Now().UnixMilli(),
		})
		return
	}

	// Send done event
	c.sendSSEEvent(w, flusher, "done", httpdto.StreamChunkPayload{
		Type:      "done",
		Timestamp: time.Now().UnixMilli(),
	})
}

// StopStream handles POST /stream/stop to cancel active generation for a session.
// Request body: { "sessionId": "uuid" }. No partial assistant message is persisted.
func (c *StreamController) StopStream(w http.ResponseWriter, r *http.Request) {
	const op serrors.Op = "StreamController.StopStream"
	type stopRequest struct {
		SessionID uuid.UUID `json:"sessionId"`
	}
	var req stopRequest
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	defer func() { _ = r.Body.Close() }()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if req.SessionID == uuid.Nil {
		http.Error(w, "sessionId is required and must be a valid UUID", http.StatusBadRequest)
		return
	}
	if _, ok := c.requireStreamSessionAuth(w, r, req.SessionID); !ok {
		return
	}

	if err := c.chatService.StopGeneration(r.Context(), req.SessionID); err != nil {
		logger := configuration.Use().Logger()
		logger.WithError(serrors.E(op, err)).Warn("StopGeneration failed")
	}
	w.WriteHeader(http.StatusOK)
}

// requireStreamSessionAuth ensures the user is authenticated, has stream access, and owns (or can read) the session.
// On failure it writes the appropriate HTTP error to w and returns (nil, false). On success it returns (session, true).
func (c *StreamController) requireStreamSessionAuth(w http.ResponseWriter, r *http.Request, sessionID uuid.UUID) (domain.Session, bool) {
	user, err := composables.UseUser(r.Context())
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	if c.opts.RequireAccessPermission != nil {
		if err := composables.CanUser(r.Context(), c.opts.RequireAccessPermission); err != nil {
			http.Error(w, "Access denied", http.StatusForbidden)
			return nil, false
		}
	}
	session, err := c.chatService.GetSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, persistence.ErrSessionNotFound) {
			http.Error(w, "Session not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return nil, false
	}
	if session.UserID() != int64(user.ID()) && composables.CanUser(r.Context(), c.opts.ReadAllPermission) != nil {
		http.Error(w, "Access denied", http.StatusForbidden)
		return nil, false
	}
	return session, true
}

// StreamStatus handles GET /stream/status?sessionId=uuid for refresh-safe resume.
// Returns JSON: { "active": bool, "runId": "uuid"?, "snapshot": { "partialContent", "partialMetadata" }?, "startedAt": "ISO8601"? }.
func (c *StreamController) StreamStatus(w http.ResponseWriter, r *http.Request) {
	sessionIDStr := r.URL.Query().Get("sessionId")
	if sessionIDStr == "" {
		http.Error(w, "sessionId is required", http.StatusBadRequest)
		return
	}
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil || sessionID == uuid.Nil {
		http.Error(w, "sessionId must be a valid UUID", http.StatusBadRequest)
		return
	}
	if _, ok := c.requireStreamSessionAuth(w, r, sessionID); !ok {
		return
	}

	status, err := c.chatService.GetStreamStatus(r.Context(), sessionID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	type statusPayload struct {
		Active    bool                           `json:"active"`
		RunID     string                         `json:"runId,omitempty"`
		Snapshot  *httpdto.StreamSnapshotPayload `json:"snapshot,omitempty"`
		StartedAt int64                          `json:"startedAt,omitempty"`
	}
	payload := statusPayload{Active: status.Active}
	if status.Active {
		payload.RunID = status.RunID.String()
		payload.Snapshot = &httpdto.StreamSnapshotPayload{
			PartialContent:  status.Snapshot.PartialContent,
			PartialMetadata: status.Snapshot.PartialMetadata,
		}
		payload.StartedAt = status.StartedAt.UnixMilli()
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// ResumeStream handles POST /stream/resume for reconnecting to an active run.
// Request body: { "sessionId": "uuid", "runId": "uuid" }. Response: SSE stream (snapshot event then content/done/error).
func (c *StreamController) ResumeStream(w http.ResponseWriter, r *http.Request) {
	const op serrors.Op = "StreamController.ResumeStream"

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	type resumeRequest struct {
		SessionID uuid.UUID `json:"sessionId"`
		RunID     uuid.UUID `json:"runId"`
	}
	var req resumeRequest
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	defer func() { _ = r.Body.Close() }()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if req.SessionID == uuid.Nil || req.RunID == uuid.Nil {
		http.Error(w, "sessionId and runId are required", http.StatusBadRequest)
		return
	}
	if _, ok := c.requireStreamSessionAuth(w, r, req.SessionID); !ok {
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	err := c.chatService.ResumeStream(r.Context(), req.SessionID, req.RunID, func(chunk bichatservices.StreamChunk) {
		payload := httpdto.StreamChunkPayload{
			Type:         string(chunk.Type),
			Content:      chunk.Content,
			Citation:     chunk.Citation,
			Usage:        chunk.Usage,
			GenerationMs: chunk.GenerationMs,
			Timestamp:    chunk.Timestamp.UnixMilli(),
		}
		if chunk.Tool != nil {
			toolPayload := &httpdto.ToolEventPayload{
				CallID:     chunk.Tool.CallID,
				Name:       chunk.Tool.Name,
				AgentName:  chunk.Tool.AgentName,
				Arguments:  chunk.Tool.Arguments,
				Result:     chunk.Tool.Result,
				DurationMs: chunk.Tool.DurationMs,
			}
			if chunk.Tool.Error != nil {
				toolPayload.Error = chunk.Tool.Error.Error()
			}
			payload.Tool = toolPayload
		}
		if chunk.Interrupt != nil {
			questions := make([]httpdto.InterruptQuestionPayload, 0, len(chunk.Interrupt.Questions))
			for _, q := range chunk.Interrupt.Questions {
				options := make([]httpdto.InterruptQuestionOptionPayload, 0, len(q.Options))
				for _, opt := range q.Options {
					options = append(options, httpdto.InterruptQuestionOptionPayload{ID: opt.ID, Label: opt.Label})
				}
				questions = append(questions, httpdto.InterruptQuestionPayload{ID: q.ID, Text: q.Text, Type: string(q.Type), Options: options})
			}
			payload.Interrupt = &httpdto.InterruptEventPayload{
				CheckpointID:       chunk.Interrupt.CheckpointID,
				AgentName:          chunk.Interrupt.AgentName,
				ProviderResponseID: chunk.Interrupt.ProviderResponseID,
				Questions:          questions,
			}
		}
		if chunk.Error != nil {
			payload.Error = "An error occurred while processing your request"
			if chunk.Type != bichatservices.ChunkTypeError {
				payload.Error = chunk.Error.Error()
			}
		}
		if chunk.Snapshot != nil {
			payload.Snapshot = &httpdto.StreamSnapshotPayload{
				PartialContent:  chunk.Snapshot.PartialContent,
				PartialMetadata: chunk.Snapshot.PartialMetadata,
			}
		}

		eventName := payload.Type
		if eventName == "" {
			eventName = "chunk"
		}
		c.sendSSEEvent(w, flusher, eventName, payload)
	})
	if err != nil {
		if errors.Is(err, bichatservices.ErrRunNotFoundOrFinished) {
			http.Error(w, "Run not found or already finished", http.StatusNotFound)
			return
		}
		logger := configuration.Use().Logger()
		logger.WithError(serrors.E(op, err)).Error("Resume stream error")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	c.sendSSEEvent(w, flusher, "done", httpdto.StreamChunkPayload{
		Type:      "done",
		Timestamp: time.Now().UnixMilli(),
	})
}

// Helper methods

// sendSSEEvent sends an SSE event with the given name and data
func (c *StreamController) sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, event string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		// If we can't marshal, send a plain error
		_, _ = fmt.Fprintf(w, "event: error\n")
		_, _ = fmt.Fprintf(w, "data: {\"type\":\"error\",\"error\":\"Failed to serialize data\"}\n\n")
		flusher.Flush()
		return
	}

	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", jsonData)
	flusher.Flush()
}
