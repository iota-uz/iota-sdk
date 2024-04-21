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
	Id         int64          `gql:"id" db:"id"`
	FirstName  string         `gql:"first_name" db:"first_name"`
	LastName   string         `gql:"last_name" db:"last_name"`
	MiddleName JsonNullString `gql:"middle_name,omitempty" db:"middle_name"`
	Password   string         `gql:"-" db:"password"`
	Email      string         `gql:"email" db:"email"`
	Avatar     *Uploads       `gql:"avatar" db:"avatar_id" belongs_to:"id"`
	AvatarId   JsonNullInt64  `gql:"avatar_id" db:"avatar_id"`
	EmployeeId JsonNullInt64  `gql:"employee_id" db:"employee_id"`
	LastIp     JsonNullString `gql:"last_ip" db:"last_ip"`
	LastLogin  *time.Time     `gql:"last_login" db:"last_login"`
	LastAction *time.Time     `gql:"last_action" db:"last_action"`
	CreatedAt  *time.Time     `gql:"created_at" db:"created_at"`
	UpdatedAt  *time.Time     `gql:"updated_at" db:"updated_at"`
}

func (u *User) Pk() interface{} {
	return u.Id
}

func (u *User) PkField() *Field {
	return &Field{Name: "id"}
}

func (u *User) Table() string {
	return "users"
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
