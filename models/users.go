package models

import (
	"github.com/iota-agency/iota-erp/graph/gqlmodels"
	"golang.org/x/crypto/bcrypt"
	"time"
)

type User struct {
	Id         int64
	FirstName  string
	LastName   string
	MiddleName *string
	Password   *string
	Email      string
	Avatar     *Upload `gorm:"foreignKey:AvatarId"`
	AvatarId   *int64
	EmployeeId *int64
	LastIp     *string
	LastLogin  *time.Time
	LastAction *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (u *User) CheckPassword(password string) bool {
	if u.Password == nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(*u.Password), []byte(password)) == nil
}

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	newPassword := string(hash)
	u.Password = &newPassword
	return nil
}

func (u *User) avatar2graph() *model.Upload {
	if u.Avatar == nil {
		return nil
	}
	return u.Avatar.ToGraph()
}

func (u *User) ToGraph() *model.User {
	return &model.User{
		ID:         u.Id,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		MiddleName: u.MiddleName,
		Email:      u.Email,
		AvatarID:   u.AvatarId,
		Avatar:     u.avatar2graph(),
		EmployeeID: u.EmployeeId,
		LastIP:     u.LastIp,
		LastLogin:  u.LastLogin,
		LastAction: u.LastAction,
		CreatedAt:  u.CreatedAt,
		UpdatedAt:  u.UpdatedAt,
	}
}
