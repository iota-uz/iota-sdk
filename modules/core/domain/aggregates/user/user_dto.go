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

func (u *CreateDTO) ToEntity() (User, error) {
	r, err := role.NewWithID(u.RoleID, "", "", nil, time.Now(), time.Now())
	if err != nil {
		return nil, err
	}
	return &user{
		firstName:  u.FirstName,
		lastName:   u.LastName,
		email:      u.Email,
		roles:      []role.Role{r},
		password:   u.Password,
		lastLogin:  time.Now(),
		lastAction: time.Now(),
		lastIP:     "",
		avatarID:   u.AvatarID,
		uiLanguage: UILanguage(u.UILanguage),
		employeeID: 0,
		createdAt:  time.Now(),
		updatedAt:  time.Now(),
	}, nil
}

func (u *UpdateDTO) ToEntity(id uint) (User, error) {
	r, err := role.NewWithID(u.RoleID, "", "", nil, time.Now(), time.Now())
	if err != nil {
		return nil, err
	}
	return &user{
		id:         id,
		firstName:  u.FirstName,
		lastName:   u.LastName,
		email:      u.Email,
		roles:      []role.Role{r},
		password:   u.Password,
		lastLogin:  time.Now(),
		lastAction: time.Now(),
		lastIP:     "",
		avatarID:   u.AvatarID,
		uiLanguage: UILanguage(u.UILanguage),
		employeeID: 0,
		createdAt:  time.Now(),
		updatedAt:  time.Now(),
	}, nil
}
