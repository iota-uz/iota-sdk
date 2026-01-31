package controllers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/bichat/permissions"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

// ChatController handles HTTP endpoints for chat operations.
type ChatController struct {
	app         application.Application
	chatService services.ChatService
	chatRepo    domain.ChatRepository
}

// NewChatController creates a new chat controller.
func NewChatController(
	app application.Application,
	chatService services.ChatService,
	chatRepo domain.ChatRepository,
) *ChatController {
	return &ChatController{
		app:         app,
		chatService: chatService,
		chatRepo:    chatRepo,
	}
}

// Key returns the controller key for dependency injection.
func (c *ChatController) Key() string {
	return "bichat.ChatController"
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

	subRouter := r.PathPrefix("/bichat").Subrouter()
	subRouter.Use(commonMiddleware...)

	// Session routes
	subRouter.HandleFunc("/sessions", c.ListSessions).Methods("GET")
	subRouter.HandleFunc("/sessions", c.CreateSession).Methods("POST")
	subRouter.HandleFunc("/sessions/{id}", c.GetSession).Methods("GET")
	subRouter.HandleFunc("/sessions/{id}/messages", c.SendMessage).Methods("POST")
	subRouter.HandleFunc("/sessions/{id}/resume", c.ResumeWithAnswer).Methods("POST")
	subRouter.HandleFunc("/sessions/{id}/archive", c.ArchiveSession).Methods("PUT")
	subRouter.HandleFunc("/sessions/{id}/pin", c.TogglePin).Methods("PUT")
	subRouter.HandleFunc("/sessions/{id}", c.DeleteSession).Methods("DELETE")
}

// ListSessions returns all sessions for the current user.
func (c *ChatController) ListSessions(w http.ResponseWriter, r *http.Request) {
	const op = "ChatController.ListSessions"

	user, err := composables.UseUser(r.Context())
	if err != nil {
		c.sendError(w, err, http.StatusUnauthorized)
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
		c.sendError(w, err, http.StatusInternalServerError)
		return
	}

	c.sendJSON(w, sessions, http.StatusOK)
}

// CreateSession creates a new chat session.
func (c *ChatController) CreateSession(w http.ResponseWriter, r *http.Request) {
	const op = "ChatController.CreateSession"

	user, err := composables.UseUser(r.Context())
	if err != nil {
		c.sendError(w, err, http.StatusUnauthorized)
		return
	}

	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		c.sendError(w, err, http.StatusBadRequest)
		return
	}

	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.sendError(w, err, http.StatusBadRequest)
		return
	}

	session, err := c.chatService.CreateSession(r.Context(), tenantID, int64(user.ID()), req.Title)
	if err != nil {
		c.sendError(w, err, http.StatusInternalServerError)
		return
	}

	c.sendJSON(w, session, http.StatusCreated)
}

// GetSession returns details for a specific session.
func (c *ChatController) GetSession(w http.ResponseWriter, r *http.Request) {
	const op = "ChatController.GetSession"

	user, err := composables.UseUser(r.Context())
	if err != nil {
		c.sendError(w, err, http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	sessionID, err := uuid.Parse(vars["id"])
	if err != nil {
		c.sendError(w, errors.New("invalid session ID"), http.StatusBadRequest)
		return
	}

	session, err := c.chatService.GetSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, persistence.ErrSessionNotFound) {
			c.sendError(w, err, http.StatusNotFound)
		} else {
			c.sendError(w, err, http.StatusInternalServerError)
		}
		return
	}

	// Check permission (user owns session or has read_all permission)
	if session.UserID != int64(user.ID()) && composables.CanUser(r.Context(), permissions.BiChatReadAll) != nil {
		c.sendError(w, errors.New("access denied"), http.StatusForbidden)
		return
	}

	c.sendJSON(w, session, http.StatusOK)
}

// SendMessage sends a new message to a session.
func (c *ChatController) SendMessage(w http.ResponseWriter, r *http.Request) {
	const op = "ChatController.SendMessage"

	user, err := composables.UseUser(r.Context())
	if err != nil {
		c.sendError(w, err, http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	sessionID, err := uuid.Parse(vars["id"])
	if err != nil {
		c.sendError(w, errors.New("invalid session ID"), http.StatusBadRequest)
		return
	}

	var req struct {
		Content     string              `json:"content"`
		Attachments []domain.Attachment `json:"attachments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.sendError(w, err, http.StatusBadRequest)
		return
	}

	response, err := c.chatService.SendMessage(r.Context(), services.SendMessageRequest{
		SessionID:   sessionID,
		UserID:      int64(user.ID()),
		Content:     req.Content,
		Attachments: req.Attachments,
	})
	if err != nil {
		c.sendError(w, err, http.StatusInternalServerError)
		return
	}

	c.sendJSON(w, response, http.StatusOK)
}

// ResumeWithAnswer resumes execution after HITL interrupt.
func (c *ChatController) ResumeWithAnswer(w http.ResponseWriter, r *http.Request) {
	const op = "ChatController.ResumeWithAnswer"

	vars := mux.Vars(r)
	sessionID, err := uuid.Parse(vars["id"])
	if err != nil {
		c.sendError(w, errors.New("invalid session ID"), http.StatusBadRequest)
		return
	}

	var req struct {
		CheckpointID string            `json:"checkpointId"`
		Answers      map[string]string `json:"answers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.sendError(w, err, http.StatusBadRequest)
		return
	}

	response, err := c.chatService.ResumeWithAnswer(r.Context(), services.ResumeRequest{
		SessionID:    sessionID,
		CheckpointID: req.CheckpointID,
		Answers:      req.Answers,
	})
	if err != nil {
		c.sendError(w, err, http.StatusInternalServerError)
		return
	}

	c.sendJSON(w, response, http.StatusOK)
}

// ArchiveSession archives a session.
func (c *ChatController) ArchiveSession(w http.ResponseWriter, r *http.Request) {
	const op = "ChatController.ArchiveSession"

	user, err := composables.UseUser(r.Context())
	if err != nil {
		c.sendError(w, err, http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	sessionID, err := uuid.Parse(vars["id"])
	if err != nil {
		c.sendError(w, errors.New("invalid session ID"), http.StatusBadRequest)
		return
	}

	// Check permission (user owns session)
	session, err := c.chatService.GetSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, persistence.ErrSessionNotFound) {
			c.sendError(w, err, http.StatusNotFound)
		} else {
			c.sendError(w, err, http.StatusInternalServerError)
		}
		return
	}

	if session.UserID != int64(user.ID()) {
		c.sendError(w, errors.New("access denied"), http.StatusForbidden)
		return
	}

	updatedSession, err := c.chatService.ArchiveSession(r.Context(), sessionID)
	if err != nil {
		c.sendError(w, err, http.StatusInternalServerError)
		return
	}

	c.sendJSON(w, updatedSession, http.StatusOK)
}

// TogglePin toggles the pinned status of a session.
func (c *ChatController) TogglePin(w http.ResponseWriter, r *http.Request) {
	const op = "ChatController.TogglePin"

	user, err := composables.UseUser(r.Context())
	if err != nil {
		c.sendError(w, err, http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	sessionID, err := uuid.Parse(vars["id"])
	if err != nil {
		c.sendError(w, errors.New("invalid session ID"), http.StatusBadRequest)
		return
	}

	// Get current session
	session, err := c.chatService.GetSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, persistence.ErrSessionNotFound) {
			c.sendError(w, err, http.StatusNotFound)
		} else {
			c.sendError(w, err, http.StatusInternalServerError)
		}
		return
	}

	// Check permission (user owns session)
	if session.UserID != int64(user.ID()) {
		c.sendError(w, errors.New("access denied"), http.StatusForbidden)
		return
	}

	// Toggle pin status
	var updatedSession *domain.Session
	if session.Pinned {
		updatedSession, err = c.chatService.UnpinSession(r.Context(), sessionID)
	} else {
		updatedSession, err = c.chatService.PinSession(r.Context(), sessionID)
	}

	if err != nil {
		c.sendError(w, err, http.StatusInternalServerError)
		return
	}

	c.sendJSON(w, updatedSession, http.StatusOK)
}

// DeleteSession deletes a session and all its messages.
func (c *ChatController) DeleteSession(w http.ResponseWriter, r *http.Request) {
	const op = "ChatController.DeleteSession"

	user, err := composables.UseUser(r.Context())
	if err != nil {
		c.sendError(w, err, http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	sessionID, err := uuid.Parse(vars["id"])
	if err != nil {
		c.sendError(w, errors.New("invalid session ID"), http.StatusBadRequest)
		return
	}

	// Check permission (user owns session)
	session, err := c.chatService.GetSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, persistence.ErrSessionNotFound) {
			c.sendError(w, err, http.StatusNotFound)
		} else {
			c.sendError(w, err, http.StatusInternalServerError)
		}
		return
	}

	if session.UserID != int64(user.ID()) {
		c.sendError(w, errors.New("access denied"), http.StatusForbidden)
		return
	}

	// Delete session (cascades to messages/attachments)
	if err := c.chatRepo.DeleteSession(r.Context(), sessionID); err != nil {
		c.sendError(w, err, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Helper methods

func (c *ChatController) sendJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (c *ChatController) sendError(w http.ResponseWriter, err error, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"error": err.Error(),
	})
}
