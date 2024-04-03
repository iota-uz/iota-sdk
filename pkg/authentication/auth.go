package authentication

import (
	"errors"
	"github.com/apollos-studio/sso/models"
	"github.com/apollos-studio/sso/pkg/utils"
	"github.com/jmoiron/sqlx"
	"time"
)

type Authentication struct {
	Db *sqlx.DB
}

func New(db *sqlx.DB) *Authentication {
	return &Authentication{Db: db}
}

func (a *Authentication) Authorize(token string) (*models.User, error) {
	session := &models.Session{}
	err := a.Db.Get(session, "SELECT * FROM sessions WHERE token = $1", token)
	if err != nil {
		return nil, err
	}
	user := &models.User{}

	if err := a.Db.Get(user, "SELECT * FROM users WHERE id = $1", session.UserId); err != nil {
		return nil, err
	}
	return user, nil
}

func (a *Authentication) Logout(token string) error {
	if _, err := a.Db.Exec("DELETE FROM sessions WHERE token = $1", token); err != nil {
		return err
	}
	return nil
}

func (a *Authentication) createToken() string {
	return utils.RandomString(32, false)
}

func (a *Authentication) Authenticate(email, password, ip, userAgent string) (*models.User, string, error) {
	user := &models.User{}
	err := a.Db.Get(user, "SELECT * FROM users WHERE email = $1", email)
	if err != nil {
		return nil, "", err
	}
	if !user.CheckPassword(password) {
		return nil, "", errors.New("invalid password")
	}
	token := a.createToken()
	_, err = a.Db.Exec(
		"INSERT INTO sessions (token, user_id, ip, user_agent, expires_at) VALUES ($1, $2, $3, $4, $5)",
		token, user.Id, ip, userAgent,
		time.Now().Add(utils.SessionDuration()),
	)
	if err != nil {
		return nil, "", err
	}
	if _, err := a.Db.Exec(
		"INSERT INTO authentication_logs (user_id, ip, user_agent) VALUES ($1, $2, $3)",
		user.Id, ip, userAgent,
	); err != nil {
		return nil, "", err
	}
	return user, token, nil
}
