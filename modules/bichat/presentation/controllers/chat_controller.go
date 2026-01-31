package controllers

import (
	"net/http"

	"github.com/iota-uz/iota-sdk/pkg/application"
)

// ChatController handles HTTP and GraphQL endpoints for chat operations.
// TODO: Implement when Phase 1 (Agent Framework) is complete.
//
// Responsibilities:
//   - Session management (create, list, archive, pin)
//   - Message sending (sync and async)
//   - Resume after HITL interrupts
//   - Permission checks via middleware
//   - Tenant isolation via composables.UseTenantID
//
// Dependencies:
//   - ChatService from pkg/bichat/services
//   - AgentService from pkg/bichat/services
//   - Middleware for auth and permissions
type ChatController struct {
	app application.Application
	// chatService services.ChatService  // TODO: Uncomment when Phase 1 is complete
	// agentService services.AgentService // TODO: Uncomment when Phase 1 is complete
}

// NewChatController creates a new chat controller.
// TODO: Update constructor signature when Phase 1 is complete.
func NewChatController(app application.Application) *ChatController {
	return &ChatController{
		app: app,
	}
}

// RegisterRoutes registers HTTP routes for the controller.
// TODO: Implement when Phase 1 is complete.
//
// Routes:
//
//	GET    /bichat/sessions           - List user sessions
//	POST   /bichat/sessions           - Create new session
//	GET    /bichat/sessions/:id       - Get session details
//	POST   /bichat/sessions/:id/messages - Send message
//	POST   /bichat/sessions/:id/resume   - Resume after HITL
//	PUT    /bichat/sessions/:id/archive  - Archive session
//	PUT    /bichat/sessions/:id/pin      - Pin/unpin session
//	DELETE /bichat/sessions/:id        - Delete session
func (c *ChatController) RegisterRoutes() {
	// TODO: Implement route registration
	// Example:
	// router := c.app.Router()
	// subRouter := router.PathPrefix("/bichat").Subrouter()
	//
	// // Apply auth middleware
	// subRouter.Use(middleware.Authorize())
	// subRouter.Use(middleware.RequirePermission(permissions.BIChatAccess))
	//
	// // Session routes
	// subRouter.HandleFunc("/sessions", c.ListSessions).Methods("GET")
	// subRouter.HandleFunc("/sessions", c.CreateSession).Methods("POST")
	// subRouter.HandleFunc("/sessions/{id}", c.GetSession).Methods("GET")
	// subRouter.HandleFunc("/sessions/{id}/messages", c.SendMessage).Methods("POST")
	// subRouter.HandleFunc("/sessions/{id}/resume", c.ResumeWithAnswer).Methods("POST")
	// subRouter.HandleFunc("/sessions/{id}/archive", c.ArchiveSession).Methods("PUT")
	// subRouter.HandleFunc("/sessions/{id}/pin", c.TogglePin).Methods("PUT")
	// subRouter.HandleFunc("/sessions/{id}", c.DeleteSession).Methods("DELETE")
}

// ListSessions returns all sessions for the current user.
// TODO: Implement when Phase 1 is complete.
func (c *ChatController) ListSessions(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	// 1. Get user ID from composables.UseUser(ctx)
	// 2. Parse pagination params (limit, offset)
	// 3. Call chatService.ListUserSessions()
	// 4. Return sessions as JSON
	http.Error(w, "Not implemented - Phase 1 pending", http.StatusNotImplemented)
}

// CreateSession creates a new chat session.
// TODO: Implement when Phase 1 is complete.
func (c *ChatController) CreateSession(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	// 1. Get user ID and tenant ID from context
	// 2. Parse title from request body
	// 3. Call chatService.CreateSession()
	// 4. Return session as JSON
	http.Error(w, "Not implemented - Phase 1 pending", http.StatusNotImplemented)
}

// GetSession returns details for a specific session.
// TODO: Implement when Phase 1 is complete.
func (c *ChatController) GetSession(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	// 1. Parse session ID from URL params
	// 2. Call chatService.GetSession()
	// 3. Check permission (user owns session or has read_all permission)
	// 4. Return session as JSON
	http.Error(w, "Not implemented - Phase 1 pending", http.StatusNotImplemented)
}

// SendMessage sends a new message to a session.
// TODO: Implement when Phase 1 is complete.
func (c *ChatController) SendMessage(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	// 1. Parse session ID from URL
	// 2. Parse message content and attachments from request
	// 3. Call chatService.SendMessage()
	// 4. Handle interrupt (if present)
	// 5. Return response as JSON
	http.Error(w, "Not implemented - Phase 1 pending", http.StatusNotImplemented)
}

// ResumeWithAnswer resumes execution after HITL interrupt.
// TODO: Implement when Phase 1 is complete.
func (c *ChatController) ResumeWithAnswer(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	// 1. Parse session ID and checkpoint ID from request
	// 2. Parse user answers from request body
	// 3. Call chatService.ResumeWithAnswer()
	// 4. Return response as JSON
	http.Error(w, "Not implemented - Phase 1 pending", http.StatusNotImplemented)
}

// ArchiveSession archives a session.
// TODO: Implement when Phase 1 is complete.
func (c *ChatController) ArchiveSession(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	// 1. Parse session ID from URL
	// 2. Check permission (user owns session)
	// 3. Call chatService.ArchiveSession()
	// 4. Return updated session as JSON
	http.Error(w, "Not implemented - Phase 1 pending", http.StatusNotImplemented)
}

// TogglePin toggles the pinned status of a session.
// TODO: Implement when Phase 1 is complete.
func (c *ChatController) TogglePin(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	// 1. Parse session ID from URL
	// 2. Get current pinned status
	// 3. Call chatService.PinSession() or UnpinSession()
	// 4. Return updated session as JSON
	http.Error(w, "Not implemented - Phase 1 pending", http.StatusNotImplemented)
}

// DeleteSession deletes a session and all its messages.
// TODO: Implement when Phase 1 is complete.
func (c *ChatController) DeleteSession(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	// 1. Parse session ID from URL
	// 2. Check permission (user owns session)
	// 3. Call repository.DeleteSession() (cascades to messages/attachments)
	// 4. Return 204 No Content
	http.Error(w, "Not implemented - Phase 1 pending", http.StatusNotImplemented)
}
