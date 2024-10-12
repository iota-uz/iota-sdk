package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/user"
	"github.com/iota-agency/iota-erp/internal/domain/entities/session"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/service"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
	"net/http"
)

type AuthService struct {
	app         *Application
	oAuthConfig *oauth2.Config
}

func NewAuthService(app *Application) *AuthService {
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
	u, err := s.app.UserService.GetByEmail(ctx, p.EmailAddresses[0].Value)
	if err != nil {
		return nil, nil, err
	}
	return s.authenticate(ctx, u.ID)
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
		Domain:   conf.FrontendDomain,
	}
	http.SetCookie(w, cookie)
}

func (s *AuthService) Authorize(ctx context.Context, token string) (*user.User, *session.Session, error) {
	sess, err := s.app.SessionService.GetByToken(ctx, token)
	if err != nil {
		return nil, nil, err
	}
	u, err := s.app.UserService.GetByID(ctx, sess.UserID)
	if err != nil {
		return nil, nil, err
	}
	// TODO: update last action
	// if err := s.app.UserService.UpdateLastAction(ctx, u.ID); err != nil {
	//	  return nil, nil, err
	//}
	return u, sess, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
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

func (s *AuthService) authenticate(ctx context.Context, id uint) (*user.User, *session.Session, error) {
	u, err := s.app.UserService.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	ip, _ := composables.UseIp(ctx)
	userAgent, _ := composables.UseUserAgent(ctx)
	token, err := s.newSessionToken()
	if err != nil {
		return nil, nil, err
	}
	sess := &session.CreateDTO{
		Token:     token,
		UserID:    u.ID,
		IP:        ip,
		UserAgent: userAgent,
	}
	if err := s.app.UserService.UpdateLastLogin(ctx, u.ID); err != nil {
		return nil, nil, err
	}
	if err := s.app.UserService.UpdateLastAction(ctx, u.ID); err != nil {
		return nil, nil, err
	}
	if err := s.app.SessionService.Create(ctx, sess); err != nil {
		return nil, nil, err
	}
	return u, sess.ToEntity(), nil
}

func (s *AuthService) Authenticate(ctx context.Context, email, password string) (*user.User, *session.Session, error) {
	u, err := s.app.UserService.GetByEmail(ctx, email)
	if err != nil {
		return nil, nil, err
	}
	if !u.CheckPassword(password) {
		return nil, nil, service.ErrInvalidPassword
	}
	return s.authenticate(ctx, u.ID)
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
		Domain:   conf.FrontendDomain,
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
