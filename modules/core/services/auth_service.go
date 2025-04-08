package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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

type AuthService struct {
	app            application.Application
	oAuthConfig    *oauth2.Config
	usersService   *UserService
	sessionService *SessionService
}

func NewAuthService(app application.Application) *AuthService {
	conf := configuration.Use()
	return &AuthService{
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
	}
}

func (s *AuthService) AuthenticateGoogle(ctx context.Context, code string) (user.User, *session.Session, error) {
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
	sess, err := s.authenticate(ctx, u)
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
		Value:    sess.Token,
		Expires:  sess.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   conf.GoAppEnvironment == configuration.Production,
		Domain:   conf.Domain,
		Path:     "/",
	}
	return cookie, nil
}

func (s *AuthService) Authorize(ctx context.Context, token string) (*session.Session, error) {
	sess, err := s.sessionService.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	//u, err := s.usersService.GetByID(ctx, sess.UserID)
	//if err != nil {
	//	return nil, nil, err
	//}
	// TODO: update last action
	// if err := s.usersService.UpdateLastAction(ctx, u.ID); err != nil {
	//	  return nil, nil, err
	//}
	return sess, nil
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

func (s *AuthService) authenticate(ctx context.Context, u user.User) (*session.Session, error) {
	logger := configuration.Use().Logger()
	logger.Infof("Creating session for user ID: %d, tenant ID: %d", u.ID(), u.TenantID())

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

	// Create session DTO
	sess := &session.CreateDTO{
		Token:     token,
		UserID:    u.ID(),
		IP:        ip,
		UserAgent: userAgent,
		TenantID:  u.TenantID(), // Ensure tenant ID is set in the session
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

func (s *AuthService) AuthenticateWithUserID(ctx context.Context, id uint, password string) (user.User, *session.Session, error) {
	u, err := s.usersService.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if !u.CheckPassword(password) {
		return nil, nil, composables.ErrInvalidPassword
	}
	sess, err := s.authenticate(ctx, u)
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
		Value:    sess.Token,
		Expires:  sess.ExpiresAt,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   conf.GoAppEnvironment == configuration.Production,
		Domain:   conf.Domain,
	}
	return cookie, nil
}

func (s *AuthService) Authenticate(ctx context.Context, email, password string) (user.User, *session.Session, error) {
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
	sess, err := s.authenticate(ctx, u)
	if err != nil {
		logger.Errorf("Failed to create session: %v", err)
		return nil, nil, err
	}

	logger.Infof("Session created successfully with token: %s (partial)", sess.Token[:5])
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
		Value:    sess.Token,
		Expires:  sess.ExpiresAt,
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
