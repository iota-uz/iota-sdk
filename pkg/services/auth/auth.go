package auth

import (
	"context"
	"errors"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/service"
	"gorm.io/gorm"
	"time"
)

type Service struct {
}

func New() *Service {
	return &Service{}
}

func (s *Service) Authorize(ctx context.Context, token string) (*models.User, *models.Session, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, nil, service.ErrNoTx
	}
	session := &models.Session{}
	if err := tx.First(session, "token = ?", token).Error; err != nil {
		return nil, nil, err
	}
	user := &models.User{}
	if err := tx.First(user, "id = ?", session.UserId).Error; err != nil {
		return nil, nil, err
	}
	return user, session, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Delete(&models.Session{}, "token = ?", token).Error
}

func (s *Service) newSessionToken() string {
	return utils.RandomString(32, false)
}

func (s *Service) Authenticate(ctx context.Context, email, password string) (*models.User, *models.Session, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, nil, service.ErrNoTx
	}
	user := &models.User{}
	if res := tx.First(user, "email = ?", email); res.Error != nil {
		return nil, nil, res.Error
	}
	if !user.CheckPassword(password) {
		return nil, nil, errors.New("invalid password")
	}
	ip, _ := composables.UseIp(ctx)
	userAgent, _ := composables.UseUserAgent(ctx)
	session := &models.Session{
		Token:     s.newSessionToken(),
		UserId:    user.Id,
		Ip:        ip,
		UserAgent: userAgent,
		ExpiresAt: time.Now().Add(utils.SessionDuration()),
	}
	err := tx.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(session).Error; err != nil {
			return err
		}
		log := &models.AuthenticationLog{
			UserId:    user.Id,
			Ip:        ip,
			UserAgent: userAgent,
		}
		return tx.Create(log).Error
	})
	if err != nil {
		return nil, nil, err
	}
	return user, session, nil
}
