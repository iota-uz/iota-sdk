package user

import (
	"time"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

type CreateDTO struct {
	FirstName  string `validate:"required"`
	LastName   string `validate:"required"`
	Email      string `validate:"required,email"`
	Password   string
	RoleID     uint `validate:"required"`
	AvatarID   uint
	UILanguage string
}

type UpdateDTO struct {
	FirstName  string `validate:"required"`
	LastName   string `validate:"required"`
	Email      string `validate:"required,email"`
	Password   string
	RoleID     uint
	AvatarID   uint
	UILanguage string
}

func (u *CreateDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := constants.Validate.Struct(u)
	if errs == nil {
		return errors, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (u *UpdateDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := constants.Validate.Struct(u)
	if errs == nil {
		return errors, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (u *CreateDTO) ToEntity() *User {
	return &User{
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		Email:      u.Email,
		Roles:      []*role.Role{{ID: u.RoleID}},
		Password:   u.Password,
		LastLogin:  nil,
		LastAction: nil,
		LastIP:     nil,
		AvatarID:   &u.AvatarID,
		UILanguage: UILanguage(u.UILanguage),
		EmployeeID: nil,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func (u *UpdateDTO) ToEntity(id uint) *User {
	return &User{
		ID:         id,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		Email:      u.Email,
		Roles:      []*role.Role{{ID: u.RoleID}},
		Password:   u.Password,
		LastLogin:  nil,
		LastAction: nil,
		LastIP:     nil,
		AvatarID:   &u.AvatarID,
		UILanguage: UILanguage(u.UILanguage),
		EmployeeID: nil,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}
