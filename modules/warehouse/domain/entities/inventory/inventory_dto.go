package inventory

import (
	"time"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-agency/iota-sdk/pkg/constants"
)

type CreateCheckDTO struct {
	Type      string
	Name      string
	Positions []uint
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

func (d *CreateCheckDTO) ToEntity(createdBy uint) (*Check, error) {
	s, err := NewStatus(string(Incomplete))
	if err != nil {
		return nil, err
	}
	t, err := NewType(d.Type)
	if err != nil {
		return nil, err
	}
	var results []*CheckResult
	for _, id := range d.Positions {
		results = append(results, &CheckResult{
			PositionID: id,
			CreatedAt:  time.Now(),
		})
	}
	return &Check{
		ID:        0,
		Status:    s,
		Type:      t,
		Name:      d.Name,
		Results:   results,
		CreatedAt: time.Now(),
		CreatedBy: createdBy,
	}, nil
}

func (d *UpdateCheckDTO) ToEntity(id uint, finishedBy uint) (*Check, error) {
	s, err := NewStatus(string(Incomplete))
	if err != nil {
		return nil, err
	}
	return &Check{
		ID:         id,
		Status:     s,
		Name:       d.Name,
		FinishedBy: finishedBy,
		FinishedAt: d.FinishedAt,
	}, nil
}
