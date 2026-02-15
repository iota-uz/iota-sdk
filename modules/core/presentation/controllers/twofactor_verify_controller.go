package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/twofactorverify"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/core/services/twofactor"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/security"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	pkgtwofactor "github.com/iota-uz/iota-sdk/pkg/twofactor"
)

// NewTwoFactorVerifyController creates a new TwoFactorVerifyController.
// Initializes the controller with required service dependencies.
// Parameters:
//   - app: The application instance providing service registry
//
// Returns a configured TwoFactorVerifyController implementing the Controller interface.
func NewTwoFactorVerifyController(app application.Application) application.Controller {
	return &TwoFactorVerifyController{
		app:              app,
		twoFactorService: app.Service(twofactor.TwoFactorService{}).(*twofactor.TwoFactorService),
		sessionService:   app.Service(services.SessionService{}).(*services.SessionService),
		userService:      app.Service(services.UserService{}).(*services.UserService),
	}
}

// TwoFactorVerifyController handles HTTP endpoints for 2FA verification flows.
// Provides code verification, recovery code fallback, and OTP resend functionality.
// Routes are mounted at /login/2fa/verify and require authentication (pending 2FA session).
type TwoFactorVerifyController struct {
	app              application.Application
	twoFactorService *twofactor.TwoFactorService
	sessionService   *services.SessionService
	userService      *services.UserService
}

// Key returns the base route path for this controller.
// Implements the Controller interface.
func (c *TwoFactorVerifyController) Key() string {
	return "/login/2fa/verify"
}

// Register registers all HTTP routes for 2FA verification flows.
// Applies authentication, localization, transaction, and page context middleware.
// Routes:
//   - GET/POST /login/2fa/verify - Primary verification (TOTP/SMS/Email)
//   - GET/POST /login/2fa/verify/recovery - Recovery code fallback
//   - POST /login/2fa/verify/resend - Resend OTP (SMS/Email only)
//
// Implements the Controller interface.
func (c *TwoFactorVerifyController) Register(r *mux.Router) {
	verifyRouter := r.PathPrefix("/login/2fa/verify").Subrouter()
	verifyRouter.Use(
		middleware.AuthorizeAnySession(),
		middleware.ProvideLocalizer(c.app),
		middleware.WithPageContext(),
	)

	// Verification code endpoints
	verifyRouter.HandleFunc("", c.GetVerify).Methods(http.MethodGet)
	verifyRouter.HandleFunc("", c.PostVerify).Methods(http.MethodPost)

	// Recovery code endpoints
	verifyRouter.HandleFunc("/recovery", c.GetRecovery).Methods(http.MethodGet)
	verifyRouter.HandleFunc("/recovery", c.PostRecovery).Methods(http.MethodPost)

	// Resend OTP endpoint
	verifyRouter.HandleFunc("/resend", c.PostResend).Methods(http.MethodPost)
}

// GetVerify displays the verification form (adapts to user's configured method: TOTP/SMS/Email).
// For TOTP: shows code input form.
// For SMS/Email: generates and sends OTP, then shows code input form.
// GET /login/2fa/verify?next=/redirect/path
func (c *TwoFactorVerifyController) GetVerify(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())

	// Validate the redirect URL early to prevent open redirect attacks
	nextURL := security.GetValidatedRedirect(r.URL.Query().Get("next"))

	// Get session
	sess, err := composables.UseSession(r.Context())
	if err != nil {
		logger.Error("failed to get session", "error", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate session status
	if sess.Status() != session.StatusPending2FA {
		logger.Error("session not in pending 2FA status", "status", sess.Status())
		http.Error(w, "invalid session state", http.StatusBadRequest)
		return
	}

	// Get user
	u, err := c.userService.GetByID(r.Context(), sess.UserID())
	if err != nil {
		logger.Error("failed to get user", "error", err)
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// Begin verification (generates OTP if SMS/Email)
	challenge, err := c.twoFactorService.BeginVerification(r.Context(), sess.UserID())
	if err != nil {
		logger.Error("failed to begin 2FA verification", "error", err)
		shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "TwoFactor.Verify.Error")))
		http.Redirect(w, r, fmt.Sprintf("/login/2fa/verify?next=%s", url.QueryEscape(nextURL)), http.StatusFound)
		return
	}

	// Render verification form
	method := u.TwoFactorMethod()
	if err := twofactorverify.Verify(&twofactorverify.VerifyProps{
		Method:      string(method),
		NextURL:     nextURL,
		Destination: challenge.Destination,
	}).Render(r.Context(), w); err != nil {
		logger.Error("failed to render verify template", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// PostVerify validates the verification code (TOTP or OTP).
// Verifies the code via TwoFactorService, updates session to active status on success,
// and redirects to the next URL.
// POST /login/2fa/verify with Code and NextURL form values.
func (c *TwoFactorVerifyController) PostVerify(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	code := r.FormValue("Code")
	// Validate the redirect URL to prevent open redirect attacks
	nextURL := security.GetValidatedRedirect(r.FormValue("NextURL"))

	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	// Get session
	sess, err := composables.UseSession(r.Context())
	if err != nil {
		logger.Error("failed to get session", "error", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate session status
	if sess.Status() != session.StatusPending2FA {
		logger.Error("session not in pending 2FA status", "status", sess.Status())
		http.Error(w, "invalid session state", http.StatusBadRequest)
		return
	}

	// Verify the code
	err = c.twoFactorService.Verify(r.Context(), sess.UserID(), code)
	if err != nil {
		logger.Error("failed to verify 2FA code", "error", err)
		var errorMsg string
		if errors.Is(err, pkgtwofactor.ErrInvalidCode) {
			errorMsg = "TwoFactor.Verify.InvalidCode"
		} else {
			errorMsg = "TwoFactor.Verify.Error"
		}
		shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), errorMsg)))
		http.Redirect(w, r, fmt.Sprintf("/login/2fa/verify?next=%s", url.QueryEscape(nextURL)), http.StatusFound)
		return
	}

	// Update session to Active status with full session duration
	// Pending sessions have 10-minute TTL, active sessions get full duration
	conf := configuration.Use()
	updatedSession := session.New(
		sess.Token(),
		sess.UserID(),
		sess.TenantID(),
		sess.IP(),
		sess.UserAgent(),
		session.WithStatus(session.StatusActive),
		session.WithAudience(sess.Audience()),
		session.WithExpiresAt(time.Now().Add(conf.SessionDuration)),
		session.WithCreatedAt(sess.CreatedAt()),
	)

	if err := c.sessionService.Update(r.Context(), updatedSession); err != nil {
		logger.Error("failed to update session to active", "error", err)
		http.Error(w, "failed to activate session", http.StatusInternalServerError)
		return
	}

	// Update the session cookie with new expiry to match the extended DB session
	sessionCookie := &http.Cookie{
		Name:     conf.SidCookieKey,
		Value:    updatedSession.Token(),
		Expires:  updatedSession.ExpiresAt(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   conf.GoAppEnvironment == configuration.Production,
		Domain:   conf.Domain,
		Path:     "/",
	}
	http.SetCookie(w, sessionCookie)

	// Redirect to next URL (already validated earlier)
	shared.SetFlash(w, "success", []byte(intl.MustT(r.Context(), "TwoFactor.Verify.Success")))
	http.Redirect(w, r, nextURL, http.StatusFound)
}

// GetRecovery displays the recovery code input form.
// Used as a fallback when users cannot access their primary 2FA method.
// GET /login/2fa/verify/recovery?next=/redirect/path
func (c *TwoFactorVerifyController) GetRecovery(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())

	// Validate the redirect URL early to prevent open redirect attacks
	nextURL := security.GetValidatedRedirect(r.URL.Query().Get("next"))

	// Get session
	sess, err := composables.UseSession(r.Context())
	if err != nil {
		logger.Error("failed to get session", "error", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate session status
	if sess.Status() != session.StatusPending2FA {
		logger.Error("session not in pending 2FA status", "status", sess.Status())
		http.Error(w, "invalid session state", http.StatusBadRequest)
		return
	}

	// Render recovery code form
	if err := twofactorverify.Recovery(&twofactorverify.RecoveryProps{
		NextURL: nextURL,
	}).Render(r.Context(), w); err != nil {
		logger.Error("failed to render recovery template", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// PostRecovery validates the recovery code.
// Verifies the code via TwoFactorService, marks it as used, updates session to active status,
// and redirects to the next URL.
// POST /login/2fa/verify/recovery with Code and NextURL form values.
func (c *TwoFactorVerifyController) PostRecovery(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	code := r.FormValue("Code")
	// Validate the redirect URL to prevent open redirect attacks
	nextURL := security.GetValidatedRedirect(r.FormValue("NextURL"))

	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	// Get session
	sess, err := composables.UseSession(r.Context())
	if err != nil {
		logger.Error("failed to get session", "error", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate session status
	if sess.Status() != session.StatusPending2FA {
		logger.Error("session not in pending 2FA status", "status", sess.Status())
		http.Error(w, "invalid session state", http.StatusBadRequest)
		return
	}

	// Verify the recovery code
	err = c.twoFactorService.VerifyRecovery(r.Context(), sess.UserID(), code)
	if err != nil {
		logger.Error("failed to verify recovery code", "error", err)
		shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "TwoFactor.Verify.InvalidRecoveryCode")))
		http.Redirect(w, r, fmt.Sprintf("/login/2fa/verify/recovery?next=%s", url.QueryEscape(nextURL)), http.StatusFound)
		return
	}

	// Update session to Active status with full session duration
	// Pending sessions have 10-minute TTL, active sessions get full duration
	conf := configuration.Use()
	updatedSession := session.New(
		sess.Token(),
		sess.UserID(),
		sess.TenantID(),
		sess.IP(),
		sess.UserAgent(),
		session.WithStatus(session.StatusActive),
		session.WithAudience(sess.Audience()),
		session.WithExpiresAt(time.Now().Add(conf.SessionDuration)),
		session.WithCreatedAt(sess.CreatedAt()),
	)

	if err := c.sessionService.Update(r.Context(), updatedSession); err != nil {
		logger.Error("failed to update session to active", "error", err)
		http.Error(w, "failed to activate session", http.StatusInternalServerError)
		return
	}

	// Update the session cookie with new expiry to match the extended DB session
	sessionCookie := &http.Cookie{
		Name:     conf.SidCookieKey,
		Value:    updatedSession.Token(),
		Expires:  updatedSession.ExpiresAt(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   conf.GoAppEnvironment == configuration.Production,
		Domain:   conf.Domain,
		Path:     "/",
	}
	http.SetCookie(w, sessionCookie)

	// Redirect to next URL (already validated earlier)
	shared.SetFlash(w, "success", []byte(intl.MustT(r.Context(), "TwoFactor.Verify.Success")))
	http.Redirect(w, r, nextURL, http.StatusFound)
}

// PostResend resends the OTP code (for SMS/Email methods only).
// Generates and sends a new OTP to the user's registered contact.
// TOTP method returns an error (does not support resend).
// POST /login/2fa/verify/resend with NextURL form value.
func (c *TwoFactorVerifyController) PostResend(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the redirect URL to prevent open redirect attacks
	nextURL := security.GetValidatedRedirect(r.FormValue("NextURL"))

	// Get session
	sess, err := composables.UseSession(r.Context())
	if err != nil {
		logger.Error("failed to get session", "error", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate session status
	if sess.Status() != session.StatusPending2FA {
		logger.Error("session not in pending 2FA status", "status", sess.Status())
		http.Error(w, "invalid session state", http.StatusBadRequest)
		return
	}

	// Get user to check method
	u, err := c.userService.GetByID(r.Context(), sess.UserID())
	if err != nil {
		logger.Error("failed to get user", "error", err)
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	method := u.TwoFactorMethod()

	// Only allow resend for SMS/Email (not TOTP)
	if method != pkgtwofactor.MethodSMS && method != pkgtwofactor.MethodEmail {
		http.Error(w, "resend not available for this method", http.StatusBadRequest)
		return
	}

	// Begin verification again to resend OTP
	_, err = c.twoFactorService.BeginVerification(r.Context(), sess.UserID())
	if err != nil {
		logger.Error("failed to resend OTP", "error", err)
		shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "TwoFactor.Verify.ResendError")))
		http.Redirect(w, r, fmt.Sprintf("/login/2fa/verify?next=%s", url.QueryEscape(nextURL)), http.StatusFound)
		return
	}

	// Set success message based on method
	successMsg := "TwoFactor.Verify.CodeSent"
	switch method {
	case pkgtwofactor.MethodSMS:
		successMsg = "TwoFactor.Verify.SMSCodeSent"
	case pkgtwofactor.MethodEmail:
		successMsg = "TwoFactor.Verify.EmailCodeSent"
	case pkgtwofactor.MethodTOTP, pkgtwofactor.MethodBackupCodes:
		// TOTP and backup codes don't use resend, use default message
	default:
		// Unknown method, use default message
	}

	shared.SetFlash(w, "success", []byte(intl.MustT(r.Context(), successMsg)))
	http.Redirect(w, r, fmt.Sprintf("/login/2fa/verify?next=%s", url.QueryEscape(nextURL)), http.StatusFound)
}
