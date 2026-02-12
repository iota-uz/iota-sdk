package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ChatController handles HTTP endpoints for chat operations.
type ChatController struct {
	app               application.Application
	chatService       services.ChatService
	chatRepo          domain.ChatRepository
	agentService      services.AgentService
	attachmentService services.AttachmentService
	artifactService   services.ArtifactService
	opts              ControllerOptions
}

// NewChatController creates a new chat controller.
// Services can be nil - they're optional for legacy REST endpoints.
func NewChatController(
	app application.Application,
	chatService services.ChatService,
	chatRepo domain.ChatRepository,
	agentService services.AgentService,
	attachmentService services.AttachmentService,
	artifactService services.ArtifactService,
	opts ...ControllerOption,
) *ChatController {
	return &ChatController{
		app:               app,
		chatService:       chatService,
		chatRepo:          chatRepo,
		agentService:      agentService,
		attachmentService: attachmentService,
		artifactService:   artifactService,
		opts:              applyControllerOptions(opts...),
	}
}

// Key returns the controller key for dependency injection.
func (c *ChatController) Key() string {
	return "bichat.ChatController"
}

// sessionResponse is the JSON shape for session endpoints
type sessionResponse struct {
	ID              string  `json:"id"`
	TenantID        string  `json:"tenant_id"`
	UserID          int64   `json:"user_id"`
	Title           string  `json:"title"`
	Status          string  `json:"status"`
	Pinned          bool    `json:"pinned"`
	ParentSessionID *string `json:"parent_session_id,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

func sessionToResponse(s domain.Session) sessionResponse {
	if s == nil {
		return sessionResponse{}
	}
	resp := sessionResponse{
		ID:        s.ID().String(),
		TenantID:  s.TenantID().String(),
		UserID:    s.UserID(),
		Title:     s.Title(),
		Status:    string(s.Status()),
		Pinned:    s.Pinned(),
		CreatedAt: s.CreatedAt().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: s.UpdatedAt().Format("2006-01-02T15:04:05Z07:00"),
	}
	if pid := s.ParentSessionID(); pid != nil {
		str := pid.String()
		resp.ParentSessionID = &str
	}
	return resp
}

// Register registers HTTP routes for the controller.
func (c *ChatController) Register(r *mux.Router) {
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

	// Session routes (legacy REST API).
	// BiChat applet UI uses applet RPC for request/response and StreamController for streaming.
	subRouter.HandleFunc("/sessions", c.ListSessions).Methods("GET")
	subRouter.HandleFunc("/sessions", c.CreateSession).Methods("POST")
	subRouter.HandleFunc("/sessions/{id}", c.GetSession).Methods("GET")
	subRouter.HandleFunc("/sessions/{id}/messages", c.SendMessage).Methods("POST")
	subRouter.HandleFunc("/sessions/{id}/resume", c.ResumeWithAnswer).Methods("POST")
	subRouter.HandleFunc("/sessions/{id}/archive", c.ArchiveSession).Methods("PUT")
	subRouter.HandleFunc("/sessions/{id}/pin", c.TogglePin).Methods("PUT")
	subRouter.HandleFunc("/sessions/{id}", c.DeleteSession).Methods("DELETE")
}

// Note: This controller is not registered by default. Prefer applet RPC for applet UI.

// ListSessions returns all sessions for the current user.
func (c *ChatController) ListSessions(w http.ResponseWriter, r *http.Request) {
	const op serrors.Op = "ChatController.ListSessions"

	user, err := composables.UseUser(r.Context())
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusUnauthorized)
		return
	}
	if err := c.enforceAccess(r.Context()); err != nil {
		c.sendError(w, serrors.E(op, serrors.PermissionDenied, errors.New("access denied")), http.StatusForbidden)
		return
	}

	// Parse pagination params
	params := composables.UsePaginated(r)
	opts := domain.ListOptions{
		Limit:  params.Limit,
		Offset: params.Offset,
	}

	sessions, err := c.chatService.ListUserSessions(r.Context(), int64(user.ID()), opts)
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusInternalServerError)
		return
	}
	resp := make([]sessionResponse, len(sessions))
	for i, s := range sessions {
		resp[i] = sessionToResponse(s)
	}
	c.sendJSON(w, resp, http.StatusOK)
}

// CreateSession creates a new chat session.
func (c *ChatController) CreateSession(w http.ResponseWriter, r *http.Request) {
	const op serrors.Op = "ChatController.CreateSession"

	user, err := composables.UseUser(r.Context())
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusUnauthorized)
		return
	}
	if err := c.enforceAccess(r.Context()); err != nil {
		c.sendError(w, serrors.E(op, serrors.PermissionDenied, errors.New("access denied")), http.StatusForbidden)
		return
	}

	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusBadRequest)
		return
	}

	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusBadRequest)
		return
	}

	session, err := c.chatService.CreateSession(r.Context(), tenantID, int64(user.ID()), req.Title)
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusInternalServerError)
		return
	}
	c.sendJSON(w, sessionToResponse(session), http.StatusCreated)
}

// GetSession returns details for a specific session.
func (c *ChatController) GetSession(w http.ResponseWriter, r *http.Request) {
	const op serrors.Op = "ChatController.GetSession"

	user, err := composables.UseUser(r.Context())
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusUnauthorized)
		return
	}
	if err := c.enforceAccess(r.Context()); err != nil {
		c.sendError(w, serrors.E(op, serrors.PermissionDenied, errors.New("access denied")), http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	sessionID, err := uuid.Parse(vars["id"])
	if err != nil {
		c.sendError(w, serrors.E(op, errors.New("invalid session ID")), http.StatusBadRequest)
		return
	}

	session, err := c.chatService.GetSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, persistence.ErrSessionNotFound) {
			c.sendError(w, serrors.E(op, err), http.StatusNotFound)
		} else {
			c.sendError(w, serrors.E(op, err), http.StatusInternalServerError)
		}
		return
	}

	if session.UserID() != int64(user.ID()) && composables.CanUser(r.Context(), c.opts.ReadAllPermission) != nil {
		c.sendError(w, serrors.E(op, errors.New("access denied")), http.StatusForbidden)
		return
	}
	c.sendJSON(w, sessionToResponse(session), http.StatusOK)
}

// SendMessage sends a new message to a session.
func (c *ChatController) SendMessage(w http.ResponseWriter, r *http.Request) {
	const op serrors.Op = "ChatController.SendMessage"

	user, err := composables.UseUser(r.Context())
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusUnauthorized)
		return
	}
	if err := c.enforceAccess(r.Context()); err != nil {
		c.sendError(w, serrors.E(op, serrors.PermissionDenied, errors.New("access denied")), http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	sessionID, err := uuid.Parse(vars["id"])
	if err != nil {
		c.sendError(w, serrors.E(op, errors.New("invalid session ID")), http.StatusBadRequest)
		return
	}

	// Validate session ownership (CRITICAL SECURITY)
	session, err := c.chatRepo.GetSession(r.Context(), sessionID)
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusNotFound)
		return
	}

	if session.UserID() != int64(user.ID()) {
		if err := composables.CanUser(r.Context(), c.opts.ReadAllPermission); err != nil {
			c.sendError(w, serrors.E(op, serrors.PermissionDenied, errors.New("access denied")), http.StatusForbidden)
			return
		}
	}

	var req struct {
		Content     string                `json:"content"`
		Attachments []AttachmentUploadDTO `json:"attachments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusBadRequest)
		return
	}

	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusBadRequest)
		return
	}

	domainAttachments, err := convertAttachmentDTOs(
		r.Context(),
		c.attachmentService,
		req.Attachments,
		tenantID,
		uuid.Nil,
	)
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusBadRequest)
		return
	}

	response, err := c.chatService.SendMessage(r.Context(), services.SendMessageRequest{
		SessionID:   sessionID,
		UserID:      int64(user.ID()),
		Content:     req.Content,
		Attachments: domainAttachments,
	})
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusInternalServerError)
		return
	}

	c.sendJSON(w, response, http.StatusOK)
}

// ResumeWithAnswer resumes execution after HITL interrupt.
func (c *ChatController) ResumeWithAnswer(w http.ResponseWriter, r *http.Request) {
	const op serrors.Op = "ChatController.ResumeWithAnswer"

	user, err := composables.UseUser(r.Context())
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusUnauthorized)
		return
	}
	if err := c.enforceAccess(r.Context()); err != nil {
		c.sendError(w, serrors.E(op, serrors.PermissionDenied, errors.New("access denied")), http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	sessionID, err := uuid.Parse(vars["id"])
	if err != nil {
		c.sendError(w, serrors.E(op, errors.New("invalid session ID")), http.StatusBadRequest)
		return
	}

	// Validate session ownership (CRITICAL SECURITY)
	session, err := c.chatRepo.GetSession(r.Context(), sessionID)
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusNotFound)
		return
	}

	if session.UserID() != int64(user.ID()) {
		if err := composables.CanUser(r.Context(), c.opts.ReadAllPermission); err != nil {
			c.sendError(w, serrors.E(op, serrors.PermissionDenied, errors.New("access denied")), http.StatusForbidden)
			return
		}
	}

	var req struct {
		CheckpointID string            `json:"checkpointId"`
		Answers      map[string]string `json:"answers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusBadRequest)
		return
	}

	response, err := c.chatService.ResumeWithAnswer(r.Context(), services.ResumeRequest{
		SessionID:    sessionID,
		CheckpointID: req.CheckpointID,
		Answers:      req.Answers,
	})
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusInternalServerError)
		return
	}

	c.sendJSON(w, response, http.StatusOK)
}

// ArchiveSession archives a session.
func (c *ChatController) ArchiveSession(w http.ResponseWriter, r *http.Request) {
	const op serrors.Op = "ChatController.ArchiveSession"

	user, err := composables.UseUser(r.Context())
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusUnauthorized)
		return
	}
	if err := c.enforceAccess(r.Context()); err != nil {
		c.sendError(w, serrors.E(op, serrors.PermissionDenied, errors.New("access denied")), http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	sessionID, err := uuid.Parse(vars["id"])
	if err != nil {
		c.sendError(w, serrors.E(op, errors.New("invalid session ID")), http.StatusBadRequest)
		return
	}

	// Check permission (user owns session)
	session, err := c.chatService.GetSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, persistence.ErrSessionNotFound) {
			c.sendError(w, serrors.E(op, err), http.StatusNotFound)
		} else {
			c.sendError(w, serrors.E(op, err), http.StatusInternalServerError)
		}
		return
	}

	if session.UserID() != int64(user.ID()) {
		c.sendError(w, serrors.E(op, errors.New("access denied")), http.StatusForbidden)
		return
	}

	updatedSession, err := c.chatService.ArchiveSession(r.Context(), sessionID)
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusInternalServerError)
		return
	}
	c.sendJSON(w, sessionToResponse(updatedSession), http.StatusOK)
}

// TogglePin toggles the pinned status of a session.
func (c *ChatController) TogglePin(w http.ResponseWriter, r *http.Request) {
	const op serrors.Op = "ChatController.TogglePin"

	user, err := composables.UseUser(r.Context())
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusUnauthorized)
		return
	}
	if err := c.enforceAccess(r.Context()); err != nil {
		c.sendError(w, serrors.E(op, serrors.PermissionDenied, errors.New("access denied")), http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	sessionID, err := uuid.Parse(vars["id"])
	if err != nil {
		c.sendError(w, serrors.E(op, errors.New("invalid session ID")), http.StatusBadRequest)
		return
	}

	// Get current session
	session, err := c.chatService.GetSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, persistence.ErrSessionNotFound) {
			c.sendError(w, serrors.E(op, err), http.StatusNotFound)
		} else {
			c.sendError(w, serrors.E(op, err), http.StatusInternalServerError)
		}
		return
	}

	if session.UserID() != int64(user.ID()) {
		c.sendError(w, serrors.E(op, errors.New("access denied")), http.StatusForbidden)
		return
	}
	var updatedSession domain.Session
	if session.Pinned() {
		updatedSession, err = c.chatService.UnpinSession(r.Context(), sessionID)
	} else {
		updatedSession, err = c.chatService.PinSession(r.Context(), sessionID)
	}
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusInternalServerError)
		return
	}
	c.sendJSON(w, sessionToResponse(updatedSession), http.StatusOK)
}

// DeleteSession deletes a session and all its messages.
func (c *ChatController) DeleteSession(w http.ResponseWriter, r *http.Request) {
	const op serrors.Op = "ChatController.DeleteSession"

	user, err := composables.UseUser(r.Context())
	if err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusUnauthorized)
		return
	}
	if err := c.enforceAccess(r.Context()); err != nil {
		c.sendError(w, serrors.E(op, serrors.PermissionDenied, errors.New("access denied")), http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	sessionID, err := uuid.Parse(vars["id"])
	if err != nil {
		c.sendError(w, serrors.E(op, errors.New("invalid session ID")), http.StatusBadRequest)
		return
	}

	// Check permission (user owns session)
	session, err := c.chatService.GetSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, persistence.ErrSessionNotFound) {
			c.sendError(w, serrors.E(op, err), http.StatusNotFound)
		} else {
			c.sendError(w, serrors.E(op, err), http.StatusInternalServerError)
		}
		return
	}

	if session.UserID() != int64(user.ID()) {
		c.sendError(w, serrors.E(op, errors.New("access denied")), http.StatusForbidden)
		return
	}

	// Delete session (cascades to messages/attachments)
	if err := c.chatRepo.DeleteSession(r.Context(), sessionID); err != nil {
		c.sendError(w, serrors.E(op, err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Helper methods

func (c *ChatController) sendJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode JSON response", "err", err)
	}
}

func (c *ChatController) sendError(w http.ResponseWriter, err error, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if encErr := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); encErr != nil {
		slog.Error("failed to encode error response", "err", encErr)
	}
}

func (c *ChatController) enforceAccess(ctx context.Context) error {
	if c.opts.RequireAccessPermission == nil {
		return nil
	}
	return composables.CanUser(ctx, c.opts.RequireAccessPermission)
}
