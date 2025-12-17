package currency

import (
	"time"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

// Option is a functional option for configuring Currency
type Option func(*currency)

// --- Option setters ---

func WithCreatedAt(createdAt time.Time) Option {
	return func(c *currency) {
		c.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(c *currency) {
		c.updatedAt = updatedAt
	}
}

// --- Interface ---

// Currency represents a currency entity
type Currency interface {
	Code() Code
	Name() string
	Symbol() Symbol
	CreatedAt() time.Time
	UpdatedAt() time.Time

	SetName(name string) Currency
	SetCode(code Code) Currency
	SetSymbol(symbol Symbol) Currency
}

// --- Implementation ---

// New creates a new Currency with required fields
func New(code Code, name string, symbol Symbol, opts ...Option) Currency {
	c := &currency{
		code:      code,
		name:      name,
		symbol:    symbol,
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type currency struct {
	code      Code
	name      string
	symbol    Symbol
	createdAt time.Time
	updatedAt time.Time
}

func (c *currency) Code() Code {
	return c.code
}

func (c *currency) Name() string {
	return c.name
}

func (c *currency) Symbol() Symbol {
	return c.symbol
}

func (c *currency) CreatedAt() time.Time {
	return c.createdAt
}

func (c *currency) UpdatedAt() time.Time {
	return c.updatedAt
}

func (c *currency) SetName(name string) Currency {
	result := *c
	result.name = name
	result.updatedAt = time.Now()
	return &result
}

func (c *currency) SetCode(code Code) Currency {
	result := *c
	result.code = code
	result.updatedAt = time.Now()
	return &result
}

func (c *currency) SetSymbol(symbol Symbol) Currency {
	result := *c
	result.symbol = symbol
	result.updatedAt = time.Now()
	return &result
}

type CreateDTO struct {
	Code   string `validate:"required"`
	Name   string `validate:"required"`
	Symbol string `validate:"required"`
}

type UpdateDTO struct {
	Code   string `validate:"len=3"`
	Name   string
	Symbol string
}

func (p *CreateDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := constants.Validate.Struct(p)
	if errs == nil {
		return errors, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (p *CreateDTO) ToEntity() (Currency, error) {
	c, err := NewCode(p.Code)
	if err != nil {
		return nil, err
	}
	s, err := NewSymbol(p.Symbol)
	if err != nil {
		return nil, err
	}
	return New(c, p.Name, s), nil
}

func (p *UpdateDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := constants.Validate.Struct(p)
	if errs == nil {
		return errors, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (p *UpdateDTO) ToEntity() (Currency, error) {
	c, err := NewCode(p.Code)
	if err != nil {
		return nil, err
	}
	s, err := NewSymbol(p.Symbol)
	if err != nil {
		return nil, err
	}
	return New(c, p.Name, s), nil
}
