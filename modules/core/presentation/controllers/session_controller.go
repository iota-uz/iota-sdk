// Package controllers provides this package.
package controllers

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/components/scaffold/filterbuilder"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	sessiontemplates "github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/sessions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/cookies"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/filterq"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type SessionController struct {
	basePath string
	cfg      *cookies.Config
}

func NewSessionController(basePath string, cfg *cookies.Config) application.Controller {
	return &SessionController{
		basePath: basePath,
		cfg:      cfg,
	}
}

func (c *SessionController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("core.session", 0, application.Route("", c.basePath))
}

func (c *SessionController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
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
	query := r.URL.Query()
	// ?search= stays a plain search box; chips come on top as ?f=.
	searchQuery := query.Get("search")
	registry := c.filterRegistry(r, logger, userService)
	conditions := registry.Decode(query)

	// Build session find params
	findParams := &session.FindParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		Search: searchQuery,
		Filters: filterq.RepoFilters(conditions, registry.Schema(), map[string]filterq.RepoField[session.Field]{
			// Compared as text, so a hand-edited non-numeric id matches
			// nothing instead of failing the query.
			sessionFilterUser:    {Column: session.UserID, Expr: func(column string) string { return column + "::text" }},
			sessionFilterStatus:  {Column: session.Status},
			sessionFilterCreated: {Column: session.CreatedAt},
			sessionFilterExpires: {Column: session.ExpiresAt},
		}, time.Now()),
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
	currentToken := ""
	if currentSession, err := composables.UseSession(r.Context()); err == nil {
		currentToken = currentSession.Token()
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
		sessionVM := viewmodels.NewAdminSessionViewModelContext(r.Context(), sess, userVM, currentToken)

		adminSessions = append(adminSessions, sessionVM)
	}

	// Get total count for pagination: the same search and chips as the rows.
	totalCount, err := sessionService.CountFound(r.Context(), findParams)
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
		BasePath:    c.basePath,
		Query:       query,
		Filters: filterbuilder.Props{
			Registry: registry, Filters: conditions, ID: "session-filters", ShowResetAlways: true,
		},
	}

	// Render template
	var component templ.Component
	switch {
	case htmx.WantsFullPage(r):
		// A normal load, or "Back" to a pushed chip URL after a history-cache miss.
		component = sessiontemplates.Index(props)
	case params.Page > 1:
		// The next page of infinite scroll: rows only.
		component = sessiontemplates.SessionRows(props)
	case htmx.Target(r) == "sessions-table-body":
		component = sessiontemplates.SessionRowsWithChips(props)
	case htmx.Target(r) == "sessions-list":
		component = sessiontemplates.SessionsListWithChips(props)
	default:
		component = sessiontemplates.SessionsContent(props)
	}
	templ.Handler(component, templ.WithStreaming()).ServeHTTP(w, r)
}

// Filter chip fields of the admin sessions list.
const (
	sessionFilterUser    = "user"
	sessionFilterStatus  = "status"
	sessionFilterCreated = "created_at"
	sessionFilterExpires = "expires_at"
)

// filterRegistry describes the chips of the sessions list: whose session it
// is, its sign-in state and its dates. The user chip is offered only to
// someone who may read users, since its options name them.
func (c *SessionController) filterRegistry(r *http.Request, logger *logrus.Entry, userService *services.UserService) *filterbuilder.Registry {
	pageCtx := composables.UsePageCtx(r.Context())
	fields := make([]filterbuilder.FieldDef, 0, 4)
	if composables.CanUser(r.Context(), permissions.UserRead) == nil {
		users, err := userService.GetAll(r.Context())
		if err != nil {
			logger.WithError(err).Warn("Failed to load users for the session filters")
		}
		options := make([]filterbuilder.Option, 0, len(users))
		for _, u := range users {
			options = append(options, filterbuilder.Opt(strconv.FormatUint(uint64(u.ID()), 10), mappers.UserToViewModel(u).Title()))
		}
		fields = append(fields, filterbuilder.FieldDef{
			Key: sessionFilterUser, Type: filterq.FieldTypeReference, Label: pageCtx.T("Sessions.List.User"), Options: options,
		})
	}
	fields = append(fields,
		filterbuilder.FieldDef{
			Key: sessionFilterStatus, Type: filterq.FieldTypeReference, Label: pageCtx.T("Sessions.List.Status"),
			Options: []filterbuilder.Option{
				filterbuilder.Opt(string(session.StatusActive), pageCtx.T("Sessions.List.StatusActive")),
				filterbuilder.Opt(string(session.StatusPending2FA), pageCtx.T("Sessions.List.StatusPending2FA")),
				filterbuilder.Opt(string(session.StatusPendingOnboarding), pageCtx.T("Sessions.List.StatusPendingOnboarding")),
			},
		},
		filterbuilder.FieldDef{Key: sessionFilterCreated, Type: filterq.FieldTypeDate, Label: pageCtx.T("Sessions.List.CreatedAt")},
		filterbuilder.FieldDef{Key: sessionFilterExpires, Type: filterq.FieldTypeDate, Label: pageCtx.T("Sessions.List.ExpiresAt")},
	)
	return filterbuilder.NewRegistry(fields...)
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

	sess, err := sessionService.GetByToken(r.Context(), token)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve session")
		if errors.Is(err, persistence.ErrSessionNotFound) {
			http.Error(w, "Session not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error terminating session", http.StatusInternalServerError)
		}
		return
	}

	if err := sessionService.TerminateUserSession(r.Context(), sess.UserID(), token); err != nil {
		if respondPrivilegeDenied(w, r, err) {
			return
		}
		logger.WithError(err).Error("Failed to terminate session")
		if errors.Is(err, persistence.ErrSessionNotFound) {
			http.Error(w, "Session not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error terminating session", http.StatusInternalServerError)
		}
		return
	}

	logger.
		WithField("token_hash", hashToken(token)).
		WithField("revoked_by", composables.MustUseUser(r.Context()).ID()).
		Info("Session revoked")

	// Return OOB swap to delete row using the hashed token ID
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	// Use hash of token for the ID (consistent with the FullToken field in viewmodel)
	tokenHash := hashToken(token)
	_, _ = w.Write([]byte(`<tr id="session-row-` + tokenHash + `" hx-swap-oob="delete"></tr>`))
}

// hashToken creates a SHA-256 hash of the token for safe comparison
// Must match the implementation in viewmodels/session.go
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", hash)
}
