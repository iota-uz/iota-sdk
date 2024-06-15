package services

import (
	"context"
	"errors"
	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/internal/domain/session"
	"github.com/iota-agency/iota-erp/internal/domain/user"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/service"
	"github.com/iota-agency/iota-erp/sdk/utils/random"
	"time"
)

type AuthService struct {
	app *Application
}

func NewAuthService(app *Application) *AuthService {
	return &AuthService{
		app: app,
	}
}

func (s *AuthService) Authorize(ctx context.Context, token string) (*user.User, *session.Session, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, nil, service.ErrNoTx
	}
	sess := &session.Session{}
	if err := tx.First(sess, "token = ?", token).Error; err != nil {
		return nil, nil, err
	}
	u := &user.User{}
	if err := tx.First(u, "id = ?", sess.UserId).Error; err != nil {
		return nil, nil, err
	}
	return u, sess, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Delete(&session.Session{}, "token = ?", token).Error
}

func (s *AuthService) newSessionToken() string {
	// TODO: use a more secure random generator
	return random.String(32, random.AlphaNumericSet)
}

func (s *AuthService) Authenticate(ctx context.Context, email, password string) (*user.User, *session.Session, error) {
	u, err := s.app.UserService.GetByEmail(ctx, email)
	if err != nil {
		return nil, nil, err
	}
	if !u.CheckPassword(password) {
		return nil, nil, errors.New("invalid password")
	}
	ip, _ := composables.UseIp(ctx)
	userAgent, _ := composables.UseUserAgent(ctx)
	duration, err := configuration.Use().SessionDuration()
	if err != nil {
		return nil, nil, err
	}
	sess := &session.Session{
		Token:     s.newSessionToken(),
		UserId:    u.Id,
		Ip:        ip,
		UserAgent: userAgent,
		ExpiresAt: time.Now().Add(duration),
	}
	if err := s.app.SessionService.Create(ctx, sess); err != nil {
		return nil, nil, err
	}
	return u, sess, nil
}
