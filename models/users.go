package models

import (
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

func (u *User) Validate() []*ValidationError {
	var errs []*ValidationError
	if u.FirstName == "" {
		errs = append(errs, NewValidationError("first_name", "first_name is required"))
	}
	if u.LastName == "" {
		errs = append(errs, NewValidationError("last_name", "last_name is required"))
	}
	if u.Email == "" {
		errs = append(errs, NewValidationError("email", "email is required"))
	}
	if u.Password == "" {
		errs = append(errs, NewValidationError("password", "password is required"))
	}
	return errs
}

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hash)
	return nil
}
