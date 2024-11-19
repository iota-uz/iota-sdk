package employee

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"time"
)

type CreateDTO struct {
	FirstName   string
	LastName    string
	MiddleName  string
	Email       string
	Phone       string
	Salary      float64
	Coefficient float64
}

type UpdateDTO struct {
	FirstName   string
	LastName    string
	MiddleName  string
	Email       string
	Phone       string
	Salary      float64
	Coefficient float64
}

func (d *CreateDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(d)
	if errs == nil {
		return errorMessages, true
	}

	for _, err := range errs.(validator.ValidationErrors) {
		errorMessages[err.Field()] = err.Translate(l)
	}
	return errorMessages, len(errorMessages) == 0
}

func (d *UpdateDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := constants.Validate.Struct(d)
	if errs == nil {
		return errors, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (d *CreateDTO) ToEntity() *Employee {
	return &Employee{
		ID:          0,
		FirstName:   d.FirstName,
		LastName:    d.LastName,
		MiddleName:  d.MiddleName,
		Email:       d.Email,
		Phone:       d.Phone,
		Salary:      d.Salary,
		Coefficient: d.Coefficient,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func (d *UpdateDTO) ToEntity(id uint) *Employee {
	return &Employee{
		ID:          id,
		FirstName:   d.FirstName,
		LastName:    d.LastName,
		MiddleName:  d.MiddleName,
		Email:       d.Email,
		Phone:       d.Phone,
		Salary:      d.Salary,
		Coefficient: d.Coefficient,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}
