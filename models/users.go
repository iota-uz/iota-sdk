package models

import (
	"github.com/iota-agency/iota-erp/graph/gqlmodels"
	"golang.org/x/crypto/bcrypt"
	"time"
)

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"error"`
}

func (v *ValidationError) Error() string {
	return v.Message
}

func NewValidationError(field, err string) *ValidationError {
	return &ValidationError{Field: field, Message: err}
}

type User struct {
	Id         int64          `gql:"id"`
	FirstName  string         `gql:"firstName"`
	LastName   string         `gql:"lastName"`
	MiddleName JsonNullString `gql:"middleName"`
	Password   string         `gql:"password,!read"`
	Email      string         `gql:"email"`
	Avatar     *Uploads       `gql:"avatar" gorm:"foreignKey:AvatarId"`
	AvatarId   JsonNullInt64  `gql:"avatarId"`
	EmployeeId JsonNullInt64  `gql:"employeeId"`
	LastIp     JsonNullString `gql:"lastIp"`
	LastLogin  *time.Time     `gql:"lastLogin"`
	LastAction *time.Time     `gql:"lastAction"`
	CreatedAt  *time.Time     `gql:"createdAt"`
	UpdatedAt  *time.Time     `gql:"updatedAt"`
}

func (u *User) CheckPassword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) == nil
}

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hash)
	return nil
}

func (u *User) ToGraph() *model.User {
	return &model.User{
		ID:        u.Id,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Email:     u.Email,
	}
}
