package auth

import (
	"context"
	"errors"
	"github.com/iota-agency/iota-erp/internal/domain/user"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/service"
	"github.com/iota-agency/iota-erp/sdk/utils"
	"github.com/iota-agency/iota-erp/sdk/utils/random"
	"gorm.io/gorm"
	"time"
)

type Service struct {
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Authorize(ctx context.Context, token string) (*user.User, *models.Session, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, nil, service.ErrNoTx
	}
	session := &models.Session{}
	if err := tx.First(session, "token = ?", token).Error; err != nil {
		return nil, nil, err
	}
	u := &user.User{}
	if err := tx.First(u, "id = ?", session.UserId).Error; err != nil {
		return nil, nil, err
	}
	return u, session, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Delete(&models.Session{}, "token = ?", token).Error
}

func (s *Service) newSessionToken() string {
	// TODO: use a more secure random generator
	return random.String(32, random.AlphaNumericSet)
}

func (s *Service) Authenticate(ctx context.Context, email, password string) (*user.User, *models.Session, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, nil, service.ErrNoTx
	}
	u := &user.User{}
	if res := tx.First(u, "email = ?", email); res.Error != nil {
		return nil, nil, res.Error
	}
	if !u.CheckPassword(password) {
		return nil, nil, errors.New("invalid password")
	}
	ip, _ := composables.UseIp(ctx)
	userAgent, _ := composables.UseUserAgent(ctx)
	session := &models.Session{
		Token:     s.newSessionToken(),
		UserId:    u.Id,
		Ip:        ip,
		UserAgent: userAgent,
		ExpiresAt: time.Now().Add(utils.SessionDuration()),
	}
	err := tx.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(session).Error; err != nil {
			return err
		}
		log := &models.AuthenticationLog{
			UserId:    u.Id,
			Ip:        ip,
			UserAgent: userAgent,
		}
		return tx.Create(log).Error
	})
	if err != nil {
		return nil, nil, err
	}
	return u, session, nil
}
