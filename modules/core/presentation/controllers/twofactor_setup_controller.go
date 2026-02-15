package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/twofactorsetup"
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

// NewTwoFactorSetupController creates a new TwoFactorSetupController.
// Initializes the controller with required service dependencies.
// Parameters:
//   - app: The application instance providing service registry
//
// Returns a configured TwoFactorSetupController implementing the Controller interface.
func NewTwoFactorSetupController(app application.Application) application.Controller {
	return &TwoFactorSetupController{
		app:              app,
		twoFactorService: app.Service(twofactor.TwoFactorService{}).(*twofactor.TwoFactorService),
		sessionService:   app.Service(services.SessionService{}).(*services.SessionService),
		userService:      app.Service(services.UserService{}).(*services.UserService),
	}
}

// TwoFactorSetupController handles HTTP endpoints for 2FA setup flows.
// Provides method selection, TOTP QR code display, OTP delivery, and setup confirmation.
// Routes are mounted at /login/2fa/setup and require authentication.
type TwoFactorSetupController struct {
	app              application.Application
	twoFactorService *twofactor.TwoFactorService
	sessionService   *services.SessionService
	userService      *services.UserService
}

// Key returns the base route path for this controller.
// Implements the Controller interface.
func (c *TwoFactorSetupController) Key() string {
	return "/login/2fa/setup"
}

// Register registers all HTTP routes for 2FA setup flows.
// Applies authentication, localization, transaction, and page context middleware.
// Routes:
//   - GET/POST /login/2fa/setup - Method selection
//   - GET /login/2fa/setup/totp - TOTP QR code display
//   - POST /login/2fa/setup/totp/confirm - TOTP verification
//   - GET /login/2fa/setup/otp - OTP input form
//   - POST /login/2fa/setup/otp/send - Resend OTP
//   - POST /login/2fa/setup/otp/confirm - OTP verification
//
// Implements the Controller interface.
func (c *TwoFactorSetupController) Register(r *mux.Router) {
	setupRouter := r.PathPrefix("/login/2fa/setup").Subrouter()
	setupRouter.Use(
		middleware.Authorize(),
		middleware.ProvideLocalizer(c.app),
		middleware.WithPageContext(),
	)

	// Method choice endpoints
	setupRouter.HandleFunc("", c.GetMethodChoice).Methods(http.MethodGet)
	setupRouter.HandleFunc("", c.PostMethodChoice).Methods(http.MethodPost)

	// TOTP setup endpoints
	setupRouter.HandleFunc("/totp", c.GetTOTPSetup).Methods(http.MethodGet)
	setupRouter.HandleFunc("/totp/confirm", c.PostTOTPConfirm).Methods(http.MethodPost)

	// OTP setup endpoints (SMS/Email)
	setupRouter.HandleFunc("/otp", c.GetOTPSetup).Methods(http.MethodGet)
	setupRouter.HandleFunc("/otp/send", c.PostOTPSend).Methods(http.MethodPost)
	setupRouter.HandleFunc("/otp/confirm", c.PostOTPConfirm).Methods(http.MethodPost)
}

// GetMethodChoice displays the 2FA method selection page.
// Shows available options: TOTP (authenticator app), SMS, or Email.
// GET /login/2fa/setup?next=/redirect/path
func (c *TwoFactorSetupController) GetMethodChoice(w http.ResponseWriter, r *http.Request) {
	// Validate the redirect URL early to prevent open redirect attacks
	nextURL := security.GetValidatedRedirect(r.URL.Query().Get("next"))

	if err := twofactorsetup.MethodChoice(&twofactorsetup.MethodChoiceProps{
		NextURL: nextURL,
	}).Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// PostMethodChoice handles method selection and initiates the setup flow.
// Validates the selected method, begins setup via TwoFactorService, and redirects to method-specific page.
// POST /login/2fa/setup with Method and NextURL form values.
func (c *TwoFactorSetupController) PostMethodChoice(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	method := r.FormValue("Method")
	// Validate the redirect URL to prevent open redirect attacks
	nextURL := security.GetValidatedRedirect(r.FormValue("NextURL"))

	// Validate method
	validMethods := []string{string(pkgtwofactor.MethodTOTP), string(pkgtwofactor.MethodSMS), string(pkgtwofactor.MethodEmail)}
	isValid := false
	for _, valid := range validMethods {
		if method == valid {
			isValid = true
			break
		}
	}
	if !isValid {
		http.Error(w, "invalid method", http.StatusBadRequest)
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

	// Get user
	u, err := c.userService.GetByID(r.Context(), sess.UserID())
	if err != nil {
		logger.Error("failed to get user", "error", err)
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// Begin setup
	methodType := pkgtwofactor.Method(method)
	challenge, err := c.twoFactorService.BeginSetup(r.Context(), u.ID(), methodType)
	if err != nil {
		logger.Error("failed to begin 2FA setup", "error", err, "method", method)
		shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "TwoFactor.Setup.Error")))
		http.Redirect(w, r, fmt.Sprintf("/login/2fa/setup?next=%s", url.QueryEscape(nextURL)), http.StatusFound)
		return
	}

	// Redirect based on method
	var redirectURL string
	switch methodType {
	case pkgtwofactor.MethodTOTP:
		redirectURL = fmt.Sprintf("/login/2fa/setup/totp?challengeId=%s&next=%s", challenge.ChallengeID, url.QueryEscape(nextURL))
	case pkgtwofactor.MethodSMS, pkgtwofactor.MethodEmail:
		redirectURL = fmt.Sprintf("/login/2fa/setup/otp?method=%s&challengeId=%s&next=%s", method, challenge.ChallengeID, url.QueryEscape(nextURL))
	case pkgtwofactor.MethodBackupCodes:
		// Backup codes are not set up directly, return error
		logger.Error("backup codes cannot be set up directly", "method", method)
		shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "TwoFactor.Setup.InvalidMethod")))
		http.Redirect(w, r, fmt.Sprintf("/login/2fa/setup?next=%s", url.QueryEscape(nextURL)), http.StatusFound)
		return
	default:
		logger.Error("unknown 2FA method", "method", method)
		shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "TwoFactor.Setup.InvalidMethod")))
		http.Redirect(w, r, fmt.Sprintf("/login/2fa/setup?next=%s", url.QueryEscape(nextURL)), http.StatusFound)
		return
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// GetTOTPSetup displays the TOTP setup page with QR code.
// Retrieves the setup challenge and renders the QR code for scanning with authenticator apps.
// GET /login/2fa/setup/totp?challengeId=xxx&next=/redirect/path
func (c *TwoFactorSetupController) GetTOTPSetup(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())
	challengeID := r.URL.Query().Get("challengeId")
	// Validate the redirect URL to prevent open redirect attacks
	nextURL := security.GetValidatedRedirect(r.URL.Query().Get("next"))

	if challengeID == "" {
		http.Error(w, "missing challenge ID", http.StatusBadRequest)
		return
	}

	// Retrieve challenge data to get QR code
	challenge, err := c.twoFactorService.GetSetupChallenge(challengeID)
	if err != nil {
		logger.Error("failed to get setup challenge", "error", err)
		shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "TwoFactor.Setup.Error")))
		http.Redirect(w, r, fmt.Sprintf("/login/2fa/setup?next=%s", url.QueryEscape(nextURL)), http.StatusFound)
		return
	}

	// Convert base64 PNG to data URL
	qrImageURL := fmt.Sprintf("data:image/png;base64,%s", challenge.QRCodePNG)

	if err := twofactorsetup.TOTPSetup(&twofactorsetup.TOTPSetupProps{
		ChallengeID: challengeID,
		NextURL:     nextURL,
		QRImageURL:  qrImageURL,
	}).Render(r.Context(), w); err != nil {
		logger.Error("failed to render TOTP setup template", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// GetOTPSetup displays the OTP setup page for SMS/Email verification.
// Shows the destination (masked phone/email) and code input form.
// GET /login/2fa/setup/otp?method=sms&challengeId=xxx&next=/redirect/path
func (c *TwoFactorSetupController) GetOTPSetup(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())
	method := r.URL.Query().Get("method")
	challengeID := r.URL.Query().Get("challengeId")
	// Validate the redirect URL to prevent open redirect attacks
	nextURL := security.GetValidatedRedirect(r.URL.Query().Get("next"))

	if challengeID == "" {
		http.Error(w, "missing challenge ID", http.StatusBadRequest)
		return
	}

	if method == "" {
		http.Error(w, "missing method", http.StatusBadRequest)
		return
	}

	// Validate method
	validMethods := []string{string(pkgtwofactor.MethodSMS), string(pkgtwofactor.MethodEmail)}
	isValid := false
	for _, valid := range validMethods {
		if method == valid {
			isValid = true
			break
		}
	}
	if !isValid {
		http.Error(w, "invalid method", http.StatusBadRequest)
		return
	}

	// Retrieve challenge data to get destination (phone/email)
	challenge, err := c.twoFactorService.GetSetupChallenge(challengeID)
	if err != nil {
		logger.Error("failed to get setup challenge", "error", err)
		shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "TwoFactor.Setup.Error")))
		http.Redirect(w, r, fmt.Sprintf("/login/2fa/setup?next=%s", url.QueryEscape(nextURL)), http.StatusFound)
		return
	}

	if err := twofactorsetup.OTPSetup(&twofactorsetup.OTPSetupProps{
		Method:      method,
		ChallengeID: challengeID,
		NextURL:     nextURL,
		Destination: challenge.Destination,
	}).Render(r.Context(), w); err != nil {
		logger.Error("failed to render OTP setup template", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// PostTOTPConfirm validates the TOTP code and completes setup.
// Confirms the setup, enables 2FA, updates session to active status, and displays recovery codes.
// POST /login/2fa/setup/totp/confirm with ChallengeID, Code, and NextURL form values.
func (c *TwoFactorSetupController) PostTOTPConfirm(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	challengeID := r.FormValue("ChallengeID")
	code := r.FormValue("Code")
	// Validate the redirect URL to prevent open redirect attacks
	nextURL := security.GetValidatedRedirect(r.FormValue("NextURL"))

	if challengeID == "" {
		http.Error(w, "missing challenge ID", http.StatusBadRequest)
		return
	}

	// Get session
	sess, err := composables.UseSession(r.Context())
	if err != nil {
		logger.Error("failed to get session", "error", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Confirm setup
	result, err := c.twoFactorService.ConfirmSetup(r.Context(), sess.UserID(), challengeID, code)
	if err != nil {
		logger.Error("failed to confirm 2FA setup", "error", err)
		var errorMsg string
		if errors.Is(err, pkgtwofactor.ErrInvalidCode) {
			errorMsg = "TwoFactor.Setup.InvalidCode"
		} else {
			errorMsg = "TwoFactor.Setup.Error"
		}
		shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), errorMsg)))
		http.Redirect(w, r, fmt.Sprintf("/login/2fa/setup/totp?challengeId=%s&next=%s", url.QueryEscape(challengeID), url.QueryEscape(nextURL)), http.StatusFound)
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

	// Show recovery codes
	if err := twofactorsetup.SetupComplete(&twofactorsetup.SetupCompleteProps{
		Method:        result.Method,
		RecoveryCodes: result.RecoveryCodes,
		NextURL:       nextURL,
	}).Render(r.Context(), w); err != nil {
		logger.Error("failed to render setup complete template", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// PostOTPSend resends the OTP code via SMS or Email.
// Generates and sends a new code to the same destination.
// POST /login/2fa/setup/otp/send with Method, ChallengeID, and NextURL form values.
func (c *TwoFactorSetupController) PostOTPSend(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	method := r.FormValue("Method")
	challengeID := r.FormValue("ChallengeID")
	// Validate the redirect URL to prevent open redirect attacks
	nextURL := security.GetValidatedRedirect(r.FormValue("NextURL"))

	if challengeID == "" {
		http.Error(w, "missing challenge ID", http.StatusBadRequest)
		return
	}

	// Resend OTP for SMS/Email methods
	methodType := pkgtwofactor.Method(method)
	var successMsg string

	switch methodType {
	case pkgtwofactor.MethodSMS, pkgtwofactor.MethodEmail:
		// Call the resend service
		_, err := c.twoFactorService.ResendSetupOTP(r.Context(), challengeID)
		if err != nil {
			logger.Error("failed to resend OTP", "error", err, "method", method, "challengeID", challengeID)
			shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "TwoFactor.Setup.ResendFailed")))
			http.Redirect(w, r, fmt.Sprintf("/login/2fa/setup/otp?method=%s&challengeId=%s&next=%s", method, url.QueryEscape(challengeID), url.QueryEscape(nextURL)), http.StatusFound)
			return
		}

		// Set success message based on method
		if methodType == pkgtwofactor.MethodSMS {
			successMsg = "TwoFactor.Setup.SMSSent"
		} else {
			successMsg = "TwoFactor.Setup.EmailSent"
		}

	case pkgtwofactor.MethodTOTP, pkgtwofactor.MethodBackupCodes:
		// TOTP and backup codes don't use OTP send
		logger.Warn("invalid method for OTP send", "method", method)
		shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "TwoFactor.Setup.InvalidMethod")))
		http.Redirect(w, r, fmt.Sprintf("/login/2fa/setup/otp?method=%s&challengeId=%s&next=%s", method, url.QueryEscape(challengeID), url.QueryEscape(nextURL)), http.StatusFound)
		return

	default:
		// Unknown method
		logger.Warn("unknown method for OTP send", "method", method)
		shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "TwoFactor.Setup.InvalidMethod")))
		http.Redirect(w, r, fmt.Sprintf("/login/2fa/setup/otp?method=%s&challengeId=%s&next=%s", method, url.QueryEscape(challengeID), url.QueryEscape(nextURL)), http.StatusFound)
		return
	}

	shared.SetFlash(w, "success", []byte(intl.MustT(r.Context(), successMsg)))
	http.Redirect(w, r, fmt.Sprintf("/login/2fa/setup/otp?method=%s&challengeId=%s&next=%s", method, url.QueryEscape(challengeID), url.QueryEscape(nextURL)), http.StatusFound)
}

// PostOTPConfirm validates the OTP code and completes setup.
// Confirms the setup, enables 2FA, updates session to active status, and redirects to next URL.
// POST /login/2fa/setup/otp/confirm with ChallengeID, Code, Method, and NextURL form values.
func (c *TwoFactorSetupController) PostOTPConfirm(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	challengeID := r.FormValue("ChallengeID")
	code := r.FormValue("Code")
	method := r.FormValue("Method")
	// Validate the redirect URL to prevent open redirect attacks
	nextURL := security.GetValidatedRedirect(r.FormValue("NextURL"))

	if challengeID == "" {
		http.Error(w, "missing challenge ID", http.StatusBadRequest)
		return
	}

	// Get session
	sess, err := composables.UseSession(r.Context())
	if err != nil {
		logger.Error("failed to get session", "error", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Confirm setup
	_, err = c.twoFactorService.ConfirmSetup(r.Context(), sess.UserID(), challengeID, code)
	if err != nil {
		logger.Error("failed to confirm 2FA setup", "error", err)
		var errorMsg string
		if errors.Is(err, pkgtwofactor.ErrInvalidCode) {
			errorMsg = "TwoFactor.Setup.InvalidCode"
		} else {
			errorMsg = "TwoFactor.Setup.Error"
		}
		shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), errorMsg)))
		http.Redirect(w, r, fmt.Sprintf("/login/2fa/setup/otp?method=%s&challengeId=%s&next=%s", method, url.QueryEscape(challengeID), url.QueryEscape(nextURL)), http.StatusFound)
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

	// For OTP methods, show success and redirect (nextURL already validated earlier)
	shared.SetFlash(w, "success", []byte(intl.MustT(r.Context(), "TwoFactor.Setup.Success")))
	http.Redirect(w, r, nextURL, http.StatusFound)
}
