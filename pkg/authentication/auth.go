package authentication

import (
	"errors"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"gorm.io/gorm"
	"time"
)

type Authentication struct {
	Db *gorm.DB
}

func New(db *gorm.DB) *Authentication {
	return &Authentication{Db: db}
}

func (a *Authentication) Authorize(token string) (*models.User, error) {
	session := &models.Session{}

	if err := a.Db.First(session, "token = ?", token).Error; err != nil {
		return nil, err
	}
	user := &models.User{}
	if err := a.Db.First(user, "id = ?", session.UserId).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (a *Authentication) Logout(token string) error {
	return a.Db.Delete(&models.Session{}, "token = ?", token).Error
}

func (a *Authentication) createToken() string {
	return utils.RandomString(32, false)
}

func (a *Authentication) Authenticate(email, password, ip, userAgent string) (*models.User, string, error) {
	user := &models.User{}
	if res := a.Db.First(user, "email = ?", email); res.Error != nil {
		return nil, "", res.Error
	}
	if !user.CheckPassword(password) {
		return nil, "", errors.New("invalid password")
	}
	token := a.createToken()
	session := &models.Session{
		Token:     token,
		UserId:    user.Id,
		Ip:        ip,
		UserAgent: userAgent,
		ExpiresAt: time.Now().Add(utils.SessionDuration()),
	}
	err := a.Db.Transaction(func(tx *gorm.DB) error {
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
		return nil, "", err
	}
	return user, token, nil
}
