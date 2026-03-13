// Package controllers provides this package.
package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
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
	streamService     bichatservices.StreamCommands
	sessionService    bichatservices.SessionQueries
	attachmentService bichatservices.AttachmentService
	opts              ControllerOptions
}

const maxStreamRequestBodyBytes int64 = 32 << 20 // 32 MiB
const streamHeartbeatInterval = 15 * time.Second

// NewStreamController creates a new stream controller.
func NewStreamController(
	app application.Application,
	streamService bichatservices.StreamCommands,
	sessionService bichatservices.SessionQueries,
	attachmentService bichatservices.AttachmentService,
	opts ...ControllerOption,
) *StreamController {
	return &StreamController{
		app:               app,
		streamService:     streamService,
		sessionService:    sessionService,
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

	// 2b. Require access permission (so 403 is returned before body validation)
	if c.opts.RequireAccessPermission != nil {
		if err := composables.CanUser(r.Context(), c.opts.RequireAccessPermission); err != nil {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
	}

	// 3. Parse request
	type streamRequest struct {
		SessionID       uuid.UUID             `json:"sessionId"`
		Content         string                `json:"content"`
		Attachments     []AttachmentUploadDTO `json:"attachments"`
		DebugMode       bool                  `json:"debugMode"`
		ReplaceFrom     *uuid.UUID            `json:"replaceFromMessageId,omitempty"`
		ReasoningEffort *string               `json:"reasoningEffort,omitempty"`
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
	if req.SessionID == uuid.Nil {
		http.Error(w, "sessionId is required and must be a valid UUID", http.StatusBadRequest)
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

	// 5. Validate write access
	if !c.requireStreamSessionAuth(w, r, req.SessionID, true) {
		return
	}

	// 6. Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
	c.sendSSEComment(w, flusher, "stream-open")

	// 7. Stream chunks
	ctx := r.Context()
	var writeMu sync.Mutex
	sendEvent := func(event string, payload interface{}) {
		writeMu.Lock()
		defer writeMu.Unlock()
		c.sendSSEEvent(w, flusher, event, payload)
	}
	heartbeatStop := make(chan struct{})
	defer close(heartbeatStop)
	go func() {
		ticker := time.NewTicker(streamHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-heartbeatStop:
				return
			case <-ticker.C:
				sendEvent("ping", httpdto.StreamChunkPayload{
					Type:      string(bichatservices.ChunkTypePing),
					Timestamp: time.Now().UnixMilli(),
				})
			}
		}
	}()
	if req.DebugMode {
		ctx = bichatservices.WithDebugMode(ctx, true)
	}

	err = c.streamService.SendMessageStream(ctx, bichatservices.SendMessageRequest{
		SessionID:            req.SessionID,
		UserID:               int64(user.ID()),
		Content:              req.Content,
		Attachments:          domainAttachments,
		DebugMode:            req.DebugMode,
		ReplaceFromMessageID: req.ReplaceFrom,
		ReasoningEffort:      req.ReasoningEffort,
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
			// Preserve known provider-facing failures (quota/auth/rate-limit), and
			// keep all other internal failures generic.
			payload.Error = c.streamClientErrorMessage(chunk.Error, chunk.Type)
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
		sendEvent(eventName, payload)
	})

	if err != nil {
		// Log actual error server-side
		logger := configuration.Use().Logger()
		entry := logger.WithError(serrors.E(op, err))
		entry.Error("Stream error")

		// Send sanitized error to client
		sendEvent("error", httpdto.StreamChunkPayload{
			Type:      "error",
			Error:     c.streamClientErrorMessage(err, bichatservices.ChunkTypeError),
			Timestamp: time.Now().UnixMilli(),
		})
		return
	}

	// Send done event
	sendEvent("done", httpdto.StreamChunkPayload{
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
	if !c.requireStreamSessionAuth(w, r, req.SessionID, true) {
		return
	}

	if err := c.streamService.StopGeneration(r.Context(), req.SessionID); err != nil {
		logger := configuration.Use().Logger()
		wrapped := serrors.E(op, err)
		if errors.Is(err, domain.ErrNoActiveRun) || errors.Is(err, bichatservices.ErrRunNotFoundOrFinished) {
			// Known condition: stop called when session is no longer streaming.
			// Preserve idempotent API behavior and just log for observability.
			logger.WithError(wrapped).Info("StopGeneration called for non-streaming session")
			w.WriteHeader(http.StatusOK)
			return
		}
		logger.WithError(wrapped).Error("StopGeneration failed")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// requireStreamSessionAuth ensures the user is authenticated, has stream access, and has the required capability.
// On failure it writes the appropriate HTTP error to w and returns false. On success it returns true.
func (c *StreamController) requireStreamSessionAuth(w http.ResponseWriter, r *http.Request, sessionID uuid.UUID, requireWrite bool) bool {
	user, err := composables.UseUser(r.Context())
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	if c.opts.RequireAccessPermission != nil {
		if err := composables.CanUser(r.Context(), c.opts.RequireAccessPermission); err != nil {
			http.Error(w, "Access denied", http.StatusForbidden)
			return false
		}
	}
	readAll := false
	if c.opts.ReadAllPermission != nil {
		readAll = composables.CanUser(r.Context(), c.opts.ReadAllPermission) == nil
	}
	access, err := c.sessionService.ResolveSessionAccess(r.Context(), sessionID, int64(user.ID()), readAll)
	if err != nil {
		if errors.Is(err, persistence.ErrSessionNotFound) {
			http.Error(w, "Session not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return false
	}
	if err := access.Require(requireWrite, false); err != nil {
		http.Error(w, "Access denied", http.StatusForbidden)
		return false
	}
	return true
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
	if !c.requireStreamSessionAuth(w, r, sessionID, false) {
		return
	}

	status, err := c.streamService.GetStreamStatus(r.Context(), sessionID)
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
	if !c.requireStreamSessionAuth(w, r, req.SessionID, false) {
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	c.sendSSEComment(w, flusher, "stream-open")

	var writeMu sync.Mutex
	sendEvent := func(event string, payload interface{}) {
		writeMu.Lock()
		defer writeMu.Unlock()
		c.sendSSEEvent(w, flusher, event, payload)
	}
	heartbeatStop := make(chan struct{})
	defer close(heartbeatStop)
	go func() {
		ticker := time.NewTicker(streamHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-heartbeatStop:
				return
			case <-ticker.C:
				sendEvent("ping", httpdto.StreamChunkPayload{
					Type:      string(bichatservices.ChunkTypePing),
					Timestamp: time.Now().UnixMilli(),
				})
			}
		}
	}()

	err := c.streamService.ResumeStream(r.Context(), req.SessionID, req.RunID, func(chunk bichatservices.StreamChunk) {
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
			payload.Error = c.streamClientErrorMessage(chunk.Error, chunk.Type)
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
		sendEvent(eventName, payload)
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

	sendEvent("done", httpdto.StreamChunkPayload{
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

func (c *StreamController) sendSSEComment(w http.ResponseWriter, flusher http.Flusher, comment string) {
	if strings.TrimSpace(comment) == "" {
		comment = "stream-open"
	}
	_, _ = fmt.Fprintf(w, ": %s\n\n", comment)
	flusher.Flush()
}

func (c *StreamController) streamClientErrorMessage(err error, chunkType bichatservices.ChunkType) string {
	const generic = "An error occurred while processing your request"
	if err == nil {
		return ""
	}
	if chunkType != bichatservices.ChunkTypeError {
		return sanitizeErrorString(err)
	}

	code, message, ok := parseProviderStreamError(err.Error())
	if !ok {
		return generic
	}

	switch strings.ToLower(code) {
	case "insufficient_quota":
		if strings.TrimSpace(message) != "" {
			return message
		}
		return "You exceeded your current quota. Please check your plan and billing details."
	case "rate_limit_exceeded", "rate_limit":
		if strings.TrimSpace(message) != "" {
			return message
		}
		return "Rate limit exceeded. Please retry shortly."
	case "invalid_api_key", "authentication_error", "auth_error":
		if strings.TrimSpace(message) != "" {
			return message
		}
		return "Authentication failed with the model provider. Please verify API credentials."
	default:
		return generic
	}
}

func sanitizeErrorString(err error) string {
	if err == nil {
		return ""
	}
	return "internal error"
}

// parseProviderStreamError expects provider errors to include a JSON fragment
// containing "type"/"code"/"message". It scans the raw string for {"type": or
// {"code":, then attempts to unmarshal that fragment. If parsing fails, it
// intentionally returns ok=false so callers fall back to a generic safe message.
func parseProviderStreamError(raw string) (string, string, bool) {
	start := strings.Index(raw, "{\"type\":")
	if start < 0 {
		start = strings.Index(raw, "{\"code\":")
	}
	if start < 0 {
		return "", "", false
	}
	fragment := raw[start:]
	end := strings.LastIndex(fragment, "}")
	if end < 0 {
		return "", "", false
	}
	fragment = fragment[:end+1]

	var providerErr struct {
		Type    string `json:"type"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(fragment), &providerErr); err != nil {
		return "", "", false
	}

	code := providerErr.Code
	if strings.TrimSpace(code) == "" {
		code = providerErr.Type
	}
	return code, providerErr.Message, strings.TrimSpace(code) != ""
}
