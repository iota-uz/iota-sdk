package aichatconfig

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidID          = errors.New("invalid ID")
	ErrEmptySystemPrompt  = errors.New("empty system prompt")
	ErrInvalidTemperature = errors.New("temperature must be between 0.0 and 2.0")
	ErrEmptyModelName     = errors.New("empty model name")
	ErrConfigNotFound     = errors.New("AI chat configuration not found")
)

type AIModelType string

const (
	AIModelTypeOpenAI AIModelType = "openai"
)

type AIConfig interface {
	ID() uuid.UUID
	ModelName() string
	ModelType() AIModelType
	SystemPrompt() string
	Temperature() float32
	MaxTokens() int
	CreatedAt() time.Time
	UpdatedAt() time.Time
	WithSystemPrompt(prompt string) (AIConfig, error)
	WithTemperature(temp float32) (AIConfig, error)
	WithMaxTokens(tokens int) (AIConfig, error)
	WithModelName(modelName string) (AIConfig, error)
}

type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (AIConfig, error)
	GetDefault(ctx context.Context) (AIConfig, error)
	Save(ctx context.Context, config AIConfig) (AIConfig, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) ([]AIConfig, error)
	SetDefault(ctx context.Context, id uuid.UUID) error
}

type aiConfig struct {
	id           uuid.UUID
	modelName    string
	modelType    AIModelType
	systemPrompt string
	temperature  float32
	maxTokens    int
	createdAt    time.Time
	updatedAt    time.Time
}

func New(
	modelName string,
	modelType AIModelType,
	systemPrompt string,
	opts ...Option,
) (AIConfig, error) {
	if modelName == "" {
		return nil, ErrEmptyModelName
	}

	if systemPrompt == "" {
		return nil, ErrEmptySystemPrompt
	}

	cfg := &aiConfig{
		id:           uuid.New(),
		modelName:    modelName,
		modelType:    modelType,
		systemPrompt: systemPrompt,
		temperature:  0.7,  // Default temperature
		maxTokens:    1024, // Default max tokens
		createdAt:    time.Now(),
		updatedAt:    time.Now(),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg, nil
}

type Option func(*aiConfig)

func WithID(id uuid.UUID) Option {
	return func(c *aiConfig) {
		if id != uuid.Nil {
			c.id = id
		}
	}
}

func WithTemperature(temp float32) Option {
	return func(c *aiConfig) {
		if temp >= 0.0 && temp <= 2.0 {
			c.temperature = temp
		}
	}
}

func WithMaxTokens(tokens int) Option {
	return func(c *aiConfig) {
		if tokens > 0 {
			c.maxTokens = tokens
		}
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(c *aiConfig) {
		if !createdAt.IsZero() {
			c.createdAt = createdAt
		}
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(c *aiConfig) {
		if !updatedAt.IsZero() {
			c.updatedAt = updatedAt
		}
	}
}

func (c *aiConfig) ID() uuid.UUID {
	return c.id
}

func (c *aiConfig) ModelName() string {
	return c.modelName
}

func (c *aiConfig) ModelType() AIModelType {
	return c.modelType
}

func (c *aiConfig) SystemPrompt() string {
	return c.systemPrompt
}

func (c *aiConfig) Temperature() float32 {
	return c.temperature
}

func (c *aiConfig) MaxTokens() int {
	return c.maxTokens
}

func (c *aiConfig) CreatedAt() time.Time {
	return c.createdAt
}

func (c *aiConfig) UpdatedAt() time.Time {
	return c.updatedAt
}

func (c *aiConfig) WithSystemPrompt(prompt string) (AIConfig, error) {
	if prompt == "" {
		return nil, ErrEmptySystemPrompt
	}

	newConfig := *c
	newConfig.systemPrompt = prompt
	newConfig.updatedAt = time.Now()

	return &newConfig, nil
}

func (c *aiConfig) WithTemperature(temp float32) (AIConfig, error) {
	if temp < 0.0 || temp > 2.0 {
		return nil, ErrInvalidTemperature
	}

	newConfig := *c
	newConfig.temperature = temp
	newConfig.updatedAt = time.Now()

	return &newConfig, nil
}

func (c *aiConfig) WithMaxTokens(tokens int) (AIConfig, error) {
	if tokens <= 0 {
		return nil, errors.New("max tokens must be positive")
	}

	newConfig := *c
	newConfig.maxTokens = tokens
	newConfig.updatedAt = time.Now()

	return &newConfig, nil
}

func (c *aiConfig) WithModelName(modelName string) (AIConfig, error) {
	if modelName == "" {
		return nil, ErrEmptyModelName
	}

	newConfig := *c
	newConfig.modelName = modelName
	newConfig.updatedAt = time.Now()

	return &newConfig, nil
}
