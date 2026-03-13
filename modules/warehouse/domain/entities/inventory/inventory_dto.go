// Package inventory provides this package.
package inventory

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

type PositionCheckDTO struct {
	PositionID uint
	Found      uint
}

type CreateCheckDTO struct {
	Name      string `validate:"required"`
	Positions []*PositionCheckDTO
}

type UpdateCheckDTO struct {
	FinishedAt time.Time
	Name       string
}

func (d *CreateCheckDTO) Ok(l ut.Translator) (map[string]string, bool) {
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

func (d *UpdateCheckDTO) Ok(l ut.Translator) (map[string]string, bool) {
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

func (d *CreateCheckDTO) ToEntity(createdBy user.User) (*Check, error) {
	s, err := NewStatus(string(Incomplete))
	if err != nil {
		return nil, err
	}
	results := make([]*CheckResult, len(d.Positions))
	for i, p := range d.Positions {
		results[i] = &CheckResult{
			ID:               0,
			TenantID:         uuid.Nil,
			PositionID:       p.PositionID,
			Position:         nil,
			ExpectedQuantity: 0,
			ActualQuantity:   int(p.Found),
			Difference:       0,
			CreatedAt:        time.Time{},
		}
	}
	return &Check{
		ID:           0,
		TenantID:     uuid.Nil,
		Status:       s,
		Name:         d.Name,
		Results:      results,
		CreatedAt:    time.Now(),
		FinishedAt:   time.Time{},
		CreatedByID:  createdBy.ID(),
		CreatedBy:    createdBy,
		FinishedBy:   nil,
		FinishedByID: 0,
	}, nil
}

func (d *UpdateCheckDTO) ToEntity(id uint) (*Check, error) {
	check := &Check{
		ID:           id,
		TenantID:     uuid.Nil,
		Status:       "",
		Name:         d.Name,
		Results:      nil,
		CreatedAt:    time.Time{},
		FinishedAt:   d.FinishedAt,
		CreatedByID:  0,
		CreatedBy:    nil,
		FinishedBy:   nil,
		FinishedByID: 0,
	}
	return check, nil
}
