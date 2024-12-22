package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/user"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/session"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
	"net/http"
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
			RedirectURL:  conf.GoogleRedirectURL,
			ClientID:     conf.GoogleClientID,
			ClientSecret: conf.GoogleClientSecret,
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

func (s *AuthService) AuthenticateGoogle(ctx context.Context, code string) (*user.User, *session.Session, error) {
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

func (s *AuthService) OauthGoogleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "code not found", http.StatusBadRequest)
		return
	}
	_, sess, err := s.AuthenticateGoogle(r.Context(), code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	conf := configuration.Use()
	cookie := &http.Cookie{
		//nolint:exhaustruct
		Name:     conf.SidCookieKey,
		Value:    sess.Token,
		Expires:  sess.ExpiresAt,
		HttpOnly: false,
		SameSite: http.SameSiteNoneMode,
		Secure:   false,
		Domain:   conf.Domain,
	}
	http.SetCookie(w, cookie)
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
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Delete(&session.Session{}, "token = ?", token).Error //nolint:exhaustruct
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

func (s *AuthService) authenticate(ctx context.Context, u *user.User) (*session.Session, error) {
	ip, _ := composables.UseIP(ctx)
	userAgent, _ := composables.UseUserAgent(ctx)
	token, err := s.newSessionToken()
	if err != nil {
		return nil, err
	}
	sess := &session.CreateDTO{
		Token:     token,
		UserID:    u.ID,
		IP:        ip,
		UserAgent: userAgent,
	}
	if err := s.usersService.UpdateLastLogin(ctx, u.ID); err != nil {
		return nil, err
	}
	if err := s.usersService.UpdateLastAction(ctx, u.ID); err != nil {
		return nil, err
	}
	if err := s.sessionService.Create(ctx, sess); err != nil {
		return nil, err
	}
	return sess.ToEntity(), nil
}

func (s *AuthService) AuthenticateWithUserId(ctx context.Context, id uint, password string) (*user.User, *session.Session, error) {
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

func (s *AuthService) CoockieAuthenticateWithUserId(ctx context.Context, id uint, password string) (*http.Cookie, error) {
	_, sess, err := s.AuthenticateWithUserId(ctx, id, password)
	if err != nil {
		return nil, err
	}
	conf := configuration.Use()
	cookie := &http.Cookie{
		//nolint:exhaustruct
		Name:     conf.SidCookieKey,
		Value:    sess.Token,
		Expires:  sess.ExpiresAt,
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
		Secure:   conf.GoAppEnvironment == "production",
		Domain:   conf.Domain,
	}
	return cookie, nil
}

func (s *AuthService) Authenticate(ctx context.Context, email, password string) (*user.User, *session.Session, error) {
	u, err := s.usersService.GetByEmail(ctx, email)
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

func (s *AuthService) CookieAuthenticate(ctx context.Context, email, password string) (*http.Cookie, error) {
	_, sess, err := s.Authenticate(ctx, email, password)
	if err != nil {
		return nil, err
	}
	conf := configuration.Use()
	cookie := &http.Cookie{
		//nolint:exhaustruct
		Name:     conf.SidCookieKey,
		Value:    sess.Token,
		Expires:  sess.ExpiresAt,
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
		Secure:   conf.GoAppEnvironment == "production",
		Domain:   conf.Domain,
	}
	return cookie, nil
}

func generateStateOauthCookie() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (s *AuthService) GoogleAuthenticate() (string, error) {
	oauthState, err := generateStateOauthCookie()
	if err != nil {
		return "", err
	}
	u := s.oAuthConfig.AuthCodeURL(oauthState, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	return u, nil
}
