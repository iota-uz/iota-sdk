package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
)

// IPBindingMode defines how strictly IP addresses are validated for sessions
type IPBindingMode string

const (
	// IPBindingDisabled means IP binding validation is not performed
	IPBindingDisabled IPBindingMode = "disabled"
	// IPBindingWarn means IP mismatches are logged but allowed
	IPBindingWarn IPBindingMode = "warn"
	// IPBindingStrict means IP mismatches result in authorization failure
	IPBindingStrict IPBindingMode = "strict"
)

var (
	ErrAudienceMismatch = errors.New("session audience mismatch")
	ErrIPMismatch       = errors.New("session IP address mismatch")
)

type AuthService struct {
	app            application.Application
	oAuthConfig    *oauth2.Config
	usersService   *UserService
	sessionService *SessionService
	ipBindingMode  IPBindingMode
}

// AuthServiceOption is a functional option for configuring AuthService
type AuthServiceOption func(*AuthService)

// WithIPBindingMode sets the IP binding mode for session authorization
func WithIPBindingMode(mode IPBindingMode) AuthServiceOption {
	return func(s *AuthService) {
		s.ipBindingMode = mode
	}
}

func NewAuthService(app application.Application, opts ...AuthServiceOption) *AuthService {
	conf := configuration.Use()
	svc := &AuthService{
		app: app,
		oAuthConfig: &oauth2.Config{
			RedirectURL:  conf.Google.RedirectURL,
			ClientID:     conf.Google.ClientID,
			ClientSecret: conf.Google.ClientSecret,
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		},
		usersService:   app.Service(UserService{}).(*UserService),
		sessionService: app.Service(SessionService{}).(*SessionService),
		ipBindingMode:  IPBindingDisabled, // Default to disabled for backward compatibility
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

func (s *AuthService) AuthenticateGoogle(ctx context.Context, code string) (user.User, session.Session, error) {
	// Use code to get token and get user info from Google.
	token, err := s.oAuthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, nil, err
	}
	client := s.oAuthConfig.Client(ctx, token)
	svc, err := people.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, nil, err
	}
	p, err := svc.People.Get("people/me").PersonFields("emailAddresses,names").Do()
	if err != nil {
		return nil, nil, err
	}
	u, err := s.usersService.GetByEmail(ctx, p.EmailAddresses[0].Value)
	if err != nil {
		return nil, nil, err
	}
	sess, err := s.authenticate(ctx, u, session.AudienceGranite)
	if err != nil {
		return nil, nil, err
	}
	return u, sess, nil
}

// AuthenticateGoogleWithAudience authenticates a user via Google OAuth and creates a session with a specific audience
func (s *AuthService) AuthenticateGoogleWithAudience(ctx context.Context, code string, audience session.SessionAudience) (user.User, session.Session, error) {
	// Use code to get token and get user info from Google.
	token, err := s.oAuthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, nil, err
	}
	client := s.oAuthConfig.Client(ctx, token)
	svc, err := people.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, nil, err
	}
	p, err := svc.People.Get("people/me").PersonFields("emailAddresses,names").Do()
	if err != nil {
		return nil, nil, err
	}
	u, err := s.usersService.GetByEmail(ctx, p.EmailAddresses[0].Value)
	if err != nil {
		return nil, nil, err
	}
	sess, err := s.authenticate(ctx, u, audience)
	if err != nil {
		return nil, nil, err
	}
	return u, sess, nil
}

func (s *AuthService) CookieGoogleAuthenticate(ctx context.Context, code string) (*http.Cookie, error) {
	_, sess, err := s.AuthenticateGoogle(ctx, code)
	if err != nil {
		return nil, err
	}
	conf := configuration.Use()
	cookie := &http.Cookie{
		Name:     conf.SidCookieKey,
		Value:    sess.Token(),
		Expires:  sess.ExpiresAt(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   conf.GoAppEnvironment == configuration.Production,
		Domain:   conf.Domain,
		Path:     "/",
	}
	return cookie, nil
}

func (s *AuthService) Authorize(ctx context.Context, token string) (session.Session, error) {
	sess, err := s.sessionService.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	// Validate IP binding if configured
	if s.ipBindingMode != IPBindingDisabled {
		if err := s.validateIPBinding(ctx, sess); err != nil {
			if s.ipBindingMode == IPBindingStrict {
				return nil, err
			}
			// Log warning but allow access in warn mode
			logger := configuration.Use().Logger()
			logger.Warnf("IP binding validation warning for session: %v", err)
		}
	}

	return sess, nil
}

// AuthorizeWithAudience validates a token and ensures the session audience matches the expected audience
func (s *AuthService) AuthorizeWithAudience(ctx context.Context, token string, expectedAudience session.SessionAudience) (session.Session, error) {
	sess, err := s.Authorize(ctx, token)
	if err != nil {
		return nil, err
	}

	// Validate audience
	if sess.Audience() != expectedAudience {
		logger := configuration.Use().Logger()
		logger.Warnf("Session audience mismatch: expected %s, got %s", expectedAudience, sess.Audience())
		return nil, ErrAudienceMismatch
	}

	return sess, nil
}

// validateIPBinding checks if the request IP matches the session IP
func (s *AuthService) validateIPBinding(ctx context.Context, sess session.Session) error {
	currentIP, ok := composables.UseIP(ctx)
	if !ok {
		// If we can't get the current IP, we can't validate
		return nil
	}

	if currentIP != sess.IP() {
		return ErrIPMismatch
	}

	return nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	return s.sessionService.Delete(ctx, token)
}

func (s *AuthService) newSessionToken() (string, error) {
	b := make([]byte, 24)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(b)
	return encoded, nil
}

func (s *AuthService) authenticate(ctx context.Context, u user.User, audience session.SessionAudience) (session.Session, error) {
	logger := configuration.Use().Logger()
	logger.Infof("Creating session for user ID: %d, tenant ID: %d, audience: %s", u.ID(), u.TenantID(), audience)

	// Get IP and user agent
	ip, ok := composables.UseIP(ctx)
	if !ok {
		logger.Warnf("Could not get IP, using default")
		ip = "0.0.0.0"
	}

	userAgent, ok := composables.UseUserAgent(ctx)
	if !ok {
		logger.Warnf("Could not get User-Agent, using default")
		userAgent = "Unknown"
	}

	// Generate session token
	token, err := s.newSessionToken()
	if err != nil {
		logger.Errorf("Failed to generate session token: %v", err)
		return nil, err
	}

	// Create session DTO with specified audience
	sess := &session.CreateDTO{
		Token:     token,
		UserID:    u.ID(),
		IP:        ip,
		UserAgent: userAgent,
		TenantID:  u.TenantID(),
		Audience:  audience,
	}

	// Update user last login
	if err := s.usersService.UpdateLastLogin(ctx, u.ID()); err != nil {
		logger.Errorf("Failed to update last login: %v", err)
		return nil, err
	}

	// Update user last action
	if err := s.usersService.UpdateLastAction(ctx, u.ID()); err != nil {
		logger.Errorf("Failed to update last action: %v", err)
		return nil, err
	}

	// Create the session
	logger.Infof("Creating session in DB for user ID: %d, token: %s (partial)", u.ID(), token[:5])
	if err := s.sessionService.Create(ctx, sess); err != nil {
		logger.Errorf("Failed to create session in DB: %v", err)
		return nil, err
	}

	logger.Infof("Session created successfully")
	return sess.ToEntity(), nil
}

func (s *AuthService) AuthenticateWithUserID(ctx context.Context, id uint, password string) (user.User, session.Session, error) {
	u, err := s.usersService.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if !u.CheckPassword(password) {
		return nil, nil, composables.ErrInvalidPassword
	}
	sess, err := s.authenticate(ctx, u, session.AudienceGranite)
	if err != nil {
		return nil, nil, err
	}
	return u, sess, nil
}

// AuthenticateWithUserIDAndAudience authenticates a user by ID and password, creating a session with a specific audience
func (s *AuthService) AuthenticateWithUserIDAndAudience(ctx context.Context, id uint, password string, audience session.SessionAudience) (user.User, session.Session, error) {
	u, err := s.usersService.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if !u.CheckPassword(password) {
		return nil, nil, composables.ErrInvalidPassword
	}
	sess, err := s.authenticate(ctx, u, audience)
	if err != nil {
		return nil, nil, err
	}
	return u, sess, nil
}

func (s *AuthService) CookieAuthenticateWithUserID(ctx context.Context, id uint, password string) (*http.Cookie, error) {
	_, sess, err := s.AuthenticateWithUserID(ctx, id, password)
	if err != nil {
		return nil, err
	}
	conf := configuration.Use()
	cookie := &http.Cookie{
		Name:     conf.SidCookieKey,
		Value:    sess.Token(),
		Expires:  sess.ExpiresAt(),
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   conf.GoAppEnvironment == configuration.Production,
		Domain:   conf.Domain,
	}
	return cookie, nil
}

func (s *AuthService) Authenticate(ctx context.Context, email, password string) (user.User, session.Session, error) {
	logger := configuration.Use().Logger()
	logger.Infof("Authentication attempt for email: %s", email)

	u, err := s.usersService.GetByEmail(ctx, email)
	if err != nil {
		logger.Errorf("Failed to get user by email: %v", err)
		return nil, nil, err
	}

	if !u.CheckPassword(password) {
		logger.Errorf("Invalid password for user: %s", email)
		return nil, nil, composables.ErrInvalidPassword
	}

	logger.Infof("User authenticated, creating session for user ID: %d", u.ID())
	sess, err := s.authenticate(ctx, u, session.AudienceGranite)
	if err != nil {
		logger.Errorf("Failed to create session: %v", err)
		return nil, nil, err
	}

	logger.Infof("Session created successfully with token: %s (partial)", sess.Token()[:5])
	return u, sess, nil
}

// AuthenticateWithAudience authenticates a user by email and password, creating a session with a specific audience
func (s *AuthService) AuthenticateWithAudience(ctx context.Context, email, password string, audience session.SessionAudience) (user.User, session.Session, error) {
	logger := configuration.Use().Logger()
	logger.Infof("Authentication attempt for email: %s, audience: %s", email, audience)

	u, err := s.usersService.GetByEmail(ctx, email)
	if err != nil {
		logger.Errorf("Failed to get user by email: %v", err)
		return nil, nil, err
	}

	if !u.CheckPassword(password) {
		logger.Errorf("Invalid password for user: %s", email)
		return nil, nil, composables.ErrInvalidPassword
	}

	sess, err := s.authenticate(ctx, u, audience)
	if err != nil {
		return nil, nil, err
	}

	return u, sess, nil
}

func (s *AuthService) CookieAuthenticate(ctx context.Context, email, password string) (*http.Cookie, error) {
	_, sess, err := s.Authenticate(ctx, email, password)
	if err != nil {
		return nil, err
	}
	conf := configuration.Use()
	cookie := &http.Cookie{
		Name:     conf.SidCookieKey,
		Value:    sess.Token(),
		Expires:  sess.ExpiresAt(),
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   conf.GoAppEnvironment == configuration.Production,
		Domain:   conf.Domain,
	}
	return cookie, nil
}

func generateStateOauthCookie() (*http.Cookie, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	state := base64.URLEncoding.EncodeToString(b)
	conf := configuration.Use()
	cookie := &http.Cookie{
		Name:     conf.OauthStateCookieKey,
		Value:    state,
		Expires:  time.Now().Add(time.Minute * 5),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   conf.GoAppEnvironment == configuration.Production,
		Domain:   conf.Domain,
	}
	return cookie, nil
}

func (s *AuthService) GoogleAuthenticate(w http.ResponseWriter) (string, error) {
	cookie, err := generateStateOauthCookie()
	if err != nil {
		return "", err
	}
	http.SetCookie(w, cookie)
	u := s.oAuthConfig.AuthCodeURL(cookie.Value, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	return u, nil
}
