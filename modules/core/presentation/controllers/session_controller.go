package controllers

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	sessiontemplates "github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/sessions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type SessionController struct {
	app      application.Application
	basePath string
}

func NewSessionController(app application.Application, basePath string) application.Controller {
	return &SessionController{
		app:      app,
		basePath: basePath,
	}
}

func (c *SessionController) Key() string {
	return c.basePath
}

func (c *SessionController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(c.app),
		middleware.ProvideLocalizer(c.app),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)

	router.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
	router.HandleFunc("/{token}", di.H(c.Revoke)).Methods(http.MethodDelete)
}

func (c *SessionController) List(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	sessionService *services.SessionService,
	userService *services.UserService,
) {
	// Check permission
	if err := composables.CanUser(r.Context(), permissions.SessionRead); err != nil {
		logger.WithError(err).Error("User lacks SessionRead permission")
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	// Parse query parameters
	params := composables.UsePaginated(r)
	searchQuery := r.URL.Query().Get("search")

	// Build session find params
	findParams := &session.FindParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		SortBy: session.SortBy{
			Fields: []session.SortByField{
				{
					Field:     session.CreatedAt,
					Ascending: false,
				},
			},
		},
	}

	// Get sessions
	sessions, err := sessionService.GetPaginated(r.Context(), findParams)
	if err != nil {
		logger.WithError(err).Error("Failed to fetch sessions")
		http.Error(w, "Error retrieving sessions", http.StatusInternalServerError)
		return
	}

	// Get current user's session token for highlighting
	config := configuration.Use()
	currentToken := ""
	if cookie, err := r.Cookie(config.SidCookieKey); err == nil {
		currentToken = cookie.Value
	}

	// Build admin session view models with user info
	adminSessions := make([]*viewmodels.AdminSessionViewModel, 0, len(sessions))
	for _, sess := range sessions {
		// Get user info
		user, err := userService.GetByID(r.Context(), sess.UserID())
		if err != nil {
			logger.WithError(err).Warnf("Failed to fetch user %d for session", sess.UserID())
			// Skip sessions with missing users
			continue
		}

		userVM := mappers.UserToViewModel(user)
		sessionVM := viewmodels.NewAdminSessionViewModel(sess, userVM, currentToken)

		// Apply search filter if provided
		if searchQuery != "" {
			if !containsSearch(userVM, searchQuery) {
				continue
			}
		}

		adminSessions = append(adminSessions, sessionVM)
	}

	// Get total count for pagination
	totalCount, err := sessionService.GetCount(r.Context())
	if err != nil {
		logger.WithError(err).Error("Failed to get session count")
		http.Error(w, "Error retrieving session count", http.StatusInternalServerError)
		return
	}

	// Build page props
	props := &sessiontemplates.IndexPageProps{
		Sessions:    adminSessions,
		Page:        params.Page,
		PerPage:     params.Limit,
		TotalCount:  int(totalCount),
		HasMore:     int64(params.Page*params.Limit) < totalCount,
		SearchQuery: searchQuery,
	}

	// Render template
	if htmx.IsHxRequest(r) {
		if params.Page > 1 {
			templ.Handler(sessiontemplates.SessionRows(props), templ.WithStreaming()).ServeHTTP(w, r)
		} else {
			if htmx.Target(r) == "sessions-table-body" {
				templ.Handler(sessiontemplates.SessionRows(props), templ.WithStreaming()).ServeHTTP(w, r)
			} else {
				templ.Handler(sessiontemplates.SessionsContent(props), templ.WithStreaming()).ServeHTTP(w, r)
			}
		}
	} else {
		templ.Handler(sessiontemplates.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *SessionController) Revoke(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	sessionService *services.SessionService,
) {
	// Check permission
	if err := composables.CanUser(r.Context(), permissions.SessionDelete); err != nil {
		logger.WithError(err).Error("User lacks SessionDelete permission")
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	// Parse token from path
	vars := mux.Vars(r)
	token := vars["token"]
	if token == "" {
		logger.Error("Missing session token")
		http.Error(w, "Missing session token", http.StatusBadRequest)
		return
	}

	// Terminate session
	if err := sessionService.TerminateSession(r.Context(), token); err != nil {
		logger.WithError(err).Error("Failed to terminate session")
		http.Error(w, "Error terminating session", http.StatusInternalServerError)
		return
	}

	logger.
		WithField("token", token).
		WithField("revoked_by", composables.MustUseUser(r.Context()).ID()).
		Info("Session revoked")

	// Return OOB swap to delete row using the hashed token ID
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	// Use hash of token for the ID (consistent with the FullToken field in viewmodel)
	tokenHash := hashToken(token)
	_, _ = w.Write([]byte(`<tr id="session-row-` + tokenHash + `" hx-swap-oob="delete"></tr>`))
}

// containsSearch checks if user matches the search query
func containsSearch(user *viewmodels.User, query string) bool {
	if query == "" {
		return true
	}

	// Convert to lowercase for case-insensitive search
	query = strings.ToLower(query)

	// Search in name and email
	if strings.Contains(strings.ToLower(user.FirstName), query) ||
		strings.Contains(strings.ToLower(user.LastName), query) ||
		strings.Contains(strings.ToLower(user.Email), query) {
		return true
	}

	return false
}

// hashToken creates a SHA-256 hash of the token for safe comparison
// Must match the implementation in viewmodels/session.go
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", hash)
}
