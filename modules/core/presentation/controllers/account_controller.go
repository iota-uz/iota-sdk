// Package controllers provides this package.
package controllers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/components/scaffold/filterbuilder"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/account"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/cookies"
	"github.com/iota-uz/iota-sdk/pkg/filterq"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/serrors/serrorhttp"
)

type AccountController struct {
	app            application.Application
	userService    *services.UserService
	tenantService  *services.TenantService
	uploadService  *services.UploadService
	sessionService *services.SessionService
	cfg            *cookies.Config
	basePath       string
}

func NewAccountController(
	app application.Application,
	userService *services.UserService,
	tenantService *services.TenantService,
	uploadService *services.UploadService,
	sessionService *services.SessionService,
	cfg *cookies.Config,
) application.Controller {
	return &AccountController{
		app:            app,
		userService:    userService,
		tenantService:  tenantService,
		uploadService:  uploadService,
		sessionService: sessionService,
		cfg:            cfg,
		basePath:       "/account",
	}
}

func (c *AccountController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor(
		"core.account",
		0,
		application.Route("", c.basePath),
		application.Route("", c.basePath+"/password"),
		application.Route("", c.basePath+"/sessions"),
	).WithNav(
		application.NavNode{
			ID:       "core.account",
			TitleKey: "Account.Meta.Index.Title",
			Path:     c.basePath,
			Surfaces: spotlightOnlySurface(),
		},
		application.NavNode{
			ID:       "core.account.password",
			Parent:   "core.account",
			TitleKey: "Account.ChangePassword.Title",
			Path:     c.basePath + "/password",
			Surfaces: spotlightOnlySurface(),
		},
		application.NavNode{
			ID:       "core.account.sessions",
			Parent:   "core.account",
			TitleKey: "Account.Sessions.Title",
			Path:     c.basePath + "/sessions",
			Surfaces: spotlightOnlySurface(),
		},
	)
}

func (c *AccountController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}
	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)
	getRouter.HandleFunc("", c.Get).Methods(http.MethodGet)
	getRouter.HandleFunc("/password", c.GetPassword).Methods(http.MethodGet)
	getRouter.HandleFunc("/sessions", c.GetSessions).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.HandleFunc("", c.Update).Methods(http.MethodPost)
	setRouter.HandleFunc("/password", c.ChangePassword).Methods(http.MethodPost)

	deleteRouter := r.PathPrefix(c.basePath).Subrouter()
	deleteRouter.Use(commonMiddleware...)
	// Register specific routes before parameterized ones to avoid route conflicts
	deleteRouter.HandleFunc("/sessions/others", c.RevokeOtherSessions).Methods(http.MethodDelete)
	deleteRouter.HandleFunc("/sessions/{token}", c.RevokeSession).Methods(http.MethodDelete)
}

func (c *AccountController) ChangePassword(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())
	dto, err := composables.UseForm(&dtos.ChangePasswordDTO{}, r)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if fieldErrors, ok := dto.Ok(r.Context()); !ok {
		templ.Handler(account.PasswordForm(c.basePath+"/password", fieldErrors)).ServeHTTP(w, r)
		return
	}
	if err := c.userService.ChangePassword(r.Context(), dto.CurrentPassword, dto.NewPassword); err != nil {
		if errors.Is(err, services.ErrCurrentPasswordMismatch) {
			pageCtx := composables.UsePageCtx(r.Context())
			templ.Handler(account.PasswordForm(c.basePath+"/password", map[string]string{
				"CurrentPassword": pageCtx.T("Account.ChangePassword.Errors.CurrentIncorrect"),
			})).ServeHTTP(w, r)
			return
		}
		logger.WithError(err).Error("failed to change password")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Every session is gone now, so the confirmation waits on the login page.
	// The flash is set for "/": by default the cookie would stay under
	// /account and the login page would never see it.
	http.SetCookie(w, &http.Cookie{
		Name: "notice", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Value: base64.URLEncoding.EncodeToString([]byte(composables.UsePageCtx(r.Context()).T("Account.ChangePassword.Success"))),
	})
	http.SetCookie(w, &http.Cookie{
		Name: c.cfg.SID, Value: "", Path: "/", Domain: c.cfg.Domain,
		MaxAge: -1, Expires: time.Unix(0, 0), HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	if htmx.IsHxRequest(r) {
		htmx.Redirect(w, "/login")
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (c *AccountController) defaultProps(r *http.Request, errors map[string]string) (*account.ProfilePageProps, error) {
	u, err := composables.UseUser(r.Context())
	if err != nil {
		return nil, err
	}
	return c.profileProps(mappers.UserToViewModel(u), errors), nil
}

func (c *AccountController) profileProps(u *viewmodels.User, errors map[string]string) *account.ProfilePageProps {
	if errors == nil {
		errors = map[string]string{}
	}
	return &account.ProfilePageProps{
		PostPath:  c.basePath,
		User:      u,
		Errors:    errors,
		Languages: intl.GetSupportedLanguages(c.app.GetSupportedLanguages()),
	}
}

// Get serves the «Profile» tab. The ?tab= links of the single page it used to
// be go to the tab's own address.
func (c *AccountController) Get(w http.ResponseWriter, r *http.Request) {
	if name := r.URL.Query().Get("tab"); name != "" {
		if t, ok := account.TabByName(name); ok && t.Tab != account.TabProfile {
			http.Redirect(w, r, t.Path, http.StatusFound)
			return
		}
	}
	props, err := c.defaultProps(r, nil)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}
	renderAccountTab(w, r, props.User, account.TabProfile, account.ProfileForm(props))
}

// GetPassword serves the «Password» tab.
func (c *AccountController) GetPassword(w http.ResponseWriter, r *http.Request) {
	u, err := composables.UseUser(r.Context())
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}
	renderAccountTab(w, r, mappers.UserToViewModel(u), account.TabPassword, account.PasswordForm(account.PasswordPath, map[string]string{}))
}

// isAccountTabSwitch reports whether the request is a tab switch: htmx aims it
// at the tab panel. Other htmx requests of the same URLs (a section's own
// refresh, a filter change) keep their own answers.
func isAccountTabSwitch(r *http.Request) bool {
	return htmx.IsHxRequest(r) && !htmx.IsHistoryRestoreRequest(r) && htmx.Target(r) == account.TabPanelID
}

// renderAccountTab answers with the tab alone on a tab switch and with the
// whole account page otherwise.
func renderAccountTab(w http.ResponseWriter, r *http.Request, u *viewmodels.User, tab account.Tab, content templ.Component) {
	if isAccountTabSwitch(r) {
		templ.Handler(account.TabContent(tab, content)).ServeHTTP(w, r)
		return
	}
	templ.Handler(account.Page(account.ShellProps{User: u, Active: tab}, content)).ServeHTTP(w, r)
}

func (c *AccountController) Update(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())
	dto, err := composables.UseForm(&dtos.SaveAccountDTO{}, r)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusBadRequest, nil)
		return
	}
	u, err := composables.UseUser(r.Context())
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}
	if errors, ok := dto.Ok(r.Context()); !ok {
		logger.WithField("errors", errors).Debug("validation failed")
		// Keep what the person typed next to the errors.
		vm := mappers.UserToViewModel(u)
		vm.FirstName, vm.LastName, vm.MiddleName = dto.FirstName, dto.LastName, dto.MiddleName
		vm.Email, vm.Phone, vm.Language = dto.Email, dto.Phone, dto.Language
		templ.Handler(account.ProfileForm(c.profileProps(vm, errors))).ServeHTTP(w, r)
		return
	}
	entity, err := dto.Apply(u)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}

	if cannot := composables.CanUser(r.Context(), permissions.UserUpdate); cannot != nil {
		if u.Email() != nil && u.Email().Value() != "" && u.Email().Value() != entity.Email().Value() {
			logger.Error("normal user can not change non empty email")
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		if u.Phone() != nil && u.Phone().Value() != "" && u.Phone().Value() != entity.Phone().Value() {
			logger.Error("normal user can not change non empty phone")
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
	}

	updated, err := c.userService.UpdateSelf(r.Context(), entity)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}
	if updated == nil {
		updated = entity
	}

	// A new language changes every string on the page, a new photo the navbar
	// avatar too: both need the whole page again.
	if u.UILanguage() != updated.UILanguage() || u.AvatarID() != updated.AvatarID() {
		htmx.Refresh(w)
		return
	}

	pageCtx := composables.UsePageCtx(r.Context())
	htmx.ToastSuccess(w, pageCtx.T("Account.Profile.Saved"), "")
	vm := mappers.UserToViewModel(updated)
	templ.Handler(account.ProfileSaved(c.profileProps(vm, nil))).ServeHTTP(w, r)
}

func (c *AccountController) GetSessions(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())

	// Get current user
	user, err := composables.UseUser(r.Context())
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}

	currentSession, err := c.currentSession(r)
	if err != nil {
		logger.WithError(err).Error("failed to get current session")
		http.Error(w, "Session not found", http.StatusUnauthorized)
		return
	}
	currentToken := currentSession.Token()

	// Fetch all sessions for the user
	sessions, err := c.sessionService.GetByUserID(r.Context(), user.ID())
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}

	// The user's sessions are few and already loaded, so the chips filter
	// them in memory.
	registry := accountSessionFilterRegistry(r)
	conditions := registry.Decode(r.URL.Query())
	now := time.Now()
	if loc, ok := composables.UseLocation(r.Context()); ok {
		now = now.In(loc)
	}

	// Map sessions to ViewModels
	sessionVMs := make([]*viewmodels.Session, 0, len(sessions))
	hasOtherSessions := false
	for _, s := range sessions {
		isCurrent := s.Token() == currentToken
		if !isCurrent {
			hasOtherSessions = true
		}
		if !accountSessionMatches(conditions, s, isCurrent, now) {
			continue
		}
		sessionVMs = append(sessionVMs, viewmodels.SessionToViewModelContext(r.Context(), s, currentToken))
	}

	// Render sessions page
	props := &account.SessionsPageProps{
		Sessions:         sessionVMs,
		CurrentToken:     currentToken,
		HasOtherSessions: hasOtherSessions,
		Errors:           make(map[string]string),
		Filters: filterbuilder.Props{
			Registry: registry, Filters: conditions, ID: "account-session-filters", ShowResetAlways: true,
		},
	}

	// A chip change re-renders the list; a tab switch, a page load or a history
	// restore (cache miss on Back) get the tab.
	if !htmx.WantsFullPage(r) && !isAccountTabSwitch(r) {
		templ.Handler(account.SessionsListWithChips(props)).ServeHTTP(w, r)
		return
	}
	renderAccountTab(w, r, mappers.UserToViewModel(user), account.TabSessions, account.Sessions(props))
}

// Filter chip field keys of the account sessions list.
const (
	accountSessionFilterCreatedAt = "created_at"
	accountSessionFilterExpiresAt = "expires_at"
	accountSessionFilterCurrent   = "current"
)

// accountSessionFilterRegistry describes the chips of the user's own
// sessions: when a session was signed in, when it expires and whether it is
// the session of this browser.
func accountSessionFilterRegistry(r *http.Request) *filterbuilder.Registry {
	pageCtx := composables.UsePageCtx(r.Context())
	return filterbuilder.NewRegistry(
		filterbuilder.FieldDef{Key: accountSessionFilterCreatedAt, Type: filterq.FieldTypeDate, Label: pageCtx.T("Account.Sessions.CreatedAt")},
		filterbuilder.FieldDef{Key: accountSessionFilterExpiresAt, Type: filterq.FieldTypeDate, Label: pageCtx.T("Account.Sessions.ExpiresAt")},
		filterbuilder.FieldDef{Key: accountSessionFilterCurrent, Type: filterq.FieldTypeBool, Label: pageCtx.T("Account.Sessions.CurrentSession")},
	)
}

// accountSessionMatches reports whether a session satisfies every chip.
func accountSessionMatches(fs filterq.FilterSet, s session.Session, isCurrent bool, now time.Time) bool {
	for _, c := range fs {
		var ok bool
		switch c.Field {
		case accountSessionFilterCreatedAt:
			ok = c.MatchDate(s.CreatedAt(), now)
		case accountSessionFilterExpiresAt:
			ok = c.MatchDate(s.ExpiresAt(), now)
		case accountSessionFilterCurrent:
			ok = c.MatchBool(isCurrent)
		default:
			ok = true
		}
		if !ok {
			return false
		}
	}
	return true
}

func (c *AccountController) RevokeSession(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())
	pageCtx := composables.UsePageCtx(r.Context())

	// Get token from URL path
	vars := mux.Vars(r)
	tokenHash := vars["token"]

	currentSession, err := c.currentSession(r)
	if err != nil {
		logger.WithError(err).Error("failed to get current session")
		http.Error(w, "Session not found", http.StatusUnauthorized)
		return
	}
	currentToken := currentSession.Token()

	// Get current user
	user, err := composables.UseUser(r.Context())
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}

	// Fetch all sessions to find the one to revoke
	sessions, err := c.sessionService.GetByUserID(r.Context(), user.ID())
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}

	// Find the session to revoke by comparing token hash (TokenID) from URL
	var sessionToRevoke string
	for _, s := range sessions {
		sessionVM := viewmodels.SessionToViewModelContext(r.Context(), s, currentToken)
		if sessionVM.TokenID == tokenHash {
			sessionToRevoke = s.Token()
			break
		}
	}

	if sessionToRevoke == "" {
		logger.Error("session not found")
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Prevent revoking current session
	if sessionToRevoke == currentToken {
		logger.Error("cannot revoke current session")
		errorPayload, err := json.Marshal(map[string]string{
			"message": pageCtx.T("Account.Sessions.CannotRevokeCurrent"),
		})
		if err != nil {
			logger.WithError(err).Error("failed to marshal session revoke toast")
		} else {
			htmx.SetTrigger(w, "error", string(errorPayload))
		}
		http.Error(w, pageCtx.T("Account.Sessions.CannotRevokeCurrent"), http.StatusForbidden)
		return
	}

	// Revoke the session
	if err := c.sessionService.TerminateSession(r.Context(), sessionToRevoke); err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}

	// Return success with HTMX trigger
	successPayload, err := json.Marshal(map[string]string{
		"message": pageCtx.T("Account.Sessions.RevokeSuccess"),
	})
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}
	htmx.SetTrigger(w, "success", string(successPayload))
	w.WriteHeader(http.StatusOK)
}

func (c *AccountController) RevokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())
	pageCtx := composables.UsePageCtx(r.Context())

	// Get current user
	user, err := composables.UseUser(r.Context())
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}

	currentSession, err := c.currentSession(r)
	if err != nil {
		logger.WithError(err).Error("failed to get current session")
		http.Error(w, "Session not found", http.StatusUnauthorized)
		return
	}
	currentToken := currentSession.Token()

	// Terminate all other sessions
	count, err := c.sessionService.TerminateOtherSessions(r.Context(), user.ID(), currentToken)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}

	// Return success with count and refresh page
	successMsg := fmt.Sprintf(pageCtx.T("Account.Sessions.RevokeAllSuccess"), count)
	successPayload, err := json.Marshal(map[string]string{"message": successMsg})
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}
	htmx.SetTrigger(w, "success", string(successPayload))
	htmx.Refresh(w)
}

func (c *AccountController) currentSession(r *http.Request) (session.Session, error) {
	const op serrors.Op = "accountController.currentSession"

	cookieName := "sid"
	if c.cfg != nil && c.cfg.SID != "" {
		cookieName = c.cfg.SID
	}
	if _, err := r.Cookie(cookieName); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	sess, err := composables.UseSession(r.Context())
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return sess, nil
}
